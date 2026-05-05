package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/guysoft/guyide-cli/internal/dap"
	"github.com/guysoft/guyide-cli/internal/discover"
	gnvim "github.com/guysoft/guyide-cli/internal/nvim"
	"github.com/guysoft/guyide-cli/internal/output"
	"github.com/guysoft/guyide-cli/pkg/schema"
	"github.com/spf13/cobra"
)

// resolveDap returns a connected dap.Client and the underlying nvim Client
// (so the caller can defer Close). It honors --socket / --session globals.
func resolveDap(g *Globals) (*dap.Client, *gnvim.Client, schema.EnvInfo, error) {
	env := discover.Resolve(discover.Options{
		ExplicitSocket:  g.Socket,
		ExplicitSession: g.Session,
	})
	if env.Socket == "" {
		return nil, nil, env, errors.New("no nvim socket found (try guyide env)")
	}
	n, err := gnvim.Dial(env.Socket)
	if err != nil {
		return nil, nil, env, err
	}
	return dap.New(n), n, env, nil
}

func newDebugCmd(g *Globals) *cobra.Command {
	c := &cobra.Command{
		Use:   "debug",
		Short: "Drive nvim-dap debug sessions",
	}
	c.AddCommand(newDebugStartCmd(g))
	c.AddCommand(newDebugStopCmd(g))
	c.AddCommand(newDebugStateCmd(g))
	c.AddCommand(newDebugStepCmd(g))
	c.AddCommand(newDebugContinueCmd(g))
	c.AddCommand(newDebugBreakCmd(g))
	c.AddCommand(newDebugListConfigsCmd(g))
	return c
}

// ---- start --------------------------------------------------------------

func newDebugStartCmd(g *Globals) *cobra.Command {
	var configName string
	c := &cobra.Command{
		Use:   "start",
		Short: "Start a debug session by config name (default: first available)",
		RunE: func(_ *cobra.Command, _ []string) error {
			out := writerFor(g)
			d, n, _, err := resolveDap(g)
			if err != nil {
				out.Error("%v", err)
				return err
			}
			defer n.Close()
			used, err := d.Start(configName)
			if err != nil {
				out.Error("%v", err)
				return err
			}
			emitSimple(out, "success", map[string]any{
				"started": true,
				"config":  used,
			}, fmt.Sprintf("started debug session: %s", used))
			return nil
		},
	}
	c.Flags().StringVar(&configName, "config", "", "launch.json config name")
	return c
}

// ---- stop ---------------------------------------------------------------

func newDebugStopCmd(g *Globals) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the active debug session",
		RunE: func(_ *cobra.Command, _ []string) error {
			out := writerFor(g)
			d, n, _, err := resolveDap(g)
			if err != nil {
				out.Error("%v", err)
				return err
			}
			defer n.Close()
			if err := d.Stop(); err != nil {
				out.Error("%v", err)
				return err
			}
			emitSimple(out, "success", map[string]any{"stopped": true}, "debug session stopped")
			return nil
		},
	}
}

// ---- continue -----------------------------------------------------------

func newDebugContinueCmd(g *Globals) *cobra.Command {
	return &cobra.Command{
		Use:   "continue",
		Short: "Continue execution",
		RunE: func(_ *cobra.Command, _ []string) error {
			out := writerFor(g)
			d, n, _, err := resolveDap(g)
			if err != nil {
				out.Error("%v", err)
				return err
			}
			defer n.Close()
			if err := d.Continue(); err != nil {
				out.Error("%v", err)
				return err
			}
			emitSimple(out, "success", map[string]any{"continued": true}, "continued")
			return nil
		},
	}
}

// ---- step ---------------------------------------------------------------

func newDebugStepCmd(g *Globals) *cobra.Command {
	var into, outOf bool
	c := &cobra.Command{
		Use:   "step",
		Short: "Step over (default), --into, or --out",
		RunE: func(_ *cobra.Command, _ []string) error {
			out := writerFor(g)
			d, n, _, err := resolveDap(g)
			if err != nil {
				out.Error("%v", err)
				return err
			}
			defer n.Close()
			kind := dap.StepOver
			switch {
			case into && outOf:
				return errors.New("cannot combine --into and --out")
			case into:
				kind = dap.StepInto
			case outOf:
				kind = dap.StepOut
			}
			if err := d.Step(kind); err != nil {
				out.Error("%v", err)
				return err
			}
			emitSimple(out, "success", map[string]any{"stepped": string(kind)}, "step "+string(kind))
			return nil
		},
	}
	c.Flags().BoolVar(&into, "into", false, "step into")
	c.Flags().BoolVar(&outOf, "out", false, "step out")
	return c
}

