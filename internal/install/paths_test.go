package install

import (
	"path/filepath"
	"testing"
)

func TestPathsEnsureLayoutAndAccessors(t *testing.T) {
	root := t.TempDir()
	p := NewPathsAt(root)

	if err := p.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout: %v", err)
	}
	// Idempotent.
	if err := p.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout (rerun): %v", err)
	}

	cases := map[string]string{
		"manifest":   p.Manifest(),
		"userconfig": p.UserConfig(),
		"channel":    p.ChannelFile(),
		"binary":     p.Binary(),
		"component":  p.Component("nvim"),
		"backupAt":   p.BackupAt("2026-05-05T12-00-00Z"),
	}
	for name, got := range cases {
		if filepath.Dir(got) == "" {
			t.Errorf("%s: empty dir", name)
		}
		if rel, err := filepath.Rel(root, got); err != nil || rel == "" {
			t.Errorf("%s: %q not under root %q", name, got, root)
		}
	}
}
