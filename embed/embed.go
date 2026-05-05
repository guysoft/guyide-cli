// Package embed bundles installer data assets into the guyide binary.
//
// All files in this directory are baked at build time so the installer
// runs fully offline once the binary is on disk. Consumers should treat
// the contents as read-only and parse them via the helpers exposed here.
package embed

import (
	_ "embed"

	"github.com/guysoft/guyide-cli/internal/version"
)

//go:embed compat.json
var compatJSON []byte

//go:embed support_matrix.yaml
var supportMatrixYAML []byte

//go:embed welcome/WELCOME.md
var welcomeMD []byte

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
