package install

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateBackup_TarballRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a small file under fake $HOME.
	target := filepath.Join(home, ".tmux.conf")
	if err := os.WriteFile(target, []byte("set -g prefix C-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewPathsAt(filepath.Join(home, ".guyide"))
	entry, err := CreateBackup(p, BackupRequest{
		Paths:   []string{target},
		Reason:  "install",
		HomeDir: home,
	})
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	if _, err := os.Stat(entry.Path); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}

	// Read back tarball and confirm the file lives at home/.tmux.conf
	// with the right contents.
	got := readTarball(t, entry.Path)
	if c, ok := got["home/.tmux.conf"]; !ok {
		t.Fatalf("missing entry in tarball; got keys=%v", keys(got))
	} else if !strings.Contains(c, "prefix C-a") {
		t.Fatalf("unexpected tarball contents: %q", c)
	}
}

func TestCreateBackup_MissingPathsProducesEmptyMarker(t *testing.T) {
	tmp := t.TempDir()
	p := NewPathsAt(filepath.Join(tmp, ".guyide"))
	entry, err := CreateBackup(p, BackupRequest{
		Paths:   []string{filepath.Join(tmp, "does-not-exist")},
		Reason:  "install",
		HomeDir: tmp,
	})
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	got := readTarball(t, entry.Path)
	if _, ok := got["MANIFEST.txt"]; !ok {
		t.Fatalf("expected MANIFEST.txt, got %v", keys(got))
	}
}

func TestCreateBackup_RequiresReason(t *testing.T) {
	p := NewPathsAt(t.TempDir())
	if _, err := CreateBackup(p, BackupRequest{}); err == nil {
		t.Fatal("expected error for missing reason")
	}
}

func TestAppendBackup_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	p := NewPathsAt(filepath.Join(tmp, ".guyide"))
	if err := p.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	entry, err := CreateBackup(p, BackupRequest{
		Paths:   []string{filepath.Join(tmp, "nope")},
		Reason:  "manual",
		HomeDir: tmp,
	})
	if err != nil {
		t.Fatal(err)
	}

	m, err := AppendBackup(p, entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(m.Backups))
	}
	// Append the same entry: should be a no-op.
	m, err = AppendBackup(p, entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Backups) != 1 {
		t.Fatalf("expected idempotent append, got %d entries", len(m.Backups))
	}

	// New entry: should land at index 0 (newest first).
	entry2, err := CreateBackup(p, BackupRequest{
		Paths:   []string{filepath.Join(tmp, "nope2")},
		Reason:  "update",
		HomeDir: tmp,
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err = AppendBackup(p, entry2)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Backups) != 2 || m.Backups[0].Path != entry2.Path {
		t.Fatalf("expected newest-first ordering, got %+v", m.Backups)
	}
}

func TestListBackups(t *testing.T) {
	tmp := t.TempDir()
	p := NewPathsAt(filepath.Join(tmp, ".guyide"))
	if err := p.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	entry, err := CreateBackup(p, BackupRequest{
		Paths:   []string{filepath.Join(tmp, "nope")},
		Reason:  "manual",
		HomeDir: tmp,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendBackup(p, entry); err != nil {
		t.Fatal(err)
	}
	got, err := ListBackups(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != entry.Path {
		t.Fatalf("ListBackups: got %+v", got)
	}
}

// helpers ---------------------------------------------------------------

func readTarball(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	got := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var sb strings.Builder
		if _, err := io.Copy(&sb, tr); err != nil {
			t.Fatal(err)
		}
		got[hdr.Name] = sb.String()
	}
	return got
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
