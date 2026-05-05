// Package tmux provides a thin wrapper over the `tmux` CLI for guyide.
//
// We shell out to tmux rather than reimplement its control protocol; tmux is
// the source of truth for pane state, and the CLI is stable. All commands
// honor an optional explicit session name.
package tmux

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Pane describes one tmux pane.
type Pane struct {
	ID       string `json:"id"`        // e.g. "%12"
	Index    int    `json:"index"`     // pane index within window
	Title    string `json:"title"`     // pane_title
	Command  string `json:"command"`   // current command
	Active   bool   `json:"active"`    // is the active pane in its window
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Session  string `json:"session"`
	Window   string `json:"window"`
}

// Available reports whether the tmux binary is on PATH.
func Available() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// ServerRunning reports whether a tmux server is reachable.
func ServerRunning() bool {
	return exec.Command("tmux", "list-sessions").Run() == nil
}

// ListPanes returns panes in the current (or named) window.
//
// If session is empty, tmux uses the caller's current session.
func ListPanes(session string) ([]Pane, error) {
	args := []string{"list-panes"}
	if session != "" {
		args = append(args, "-t", session)
	}
	// One field per column. tmux's format DSL passes through escape sequences
	// like \037 as literal backslash-octal, so we use a multi-char sentinel
	// unlikely to appear in pane titles or commands.
	const sep = "<<|GUYIDE|>>"
	fmtStr := strings.Join([]string{
		"#{pane_id}",
		"#{pane_index}",
		"#{pane_title}",
		"#{pane_current_command}",
		"#{?pane_active,1,0}",
		"#{pane_width}",
		"#{pane_height}",
		"#{session_name}",
		"#{window_name}",
	}, sep)
	args = append(args, "-F", fmtStr)

	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes: %w", err)
	}

	var panes []Pane
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, sep)
		if len(f) < 9 {
			continue
		}
		idx, _ := strconv.Atoi(f[1])
		w, _ := strconv.Atoi(f[5])
		h, _ := strconv.Atoi(f[6])
		panes = append(panes, Pane{
			ID:      f[0],
			Index:   idx,
			Title:   f[2],
			Command: f[3],
			Active:  f[4] == "1",
			Width:   w,
			Height:  h,
			Session: f[7],
			Window:  f[8],
		})
	}
	return panes, nil
}

// SendKeys sends keys to the given pane. Each arg is one key spec
// (e.g. "echo hello", "Enter"). If literal is true, keys are sent as-is
// (-l flag) so backslash sequences are not interpreted.
func SendKeys(target string, literal bool, keys ...string) error {
	if target == "" {
		return fmt.Errorf("tmux send: target pane is required")
	}
	args := []string{"send-keys", "-t", target}
	if literal {
		args = append(args, "-l")
	}
	args = append(args, keys...)
	cmd := exec.Command("tmux", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CapturePane returns the visible buffer of a pane as a string.
// If history is true, includes scrollback (-p -S - -E -).
func CapturePane(target string, history bool) (string, error) {
	args := []string{"capture-pane", "-p"}
	if history {
		args = append(args, "-S", "-", "-E", "-")
	}
	if target != "" {
		args = append(args, "-t", target)
	}
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", fmt.Errorf("tmux capture-pane: %w", err)
	}
	return string(out), nil
}

// PipePane installs/removes a pipe on a pane. When cmd is non-empty, its
// stdout receives every byte the pane outputs (-o overwrites prior pipes).
// Passing an empty cmd toggles the pipe off for that pane.
func PipePane(target, cmd string) error {
	args := []string{"pipe-pane", "-o"}
	if target != "" {
		args = append(args, "-t", target)
	}
	if cmd != "" {
		args = append(args, cmd)
	}
	c := exec.Command("tmux", args...)
	c.Stderr = os.Stderr
	return c.Run()
}

// ActivePaneID returns the active pane id of the current (or named) session.
func ActivePaneID(session string) (string, error) {
	args := []string{"display-message", "-p"}
	if session != "" {
		args = append(args, "-t", session)
	}
	args = append(args, "#{pane_id}")
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", fmt.Errorf("tmux display-message: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ScanLines runs fn for every newline-terminated line read from r, until
// either the reader hits EOF, the context is canceled, or fn returns true.
// Returns true if fn matched a line, false on EOF/cancel.
func ScanLines(ctx context.Context, r *os.File, fn func(string) bool) (bool, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	lines := make(chan string, 64)
	errs := make(chan error, 1)
	go func() {
		defer close(lines)
		for sc.Scan() {
			lines <- sc.Text()
		}
		if err := sc.Err(); err != nil {
			errs <- err
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case line, ok := <-lines:
			if !ok {
				select {
				case err := <-errs:
					return false, err
				default:
					return false, nil
				}
			}
			if fn(line) {
				return true, nil
			}
		}
	}
}
