package output

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type humanWriter struct {
	w     io.Writer
	t     theme
	mode  Mode
	emoji bool
}

func newHumanWriter(w io.Writer, t theme, mode Mode) *humanWriter {
	return &humanWriter{w: w, t: t, mode: mode, emoji: mode == ModeHumanStyled}
}

func (h *humanWriter) Mode() Mode { return h.mode }

func (h *humanWriter) emit(line string) {
	fmt.Fprintln(h.w, line)
}

func (h *humanWriter) prefix(emoji, fallback string) string {
	if h.emoji {
		return emoji + " "
	}
	if fallback == "" {
		return ""
	}
	return fallback + " "
}

func (h *humanWriter) Success(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	h.emit(h.prefix("✅", "[ok]") + h.t.success.Render(msg))
}

func (h *humanWriter) Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	h.emit(h.prefix("❌", "[err]") + h.t.errStyle.Render(msg))
}

func (h *humanWriter) Warning(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	h.emit(h.prefix("⚠️ ", "[warn]") + h.t.warning.Render(msg))
}

func (h *humanWriter) Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	h.emit(h.prefix("💡", "[info]") + h.t.info.Render(msg))
}

func (h *humanWriter) Header(title string) {
	icon := h.prefix("🚀", "")
	rendered := h.t.header.Render(icon + title)
	h.emit("")
	h.emit(rendered)
	h.emit(h.t.muted.Render(strings.Repeat("─", len(stripANSI(rendered)))))
}

func (h *humanWriter) Step(n, total int, name string) {
	badge := fmt.Sprintf("⟦ %d/%d ⟧", n, total)
	if !h.emoji {
		badge = fmt.Sprintf("[%d/%d]", n, total)
	}
	line := h.t.step.Render(badge) + " " + h.t.primary.Render(name)
	h.emit("")
	h.emit(line)
}

func (h *humanWriter) KeyValue(key, value string) {
	h.emit("  " + h.t.muted.Render(key+":") + " " + value)
}

func (h *humanWriter) Panel(title string, lines []string) {
	body := strings.Join(lines, "\n")
	if title != "" {
		body = h.t.primary.Render(title) + "\n" + body
	}
	h.emit(h.t.panel.Render(body))
}

func (h *humanWriter) Summary(title string, data map[string]string) {
	h.emit("")
	h.emit(h.t.muted.Render("──────────────── " + title + " ────────────────"))
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.KeyValue(k, data[k])
	}
}

func (h *humanWriter) DryRun(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	badge := "DRY RUN"
	if h.emoji {
		badge = "🔸 DRY RUN"
	}
	h.emit(h.t.dryRun.Render(badge) + "  " + msg)
}

func (h *humanWriter) JSON(doc any) {
	// Human mode renders JSON as a key-value panel best-effort. For complex
	// docs the user should pass --json instead.
	h.emit(h.t.subtle.Render(fmt.Sprintf("%+v", doc)))
}

func (h *humanWriter) Raw(line string) {
	fmt.Fprintln(h.w, line)
}

// stripANSI removes ANSI escape sequences for length calculation. Tiny
// implementation sufficient for our uses; fancy parsing not needed.
func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		if r == 0x1b {
			in = true
			continue
		}
		if in {
			if r == 'm' {
				in = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
