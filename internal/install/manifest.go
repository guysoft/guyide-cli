package install

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"time"

	"github.com/guysoft/guyide-cli/pkg/schema"
)

// LoadManifest reads ~/.guyide/manifest.json. Returns os.ErrNotExist if
// guyide has never been installed.
func LoadManifest(p Paths) (schema.Manifest, error) {
	var m schema.Manifest
	b, err := os.ReadFile(p.Manifest())
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if m.Schema != schema.ManifestSchema {
		return m, errors.New("install: manifest schema mismatch: " + m.Schema)
	}
	return m, nil
}

// SaveManifest writes the manifest atomically (tmp file + rename).
// Callers must hold logical exclusion themselves; we don't take a lock.
func SaveManifest(p Paths, m schema.Manifest) error {
	if m.Schema == "" {
		m.Schema = schema.ManifestSchema
	}
	if m.InstalledAt.IsZero() {
		m.InstalledAt = time.Now().UTC()
	}
	m.UpdatedAt = time.Now().UTC()

	if err := p.EnsureLayout(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(&m, "", "  ")
	if err != nil {
		return err
	}
	tmp := p.Manifest() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p.Manifest())
}

// ManifestExists reports whether the manifest file is present.
func ManifestExists(p Paths) bool {
	_, err := os.Stat(p.Manifest())
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}
