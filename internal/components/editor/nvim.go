// Package editor holds drivers that fill the editor slot. v0.2 ships
// the nvim driver (NvGuy clone + headless lazy.sync drain + symlink
// to ~/.config/nvim).
package editor

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/guysoft/guyide-cli/internal/components"
	guyembed "github.com/guysoft/guyide-cli/embed"
	"github.com/guysoft/guyide-cli/internal/install"
	"github.com/guysoft/guyide-cli/pkg/schema"
)

func init() {
	components.Register(components.SlotEditor, "nvim", func() components.Component {
		return &NvimDriver{
			repo:       "https://github.com/guysoft/NvGuy.git",
			devEnvVar:  "GUYIDE_DEV_NVGUY",
			installDir: "nvim",
		}
	})
}

// NvimDriver installs NvGuy under ~/.guyide/components/nvim and
// symlinks ~/.config/nvim -> there.
type NvimDriver struct {
	repo       string
	devEnvVar  string
	installDir string // dir name under ~/.guyide/components/
}

// Compile-time interface check.
var _ components.Component = (*NvimDriver)(nil)

func (d *NvimDriver) Name() string              { return "nvim (NvGuy)" }
func (d *NvimDriver) Slot() components.Slot     { return components.SlotEditor }
func (d *NvimDriver) Driver() string            { return "nvim" }

// resolveSource returns either the local override dir or the git URL.
func (d *NvimDriver) resolveSource() (source string, isLocal bool) {
	if local := os.Getenv(d.devEnvVar); local != "" {
		return local, true
	}
	return d.repo, false
}

// resolveRef returns the git ref to install based on channel +
// embedded compat matrix.
func (d *NvimDriver) resolveRef(c *components.Context) (string, error) {
	if c.UserRef != "" {
		return c.UserRef, nil
	}
	if c.Channel == "dev" {
		return "main", nil
	}
	// Stable: look up embedded compat.json.
	cm, err := install.LoadCompat()
	if err != nil {
		return "", fmt.Errorf("load compat: %w", err)
	}
	cliVersion := guyembed.CLIVersion()
	if pin, ok := cm.PinFor(cliVersion, "nvguy"); ok {
		return pin, nil
	}
	// No pin for this CLI version: fall back to "main" but warn.
	return "main", nil
}

func (d *NvimDriver) componentPath(c *components.Context) string {
	p := install.NewPathsAt(c.GuyideDir)
	return p.Component(d.installDir)
}

func (d *NvimDriver) symlinkTarget(c *components.Context) string {
	return filepath.Join(c.HomeDir, ".config", "nvim")
}

func (d *NvimDriver) Plan(c *components.Context) (components.Plan, error) {
	source, isLocal := d.resolveSource()
	ref, err := d.resolveRef(c)
	if err != nil {
		return components.Plan{}, err
	}

	plan := components.Plan{
		Component: d.Name(),
		Slot:      d.Slot(),
		Driver:    d.Driver(),
		Ref:       ref,
		Source:    source,
	}

	dst := d.componentPath(c)
	link := d.symlinkTarget(c)

	if _, err := os.Stat(dst); errors.Is(err, fs.ErrNotExist) {
		plan.Status = components.StatusInstall
		if isLocal {
			plan.Actions = append(plan.Actions,
				fmt.Sprintf("copy NvGuy from %s to %s", source, dst))
		} else {
			plan.Actions = append(plan.Actions,
				fmt.Sprintf("git clone --depth 1 --branch %s %s %s", ref, source, dst))
		}
	} else {
		plan.Status = components.StatusUpdate
		plan.Actions = append(plan.Actions,
			fmt.Sprintf("git fetch + checkout %s in %s", ref, dst))
	}

	if existing, err := os.Lstat(link); err == nil {
		if existing.Mode()&fs.ModeSymlink != 0 {
			cur, _ := os.Readlink(link)
			if cur != dst {
				plan.Actions = append(plan.Actions,
					fmt.Sprintf("repoint %s -> %s (was %s)", link, dst, cur))
			}
		} else {
			plan.Notes = append(plan.Notes,
				fmt.Sprintf("%s exists and is not a symlink; will be backed up before replacement", link))
		}
	} else {
		plan.Actions = append(plan.Actions,
			fmt.Sprintf("symlink %s -> %s", link, dst))
	}

	plan.Actions = append(plan.Actions,
		"run nvim --headless lazy.sync (drain loop, up to 20 iterations)",
	)
	return plan, nil
}

