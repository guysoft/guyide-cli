// Package output provides human and machine output writers for guyide.
//
// Two implementations satisfy the Writer interface: a styled human writer
// (Charmtone palette via lipgloss) and a JSON ndjson machine writer.
// Mode selection happens once at startup based on flags, env, and TTY state.
package output

// Writer is the unified output sink used by every guyide command.
//
// Implementations: humanWriter (styled), plainWriter (no ANSI), machineWriter
// (ndjson). Commands MUST write through this interface and never touch
// os.Stdout directly.
type Writer interface {
	// Success emits a successful-step message.
	Success(format string, args ...any)
	// Error emits an error message. Does not exit; caller decides.
	Error(format string, args ...any)
	// Warning emits a warning.
	Warning(format string, args ...any)
	// Info emits an informational message.
	Info(format string, args ...any)
	// Header emits a top-level section heading.
	Header(title string)
	// Step emits a numbered step badge "⟦ n/total ⟧ name".
	Step(n, total int, name string)
	// KeyValue emits a key→value pair (used in panels and summaries).
	KeyValue(key, value string)
	// Panel renders a titled panel containing the lines.
	Panel(title string, lines []string)
	// Summary emits a numeric/textual summary block.
	Summary(title string, data map[string]string)
	// DryRun marks output as dry-run (zest/warning emphasis).
	DryRun(format string, args ...any)
	// JSON emits a single machine-mode document. In human mode this is
	// rendered as a Panel with key/value rows.
	JSON(doc any)
	// Raw writes a literal line (used for ndjson streaming).
	Raw(line string)
	// Mode reports which mode this writer is in.
	Mode() Mode
}

// Mode identifies the active output mode.
type Mode int

const (
	ModeHumanStyled Mode = iota
	ModeHumanPlain
	ModeMachine
)

func (m Mode) String() string {
	switch m {
	case ModeHumanStyled:
		return "human-styled"
	case ModeHumanPlain:
		return "human-plain"
	case ModeMachine:
		return "machine"
	}
	return "unknown"
}
