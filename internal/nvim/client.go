// Package nvim wraps github.com/neovim/go-client/nvim with a guyide-shaped
// API: a single Client constructor that resolves the socket via discover,
// plus high-level helpers used by the cli package.
//
// Direct msgpack-rpc is preferred over `nvim --remote-expr` to avoid
// shell-escaping issues and to surface errors as Go values.
package nvim

import (
	"errors"
	"fmt"

	"github.com/neovim/go-client/nvim"
)

// Client is a thin guyide wrapper over *nvim.Nvim. It exists so we can
// add tracing, retries, and timeouts without leaking the upstream API
// across guyide internals.
type Client struct {
	v *nvim.Nvim
}

// Dial connects to the given unix socket path. The caller is responsible
// for closing the client.
func Dial(socket string) (*Client, error) {
	if socket == "" {
		return nil, errors.New("nvim: empty socket path")
	}
	v, err := nvim.Dial(socket)
	if err != nil {
		return nil, fmt.Errorf("nvim: dial %s: %w", socket, err)
	}
	return &Client{v: v}, nil
}

// Close shuts down the underlying connection.
func (c *Client) Close() error {
	if c == nil || c.v == nil {
		return nil
	}
	return c.v.Close()
}

// APIInfo returns nvim's [channel-id, api-info] tuple as produced by
// nvim_get_api_info. We surface only the channel id and the version map
// for now; callers that need more should use Raw().
func (c *Client) APIInfo() (channelID int, apiVersion map[string]any, err error) {
	raw, err := c.v.APIInfo()
	if err != nil {
		return 0, nil, fmt.Errorf("nvim_get_api_info: %w", err)
	}
	if len(raw) < 2 {
		return 0, nil, fmt.Errorf("nvim_get_api_info: unexpected shape, got %d elements", len(raw))
	}
	switch id := raw[0].(type) {
	case int64:
		channelID = int(id)
	case uint64:
		channelID = int(id)
	case int:
		channelID = id
	}
	if info, ok := raw[1].(map[string]any); ok {
		if ver, ok := info["version"].(map[string]any); ok {
			apiVersion = ver
		}
	}
	return channelID, apiVersion, nil
}

// Command executes a vimscript ex-command (no return value).
func (c *Client) Command(cmd string) error {
	if err := c.v.Command(cmd); err != nil {
		return fmt.Errorf("nvim_command(%q): %w", cmd, err)
	}
	return nil
}

// Eval evaluates a vimscript expression and returns the result as an any.
// Callers can type-assert; for simple cases use EvalString / EvalInt.
func (c *Client) Eval(expr string) (any, error) {
	var out any
	if err := c.v.Eval(expr, &out); err != nil {
		return nil, fmt.Errorf("nvim_eval(%q): %w", expr, err)
	}
	return out, nil
}

// Raw exposes the underlying *nvim.Nvim for advanced callers (the dap
// package uses this for nvim_exec_lua of the debug-rpc helpers).
func (c *Client) Raw() *nvim.Nvim { return c.v }