func (d *NvimDriver) Install(c *components.Context) error {
	dst := d.componentPath(c)
	source, isLocal := d.resolveSource()
	ref, err := d.resolveRef(c)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dst); errors.Is(err, fs.ErrNotExist) {
		if c.DryRun {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if isLocal {
			if err := copyTree(source, dst); err != nil {
				return fmt.Errorf("copy %s -> %s: %w", source, dst, err)
			}
		} else {
			if err := runCmd("git", "clone", "--depth", "1", "--branch", ref, source, dst); err != nil {
				return fmt.Errorf("git clone: %w", err)
			}
		}
	} else {
		// Already cloned: bring it up to ref.
		if !isLocal && !c.DryRun {
			if err := runCmd("git", "-C", dst, "fetch", "--depth", "1", "origin", ref); err != nil {
				return fmt.Errorf("git fetch: %w", err)
			}
			if err := runCmd("git", "-C", dst, "checkout", ref); err != nil {
				return fmt.Errorf("git checkout: %w", err)
			}
		}
	}

	// Symlink ~/.config/nvim -> dst, backing up any non-symlink
	// already there.
	if err := d.linkConfig(c, dst); err != nil {
		return err
	}

	if c.DryRun {
		return nil
	}
	if err := d.lazySyncDrain(c, dst); err != nil {
		return fmt.Errorf("lazy sync: %w", err)
	}
	return nil
}

func (d *NvimDriver) Update(c *components.Context) error { return d.Install(c) }

func (d *NvimDriver) Uninstall(c *components.Context) error {
	if c.DryRun {
		return nil
	}
	link := d.symlinkTarget(c)
	if fi, err := os.Lstat(link); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
		_ = os.Remove(link)
	}
	dst := d.componentPath(c)
	return os.RemoveAll(dst)
}

func (d *NvimDriver) Doctor(c *components.Context) []schema.DoctorCheck {
	checks := []schema.DoctorCheck{}
	dst := d.componentPath(c)
	if _, err := os.Stat(filepath.Join(dst, "init.lua")); err == nil {
		checks = append(checks, schema.DoctorCheck{
			Group: "editor", Name: "nvim init.lua present", Status: "ok",
			Message: dst,
		})
	} else {
		checks = append(checks, schema.DoctorCheck{
			Group: "editor", Name: "nvim init.lua present", Status: "fail",
			Message: "missing " + filepath.Join(dst, "init.lua"),
		})
		return checks
	}

	link := d.symlinkTarget(c)
	if fi, err := os.Lstat(link); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
		t, _ := os.Readlink(link)
		if t == dst {
			checks = append(checks, schema.DoctorCheck{
				Group: "editor", Name: "~/.config/nvim symlink", Status: "ok",
				Message: link + " -> " + dst,
			})
		} else {
			checks = append(checks, schema.DoctorCheck{
				Group: "editor", Name: "~/.config/nvim symlink", Status: "warn",
				Message: link + " points to " + t,
			})
		}
	} else {
		checks = append(checks, schema.DoctorCheck{
			Group: "editor", Name: "~/.config/nvim symlink", Status: "fail",
			Message: link + " missing or not a symlink",
		})
	}

	if _, err := exec.LookPath("nvim"); err != nil {
		checks = append(checks, schema.DoctorCheck{
			Group: "editor", Name: "nvim on PATH", Status: "fail",
			Message: err.Error(),
		})
	} else {
		checks = append(checks, schema.DoctorCheck{
			Group: "editor", Name: "nvim on PATH", Status: "ok",
		})
	}
	return checks
}

func (d *NvimDriver) OwnedPaths(c *components.Context) []string {
	return []string{
		d.componentPath(c),
		d.symlinkTarget(c),
	}
}

