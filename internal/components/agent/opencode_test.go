package agent

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	guyembed "github.com/guysoft/guyide-cli/embed"
	"github.com/guysoft/guyide-cli/internal/components"
	"github.com/guysoft/guyide-cli/internal/install"
	"github.com/guysoft/guyide-cli/pkg/schema"
)

// newCtx builds an isolated context with a fake $HOME, a fresh
// ~/.guyide dir, and XDG_CONFIG_HOME pointed at a tempdir so tests
// never touch the developer's real opencode config.
func newCtx(t *testing.T) *components.Context {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	guy := filepath.Join(home, ".guyide")
	xdg := filepath.Join(home, ".config")
	if err := os.MkdirAll(guy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(xdg, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)
	cfg := schema.DefaultUserConfig()
	return &components.Context{
		HomeDir:    home,
		GuyideDir:  guy,
		Channel:    "stable",
		UserConfig: &cfg,
	}
}

// withFakeOpencodeOnPath places a no-op `opencode` script into a
// tempdir and prepends it to PATH so opencodeOnPath() returns true.
func withFakeOpencodeOnPath(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("PATH shim assumes POSIX shell")
	}
	bin := t.TempDir()
	script := filepath.Join(bin, "opencode")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
}

// withoutOpencodeOnPath strips PATH down to a directory that does not
// contain an opencode binary. We can't trust the developer's machine
// not to have one already installed.
func withoutOpencodeOnPath(t *testing.T) {
	t.Helper()
	empty := t.TempDir()
	t.Setenv("PATH", empty)
}

// --- registration ----------------------------------------------------

func TestOpenCodeDriverRegistered(t *testing.T) {
	c, err := components.Get(components.SlotAgent, "opencode")
	if err != nil {
		t.Fatalf("opencode not registered: %v", err)
	}
	if c.Name() != "opencode" || c.Driver() != "opencode" {
		t.Fatalf("unexpected names: %s / %s", c.Name(), c.Driver())
	}
	if c.Slot() != components.SlotAgent {
		t.Fatalf("slot = %s, want %s", c.Slot(), components.SlotAgent)
	}
}

// --- shipped skills metadata ----------------------------------------

func TestShippedSkillsAtLeastGuyide(t *testing.T) {
	names, err := shippedSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("expected at least one shipped skill")
	}
	found := false
	for _, n := range names {
		if n == "guyide" {
			found = true
		}
	}
	if !found {
		t.Fatalf("guyide skill missing from shipped list: %v", names)
	}
}

// --- Plan ------------------------------------------------------------

func TestPlan_OpencodeMissingOnPath(t *testing.T) {
	withoutOpencodeOnPath(t)
	ctx := newCtx(t)
	d := NewOpenCode()

	plan, err := d.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != components.StatusError {
		t.Fatalf("status = %s, want %s", plan.Status, components.StatusError)
	}
	if len(plan.Notes) == 0 || !strings.Contains(plan.Notes[0], "opencode not found on PATH") {
		t.Fatalf("expected install note, got %v", plan.Notes)
	}
}

func TestPlan_FreshInstall(t *testing.T) {
	withFakeOpencodeOnPath(t)
	ctx := newCtx(t)
	d := NewOpenCode()

	plan, err := d.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != components.StatusInstall {
		t.Fatalf("status = %s, want %s", plan.Status, components.StatusInstall)
	}
	if !containsActionPrefix(plan.Actions, "install skill guyide") {
		t.Fatalf("expected install action for guyide, got %v", plan.Actions)
	}
}

func TestPlan_UnchangedAfterInstall(t *testing.T) {
	withFakeOpencodeOnPath(t)
	ctx := newCtx(t)
	d := NewOpenCode()

	if err := d.Install(ctx); err != nil {
		t.Fatal(err)
	}
	plan, err := d.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != components.StatusUnchanged {
		t.Fatalf("status = %s, want %s", plan.Status, components.StatusUnchanged)
	}
	if len(plan.Actions) != 0 {
		t.Fatalf("expected no actions after clean install, got %v", plan.Actions)
	}
}

func TestPlan_DriftOnManagedSkill(t *testing.T) {
	withFakeOpencodeOnPath(t)
	ctx := newCtx(t)
	d := NewOpenCode()

	if err := d.Install(ctx); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(d.skillsRoot(ctx), "guyide")
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("hand-edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := d.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != components.StatusUpdate {
		t.Fatalf("status = %s, want %s", plan.Status, components.StatusUpdate)
	}
	if !containsActionPrefix(plan.Actions, "update managed skill guyide") {
		t.Fatalf("expected update action, got %v", plan.Actions)
	}
}

func TestPlan_ForeignSkillWarns(t *testing.T) {
	withFakeOpencodeOnPath(t)
	ctx := newCtx(t)
	d := NewOpenCode()

	dst := filepath.Join(d.skillsRoot(ctx), "guyide")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "SKILL.md"), []byte("user wrote this\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := d.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != components.StatusUpdate {
		t.Fatalf("status = %s, want %s", plan.Status, components.StatusUpdate)
	}
	if !containsNoteContaining(plan.Notes, "not guyide-managed") {
		t.Fatalf("expected foreign-warn note, got %v", plan.Notes)
	}
	if !containsActionPrefix(plan.Actions, "backup foreign skill") {
		t.Fatalf("expected backup action, got %v", plan.Actions)
	}
}

// --- Install --------------------------------------------------------

