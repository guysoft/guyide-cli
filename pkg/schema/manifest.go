package schema

import "time"

// ManifestSchema is the stable identifier for the on-disk install manifest.
const ManifestSchema = "guyide/manifest/v1"

// Manifest is the source of truth for what guyide has installed locally.
//
// It lives at ~/.guyide/manifest.json and is rewritten atomically on every
// install/update/uninstall. Drift detection, rollback, and `guyide doctor`
// all read from this file.
type Manifest struct {
	Schema     string              `json:"schema"`
	Version    string              `json:"version"`              // guyide-cli version that wrote this manifest
	Channel    string              `json:"channel"`              // stable | dev
	InstalledAt time.Time          `json:"installed_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
	Components map[string]ComponentEntry `json:"components"` // keyed by slot: editor|multiplexer|agent and component name (e.g. nvim, tmux, opencode, vscodium.nvim, nvguy)
	OwnedFiles []OwnedFile         `json:"owned_files"`          // files+symlinks guyide may safely overwrite/remove
	Backups    []BackupEntry       `json:"backups"`              // descending by timestamp
}

// ComponentEntry records what driver is active in a slot and which ref is
// installed. Drift hashes let `doctor` detect user edits we should warn
// about before clobbering on update.
type ComponentEntry struct {
	Slot     string            `json:"slot"`     // editor|multiplexer|agent
	Driver   string            `json:"driver"`   // nvim|tmux|opencode|claude-code
	Ref      string            `json:"ref"`      // git tag/sha or package version
	Source   string            `json:"source"`   // git url or pkg manager
	Path     string            `json:"path"`     // install dir under ~/.guyide/components/
	Hashes   map[string]string `json:"hashes,omitempty"` // path -> sha256 of files we wrote
	Metadata map[string]string `json:"metadata,omitempty"`
}

// OwnedFile records a path guyide is allowed to write/remove. Anything
// not listed here is treated as user-owned and protected.
type OwnedFile struct {
	Path string `json:"path"`           // absolute or $HOME-relative
	Kind string `json:"kind"`           // file|symlink|dir
	Hash string `json:"hash,omitempty"` // sha256 at last write (file/symlink target)
}

// BackupEntry references one tarball under ~/.guyide/backups/.
type BackupEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path"`            // ~/.guyide/backups/<RFC3339>.tar.gz
	Reason    string    `json:"reason"`          // install|update|uninstall|manual
	Component string    `json:"component,omitempty"`
}
