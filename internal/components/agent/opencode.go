// Package agent provides agent-slot drivers (opencode, future claude-code).
package agent

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	guyembed "github.com/guysoft/guyide-cli/embed"
	"github.com/guysoft/guyide-cli/internal/components"
	"github.com/guysoft/guyide-cli/internal/install"
	"github.com/guysoft/guyide-cli/pkg/schema"
)

// OpenCodeInstallURL points at the canonical install instructions
// referenced when opencode is missing from PATH.
const OpenCodeInstallURL = "https://opencode.ai/docs"

// OpenCodeDriver installs guyide-managed opencode skills into
// $XDG_CONFIG_HOME/opencode/skills/<name>/. It deliberately never
// touches AGENTS.md, CLAUDE.md, or skills the user wrote themselves.
type OpenCodeDriver struct{}

// NewOpenCode constructs a driver instance. Used by the registry
// factory and exposed for tests that bypass the registry.
func NewOpenCode() *OpenCodeDriver { return &OpenCodeDriver{} }

func init() {
	components.Register(components.SlotAgent, "opencode", func() components.Component {
		return NewOpenCode()
	})
}

// Name returns the user-visible label.
func (d *OpenCodeDriver) Name() string { return "opencode" }

// Slot returns the agent slot.
func (d *OpenCodeDriver) Slot() components.Slot { return components.SlotAgent }

// Driver returns the registered driver id.
func (d *OpenCodeDriver) Driver() string { return "opencode" }

// configDir resolves $XDG_CONFIG_HOME/opencode, falling back to
// $HOME/.config/opencode. It honours c.HomeDir for testability.
func (d *OpenCodeDriver) configDir(c *components.Context) string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "opencode")
	}
	return filepath.Join(c.HomeDir, ".config", "opencode")
}

// skillsRoot returns the parent directory containing every opencode skill.
func (d *OpenCodeDriver) skillsRoot(c *components.Context) string {
	return filepath.Join(d.configDir(c), "skills")
}

