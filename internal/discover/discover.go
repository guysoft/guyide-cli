// Package discover resolves the runtime environment guyide will operate in:
// the nvim msgpack socket, the tmux session/window, and any pane-role
// hints provided by tmux-ide.
//
// Resolution order for the nvim socket (first non-empty wins):
//  1. Explicit --socket flag
//  2. NVIM_IDE_SOCK env var (current shell)
//  3. tmux show-environment NVIM_IDE_SOCK (current session)
//  4. Socket scan in $XDG_RUNTIME_DIR matching nvim.*.0
//  5. (Phase 2) GUYIDE_PANE_ROLE-driven lookup
package discover

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/guysoft/guyide-cli/pkg/schema"
)

// Options controls Resolve behavior.
type Options struct {
	ExplicitSocket  string
	ExplicitSession string
	// Env overrides os.Getenv, used by tests.
	Env func(string) string
	// Runtime overrides $XDG_RUNTIME_DIR for tests.
	Runtime string
	// TmuxEnv overrides the tmux show-environment lookup; if nil the real
	// tmux binary is invoked when present.
	TmuxEnv func(varname string) string
}

// Resolve performs the discovery cascade and returns a populated EnvInfo.
//
// It never errors: missing components simply leave fields blank. Callers
// inspect Socket and NvimReachable to decide if they can proceed.
func Resolve(opts Options) schema.EnvInfo {
	getenv := opts.Env
	if getenv == nil {
		getenv = os.Getenv
	}
	info := schema.EnvInfo{
		Envelope:       schema.Envelope{Schema: schema.SchemaVersion, Level: "info"},
		NvimIDESock:    getenv("NVIM_IDE_SOCK"),
		GuyidePaneRole: getenv("GUYIDE_PANE_ROLE"),
		TmuxSession:    getenv("TMUX_SESSION"),
	}

	// Step 1: explicit
	if opts.ExplicitSocket != "" {
		info.Socket = opts.ExplicitSocket
		info.SocketSource = "flag"
	}

	// Step 2: env var
	if info.Socket == "" && info.NvimIDESock != "" {
		info.Socket = info.NvimIDESock
		info.SocketSource = "env"
	}

	// Step 3: tmux show-environment
	if info.Socket == "" {
		if v := tmuxShowEnv(opts.TmuxEnv, "NVIM_IDE_SOCK"); v != "" {
			info.Socket = v
			info.SocketSource = "tmux"
		}
	}

	// Step 4: scan
	candidates := scanSockets(opts.Runtime, getenv)
	info.Candidates = candidates
	if info.Socket == "" && len(candidates) > 0 {
		info.Socket = candidates[0]
		info.SocketSource = "scan"
	}

	// Tmux session/window
	if info.TmuxSession == "" {
		info.TmuxSession = tmuxDisplay("#S")
	}
	info.TmuxWindow = tmuxDisplay("#W")

	// Probe reachability of the chosen socket. If the chosen socket came
	// from flag/env/tmux but is stale (no longer accepting connections),
	// fall back to the first reachable candidate from the scan.
	if info.Socket != "" {
		info.NvimReachable = pingSocket(info.Socket, 500*time.Millisecond)
		if !info.NvimReachable && info.SocketSource != "scan" {
			for _, cand := range candidates {
				if pingSocket(cand, 500*time.Millisecond) {
					info.Socket = cand
					info.SocketSource = "scan"
					info.NvimReachable = true
					break
				}
			}
		}
	}

	return info
}

func scanSockets(runtime string, getenv func(string) string) []string {
	if runtime == "" {
		runtime = getenv("XDG_RUNTIME_DIR")
	}
	if runtime == "" {
		return nil
	}
	matches, err := filepath.Glob(filepath.Join(runtime, "nvim.*.0"))
	if err != nil {
		return nil
	}
	return matches
}

func tmuxShowEnv(override func(string) string, name string) string {
	if override != nil {
		return override(name)
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return ""
	}
	out, err := exec.Command("tmux", "show-environment", name).Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(out))
	// Format: NVIM_IDE_SOCK=/path/to/sock or "-NVIM_IDE_SOCK" if unset
	if strings.HasPrefix(line, "-") {
		return ""
	}
	if i := strings.Index(line, "="); i >= 0 {
		return line[i+1:]
	}
	return ""
}

func tmuxDisplay(fmt string) string {
	if _, err := exec.LookPath("tmux"); err != nil {
		return ""
	}
	out, err := exec.Command("tmux", "display-message", "-p", fmt).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// pingSocket attempts a brief connection to the unix socket. It does NOT
// speak msgpack; that's the nvim package's job. We only verify that the
// socket exists and accepts connections.
func pingSocket(path string, timeout time.Duration) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("unix", path)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
