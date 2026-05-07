// Package install owns the on-disk layout under ~/.guyide and the
// helpers that read/write its manifest, backups, and channel state.
//
// Subcommand glue (cobra) lives in internal/cli; this package is pure
// filesystem + data and is therefore safe to unit-test without any
// process orchestration.
package install

import (
	"errors"
	"os"
	"path/filepath"
)

// Paths resolves every well-known path under the install root. All
// callers go through this struct so we never sprinkle filepath.Join
// across the codebase.
type Paths struct {
	Root string // ~/.guyide
}

// NewPaths returns Paths rooted at the conventional ~/.guyide. If
// $HOME is unset (which would be very weird) it returns an error
// rather than silently writing to /.guyide.
func NewPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	if home == "" {
		return Paths{}, errors.New("install: $HOME is empty")
	}
	return Paths{Root: filepath.Join(home, ".guyide")}, nil
}

// NewPathsAt returns Paths rooted at an explicit directory. Used by
// tests and by GUYIDE_HOME overrides.
func NewPathsAt(root string) Paths { return Paths{Root: root} }

// Manifest is ~/.guyide/manifest.json.
func (p Paths) Manifest() string { return filepath.Join(p.Root, "manifest.json") }

// UserConfig is ~/.guyide/config.yaml.
func (p Paths) UserConfig() string { return filepath.Join(p.Root, "config.yaml") }

// ChannelFile is ~/.guyide/channel (one line: stable|dev).
func (p Paths) ChannelFile() string { return filepath.Join(p.Root, "channel") }

// EnvFile is ~/.guyide/env (sourced by tmux profile + agent shells).
func (p Paths) EnvFile() string { return filepath.Join(p.Root, "env") }

// Bin is the dir containing the active guyide binary.
func (p Paths) Bin() string { return filepath.Join(p.Root, "bin") }

// Binary is ~/.guyide/bin/guyide.
func (p Paths) Binary() string { return filepath.Join(p.Bin(), "guyide") }

// Components is the parent dir for every installed component.
func (p Paths) Components() string { return filepath.Join(p.Root, "components") }

// Component returns the install dir for a named component
// (e.g. "nvim", "tmux", "vscodium.nvim").
func (p Paths) Component(name string) string {
	return filepath.Join(p.Components(), name)
}

// Backups is the dir holding tarballed snapshots.
func (p Paths) Backups() string { return filepath.Join(p.Root, "backups") }

// BackupAt returns the tarball path for a given RFC3339 timestamp.
func (p Paths) BackupAt(rfc3339 string) string {
	return filepath.Join(p.Backups(), rfc3339+".tar.gz")
}

// Logs is ~/.guyide/logs.
func (p Paths) Logs() string { return filepath.Join(p.Root, "logs") }

// UserLocalBin returns ~/.local/bin (the symlink target dir).
func (p Paths) UserLocalBin() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// UserLocalBinSymlink returns ~/.local/bin/guyide.
func (p Paths) UserLocalBinSymlink() (string, error) {
	dir, err := p.UserLocalBin()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "guyide"), nil
}

// EnsureLayout creates every directory the installer needs. It is
// idempotent and uses 0o755 which matches the rest of the user's
// tree under $HOME.
func (p Paths) EnsureLayout() error {
	for _, d := range []string{p.Root, p.Bin(), p.Components(), p.Backups(), p.Logs()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