// shippedSkills returns the names of skill subdirectories embedded in the
// binary. Stable, sorted order so plans/tests are deterministic.
func shippedSkills() ([]string, error) {
	entries, err := fs.ReadDir(guyembed.OpenCodeSkillsFS(), ".")
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// opencodeOnPath reports whether the opencode binary is reachable.
func opencodeOnPath() bool {
	_, err := exec.LookPath("opencode")
	return err == nil
}

// Plan describes what Install/Update would do.
func (d *OpenCodeDriver) Plan(c *components.Context) (components.Plan, error) {
	plan := components.Plan{
		Component: "opencode",
		Slot:      components.SlotAgent,
		Driver:    "opencode",
		Status:    components.StatusUnchanged,
		Source:    "embedded",
	}

	if !opencodeOnPath() {
		plan.Status = components.StatusError
		plan.Notes = append(plan.Notes,
			"opencode not found on PATH; install it from "+OpenCodeInstallURL+" then re-run")
		return plan, nil
	}

	skills, err := shippedSkills()
	if err != nil {
		return plan, fmt.Errorf("enumerate shipped skills: %w", err)
	}

	root := d.skillsRoot(c)
	anyChange := false
	for _, name := range skills {
		dst := filepath.Join(root, name)
		state, err := skillState(dst)
		if err != nil {
			return plan, err
		}
		switch state {
		case skillAbsent:
			plan.Actions = append(plan.Actions, "install skill "+name+" \u2192 "+dst)
			anyChange = true
		case skillManagedDrift:
			plan.Actions = append(plan.Actions, "update managed skill "+name+" \u2192 "+dst)
			anyChange = true
		case skillManagedClean:
			// no-op
		case skillForeign:
			plan.Notes = append(plan.Notes,
				dst+" exists and is not guyide-managed; will be backed up before replacement")
			plan.Actions = append(plan.Actions,
				"backup foreign skill "+name+" then install managed copy")
			anyChange = true
		}
	}

	switch {
	case !anyChange:
		plan.Status = components.StatusUnchanged
	case dirExists(root):
		plan.Status = components.StatusUpdate
	default:
		plan.Status = components.StatusInstall
	}
	return plan, nil
}

// Install lays down all guyide-managed skills.
func (d *OpenCodeDriver) Install(c *components.Context) error {
	if !opencodeOnPath() {
		return fmt.Errorf("opencode not on PATH; install from %s", OpenCodeInstallURL)
	}
	if c.DryRun {
		return nil
	}
	skills, err := shippedSkills()
	if err != nil {
		return err
	}
	root := d.skillsRoot(c)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	for _, name := range skills {
		if err := d.installSkill(c, name); err != nil {
			return fmt.Errorf("install skill %s: %w", name, err)
		}
	}
	return nil
}

// Update is identical to Install — re-extracting embedded bytes.
func (d *OpenCodeDriver) Update(c *components.Context) error { return d.Install(c) }

// Uninstall removes only those skill dirs that bear our marker.
// AGENTS.md, CLAUDE.md, and user-authored skills are left alone.
func (d *OpenCodeDriver) Uninstall(c *components.Context) error {
	if c.DryRun {
		return nil
	}
	skills, err := shippedSkills()
	if err != nil {
		return err
	}
	root := d.skillsRoot(c)
	for _, name := range skills {
		dst := filepath.Join(root, name)
		state, err := skillState(dst)
		if err != nil {
			return err
		}
		if state == skillManagedClean || state == skillManagedDrift {
			if err := os.RemoveAll(dst); err != nil {
				return err
			}
		}
	}
	return nil
}

// Doctor performs a single PATH check by design — opencode itself
// installs and updates outside guyide's purview.
func (d *OpenCodeDriver) Doctor(c *components.Context) []schema.DoctorCheck {
	if opencodeOnPath() {
		return []schema.DoctorCheck{{
			Group: "agent", Name: "opencode on PATH", Status: "ok",
		}}
	}
	return []schema.DoctorCheck{{
		Group: "agent", Name: "opencode on PATH", Status: "fail",
		Message: "install from " + OpenCodeInstallURL,
	}}
}

// OwnedPaths returns each managed skill directory.
func (d *OpenCodeDriver) OwnedPaths(c *components.Context) []string {
	skills, err := shippedSkills()
	if err != nil {
		return nil
	}
	root := d.skillsRoot(c)
	out := make([]string, 0, len(skills))
	for _, name := range skills {
		out = append(out, filepath.Join(root, name))
	}
	return out
}

// installSkill places a single skill, backing up a foreign predecessor.
func (d *OpenCodeDriver) installSkill(c *components.Context, name string) error {
	dst := filepath.Join(d.skillsRoot(c), name)
	state, err := skillState(dst)
	if err != nil {
		return err
	}
	if state == skillForeign {
		p := install.NewPathsAt(c.GuyideDir)
		entry, err := install.CreateBackup(p, install.BackupRequest{
			Paths:     []string{dst},
			Reason:    "install",
			Component: "opencode",
			HomeDir:   c.HomeDir,
		})
		if err != nil {
			return fmt.Errorf("backup %s: %w", dst, err)
		}
		if _, err := install.AppendBackup(p, entry); err != nil {
			return fmt.Errorf("record backup %s: %w", dst, err)
		}
		if err := os.RemoveAll(dst); err != nil {
			return fmt.Errorf("remove foreign %s: %w", dst, err)
		}
	}
	if state == skillManagedClean || state == skillManagedDrift {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}
	if err := extractSkill(name, dst); err != nil {
		return err
	}
	return writeMarker(dst)
}

// extractSkill copies an embedded skill subtree to dst.
func extractSkill(name, dst string) error {
	src := guyembed.OpenCodeSkillsFS()
	return fs.WalkDir(src, name, func(path string, dirent fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := strings.TrimPrefix(path, name)
		rel = strings.TrimPrefix(rel, "/")
		target := filepath.Join(dst, rel)
		if dirent.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o644)
	})
}

// writeMarker drops the guyide-managed marker into a skill directory.
func writeMarker(skillDir string) error {
	body := []byte(guyembed.CLIVersion() + "\n")
	return os.WriteFile(filepath.Join(skillDir, guyembed.OpenCodeManagedMarker), body, 0o644)
}

type skillStateValue int

const (
	skillAbsent skillStateValue = iota
	skillManagedClean
	skillManagedDrift
	skillForeign
)

// skillState classifies what's currently at the skill destination.
func skillState(dst string) (skillStateValue, error) {
	st, err := os.Stat(dst)
	if errors.Is(err, fs.ErrNotExist) {
		return skillAbsent, nil
	}
	if err != nil {
		return skillAbsent, err
	}
	if !st.IsDir() {
		return skillForeign, nil
	}
	if _, err := os.Stat(filepath.Join(dst, guyembed.OpenCodeManagedMarker)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return skillForeign, nil
		}
		return skillAbsent, err
	}
	// Marker present — compare bytes.
	name := filepath.Base(dst)
	if drifted, err := managedSkillDrifted(name, dst); err != nil {
		return skillAbsent, err
	} else if drifted {
		return skillManagedDrift, nil
	}
	return skillManagedClean, nil
}

// managedSkillDrifted compares every embedded file under <name>/ to the
// installed copy. Returns true on any mismatch or missing file.
func managedSkillDrifted(name, dst string) (bool, error) {
	src := guyembed.OpenCodeSkillsFS()
	mismatch := false
	err := fs.WalkDir(src, name, func(path string, dirent fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if dirent.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, name)
		rel = strings.TrimPrefix(rel, "/")
		want, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				mismatch = true
				return fs.SkipAll
			}
			return err
		}
		if string(want) != string(got) {
			mismatch = true
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return mismatch, nil
}

// dirExists is a small helper for Plan's status decision.
func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// Compile-time interface satisfaction.
var _ components.Component = (*OpenCodeDriver)(nil)
