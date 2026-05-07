package editor

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guysoft/guyide-cli/internal/components"
	"github.com/guysoft/guyide-cli/internal/install"
)

// newCtx returns a Context whose GuyideDir + HomeDir are isolated to
// the test's t.TempDir so we never touch the developer's real
// ~/.guyide or ~/.config.
func newCtx(t *testing.T) *components.Context {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	guy := filepath.Join(home, ".guyide")
	if err := os.MkdirAll(guy, 0o755); err != nil {
		t.Fatal(err)
	}
	return &components.Context{
		HomeDir:   home,
		GuyideDir: guy,
		Channel:   "stable",
	}
}

func TestNvimDriverRegistered(t *testing.T) {
	c, err := components.Get(components.SlotEditor, "nvim")
	if err != nil {
		t.Fatalf("nvim driver not registered for SlotEditor: %v", err)
	}
	if c.Slot() != components.SlotEditor {
		t.Errorf("slot = %q, want %q", c.Slot(), components.SlotEditor)
	}
	if c.Driver() != "nvim" {
		t.Errorf("driver = %q, want nvim", c.Driver())
	}
}

func TestResolveSource_LocalOverride(t *testing.T) {
	d := &NvimDriver{repo: "https://example.com/x.git", devEnvVar: "GUYIDE_TEST_NVGUY"}
	t.Setenv("GUYIDE_TEST_NVGUY", "/tmp/local-nvguy")
	src, isLocal := d.resolveSource()
	if !isLocal || src != "/tmp/local-nvguy" {
		t.Fatalf("local override not honoured: src=%q isLocal=%v", src, isLocal)
	}

	t.Setenv("GUYIDE_TEST_NVGUY", "")
	src, isLocal = d.resolveSource()
	if isLocal || src != "https://example.com/x.git" {
		t.Fatalf("repo fallback wrong: src=%q isLocal=%v", src, isLocal)
	}
}

func TestResolveRef_UserRefWins(t *testing.T) {
	d := &NvimDriver{}
	c := newCtx(t)
	c.UserRef = "v9.9.9"
	c.Channel = "dev"
	got, err := d.resolveRef(c)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v9.9.9" {
		t.Errorf("ref = %q, want v9.9.9", got)
	}
}

func TestResolveRef_DevChannelMain(t *testing.T) {
	d := &NvimDriver{}
	c := newCtx(t)
	c.Channel = "dev"
	got, err := d.resolveRef(c)
	if err != nil {
		t.Fatal(err)
	}
	if got != "main" {
		t.Errorf("ref = %q, want main", got)
	}
}

func TestResolveRef_StableUsesCompat(t *testing.T) {
	d := &NvimDriver{}
	c := newCtx(t)
	c.Channel = "stable"
	got, err := d.resolveRef(c)
	if err != nil {
		t.Fatal(err)
	}
	// Embedded compat.json pins nvguy to v0.1.0 for v0.2.0; if the
	// running CLI version is v0.0.0-dev there's no pin -> "main".
	if got == "" {
		t.Fatal("empty ref")
	}
}

func TestPlan_InstallWhenMissing(t *testing.T) {
	d := &NvimDriver{
		repo:       "https://example.com/x.git",
		devEnvVar:  "GUYIDE_TEST_NVGUY_PLAN",
		installDir: "nvim",
	}
	c := newCtx(t)
	plan, err := d.Plan(c)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != components.StatusInstall {
		t.Errorf("status = %q, want %q", plan.Status, components.StatusInstall)
	}
	if plan.Slot != components.SlotEditor {
		t.Errorf("slot = %q", plan.Slot)
	}
	joined := strings.Join(plan.Actions, "\n")
	if !strings.Contains(joined, "git clone") {
		t.Errorf("actions missing git clone: %q", joined)
	}
	if !strings.Contains(joined, "lazy.sync") {
		t.Errorf("actions missing lazy.sync drain: %q", joined)
	}
	if !strings.Contains(joined, "symlink") {
		t.Errorf("actions missing symlink step: %q", joined)
	}
}

func TestPlan_LocalOverrideUsesCopy(t *testing.T) {
	t.Setenv("GUYIDE_TEST_NVGUY_LOCAL", "/tmp/nvguy-checkout")
	d := &NvimDriver{
		repo:       "https://example.com/x.git",
		devEnvVar:  "GUYIDE_TEST_NVGUY_LOCAL",
		installDir: "nvim",
	}
	c := newCtx(t)
	plan, err := d.Plan(c)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Actions, "\n")
	if !strings.Contains(joined, "copy NvGuy") {
		t.Errorf("local override should produce copy action: %q", joined)
	}
	if strings.Contains(joined, "git clone") {
		t.Errorf("local override should NOT git clone: %q", joined)
	}
}

func TestPlan_UpdateWhenAlreadyInstalled(t *testing.T) {
	d := &NvimDriver{
		repo:       "https://example.com/x.git",
		devEnvVar:  "GUYIDE_TEST_NVGUY_UPD",
		installDir: "nvim",
	}
	c := newCtx(t)
	dst := d.componentPath(c)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := d.Plan(c)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != components.StatusUpdate {
		t.Errorf("status = %q, want %q", plan.Status, components.StatusUpdate)
	}
}