// linkConfig replaces ~/.config/nvim with a symlink to dst, backing up
// anything previously there into the install backup tarball.
func (d *NvimDriver) linkConfig(c *components.Context, dst string) error {
	link := d.symlinkTarget(c)
	if c.DryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if existing, err := os.Lstat(link); err == nil {
		if existing.Mode()&fs.ModeSymlink != 0 {
			cur, _ := os.Readlink(link)
			if cur == dst {
				return nil // already correct
			}
			_ = os.Remove(link)
		} else {
			// Not a symlink: stash the real dir into a tar.gz under
			// ~/.guyide/backups/ and record it in the manifest, then
			// remove the original. This is the durable, manifest-
			// tracked backup mechanism — no scattered sibling dirs.
			if err := d.backupAndRemove(c, link, "install"); err != nil {
				return fmt.Errorf("backup %s: %w", link, err)
			}
		}
	}
	return os.Symlink(dst, link)
}

// backupAndRemove creates a tar.gz backup of path under
// ~/.guyide/backups/, appends a manifest entry, then removes the
// original. Caller must have confirmed path exists.
func (d *NvimDriver) backupAndRemove(c *components.Context, path, reason string) error {
	p := install.NewPathsAt(c.GuyideDir)
	entry, err := install.CreateBackup(p, install.BackupRequest{
		Paths:     []string{path},
		Reason:    reason,
		Component: "nvim",
		HomeDir:   c.HomeDir,
	})
	if err != nil {
		return err
	}
	if _, err := install.AppendBackup(p, entry); err != nil {
		return err
	}
	return os.RemoveAll(path)
}

// lazySyncDrain runs the same drain loop the e2e harness uses, ported
// to a single embedded Lua chunk we hand to `nvim --headless`.
func (d *NvimDriver) lazySyncDrain(c *components.Context, dst string) error {
	// Strip the dofile(base46_cache) lines so the bootstrap doesn't
	// crash on a cold install (NvGuy's init.lua dofiles the cache
	// before NvChad has had a chance to write it).
	initLua, err := os.ReadFile(filepath.Join(dst, "init.lua"))
	if err != nil {
		return fmt.Errorf("read init.lua: %w", err)
	}
	var bootstrap strings.Builder
	for _, line := range strings.Split(string(initLua), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "dofile(vim.g.base46_cache") {
			continue
		}
		bootstrap.WriteString(line)
		bootstrap.WriteByte('\n')
	}
	bootstrap.WriteString(lazySyncTail)

	tmp, err := os.CreateTemp("", "guyide-bootstrap-*.lua")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(bootstrap.String()); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	cmd := exec.Command("nvim", "--headless", "-u", tmp.Name(), "+qall!")
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "TMUX=") // ensure no surprise nesting
	return cmd.Run()
}

const lazySyncTail = `
-- guyide installer: lazy.sync drain loop --
local ok, lazy = pcall(require, "lazy")
if not ok then
  io.stderr:write("guyide: lazy.nvim not loadable yet, skipping sync\n")
  return
end
lazy.sync({ wait = true, show = false })
for _ = 1, 20 do
  local pending = false
  for _, p in ipairs(lazy.plugins()) do
    if p._.installed == false then pending = true break end
  end
  if not pending then break end
  lazy.install({ wait = true, show = false })
end
local missing = {}
for _, p in ipairs(lazy.plugins()) do
  if p._.installed == false then table.insert(missing, p.name) end
end
if #missing > 0 then
  error("guyide: plugins still not installed after sync: " .. table.concat(missing, ", "))
end
pcall(function() require("base46").load_all_highlights() end)
`

// runCmd executes a command attaching its stderr/stdout to ours.
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// copyTree recursively copies src to dst preserving symlinks.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		// Skip .git to avoid bloating ~/.guyide; the dev workflow
		// uses a local checkout where keeping the source's .git is
		// pointless and slows backups.
		if rel == ".git" {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		fi, err := os.Lstat(p)
		if err != nil {
			return err
		}
		switch {
		case fi.IsDir():
			return os.MkdirAll(target, 0o755)
		case fi.Mode()&fs.ModeSymlink != 0:
			t, err := os.Readlink(p)
			if err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(t, target)
		default:
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			defer in.Close()
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fi.Mode().Perm())
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, in)
			return err
		}
	})
}

// hashFile is exposed for the install manager when populating
// ComponentEntry.Hashes.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashFile is exported for tests/diagnostics.
func HashFile(path string) (string, error) { return hashFile(path) }
