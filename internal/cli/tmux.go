package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/guysoft/guyide-cli/internal/output"
	"github.com/guysoft/guyide-cli/internal/tmux"
	"github.com/guysoft/guyide-cli/pkg/schema"
	"github.com/spf13/cobra"
)

func newTmuxCmd(g *Globals) *cobra.Command {
	c := &cobra.Command{
		Use:   "tmux",
		Short: "Tmux pane operations",
	}
	c.AddCommand(newTmuxPanesCmd(g))
	c.AddCommand(newTmuxSendCmd(g))
	c.AddCommand(newTmuxWatchCmd(g))
	return c
}

func newTmuxPanesCmd(g *Globals) *cobra.Command {
	return &cobra.Command{
		Use:   "panes",
		Short: "List panes in the current (or named) session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := writerFor(g)
			if !tmux.Available() {
				out.Error("tmux not found on PATH")
				return fmt.Errorf("tmux not available")
			}
			panes, err := tmux.ListPanes(g.Session)
			if err != nil {
				out.Error("list-panes failed: %v", err)
				return err
			}
			report := schema.PanesReport{
				Envelope: schema.Envelope{Schema: schema.SchemaVersion, Level: "info"},
				Panes:    make([]schema.PaneInfo, 0, len(panes)),
			}
			for _, p := range panes {
				report.Panes = append(report.Panes, schema.PaneInfo{
					ID: p.ID, Index: p.Index, Title: p.Title, Command: p.Command,
					Active: p.Active, Width: p.Width, Height: p.Height,
					Session: p.Session, Window: p.Window,
				})
			}
			if out.Mode() == output.ModeMachine {
				out.JSON(report)
				return nil
			}
			out.Header(fmt.Sprintf("tmux panes (%d)", len(panes)))
			for _, p := range panes {
				marker := "  "
				if p.Active {
					marker = "▶ "
				}
				out.KeyValue(
					fmt.Sprintf("%s%s [%d]", marker, p.ID, p.Index),
					fmt.Sprintf("%s — %s (%s/%s, %dx%d)",
						p.Command, p.Title, p.Session, p.Window, p.Width, p.Height),
				)
			}
			return nil
		},
	}
}

func newTmuxSendCmd(g *Globals) *cobra.Command {
	var literal bool
	c := &cobra.Command{
		Use:   "send <pane> <keys...>",
		Short: "Send keys to a pane (e.g. \"echo hi\" Enter)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := writerFor(g)
			target := args[0]
			keys := args[1:]
			if err := tmux.SendKeys(target, literal, keys...); err != nil {
				out.Error("send-keys failed: %v", err)
				return err
			}
			if out.Mode() == output.ModeMachine {
				out.JSON(map[string]any{
					"schema": schema.SchemaVersion,
					"level":  "success",
					"target": target,
					"keys":   keys,
					"sent":   true,
				})
				return nil
			}
			out.Success("sent %d key(s) to %s", len(keys), target)
			return nil
		},
	}
	c.Flags().BoolVarP(&literal, "literal", "l", false, "send keys literally (-l)")
	return c
}

func newTmuxWatchCmd(g *Globals) *cobra.Command {
	var paneID, until, timeoutStr string
	var quiet bool
	c := &cobra.Command{
		Use:   "watch",
		Short: "Watch a pane until a regex matches or timeout",
		Long: "Streams pane output via tmux pipe-pane and exits when --until\n" +
			"matches a line. Exits 0 on match, 124 on timeout, non-zero on error.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := writerFor(g)
			if until == "" {
				out.Error("--until <regex> is required")
				return fmt.Errorf("missing --until")
			}
			target := paneID
			if target == "" {
				active, err := tmux.ActivePaneID(g.Session)
				if err != nil {
					out.Error("cannot resolve active pane: %v", err)
					return err
				}
				target = active
			}
			to, err := time.ParseDuration(timeoutStr)
			if err != nil {
				out.Error("invalid --timeout: %v", err)
				return err
			}

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			machine := out.Mode() == output.ModeMachine
			start := time.Now()

			var onLine tmux.LineHandler
			if !quiet {
				onLine = func(line string) {
					if machine {
						b, _ := json.Marshal(schema.WatchEvent{
							Envelope: schema.Envelope{Schema: schema.SchemaVersion, Level: "info"},
							Line:     line,
						})
						out.Raw(string(b))
					} else if g.Verbose {
						out.Info("%s", line)
					}
				}
			}

			res, werr := tmux.WatchPane(ctx, target, until, to, onLine)
			if werr != nil && !res.Timeout {
				out.Error("watch failed: %v", werr)
				return werr
			}
			elapsed := time.Since(start).Round(time.Millisecond)

			if machine {
				out.JSON(schema.WatchEvent{
					Envelope: schema.Envelope{Schema: schema.SchemaVersion, Level: levelFor(res)},
					Line:     res.Line,
					Matched:  res.Matched,
					Timeout:  res.Timeout,
				})
			} else if res.Matched {
				out.Success("matched after %s", elapsed)
				out.KeyValue("line", res.Line)
			} else if res.Timeout {
				out.Warning("timeout after %s without a match", elapsed)
			} else {
				out.Warning("watch ended after %s without a match", elapsed)
			}

			if res.Timeout {
				// Conventional timeout exit code.
				cmd.SilenceErrors = true
				return &exitErr{code: 124, msg: "timeout"}
			}
			if !res.Matched {
				return &exitErr{code: 1, msg: "no match"}
			}
			return nil
		},
	}
	c.Flags().StringVar(&paneID, "pane", "", "pane id (default: active pane in session)")
	c.Flags().StringVar(&until, "until", "", "regex to match (required)")
	c.Flags().StringVar(&timeoutStr, "timeout", "30s", "timeout duration (e.g. 5s, 2m)")
	c.Flags().BoolVar(&quiet, "quiet", false, "suppress per-line streaming output")
	return c
}

func levelFor(r tmux.WatchResult) string {
	switch {
	case r.Matched:
		return "success"
	case r.Timeout:
		return "warning"
	default:
		return "info"
	}
}

// exitErr lets a command request a specific process exit code without
// printing cobra's usage. main.go inspects this.
type exitErr struct {
	code int
	msg  string
}

func (e *exitErr) Error() string { return e.msg }
func (e *exitErr) Code() int     { return e.code }

// keep strconv import used (for future).
var _ = strconv.Itoa
