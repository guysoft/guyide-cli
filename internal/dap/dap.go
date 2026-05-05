// Package dap drives nvim-dap over the nvim msgpack-rpc connection.
//
// It calls the helpers exposed by the nvim-launch.debug-rpc Lua module
// (shipped in vscodium.nvim). All Lua calls return a JSON-encoded string,
// which we decode into pkg/schema types.
//
// State() is the central observation point: it returns whatever the Lua
// listeners have last recorded about the dap session. WaitForStop is a
// thin polling wrapper used by `guyide debug state --wait`.
package dap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	gnvim "github.com/guysoft/guyide-cli/internal/nvim"
	"github.com/guysoft/guyide-cli/pkg/schema"
)

// Client is the high-level dap driver. Construct with New.
type Client struct {
	n *gnvim.Client
}

// New wraps a connected nvim client.
func New(n *gnvim.Client) *Client { return &Client{n: n} }

// luaJSON runs a chunk of Lua that must return a JSON string, and unmarshals
// the result into out. If out is nil, the result is discarded but the call
// still validates that the chunk returned a string.
func (c *Client) luaJSON(chunk string, args []any, out any) error {
	var raw any
	if err := c.n.Raw().ExecLua(chunk, &raw, args...); err != nil {
		return fmt.Errorf("nvim_exec_lua: %w", err)
	}
	s, ok := raw.(string)
	if !ok {
		return fmt.Errorf("expected string return from lua, got %T", raw)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal([]byte(s), out); err != nil {
		return fmt.Errorf("decode lua json: %w (raw=%s)", err, s)
	}
	return nil
}

// rpcResult is the {success, error?, ...} shape every helper returns.
type rpcResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Config  string `json:"config,omitempty"`
}

func checkResult(r rpcResult, op string) error {
	if r.Success {
		return nil
	}
	if r.Error != "" {
		return fmt.Errorf("%s: %s", op, r.Error)
	}
	return fmt.Errorf("%s: failed", op)
}

// Start starts a debug session by config name. If name is empty, the first
// available config from launch.json is used.
func (c *Client) Start(name string) (string, error) {
	const chunk = `return require("nvim-launch.debug-rpc").start_debug(...)`
	var r rpcResult
	args := []any{}
	if name != "" {
		args = append(args, name)
	} else {
		args = append(args, nil)
	}
	if err := c.luaJSON(chunk, args, &r); err != nil {
		return "", err
	}
	if err := checkResult(r, "debug start"); err != nil {
		return "", err
	}
	return r.Config, nil
}

// Stop terminates the current debug session.
func (c *Client) Stop() error {
	const chunk = `return require("nvim-launch.debug-rpc").stop()`
	var r rpcResult
	if err := c.luaJSON(chunk, nil, &r); err != nil {
		return err
	}
	return checkResult(r, "debug stop")
}

// Continue resumes from a stop point.
func (c *Client) Continue() error {
	const chunk = `return require("nvim-launch.debug-rpc").continue()`
	var r rpcResult
	if err := c.luaJSON(chunk, nil, &r); err != nil {
		return err
	}
	return checkResult(r, "debug continue")
}

// StepKind selects the step variant.
type StepKind string

const (
	StepOver StepKind = "over"
	StepInto StepKind = "into"
	StepOut  StepKind = "out"
)

// Step performs a step in the chosen direction.
func (c *Client) Step(kind StepKind) error {
	var fn string
	switch kind {
	case StepInto:
		fn = "step_into"
	case StepOut:
		fn = "step_out"
	case StepOver, "":
		fn = "step_over"
	default:
		return fmt.Errorf("unknown step kind %q", kind)
	}
	chunk := fmt.Sprintf(`return require("nvim-launch.debug-rpc").%s()`, fn)
	var r rpcResult
	if err := c.luaJSON(chunk, nil, &r); err != nil {
		return err
	}
	return checkResult(r, "debug step "+string(kind))
}

// SetBreakpoint installs a breakpoint at file:line. The Lua side currently
// uses the cursor position internally, so we navigate first by calling
// vim.fn.bufadd + nvim_win_set_cursor inside the helper.
func (c *Client) SetBreakpoint(file string, line int, condition string) error {
	const chunk = `return require("nvim-launch.debug-rpc").set_breakpoint(...)`
	args := []any{file, line}
	if condition != "" {
		args = append(args, condition)
	} else {
		args = append(args, nil)
	}
	var r rpcResult
	if err := c.luaJSON(chunk, args, &r); err != nil {
		return err
	}
	return checkResult(r, "debug break set")
}

// ClearBreakpoints removes breakpoints from a file (or all files when "").
func (c *Client) ClearBreakpoints(file string) error {
	const chunk = `return require("nvim-launch.debug-rpc").clear_breakpoints(...)`
	args := []any{}
	if file != "" {
		args = append(args, file)
	} else {
		args = append(args, nil)
	}
	var r rpcResult
	if err := c.luaJSON(chunk, args, &r); err != nil {
		return err
	}
	return checkResult(r, "debug break clear")
}

