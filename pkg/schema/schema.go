// Package schema defines the stable JSON output schema for guyide.
//
// All machine-mode (--json) output is encoded against types in this package
// and tagged with Schema = "guyide/v1". Skills and other consumers should
// pin to the schema string and treat unknown fields as forward-compatible.
package schema

// SchemaVersion is the stable identifier emitted on every JSON document.
const SchemaVersion = "guyide/v1"

// Envelope is the common header on every machine-mode document.
type Envelope struct {
	Schema string `json:"schema"`
	Level  string `json:"level,omitempty"` // success|error|warning|info|debug
	Msg    string `json:"msg,omitempty"`
}

// VersionInfo is emitted by `guyide version --json`.
type VersionInfo struct {
	Envelope
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// EnvInfo is emitted by `guyide env --json`.
type EnvInfo struct {
	Envelope
	Socket          string   `json:"socket,omitempty"`
	SocketSource    string   `json:"socket_source,omitempty"` // flag|env|tmux|scan|pane
	TmuxSession     string   `json:"tmux_session,omitempty"`
	TmuxWindow      string   `json:"tmux_window,omitempty"`
	NvimReachable   bool     `json:"nvim_reachable"`
	Candidates      []string `json:"candidates,omitempty"`
	NvimIDESock     string   `json:"NVIM_IDE_SOCK,omitempty"`
	GuyidePaneRole  string   `json:"GUYIDE_PANE_ROLE,omitempty"`
}

// DoctorCheck is one row in the doctor report.
type DoctorCheck struct {
	Group   string `json:"group"`
	Name    string `json:"name"`
	Status  string `json:"status"` // ok|warn|fail|skip
	Message string `json:"message,omitempty"`
}

// DoctorReport is emitted by `guyide doctor --json`.
type DoctorReport struct {
	Envelope
	Checks   []DoctorCheck `json:"checks"`
	Passed   int           `json:"passed"`
	Warnings int           `json:"warnings"`
	Failures int           `json:"failures"`
	Ready    bool          `json:"ready"`
}

// Frame is one stack frame in a debug session.
type Frame struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
}

// Variable is a scope-local variable.
type Variable struct {
	Name  string `json:"name"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

// DebugState is emitted by `guyide debug state --json`.
type DebugState struct {
	Envelope
	SessionActive bool       `json:"session_active"`
	Stopped       bool       `json:"stopped"`
	Reason        string     `json:"reason,omitempty"`
	File          string     `json:"file,omitempty"`
	Line          int        `json:"line,omitempty"`
	Frames        []Frame    `json:"frames,omitempty"`
	Variables     []Variable `json:"variables,omitempty"`
}

// WatchEvent is one ndjson record from `guyide tmux watch`.
type WatchEvent struct {
	Envelope
	Line    string `json:"line,omitempty"`
	Matched bool   `json:"matched,omitempty"`
	Timeout bool   `json:"timeout,omitempty"`
}