func TestPlan_NotesWhenConfigDirIsRealDir(t *testing.T) {
	d := &NvimDriver{
		repo:       "https://example.com/x.git",
		devEnvVar:  "GUYIDE_TEST_NVGUY_NOTES",
		installDir: "nvim",
	}
	c := newCtx(t)
	link := d.symlinkTarget(c)
	if err := os.MkdirAll(link, 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := d.Plan(c)
	if err != nil {
		t.Fatal(err)
	}
	notes := strings.Join(plan.Notes, "\n")
	if !strings.Contains(notes, "not a symlink") {
		t.Errorf("missing not-a-symlink note: %q", notes)
	}
}

func TestLinkConfig_CreatesSymlink(t *testing.T) {
	d := &NvimDriver{installDir: "nvim"}
	c := newCtx(t)
	dst := d.componentPath(c)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := d.linkConfig(c, dst); err != nil {
		t.Fatal(err)
	}
	link := d.symlinkTarget(c)
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if target != dst {
		t.Errorf("link -> %q, want %q", target, dst)
	}
}

func TestLinkConfig_ReplacesWrongSymlink(t *testing.T) {
	d := &NvimDriver{installDir: "nvim"}
	c := newCtx(t)
	dst := d.componentPath(c)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	link := d.symlinkTarget(c)
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	bogus := filepath.Join(c.HomeDir, "bogus")
	if err := os.Symlink(bogus, link); err != nil {
		t.Fatal(err)
	}
	if err := d.linkConfig(c, dst); err != nil {
		t.Fatal(err)
	}
	target, _ := os.Readlink(link)
	if target != dst {
		t.Errorf("link -> %q, want %q", target, dst)
	}
}

func TestLinkConfig_BacksUpRealDir(t *testing.T) {
	d := &NvimDriver{installDir: "nvim"}
	c := newCtx(t)
	dst := d.componentPath(c)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	link := d.symlinkTarget(c)
	if err := os.MkdirAll(link, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(link, "user-stuff.txt")
	if err := os.WriteFile(marker, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.linkConfig(c, dst); err != nil {
		t.Fatal(err)
	}
	// Original location is now a symlink.
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected symlink")
	}
	// No sibling .guyide-bak-* dir exists (the legacy mechanism).
	if matches, _ := filepath.Glob(link + ".guyide-bak-*"); len(matches) != 0 {
		t.Fatalf("unexpected sibling backup: %v", matches)
	}
	// Manifest records exactly one backup for component=nvim.
	p := install.NewPathsAt(c.GuyideDir)
	backups, err := install.ListBackups(p)
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 manifest backup, got %d", len(backups))
	}
	if backups[0].Component != "nvim" {
		t.Errorf("backup.Component = %q, want nvim", backups[0].Component)
	}
	if backups[0].Reason != "install" {
		t.Errorf("backup.Reason = %q, want install", backups[0].Reason)
	}
	// Tarball file exists on disk under ~/.guyide/backups/.
	if _, err := os.Stat(backups[0].Path); err != nil {
		t.Fatalf("backup tarball missing: %v", err)
	}
	// And precious content is preserved inside it (verified by
	// extracting via tar pipeline).
	if !backupTarballContains(t, backups[0].Path, "home/.config/nvim/user-stuff.txt", "precious") {
		t.Error("backup did not capture user-stuff.txt content")
	}
}

// backupTarballContains opens a gzipped tarball at path and returns
// true iff a file with the given archive-relative name exists with
// matching content.
func backupTarballContains(t *testing.T, path, name, want string) bool {
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
			return false
		}
		if err != nil {
			return false
		}
		if hdr.Name == name {
			buf, _ := io.ReadAll(tr)
			return string(buf) == want
		}
	}
}

func TestCopyTree_SkipsGit(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "out")
	for _, p := range []string{"init.lua", ".git/HEAD", "lua/plugins/x.lua"} {
		full := filepath.Join(src, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "init.lua")); err != nil {
		t.Errorf("init.lua not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "lua/plugins/x.lua")); err != nil {
		t.Errorf("nested file not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Errorf(".git should be skipped, got err=%v", err)
	}
}

func TestDoctor_FailsWhenMissing(t *testing.T) {
	d := &NvimDriver{installDir: "nvim"}
	c := newCtx(t)
	checks := d.Doctor(c)
	if len(checks) == 0 {
		t.Fatal("no checks")
	}
	if checks[0].Status != "fail" {
		t.Errorf("first check status = %q, want fail", checks[0].Status)
	}
}

func TestDoctor_OkWhenInstalledAndLinked(t *testing.T) {
	d := &NvimDriver{installDir: "nvim"}
	c := newCtx(t)
	dst := d.componentPath(c)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "init.lua"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.linkConfig(c, dst); err != nil {
		t.Fatal(err)
	}
	checks := d.Doctor(c)
	// First check (init.lua present) and second (symlink) must be ok.
	if checks[0].Status != "ok" {
		t.Errorf("init.lua check: %+v", checks[0])
	}
	if checks[1].Status != "ok" {
		t.Errorf("symlink check: %+v", checks[1])
	}
}

func TestOwnedPaths(t *testing.T) {
	d := &NvimDriver{installDir: "nvim"}
	c := newCtx(t)
	paths := d.OwnedPaths(c)
	want := []string{d.componentPath(c), d.symlinkTarget(c)}
	if len(paths) != len(want) {
		t.Fatalf("len = %d, want %d", len(paths), len(want))
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := HashFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// sha256("hello")
	const want = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if h != want {
		t.Errorf("hash = %q, want %q", h, want)
	}
}

func TestUninstall_RemovesEverything(t *testing.T) {
	d := &NvimDriver{installDir: "nvim"}
	c := newCtx(t)
	dst := d.componentPath(c)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := d.linkConfig(c, dst); err != nil {
		t.Fatal(err)
	}
	if err := d.Uninstall(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Errorf("component dir not removed: %v", err)
	}
	if _, err := os.Lstat(d.symlinkTarget(c)); !os.IsNotExist(err) {
		t.Errorf("symlink not removed: %v", err)
	}
}
