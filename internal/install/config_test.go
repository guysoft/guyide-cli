package install

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/guysoft/guyide-cli/pkg/schema"
)

func TestLoadOrInitUserConfig_FreshWritesDefault(t *testing.T) {
	root := t.TempDir()
	p := NewPathsAt(filepath.Join(root, ".guyide"))

	cfg, fresh, err := LoadOrInitUserConfig(p)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !fresh {
		t.Fatalf("expected fresh=true on first call")
	}
	if cfg.Schema != schema.UserConfigSchema {
		t.Fatalf("schema = %q", cfg.Schema)
	}
	if cfg.Components.Editor.Driver != "nvim" {
		t.Fatalf("editor driver = %q", cfg.Components.Editor.Driver)
	}
	if cfg.Components.Multiplexer.Driver != "tmux" {
		t.Fatalf("mpx driver = %q", cfg.Components.Multiplexer.Driver)
	}
	if cfg.Components.Agent.Driver != "opencode" {
		t.Fatalf("agent driver = %q", cfg.Components.Agent.Driver)
	}
	if _, err := os.Stat(p.UserConfig()); err != nil {
		t.Fatalf("config not written: %v", err)
	}
}

func TestLoadOrInitUserConfig_SecondCallNotFresh(t *testing.T) {
	root := t.TempDir()
	p := NewPathsAt(filepath.Join(root, ".guyide"))

	if _, _, err := LoadOrInitUserConfig(p); err != nil {
		t.Fatalf("first: %v", err)
	}
	cfg, fresh, err := LoadOrInitUserConfig(p)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if fresh {
		t.Fatalf("expected fresh=false on second call")
	}
	if cfg.Channel != "stable" {
		t.Fatalf("channel = %q", cfg.Channel)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	p := NewPathsAt(filepath.Join(root, ".guyide"))

	in := schema.DefaultUserConfig()
	in.Channel = "dev"
	in.Components.Editor.Ref = "v1.2.3"
	if err := SaveUserConfig(p, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := LoadUserConfig(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if out.Channel != "dev" {
		t.Fatalf("channel = %q", out.Channel)
	}
	if out.Components.Editor.Ref != "v1.2.3" {
		t.Fatalf("editor ref = %q", out.Components.Editor.Ref)
	}
}

func TestLoadUserConfig_MissingReturnsErrNotExist(t *testing.T) {
	root := t.TempDir()
	p := NewPathsAt(filepath.Join(root, ".guyide"))
	_, err := LoadUserConfig(p)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestLoadUserConfig_BadSchema(t *testing.T) {
	root := t.TempDir()
	p := NewPathsAt(filepath.Join(root, ".guyide"))
	if err := p.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.UserConfig(), []byte("schema: nope/v0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUserConfig(p); err == nil {
		t.Fatalf("expected schema mismatch error")
	}
}
