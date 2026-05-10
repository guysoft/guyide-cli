// Package embed bundles installer data assets into the guyide binary.
//
// All files in this directory are baked at build time so the installer
// runs fully offline once the binary is on disk. Consumers should treat
// the contents as read-only and parse them via the helpers exposed here.
package embed

import (
	"embed"
	"io/fs"

	"github.com/guysoft/guyide-cli/internal/version"
)

//go:embed compat.json
var compatJSON []byte

//go:embed support_matrix.yaml
var supportMatrixYAML []byte

//go:embed welcome/WELCOME.md
var welcomeMD []byte

//go:embed tmux/guyide.conf
var tmuxConf []byte

// CompatJSON returns the raw bytes of the compatibility matrix used by
// the stable channel.
func CompatJSON() []byte { return compatJSON }

// SupportMatrixYAML returns the raw bytes of the support matrix that
// declares which (editor, multiplexer, agent) triples are validated.
func SupportMatrixYAML() []byte { return supportMatrixYAML }

// WelcomeMarkdown returns the first-launch cheatsheet shown by the
// quick-start flow. The installer writes this to ~/.guyide/WELCOME.md
// and opens it in nvim so new users learn the essential keybindings.
func WelcomeMarkdown() []byte { return welcomeMD }

// CLIVersion returns the build-time CLI version (proxy to the version
// package). Lives here so installer code can resolve compat pins
// without importing internal/version directly from leaf drivers.
func CLIVersion() string { return version.Version }

// TmuxGuyideConf returns the canonical tmux configuration that the
// tmux driver writes to ~/.guyide/components/tmux/guyide.conf and
// then symlinks/copies into ~/.tmux.conf.
func TmuxGuyideConf() []byte { return tmuxConf }

// TmuxManagedMarker is the second-line sentinel used to recognise a
// guyide-owned ~/.tmux.conf during drift detection.
const TmuxManagedMarker = "# guyide:managed v1"

//go:embed all:opencode
var opencodeFS embed.FS

// OpenCodeSkillsFS returns the embedded ~/.config/opencode/skills/ tree
// shipped by guyide. The returned fs is rooted at "opencode/skills";
// callers should walk that prefix to discover skill subdirectories.
func OpenCodeSkillsFS() fs.FS {
	sub, err := fs.Sub(opencodeFS, "opencode/skills")
	if err != nil {
		// Should be unreachable; embed.FS guarantees the path exists.
		panic(err)
	}
	return sub
}

// OpenCodeManagedMarker is the filename written inside each
// guyide-managed opencode skill directory. Its presence tells the
// driver "we own this dir; safe to update or remove".
const OpenCodeManagedMarker = ".guyide-managed"

//go:embed all:claude-code
var claudeCodeFS embed.FS

// ClaudeCodeSkillsFS returns the embedded ~/.claude/skills/ tree
// shipped by guyide. The returned fs is rooted at "claude-code/skills";
// callers should walk that prefix to discover skill subdirectories.
func ClaudeCodeSkillsFS() fs.FS {
	sub, err := fs.Sub(claudeCodeFS, "claude-code/skills")
	if err != nil {
		panic(err)
	}
	return sub
}

// ClaudeCodeManagedMarker is the filename written inside each
// guyide-managed claude-code skill directory.
const ClaudeCodeManagedMarker = ".guyide-managed"
