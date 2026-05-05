package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/guysoft/guyide-cli/pkg/schema"
)

type machineWriter struct {
	w   io.Writer
	enc *json.Encoder
}

func newMachineWriter(w io.Writer) *machineWriter {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &machineWriter{w: w, enc: enc}
}

func (m *machineWriter) Mode() Mode { return ModeMachine }

func (m *machineWriter) emit(level, msg string, extra map[string]any) {
	doc := map[string]any{
		"schema": schema.SchemaVersion,
		"level":  level,
		"msg":    msg,
	}
	for k, v := range extra {
		doc[k] = v
	}
	_ = m.enc.Encode(doc)
}

func (m *machineWriter) Success(format string, args ...any) {
	m.emit("success", fmt.Sprintf(format, args...), nil)
}
func (m *machineWriter) Error(format string, args ...any) {
	m.emit("error", fmt.Sprintf(format, args...), nil)
}
func (m *machineWriter) Warning(format string, args ...any) {
	m.emit("warning", fmt.Sprintf(format, args...), nil)
}
func (m *machineWriter) Info(format string, args ...any) {
	m.emit("info", fmt.Sprintf(format, args...), nil)
}
func (m *machineWriter) Header(title string) {
	m.emit("info", title, map[string]any{"section": title})
}
func (m *machineWriter) Step(n, total int, name string) {
	m.emit("info", name, map[string]any{"step": n, "total": total, "name": name})
}
func (m *machineWriter) KeyValue(key, value string) {
	m.emit("info", key, map[string]any{"key": key, "value": value})
}
func (m *machineWriter) Panel(title string, lines []string) {
	m.emit("info", title, map[string]any{"panel": title, "lines": lines})
}
func (m *machineWriter) Summary(title string, data map[string]string) {
	conv := make(map[string]any, len(data))
	for k, v := range data {
		conv[k] = v
	}
	m.emit("info", title, map[string]any{"summary": title, "data": conv})
}
func (m *machineWriter) DryRun(format string, args ...any) {
	m.emit("warning", fmt.Sprintf(format, args...), map[string]any{"dry_run": true})
}

func (m *machineWriter) JSON(doc any) {
	// Already-shaped documents (schema.* types) are emitted verbatim.
	_ = m.enc.Encode(doc)
}

func (m *machineWriter) Raw(line string) {
	fmt.Fprintln(m.w, line)
}