// ConfigSummary is one row from list_configs.
type ConfigSummary struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Request string `json:"request"`
}

// ListConfigs returns launch.json configs visible to nvim-launch.
func (c *Client) ListConfigs() ([]ConfigSummary, error) {
	const chunk = `return require("nvim-launch.debug-rpc").list_configs()`
	var out []ConfigSummary
	if err := c.luaJSON(chunk, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// rawState mirrors the Lua module's _state shape.
type rawState struct {
	SessionActive bool   `json:"session_active"`
	Stopped       bool   `json:"stopped"`
	File          string `json:"file"`
	Line          int    `json:"line"`
	Reason        string `json:"reason"`
	ThreadID      int    `json:"thread_id"`
	Frames        []struct {
		Name string `json:"name"`
		File string `json:"file"`
		Line int    `json:"line"`
	} `json:"frames"`
}

// State returns the current debugger state. WithVars=true additionally asks
// the active dap session for top-frame variables.
func (c *Client) State(withVars bool) (schema.DebugState, error) {
	const chunk = `return require("nvim-launch.debug-rpc").get_state()`
	var raw rawState
	if err := c.luaJSON(chunk, nil, &raw); err != nil {
		return schema.DebugState{}, err
	}
	st := schema.DebugState{
		Envelope:      schema.Envelope{Schema: schema.SchemaVersion, Level: "info"},
		SessionActive: raw.SessionActive,
		Stopped:       raw.Stopped,
		Reason:        raw.Reason,
		File:          raw.File,
		Line:          raw.Line,
	}
	for i, f := range raw.Frames {
		st.Frames = append(st.Frames, schema.Frame{
			ID: i, Name: f.Name, File: f.File, Line: f.Line,
		})
	}
	if withVars && raw.Stopped {
		vars, err := c.topFrameVariables(raw.ThreadID)
		if err == nil {
			st.Variables = vars
		}
		// non-fatal: variables are best-effort
	}
	return st, nil
}

// topFrameVariables synchronously asks the active dap session for variables
// in the top stack frame's first scope. Implemented inline because the Lua
// module does not yet expose a get_locals helper.
//
// The chunk uses dap.session():request synchronously by stashing the result
// in a global and busy-waiting up to 1.5s. This is intentionally simple for
// Phase 1; we'll move to an async ndjson stream in Phase 2.
func (c *Client) topFrameVariables(threadID int) ([]schema.Variable, error) {
	const chunk = `
local dap_ok, dap = pcall(require, "dap")
if not dap_ok then return vim.fn.json_encode({}) end
local s = dap.session()
if not s then return vim.fn.json_encode({}) end
local tid = ...
if tid == 0 then tid = 1 end

local done = false
local result = {}

local function get_vars(varsRef)
  s:request("variables", { variablesReference = varsRef }, function(err, resp)
    if not err and resp and resp.variables then
      for _, v in ipairs(resp.variables) do
        table.insert(result, { name = v.name, type = v.type or "", value = v.value or "" })
      end
    end
    done = true
  end)
end

s:request("stackTrace", { threadId = tid, startFrame = 0, levels = 1 }, function(err, resp)
  if err or not resp or not resp.stackFrames or #resp.stackFrames == 0 then
    done = true
    return
  end
  local frame = resp.stackFrames[1]
  s:request("scopes", { frameId = frame.id }, function(err2, resp2)
    if err2 or not resp2 or not resp2.scopes or #resp2.scopes == 0 then
      done = true
      return
    end
    -- pick the first non-expensive scope
    local target = resp2.scopes[1]
    for _, sc in ipairs(resp2.scopes) do
      if not sc.expensive then target = sc; break end
    end
    get_vars(target.variablesReference)
  end)
end)

local deadline = vim.loop.now() + 1500
while not done and vim.loop.now() < deadline do
  vim.wait(50, function() return done end)
end
return vim.fn.json_encode(result)
`
	var out []schema.Variable
	if err := c.luaJSON(chunk, []any{threadID}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// WaitForStop polls State() until Stopped=true (and optional reason filter)
// or the context expires. interval defaults to 100ms.
func (c *Client) WaitForStop(ctx context.Context, reason string, interval time.Duration) (schema.DebugState, error) {
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	for {
		st, err := c.State(false)
		if err != nil {
			return st, err
		}
		if st.Stopped && (reason == "" || st.Reason == reason) {
			return st, nil
		}
		select {
		case <-ctx.Done():
			return st, errors.Join(ctx.Err(), ErrWaitTimeout)
		case <-time.After(interval):
		}
	}
}

// ErrWaitTimeout is returned alongside ctx.Err() when WaitForStop deadlines.
var ErrWaitTimeout = errors.New("wait-for-stop deadline exceeded")
