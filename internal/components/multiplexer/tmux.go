// Package multiplexer holds drivers for the multiplexer slot. v0.2
// ships only the tmux driver.
package multiplexer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	guyembed "github.com/guysoft/guyide-cli/embed"
	"github.com/guysoft/guyide-cli/internal/components"
	"github.com/guysoft/guyide-cli/internal/install"
	"github.com/guysoft/guyide-cli/pkg/schema"
)

func init() {
	components.Register(components.SlotMultiplexer, "tmux", func() components.Component {
		return &TmuxDriver{}
	})
}

// TmuxDriver owns the user's tmux configuration. v0.2 behaviour:
//   - writes ~/.guyide/components/tmux/guyide.conf from embedded asset
//   - if user_config.tmux.own_conf is true (default), copies that file
//     to ~/.tmux.conf, backing up any pre-existing non-managed file
//   - on update reloads any running tmux server via `tmux source-file`
//     (never kills the server)
//   - drift detection: stores sha256 of the materialised guyide.conf
//     in the manifest; doctor warns if hashes diverge
type TmuxDriver struct{}

var _ components.Component = (*TmuxDriver)(nil)

func (d *TmuxDriver) Name() string          { return "tmux" }
func (d *TmuxDriver) Slot() components.Slot { return components.SlotMultiplexer }
func (d *TmuxDriver) Driver() string        { return "tmux" }

func (d *TmuxDriver) componentDir(c *components.Context) string {
	return install.NewPathsAt(c.GuyideDir).Component("tmux")
}

func (d *TmuxDriver) managedConfPath(c *components.Context) string {
	return filepath.Join(d.componentDir(c), "guyide.conf")
}

func (d *TmuxDriver) userConfPath(c *components.Context) string {
	return filepath.Join(c.HomeDir, ".tmux.conf")
}

// ownConf reports whether the driver should overwrite ~/.tmux.conf.
// Default is true; opt-out via UserConfig.Tmux.OwnConf=false.
func (d *TmuxDriver) ownConf(c *components.Context) bool {
	if c.UserConfig == nil {
		return true
	}
	return c.UserConfig.Tmux.OwnConf
}

// reloadOnInstall mirrors UserConfig.Tmux.ReloadOnInstall (default true).
func (d *TmuxDriver) reloadOnInstall(c *components.Context) bool {
	if c.UserConfig == nil {
		return true
	}
	return c.UserConfig.Tmux.ReloadOnInstall
}

func (d *TmuxDriver) Plan(c *components.Context) (components.Plan, error) {
	plan := components.Plan{
		Component: d.Name(),
		Slot:      d.Slot(),
		Driver:    d.Driver(),
		Source:    "embedded:tmux/guyide.conf",
		Ref:       guyembed.CLIVersion(),
	}

	managed := d.managedConfPath(c)
	embedded := guyembed.TmuxGuyideConf()
	if cur, err := os.ReadFile(managed); err == nil && bytes.Equal(cur, embedded) {
		plan.Status = components.StatusUnchanged
	} else if errors.Is(err, fs.ErrNotExist) {
		plan.Status = components.StatusInstall
		plan.Actions = append(plan.Actions, "write "+managed)
	} else {
		plan.Status = components.StatusUpdate
		plan.Actions = append(plan.Actions, "rewrite "+managed)
	}

	user := d.userConfPath(c)
	if d.ownConf(c) {
		switch managed, err := isManagedTmuxConf(user); {
		case errors.Is(err, fs.ErrNotExist):
			plan.Actions = append(plan.Actions, "write "+user)
		case err != nil:
			plan.Notes = append(plan.Notes, fmt.Sprintf("could not read %s: %v", user, err))
		case !managed:
			plan.Notes = append(plan.Notes,
				fmt.Sprintf("%s exists and is not guyide-managed; will be backed up before replacement", user))
			plan.Actions = append(plan.Actions, "replace "+user+" (after backup)")
		case managed:
			plan.Actions = append(plan.Actions, "refresh "+user)
		}
	} else {
		plan.Notes = append(plan.Notes,
			fmt.Sprintf("tmux.own_conf=false; %s left untouched. Add `source-file %s` yourself.", user, managed))
	}

	if d.reloadOnInstall(c) && tmuxRunning() {
		plan.Actions = append(plan.Actions, "reload running tmux servers via source-file")
	}
	return plan, nil
}