// ---- state --------------------------------------------------------------

func newDebugStateCmd(g *Globals) *cobra.Command {
	var wait, withVars, withFrames bool
	var reason, timeoutStr string
	c := &cobra.Command{
		Use:   "state",
		Short: "Print current debug state (optionally wait for stop)",
		RunE: func(_ *cobra.Command, _ []string) error {
			out := writerFor(g)
			d, n, _, err := resolveDap(g)
			if err != nil {
				out.Error("%v", err)
				return err
			}
			defer n.Close()

			var st schema.DebugState
			if wait {
				to, perr := time.ParseDuration(timeoutStr)
				if perr != nil {
					out.Error("invalid --timeout: %v", perr)
					return perr
				}
				ctx, cancel := context.WithTimeout(context.Background(), to)
				defer cancel()
				st, err = d.WaitForStop(ctx, reason, 100*time.Millisecond)
				if err != nil {
					if errors.Is(err, dap.ErrWaitTimeout) || errors.Is(err, context.DeadlineExceeded) {
						st.Envelope.Level = "warning"
						emitState(out, st, fmt.Sprintf("timeout waiting for stop after %s", to))
						return &exitErr{code: 124, msg: "wait timeout"}
					}
					out.Error("%v", err)
					return err
				}
			} else {
				st, err = d.State(withVars)
				if err != nil {
					out.Error("%v", err)
					return err
				}
			}

			// If --vars was requested with --wait, fetch on top of the wait result.
			if wait && withVars && st.Stopped {
				st2, verr := d.State(true)
				if verr == nil {
					st.Variables = st2.Variables
				}
			}
			if !withFrames {
				st.Frames = nil
			}

			emitState(out, st, "")
			return nil
		},
	}
	c.Flags().BoolVar(&wait, "wait", false, "block until session is stopped")
	c.Flags().StringVar(&reason, "reason", "", "filter wait by stop reason (e.g. breakpoint)")
	c.Flags().StringVar(&timeoutStr, "timeout", "30s", "timeout for --wait")
	c.Flags().BoolVar(&withVars, "vars", false, "include top-frame variables")
	c.Flags().BoolVar(&withFrames, "frames", false, "include stack frames")
	return c
}

func emitState(out output.Writer, st schema.DebugState, suffix string) {
	if out.Mode() == output.ModeMachine {
		out.JSON(st)
		return
	}
	switch {
	case st.Stopped:
		out.Success("stopped (%s) at %s:%d", st.Reason, shortPath(st.File), st.Line)
	case st.SessionActive:
		out.Info("session active, running")
	default:
		out.Warning("no active debug session")
	}
	if suffix != "" {
		out.Warning("%s", suffix)
	}
	if len(st.Frames) > 0 {
		out.Header("frames")
		for _, f := range st.Frames {
			out.KeyValue(fmt.Sprintf("#%d %s", f.ID, f.Name),
				fmt.Sprintf("%s:%d", shortPath(f.File), f.Line))
		}
	}
	if len(st.Variables) > 0 {
		out.Header("variables")
		for _, v := range st.Variables {
			label := v.Name
			if v.Type != "" {
				label += " (" + v.Type + ")"
			}
			out.KeyValue(label, v.Value)
		}
	}
}

// ---- break --------------------------------------------------------------

func newDebugBreakCmd(g *Globals) *cobra.Command {
	c := &cobra.Command{Use: "break", Short: "Manage breakpoints"}
	c.AddCommand(newDebugBreakSetCmd(g))
	c.AddCommand(newDebugBreakClearCmd(g))
	return c
}

