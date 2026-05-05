package install

import (
	"testing"
	"time"

	"github.com/guysoft/guyide-cli/pkg/schema"
)

func TestManifestRoundTrip(t *testing.T) {
	p := NewPathsAt(t.TempDir())
	if err := p.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	want := schema.Manifest{
		Version: "v0.2.0",
		Channel: "stable",
		Components: map[string]schema.ComponentEntry{
			"editor": {Slot: "editor", Driver: "nvim", Ref: "v0.1.0"},
		},
	}
	if err := SaveManifest(p, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadManifest(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Schema != schema.ManifestSchema {
		t.Errorf("schema = %q want %q", got.Schema, schema.ManifestSchema)
	}
	if got.Version != want.Version {
		t.Errorf("version = %q want %q", got.Version, want.Version)
	}
	if got.Channel != "stable" {
		t.Errorf("channel = %q", got.Channel)
	}
	if got.UpdatedAt.IsZero() {
		t.Errorf("UpdatedAt was not set")
	}
	if got.InstalledAt.After(time.Now().Add(time.Minute)) {
		t.Errorf("InstalledAt drifted into the future: %v", got.InstalledAt)
	}
	if e, ok := got.Components["editor"]; !ok || e.Driver != "nvim" {
		t.Errorf("components[editor] = %+v", e)
	}
}
