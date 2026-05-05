package tmux

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

// WatchResult describes the outcome of WatchPane.
type WatchResult struct {
	Matched bool   `json:"matched"`
	Timeout bool   `json:"timeout"`
	Line    string `json:"line,omitempty"`
}

// LineHandler is invoked for every line streamed from the pane (post-match
// continues are not delivered). Optional; pass nil to suppress.
type LineHandler func(line string)

// WatchPane streams pane output through `tmux pipe-pane` and returns when
// `until` matches a line, the context is canceled, or `timeout` elapses.
//
// Implementation:
//   - Pick a runtime dir under XDG_RUNTIME_DIR (or os.TempDir as fallback).
//   - Create a FIFO so we don't poll a regular file. Linux/macOS support this;
//     on platforms without mkfifo we fall back to a regular file + polling.
//   - tmux pipe-pane -o "cat > <fifo>" ; we open the fifo for read, scan lines.
//   - On exit, always pipe-pane -o "" to detach and remove the fifo.
//
// The active tmux server must be reachable; otherwise this returns an error.
func WatchPane(parent context.Context, target, until string, timeout time.Duration, onLine LineHandler) (WatchResult, error) {
	if target == "" {
		return WatchResult{}, errors.New("watch: target pane is required")
	}
	if until == "" {
		return WatchResult{}, errors.New("watch: --until regex is required")
	}
	re, err := regexp.Compile(until)
	if err != nil {
		return WatchResult{}, fmt.Errorf("watch: invalid regex: %w", err)
	}

	dir, err := watchDir()
	if err != nil {
		return WatchResult{}, err
	}
	fifoPath := filepath.Join(dir, fmt.Sprintf("watch-%d-%d.fifo", os.Getpid(), time.Now().UnixNano()))

	useFIFO := mkfifo(fifoPath, 0o600) == nil
	if !useFIFO {
		// Fallback: regular file (works on Windows or restricted filesystems).
		f, ferr := os.Create(fifoPath)
		if ferr != nil {
			return WatchResult{}, fmt.Errorf("watch: cannot create sink: %w", ferr)
		}
		_ = f.Close()
	}
	defer os.Remove(fifoPath)

	// Install pipe. Use shell so quoting is consistent across tmux versions.
	pipeCmd := fmt.Sprintf("cat >> %s", shellQuote(fifoPath))
	if err := PipePane(target, pipeCmd); err != nil {
		return WatchResult{}, fmt.Errorf("watch: pipe-pane install: %w", err)
	}
	defer func() { _ = PipePane(target, "") }()

	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, timeout)
		defer cancel()
	}

	// Open reader. For FIFOs this blocks until a writer connects; tmux opens
	// the file lazily on the first byte, so we open in O_RDWR to avoid the
	// blocking-open issue on Linux.
	flags := os.O_RDONLY
	if useFIFO {
		flags = os.O_RDWR
	}
	f, err := os.OpenFile(fifoPath, flags, 0)
	if err != nil {
		return WatchResult{}, fmt.Errorf("watch: open sink: %w", err)
	}
	defer f.Close()

	res, err := scanUntil(ctx, f, re, onLine, useFIFO)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		return res, err
	}
	if !res.Matched && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		res.Timeout = true
	}
	return res, nil
}

// scanUntil reads lines from r and returns when the regex matches or the
// context is done. If isFIFO is false, we poll for new data because regular
// files return EOF immediately on read past end-of-data.
func scanUntil(ctx context.Context, r io.Reader, re *regexp.Regexp, onLine LineHandler, isFIFO bool) (WatchResult, error) {
	out := WatchResult{}
	br := bufio.NewReaderSize(r, 64*1024)
	type lineMsg struct {
		s   string
		err error
	}
	ch := make(chan lineMsg, 64)

	go func() {
		defer close(ch)
		for {
			line, err := br.ReadString('\n')
			if line != "" {
				ch <- lineMsg{s: line}
			}
			if err != nil {
				if err == io.EOF {
					if isFIFO {
						// Pipe is open by us via O_RDWR; EOF means no data.
						// Brief sleep to avoid busy loop, then continue.
						time.Sleep(20 * time.Millisecond)
						continue
					}
					// Regular file: poll for growth.
					time.Sleep(50 * time.Millisecond)
					continue
				}
				ch <- lineMsg{err: err}
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return out, nil
			}
			if msg.err != nil {
				return out, msg.err
			}
			line := stripNL(msg.s)
			if onLine != nil {
				onLine(line)
			}
			if re.MatchString(line) {
				out.Matched = true
				out.Line = line
				return out, nil
			}
		}
	}
}

func stripNL(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func shellQuote(s string) string {
	// Single-quote and escape embedded single quotes for POSIX shells.
	return "'" + replaceAll(s, "'", `'\''`) + "'"
}

func replaceAll(s, old, new string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			out = append(out, new...)
			i += len(old)
			continue
		}
		out = append(out, s[i])
		i++
	}
	return string(out)
}

// watchDir picks a writable directory to host the FIFO. Prefers
// $XDG_RUNTIME_DIR/guyide, falls back to os.TempDir().
func watchDir() (string, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	d := filepath.Join(base, "guyide")
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", fmt.Errorf("watch: cannot create runtime dir: %w", err)
	}
	return d, nil
}

// helper used by status messages; unused here but kept for future telemetry.
var _ = strconv.Itoa

// silence unused import on some builds.
var _ = exec.Command