func TestInstall_FreshWritesSkillAndMarker(t *testing.T) {
	withFakeOpencodeOnPath(t)
	ctx := newCtx(t)
	d := NewOpenCode()

	if err := d.Install(ctx); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(d.skillsRoot(ctx), "guyide")
	if !dirExists(skillDir) {
		t.Fatalf("skill dir missing: %s", skillDir)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md missing: %v", err)
	}
	marker := filepath.Join(skillDir, guyembed.OpenCodeManagedMarker)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker missing: %v", err)
	}
}

func TestInstall_DryRunNoOp(t *testing.T) {
	withFakeOpencodeOnPath(t)
	ctx := newCtx(t)
	ctx.DryRun = true
	d := NewOpenCode()

	if err := d.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if dirExists(filepath.Join(d.skillsRoot(ctx), "guyide")) {
		t.Fatal("dry-run wrote skill dir")
	}
}

func TestInstall_BacksUpForeignSkill(t *testing.T) {
	withFakeOpencodeOnPath(t)
	ctx := newCtx(t)
	d := NewOpenCode()

	dst := filepath.Join(d.skillsRoot(ctx), "guyide")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	userBody := []byte("# user skill\nprecious\n")
	if err := os.WriteFile(filepath.Join(dst, "SKILL.md"), userBody, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := d.Install(ctx); err != nil {
		t.Fatal(err)
	}

	p := install.NewPathsAt(ctx.GuyideDir)
	entries, err := install.ListBackups(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(entries))
	}
	e := entries[0]
	if e.Component != "opencode" {
		t.Fatalf("component = %s, want opencode", e.Component)
	}
	if e.Reason != "install" {
		t.Fatalf("reason = %s, want install", e.Reason)
	}

	// Tarball stores HOME-relative paths under "home/...".
	relSkill, err := filepath.Rel(ctx.HomeDir, filepath.Join(dst, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	tarMember := filepath.Join("home", relSkill)
	got := readTarballMember(t, e.Path, tarMember)
	if string(got) != string(userBody) {
		t.Fatalf("backed-up SKILL.md = %q, want %q", got, userBody)
	}

	// And the new managed copy is in place.
	marker := filepath.Join(dst, guyembed.OpenCodeManagedMarker)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker missing after foreign-overwrite: %v", err)
	}
}

func TestInstall_FailsWhenOpencodeMissing(t *testing.T) {
	withoutOpencodeOnPath(t)
	ctx := newCtx(t)
	d := NewOpenCode()

	err := d.Install(ctx)
	if err == nil {
		t.Fatal("expected error when opencode missing from PATH")
	}
	if !strings.Contains(err.Error(), "PATH") {
		t.Fatalf("error = %v, expected mention of PATH", err)
	}
}

// --- Uninstall ------------------------------------------------------

func TestUninstall_RemovesOnlyManagedSkills(t *testing.T) {
	withFakeOpencodeOnPath(t)
	ctx := newCtx(t)
	d := NewOpenCode()

	if err := d.Install(ctx); err != nil {
		t.Fatal(err)
	}
	// Plant a sibling "user-skill" dir (NOT in our shipped list).
	userSkill := filepath.Join(d.skillsRoot(ctx), "user-skill")
	if err := os.MkdirAll(userSkill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userSkill, "SKILL.md"), []byte("user\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := d.Uninstall(ctx); err != nil {
		t.Fatal(err)
	}

	if dirExists(filepath.Join(d.skillsRoot(ctx), "guyide")) {
		t.Fatal("guyide skill should be gone after uninstall")
	}
	if !dirExists(userSkill) {
		t.Fatal("user-skill must remain after uninstall")
	}
}

func TestUninstall_LeavesForeignSameNameAlone(t *testing.T) {
	withFakeOpencodeOnPath(t)
	ctx := newCtx(t)
	d := NewOpenCode()

	dst := filepath.Join(d.skillsRoot(ctx), "guyide")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "SKILL.md"), []byte("user\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No marker -> foreign.

	if err := d.Uninstall(ctx); err != nil {
		t.Fatal(err)
	}
	if !dirExists(dst) {
		t.Fatal("foreign same-name skill must NOT be removed")
	}
}

// --- Doctor + OwnedPaths --------------------------------------------

func TestDoctor_OkOnPath(t *testing.T) {
	withFakeOpencodeOnPath(t)
	ctx := newCtx(t)
	d := NewOpenCode()

	checks := d.Doctor(ctx)
	if len(checks) != 1 || checks[0].Status != "ok" {
		t.Fatalf("doctor checks = %#v", checks)
	}
}

func TestDoctor_FailWithoutPath(t *testing.T) {
	withoutOpencodeOnPath(t)
	ctx := newCtx(t)
	d := NewOpenCode()

	checks := d.Doctor(ctx)
	if len(checks) != 1 || checks[0].Status != "fail" {
		t.Fatalf("doctor checks = %#v", checks)
	}
}

func TestOwnedPaths_OneEntryPerSkill(t *testing.T) {
	ctx := newCtx(t)
	d := NewOpenCode()

	paths := d.OwnedPaths(ctx)
	skills, err := shippedSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != len(skills) {
		t.Fatalf("len(OwnedPaths)=%d, len(shipped)=%d", len(paths), len(skills))
	}
}

// --- helpers --------------------------------------------------------

func containsActionPrefix(actions []string, prefix string) bool {
	for _, a := range actions {
		if strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}

func containsNoteContaining(notes []string, sub string) bool {
	for _, n := range notes {
		if strings.Contains(n, sub) {
			return true
		}
	}
	return false
}

func readTarballMember(t *testing.T, archivePath, name string) []byte {
	t.Helper()
	f, err := os.Open(archivePath)
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
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name == name {
			body, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			return body
		}
	}
	t.Fatalf("tarball %s missing member %q", archivePath, name)
	return nil
}