func newDebugBreakSetCmd(g *Globals) *cobra.Command {
	var file, condition string
	var line int
	c := &cobra.Command{
		Use:   "set",
		Short: "Set a breakpoint at --file:--line",
		RunE: func(_ *cobra.Command, _ []string) error {
			out := writerFor(g)
			if file == "" || line <= 0 {
				out.Error("--file and --line are required")
				return errors.New("missing --file/--line")
			}
			abs, _ := filepath.Abs(file)
			d, n, _, err := resolveDap(g)
			if err != nil {
				out.Error("%v", err)
				return err
			}
			defer n.Close()
			if err := d.SetBreakpoint(abs, line, condition); err != nil {
				out.Error("%v", err)
				return err
			}
			emitSimple(out, "success", map[string]any{
				"breakpoint": map[string]any{"file": abs, "line": line, "condition": condition},
			}, fmt.Sprintf("breakpoint set at %s:%d", shortPath(abs), line))
			return nil
		},
	}
	c.Flags().StringVar(&file, "file", "", "source file path")
	c.Flags().IntVar(&line, "line", 0, "1-based line number")
	c.Flags().StringVar(&condition, "condition", "", "optional condition expression")
	return c
}

func newDebugBreakClearCmd(g *Globals) *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:   "clear",
		Short: "Clear breakpoints in --file (or all files if omitted)",
		RunE: func(_ *cobra.Command, _ []string) error {
			out := writerFor(g)
			d, n, _, err := resolveDap(g)
			if err != nil {
				out.Error("%v", err)
				return err
			}
			defer n.Close()
			abs := ""
			if file != "" {
				abs, _ = filepath.Abs(file)
			}
			if err := d.ClearBreakpoints(abs); err != nil {
				out.Error("%v", err)
				return err
			}
			msg := "cleared breakpoints in all files"
			if abs != "" {
				msg = "cleared breakpoints in " + shortPath(abs)
			}
			emitSimple(out, "success", map[string]any{"cleared": true, "file": abs}, msg)
			return nil
		},
	}
	c.Flags().StringVar(&file, "file", "", "limit clear to this file")
	return c
}

// ---- list-configs -------------------------------------------------------

func newDebugListConfigsCmd(g *Globals) *cobra.Command {
	return &cobra.Command{
		Use:   "list-configs",
		Short: "List launch.json configurations visible to nvim-launch",
		RunE: func(_ *cobra.Command, _ []string) error {
			out := writerFor(g)
			d, n, _, err := resolveDap(g)
			if err != nil {
				out.Error("%v", err)
				return err
			}
			defer n.Close()
			cfgs, err := d.ListConfigs()
			if err != nil {
				out.Error("%v", err)
				return err
			}
			sort.Slice(cfgs, func(i, j int) bool { return cfgs[i].Name < cfgs[j].Name })
			if out.Mode() == output.ModeMachine {
				out.JSON(map[string]any{
					"schema":  schema.SchemaVersion,
					"level":   "info",
					"configs": cfgs,
				})
				return nil
			}
			out.Header(fmt.Sprintf("launch configs (%d)", len(cfgs)))
			for _, cf := range cfgs {
				out.KeyValue(cf.Name, fmt.Sprintf("%s/%s", cf.Type, cf.Request))
			}
			return nil
		},
	}
}

// ---- helpers ------------------------------------------------------------

func emitSimple(out output.Writer, level string, fields map[string]any, humanMsg string) {
	if out.Mode() == output.ModeMachine {
		m := map[string]any{"schema": schema.SchemaVersion, "level": level}
		for k, v := range fields {
			m[k] = v
		}
		out.JSON(m)
		return
	}
	switch level {
	case "success":
		out.Success("%s", humanMsg)
	case "warning":
		out.Warning("%s", humanMsg)
	case "error":
		out.Error("%s", humanMsg)
	default:
		out.Info("%s", humanMsg)
	}
}

// shortPath turns an absolute path into something nicer for human output.
// Uses cwd-relative when the file lives under cwd, otherwise base name.
func shortPath(p string) string {
	if p == "" {
		return ""
	}
	if rel, err := filepath.Rel(mustCwd(), p); err == nil && len(rel) < len(p) && rel[0] != '.' {
		return rel
	}
	return filepath.Base(p)
}

func mustCwd() string {
	wd, err := filepathAbs(".")
	if err != nil {
		return ""
	}
	return wd
}

// indirection so tests can override; trivial wrapper for now.
var filepathAbs = filepath.Abs
