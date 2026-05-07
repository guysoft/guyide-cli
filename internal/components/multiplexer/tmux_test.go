package multiplexer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	guyembed "github.com/guysoft/guyide-cli/embed"
	"github.com/guysoft/guyide-cli/internal/components"
	"github.com/guysoft/guyide-cli/internal/install"
	"github.com/guysoft/guyide-cli/pkg/schema"
)

func newCtx(t *testing.T) *components.Context {
	t.Helper()
	t.Setenv("GUYIDE_SKIP_TPM_BOOTSTRAP", "1")
	root := t.TempDir()
	home := filepath.Join(root, "home")
	guy := filepath.Join(home, ".guyide")
	if err := os.MkdirAll(guy, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := schema.DefaultUserConfig()
	return &components.Context{
		HomeDir:    home,
		GuyideDir:  guy,
		Channel:    "stable",
		UserConfig: &cfg,
	}
}

func TestTmuxDriverRegistered(t *testing.T) {
	c, err := components.Get(components.SlotMultiplexer, "tmux")
	if err != nil {
		t.Fatalf("tmux not registered: %v", err)
	}
	if c.Slot() != components.SlotMultiplexer {
		t.Errorf("slot = %q", c.Slot())
	}
}

func TestPlan_FreshInstall(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	plan, err := d.Plan(c)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != components.StatusInstall {
		t.Errorf("status = %q, want install", plan.Status)
	}
	joined := strings.Join(plan.Actions, "\n")
	if !strings.Contains(joined, "write") {
		t.Errorf("missing write action: %q", joined)
	}
}

func TestPlan_UnchangedWhenIdentical(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	if err := os.MkdirAll(d.componentDir(c), 0o755); err != nil {
		t.Fatal(err)
	}
	managed := d.managedConfPath(c)
	if err := os.WriteFile(managed, guyembed.TmuxGuyideConf(), 0o644); err != nil {
		t.Fatal(err)
	}
	user := d.userConfPath(c)
	if err := os.WriteFile(user, guyembed.TmuxGuyideConf(), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := d.Plan(c)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != components.StatusUnchanged {
		t.Errorf("status = %q, want unchanged", plan.Status)
	}
}

func TestPlan_NonManagedUserConfWarns(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	if err := os.WriteFile(d.userConfPath(c), []byte("# my hand-rolled tmux\nset -g prefix C-b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := d.Plan(c)
	if err != nil {
		t.Fatal(err)
	}
	notes := strings.Join(plan.Notes, "\n")
	if !strings.Contains(notes, "not guyide-managed") {
		t.Errorf("missing non-managed note: %q", notes)
	}
}

func TestPlan_OptOutOfOwnership(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	c.UserConfig.Tmux.OwnConf = false
	plan, err := d.Plan(c)
	if err != nil {
		t.Fatal(err)
	}
	notes := strings.Join(plan.Notes, "\n")
	if !strings.Contains(notes, "own_conf=false") {
		t.Errorf("expected opt-out note: %q", notes)
	}
	for _, a := range plan.Actions {
		if strings.Contains(a, ".tmux.conf") {
			t.Errorf("should not touch user conf when own_conf=false: %q", a)
		}
	}
}

func TestInstall_WritesManagedConfAndUserConf(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	if err := d.Install(c); err != nil {
		t.Fatal(err)
	}
	managed, err := os.ReadFile(d.managedConfPath(c))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(managed, guyembed.TmuxGuyideConf()) {
		t.Error("managed conf content mismatch")
	}
	user, err := os.ReadFile(d.userConfPath(c))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(user, managed) {
		t.Error("user conf doesn't match managed conf")
	}
	// Marker present.
	if !strings.Contains(string(user), guyembed.TmuxManagedMarker) {
		t.Error("managed marker missing from user conf")
	}
}

func TestInstall_BacksUpExistingNonManagedConf(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	user := d.userConfPath(c)
	if err := os.MkdirAll(filepath.Dir(user), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(user, []byte("# original user tmux\nset -g prefix C-b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.Install(c); err != nil {
		t.Fatal(err)
	}
	// No legacy sibling backup file.
	if _, err := os.Stat(user + ".guyide-bak"); !os.IsNotExist(err) {
		t.Errorf("legacy sibling backup should not exist, stat err=%v", err)
	}
	// Manifest records exactly one tmux backup with the original
	// content captured inside the tarball.
	p := install.NewPathsAt(c.GuyideDir)
	backups, err := install.ListBackups(p)
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 manifest backup, got %d", len(backups))
	}
	if backups[0].Component != "tmux" {
		t.Errorf("backup.Component = %q, want tmux", backups[0].Component)
	}
	if backups[0].Reason != "install" {
		t.Errorf("backup.Reason = %q, want install", backups[0].Reason)
	}
	body := readTarballMember(t, backups[0].Path, "home/.tmux.conf")
	if !strings.Contains(string(body), "C-b") {
		t.Errorf("backup body wrong: %q", body)
	}
	// New conf is now in place and managed.
	managed, err := isManagedTmuxConf(user)
	if err != nil || !managed {
		t.Errorf("user conf not managed after install: managed=%v err=%v", managed, err)
	}
}

// readTarballMember returns the bytes of the first archive member
// matching name, or fails the test.
func readTarballMember(t *testing.T, path, name string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			t.Fatalf("member %q not found in %s", name, path)
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		if hdr.Name == name {
			b, _ := io.ReadAll(tr)
			return b
		}
	}
}

func TestInstall_DryRunNoOp(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	c.DryRun = true
	if err := d.Install(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(d.managedConfPath(c)); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote file: %v", err)
	}
	if _, err := os.Stat(d.userConfPath(c)); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote user conf: %v", err)
	}
}

func TestInstall_OptOutLeavesUserConfAlone(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	c.UserConfig.Tmux.OwnConf = false
	user := d.userConfPath(c)
	if err := os.MkdirAll(filepath.Dir(user), 0o755); err != nil {
		t.Fatal(err)
	}
	const body = "# user owns this\n"
	if err := os.WriteFile(user, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.Install(c); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(user)
	if string(got) != body {
		t.Errorf("user conf clobbered: %q", got)
	}
	// Managed conf still written.
	if _, err := os.Stat(d.managedConfPath(c)); err != nil {
		t.Errorf("managed conf missing: %v", err)
	}
}

func TestUninstall_RemovesOnlyManagedConf(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	if err := d.Install(c); err != nil {
		t.Fatal(err)
	}
	if err := d.Uninstall(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(d.componentDir(c)); !os.IsNotExist(err) {
		t.Errorf("component dir not removed: %v", err)
	}
	if _, err := os.Lstat(d.userConfPath(c)); !os.IsNotExist(err) {
		t.Errorf("managed user conf not removed: %v", err)
	}
}

func TestUninstall_LeavesNonManagedUserConfAlone(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	if err := d.Install(c); err != nil {
		t.Fatal(err)
	}
	// Replace with non-managed body.
	if err := os.WriteFile(d.userConfPath(c), []byte("# user took it back\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.Uninstall(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(d.userConfPath(c)); err != nil {
		t.Errorf("non-managed user conf wrongly removed: %v", err)
	}
}

func TestDoctor_FailsWhenManagedConfMissing(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	checks := d.Doctor(c)
	if checks[0].Status != "fail" {
		t.Errorf("first check = %+v, want fail", checks[0])
	}
}

func TestDoctor_OkAfterInstall(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	if err := d.Install(c); err != nil {
		t.Fatal(err)
	}
	checks := d.Doctor(c)
	for i, ch := range checks {
		// "tmux on PATH" may legitimately fail in CI; skip that one.
		if ch.Name == "tmux on PATH" {
			continue
		}
		if ch.Status != "ok" {
			t.Errorf("check[%d] %+v not ok", i, ch)
		}
	}
}

func TestDoctor_DriftWarning(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	if err := d.Install(c); err != nil {
		t.Fatal(err)
	}
	// Hand-edit the managed file.
	if err := os.WriteFile(d.managedConfPath(c), []byte("# tampered\n"+guyembed.TmuxManagedMarker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checks := d.Doctor(c)
	var found bool
	for _, ch := range checks {
		if ch.Name == "guyide.conf matches embedded" && ch.Status == "warn" {
			found = true
		}
	}
	if !found {
		t.Errorf("drift not flagged: %+v", checks)
	}
}

func TestIsManagedTmuxConf(t *testing.T) {
	dir := t.TempDir()
	mk := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("with marker", func(t *testing.T) {
		p := mk("a", "# header\n"+guyembed.TmuxManagedMarker+"\nset -g prefix C-a\n")
		ok, err := isManagedTmuxConf(p)
		if err != nil || !ok {
			t.Errorf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("without marker", func(t *testing.T) {
		p := mk("b", "set -g prefix C-b\n")
		ok, err := isManagedTmuxConf(p)
		if err != nil || ok {
			t.Errorf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("missing", func(t *testing.T) {
		_, err := isManagedTmuxConf(filepath.Join(dir, "missing"))
		if !os.IsNotExist(err) {
			t.Errorf("err = %v, want fs.ErrNotExist", err)
		}
	})
	t.Run("marker after first 5 lines is ignored", func(t *testing.T) {
		body := "1\n2\n3\n4\n5\n6\n" + guyembed.TmuxManagedMarker + "\n"
		p := mk("c", body)
		ok, _ := isManagedTmuxConf(p)
		if ok {
			t.Error("marker beyond line 5 should not count as managed")
		}
	})
}

func TestHashConf_Stable(t *testing.T) {
	if HashConf() == "" {
		t.Fatal("empty hash")
	}
	// Two calls return the same value (deterministic).
	if HashConf() != HashConf() {
		t.Error("HashConf not deterministic")
	}
}

func TestOwnedPaths(t *testing.T) {
	d := &TmuxDriver{}
	c := newCtx(t)
	paths := d.OwnedPaths(c)
	if len(paths) != 2 {
		t.Errorf("paths = %v, want 2 (component dir + user conf)", paths)
	}

	c.UserConfig.Tmux.OwnConf = false
	paths = d.OwnedPaths(c)
	if len(paths) != 1 {
		t.Errorf("paths with own_conf=false = %v, want 1", paths)
	}
}
