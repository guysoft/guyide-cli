package install

import (
	"os"
	"strings"
	"testing"
)

func TestWriteWelcome(t *testing.T) {
	p := NewPathsAt(t.TempDir())
	if err := WriteWelcome(p); err != nil {
		t.Fatalf("WriteWelcome: %v", err)
	}
	got, err := os.ReadFile(p.WelcomePath())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(got)
	for _, want := range []string{
		"# Welcome to GuyIDE",
		"`Ctrl-a` then `e`",
		"Ctrl-a d",
		"Ctrl-a Ctrl-r",
		"Ctrl-a ?",
		"Space m",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("WELCOME.md missing %q", want)
		}
	}
	// Idempotent: writing twice must succeed and produce the same bytes.
	if err := WriteWelcome(p); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	got2, _ := os.ReadFile(p.WelcomePath())
	if string(got2) != s {
		t.Errorf("rewrite changed content")
	}
}