func (d *TmuxDriver) Install(c *components.Context) error {
	if c.DryRun {
		return nil
	}
	if err := os.MkdirAll(d.componentDir(c), 0o755); err != nil {
		return err
	}
	managed := d.managedConfPath(c)
	if err := os.WriteFile(managed, guyembed.TmuxGuyideConf(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", managed, err)
	}

	if d.ownConf(c) {
		if err := d.placeUserConf(c, managed); err != nil {
			return err
		}
	}

	if d.reloadOnInstall(c) && tmuxRunning() {
		// Best-effort reload: a missing tmux binary or no servers
		// running shouldn't fail the install.
		_ = exec.Command("tmux", "source-file", d.userConfPath(c)).Run()
	}
	return nil
}

func (d *TmuxDriver) Update(c *components.Context) error { return d.Install(c) }

func (d *TmuxDriver) Uninstall(c *components.Context) error {
	if c.DryRun {
		return nil
	}
	user := d.userConfPath(c)
	if managed, err := isManagedTmuxConf(user); err == nil && managed {
		// Only remove if we own it.
		_ = os.Remove(user)
	}
	return os.RemoveAll(d.componentDir(c))
}

func (d *TmuxDriver) Doctor(c *components.Context) []schema.DoctorCheck {
	checks := []schema.DoctorCheck{}
	managed := d.managedConfPath(c)
	cur, err := os.ReadFile(managed)
	if err != nil {
		checks = append(checks, schema.DoctorCheck{
			Group: "multiplexer", Name: "guyide.conf present", Status: "fail",
			Message: err.Error(),
		})
		return checks
	}
	if bytes.Equal(cur, guyembed.TmuxGuyideConf()) {
		checks = append(checks, schema.DoctorCheck{
			Group: "multiplexer", Name: "guyide.conf matches embedded", Status: "ok",
			Message: managed,
		})
	} else {
		checks = append(checks, schema.DoctorCheck{
			Group: "multiplexer", Name: "guyide.conf matches embedded", Status: "warn",
			Message: "drift detected; run `guyide update` to refresh",
		})
	}

	user := d.userConfPath(c)
	if d.ownConf(c) {
		if m, err := isManagedTmuxConf(user); err != nil {
			checks = append(checks, schema.DoctorCheck{
				Group: "multiplexer", Name: "~/.tmux.conf is guyide-managed", Status: "fail",
				Message: user + ": " + err.Error(),
			})
		} else if !m {
			checks = append(checks, schema.DoctorCheck{
				Group: "multiplexer", Name: "~/.tmux.conf is guyide-managed", Status: "warn",
				Message: user + " is not managed; run `guyide update`",
			})
		} else {
			// Compare bodies too.
			ub, _ := os.ReadFile(user)
			if bytes.Equal(ub, cur) {
				checks = append(checks, schema.DoctorCheck{
					Group: "multiplexer", Name: "~/.tmux.conf is guyide-managed", Status: "ok",
				})
			} else {
				checks = append(checks, schema.DoctorCheck{
					Group: "multiplexer", Name: "~/.tmux.conf is guyide-managed", Status: "warn",
					Message: "managed marker present but body differs; run `guyide update`",
				})
			}
		}
	}

	if _, err := exec.LookPath("tmux"); err != nil {
		checks = append(checks, schema.DoctorCheck{
			Group: "multiplexer", Name: "tmux on PATH", Status: "fail",
			Message: err.Error(),
		})
	} else {
		checks = append(checks, schema.DoctorCheck{
			Group: "multiplexer", Name: "tmux on PATH", Status: "ok",
		})
	}
	return checks
}

func (d *TmuxDriver) OwnedPaths(c *components.Context) []string {
	out := []string{d.componentDir(c)}
	if d.ownConf(c) {
		out = append(out, d.userConfPath(c))
	}
	return out
}

// placeUserConf writes embedded conf to ~/.tmux.conf, backing up any
// pre-existing non-managed file into ~/.guyide/backups/ as a tar.gz
// with a manifest entry. The original is removed before the new
// managed conf is written.
func (d *TmuxDriver) placeUserConf(c *components.Context, managedPath string) error {
	user := d.userConfPath(c)
	if existing, err := isManagedTmuxConf(user); err == nil && !existing {
		// Non-managed real file: durable backup into the install dir.
		p := install.NewPathsAt(c.GuyideDir)
		entry, err := install.CreateBackup(p, install.BackupRequest{
			Paths:     []string{user},
			Reason:    "install",
			Component: "tmux",
			HomeDir:   c.HomeDir,
		})
		if err != nil {
			return fmt.Errorf("backup %s: %w", user, err)
		}
		if _, err := install.AppendBackup(p, entry); err != nil {
			return fmt.Errorf("record backup %s: %w", user, err)
		}
		if err := os.Remove(user); err != nil {
			return fmt.Errorf("remove %s after backup: %w", user, err)
		}
	}
	body, err := os.ReadFile(managedPath)
	if err != nil {
		return err
	}
	return os.WriteFile(user, body, 0o644)
}

// isManagedTmuxConf returns true if the file exists and contains the
// guyide:managed marker within its first 5 lines. Returns
// fs.ErrNotExist if the file is missing.
func isManagedTmuxConf(path string) (bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.SplitN(string(b), "\n", 6)
	for i := 0; i < len(lines) && i < 5; i++ {
		if strings.TrimSpace(lines[i]) == guyembed.TmuxManagedMarker {
			return true, nil
		}
	}
	return false, nil
}

// tmuxRunning returns true if at least one tmux server is responding
// on the user's default socket.
func tmuxRunning() bool {
	if _, err := exec.LookPath("tmux"); err != nil {
		return false
	}
	cmd := exec.Command("tmux", "list-sessions")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// HashConf returns the sha256 of the currently-shipped tmux config.
// Used by the install manager to populate ComponentEntry.Hashes.
func HashConf() string {
	h := sha256.Sum256(guyembed.TmuxGuyideConf())
	return hex.EncodeToString(h[:])
}
