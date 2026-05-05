package cli

import (
	"errors"
	"fmt"
	"io/fs"

	_ "github.com/guysoft/guyide-cli/internal/components/registry" // register all drivers
	"github.com/guysoft/guyide-cli/internal/install"
	"github.com/guysoft/guyide-cli/internal/output"
	"github.com/spf13/cobra"
)

func newInstallCmd(g *Globals) *cobra.Command {
	var dryRun bool
	var ref string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install or update guyide components (editor, multiplexer, agent)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := writerFor(g)
			report, err := install.RunInstall(install.RunOptions{
				Ctx:     cmd.Context(),
				DryRun:  dryRun,
				UserRef: ref,
			})
			renderRunReport(out, g, "guyide install", report, dryRun)
			return err
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show planned actions without modifying disk")
	cmd.Flags().StringVar(&ref, "ref", "", "override ref (tag/branch/sha) for every component")
	return cmd
}

func newUninstallCmd(g *Globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall guyide-managed components (preserves user-owned files)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := writerFor(g)
			report, err := install.RunUninstall(install.RunOptions{
				Ctx:    cmd.Context(),
				DryRun: dryRun,
			})
			renderRunReport(out, g, "guyide uninstall", report, dryRun)
			return err
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be removed without modifying disk")
	return cmd
}

func newListBackupsCmd(g *Globals) *cobra.Command {
	return &cobra.Command{
		Use:   "list-backups",
		Short: "List backup tarballs recorded in the manifest",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := writerFor(g)
			p, err := install.NewPaths()
			if err != nil {
				return err
			}
			entries, err := install.ListBackups(p)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					if g.JSON {
						out.JSON(map[string]any{"backups": []any{}})
						return nil
					}
					out.Info("no manifest yet — guyide has not been installed")
					return nil
				}
				return err
			}
			if g.JSON {
				out.JSON(map[string]any{"backups": entries})
				return nil
			}
			out.Header("guyide backups")
			if len(entries) == 0 {
				out.Info("no backups recorded")
				return nil
			}
			for _, e := range entries {
				comp := e.Component
				if comp == "" {
					comp = "(installer)"
				}
				out.KeyValue(e.Timestamp.Format("2006-01-02 15:04:05Z"),
					fmt.Sprintf("%s [%s] %s", comp, e.Reason, e.Path))
			}
			return nil
		},
	}
}

// renderRunReport prints a Plan/Install/Uninstall report consistently
// in either human or JSON mode.
func renderRunReport(out output.Writer, g *Globals, title string, report install.RunReport, dryRun bool) {
	if g.JSON {
		out.JSON(map[string]any{
			"title":       title,
			"dry_run":     dryRun,
			"plans":       report.Plans,
			"errors":      reportErrorsToJSON(report.Errors),
			"config_init": report.ConfigInit,
		})
		return
	}
	out.Header(title)
	if report.ConfigInit {
		out.Info("wrote default ~/.guyide/config.yaml")
	}
	for i, p := range report.Plans {
		out.Step(i+1, len(report.Plans), fmt.Sprintf("%s/%s [%s]", p.Slot, p.Driver, p.Status))
		if p.Source != "" {
			out.KeyValue("source", p.Source)
		}
		if p.Ref != "" {
			out.KeyValue("ref", p.Ref)
		}
		for _, n := range p.Notes {
			out.Warning("%s", n)
		}
		for _, a := range p.Actions {
			if dryRun {
				out.DryRun("would: %s", a)
			} else {
				out.Info("→ %s", a)
			}
		}
	}
	for _, e := range report.Errors {
		out.Error("%s/%s: %v", e.Slot, e.Driver, e.Err)
	}
	mode := "live"
	if dryRun {
		mode = "dry-run"
	}
	out.Summary("Summary", map[string]string{
		"Mode":       mode,
		"Components": fmt.Sprintf("%d", len(report.Plans)),
		"Errors":     fmt.Sprintf("%d", len(report.Errors)),
	})
}

func reportErrorsToJSON(errs []install.DriverError) []map[string]string {
	out := make([]map[string]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, map[string]string{
			"slot":   string(e.Slot),
			"driver": e.Driver,
			"error":  e.Err.Error(),
		})
	}
	return out
}
