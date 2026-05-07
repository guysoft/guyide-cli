package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/guysoft/guyide-cli/internal/components"
	"github.com/guysoft/guyide-cli/pkg/schema"
)

// RunOptions parameterise installer/uninstaller orchestration.
type RunOptions struct {
	Ctx       context.Context
	HomeDir   string // if empty, os.UserHomeDir()
	Paths     Paths  // if zero, NewPathsAt(<home>/.guyide)
	DryRun    bool
	UserRef   string // optional global ref override
	Channel   string // optional override; otherwise from config
	UserConf  *schema.UserConfig // optional preloaded config; otherwise loaded/initialised
}

// RunReport summarises a Plan/Install/Uninstall run for CLI rendering.
type RunReport struct {
	Plans      []components.Plan // per-driver plan (always populated)
	Errors     []DriverError     // per-driver errors during Install/Uninstall
	ConfigInit bool              // true if config.yaml was freshly written
	DryRun     bool
}

// DriverError binds an error to a (slot, driver) pair.
type DriverError struct {
	Slot   components.Slot
	Driver string
	Err    error
}

func (d DriverError) Error() string {
	return fmt.Sprintf("%s/%s: %v", d.Slot, d.Driver, d.Err)
}

// resolve fills in defaults for opts and returns a normalised copy plus the
// loaded user config.
func resolve(opts RunOptions) (RunOptions, schema.UserConfig, bool, error) {
	if opts.Ctx == nil {
		opts.Ctx = context.Background()
	}
	if opts.HomeDir == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return opts, schema.UserConfig{}, false, err
		}
		opts.HomeDir = h
	}
	if opts.Paths.Root == "" {
		opts.Paths = NewPathsAt(opts.HomeDir + "/.guyide")
	}

	var (
		cfg  schema.UserConfig
		init bool
		err  error
	)
	if opts.UserConf != nil {
		cfg = *opts.UserConf
	} else {
		cfg, init, err = LoadOrInitUserConfig(opts.Paths)
		if err != nil {
			return opts, cfg, false, err
		}
	}
	if opts.Channel == "" {
		opts.Channel = cfg.Channel
	}
	if opts.Channel == "" {
		opts.Channel = "stable"
	}
	return opts, cfg, init, nil
}

// componentCtx builds a *components.Context for one driver run.
func componentCtx(opts RunOptions, cfg schema.UserConfig, ref string) *components.Context {
	c := *&components.Context{
		Ctx:        opts.Ctx,
		Channel:    opts.Channel,
		DryRun:     opts.DryRun,
		HomeDir:    opts.HomeDir,
		GuyideDir:  opts.Paths.Root,
		UserConfig: &cfg,
	}
	// Per-driver UserRef: explicit > config-pinned > global override.
	c.UserRef = ref
	if c.UserRef == "" {
		c.UserRef = opts.UserRef
	}
	return &c
}

// resolvedDriver bundles a Component plus the slot/driver/ref selection.
type resolvedDriver struct {
	Slot   components.Slot
	Driver string
	Ref    string
	Comp   components.Component
}

// resolveDrivers resolves every slot to its driver. Returns an error if
// any slot's driver is not registered (hard fail per ckpt 5 decision).
func resolveDrivers(cfg schema.UserConfig) ([]resolvedDriver, error) {
	type pick struct {
		slot components.Slot
		ref  schema.DriverRef
	}
	picks := []pick{
		{components.SlotEditor, cfg.Components.Editor},
		{components.SlotMultiplexer, cfg.Components.Multiplexer},
		{components.SlotAgent, cfg.Components.Agent},
	}
	out := make([]resolvedDriver, 0, len(picks))
	for _, p := range picks {
		if p.ref.Driver == "" {
			return nil, fmt.Errorf("config: no driver selected for slot %q", p.slot)
		}
		comp, err := components.Get(p.slot, p.ref.Driver)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
		out = append(out, resolvedDriver{
			Slot:   p.slot,
			Driver: p.ref.Driver,
			Ref:    p.ref.Ref,
			Comp:   comp,
		})
	}
	return out, nil
}

// RunPlan computes the plan for every selected driver without
// performing any side effects (besides writing a default config.yaml
// on first invocation).
func RunPlan(opts RunOptions) (RunReport, error) {
	opts, cfg, init, err := resolve(opts)
	if err != nil {
		return RunReport{}, err
	}
	report := RunReport{ConfigInit: init, DryRun: opts.DryRun}
	drivers, err := resolveDrivers(cfg)
	if err != nil {
		return report, err
	}
	for _, d := range drivers {
		ctx := componentCtx(opts, cfg, d.Ref)
		plan, err := d.Comp.Plan(ctx)
		if err != nil {
			report.Errors = append(report.Errors, DriverError{Slot: d.Slot, Driver: d.Driver, Err: err})
			continue
		}
		report.Plans = append(report.Plans, plan)
	}
	if len(report.Errors) > 0 {
		return report, errors.New("plan: one or more drivers reported errors")
	}
	return report, nil
}

// RunInstall installs every selected driver in order: editor →
// multiplexer → agent. Stops at the first hard error UNLESS DryRun is
// true (then it's a no-op anyway). Updates the manifest with
// per-component entries on success.
func RunInstall(opts RunOptions) (RunReport, error) {
	opts, cfg, init, err := resolve(opts)
	if err != nil {
		return RunReport{}, err
	}
	report := RunReport{ConfigInit: init, DryRun: opts.DryRun}

	if err := opts.Paths.EnsureLayout(); err != nil {
		return report, err
	}

	drivers, err := resolveDrivers(cfg)
	if err != nil {
		return report, err
	}

	// Plan first so caller can render summary; this also surfaces any
	// driver errors before we touch the disk.
	for _, d := range drivers {
		ctx := componentCtx(opts, cfg, d.Ref)
		plan, perr := d.Comp.Plan(ctx)
		if perr != nil {
			report.Errors = append(report.Errors, DriverError{Slot: d.Slot, Driver: d.Driver, Err: perr})
			continue
		}
		report.Plans = append(report.Plans, plan)
	}
	if len(report.Errors) > 0 {
		return report, errors.New("install: plan reported errors; aborting")
	}

	if opts.DryRun {
		return report, nil
	}

	// Install in canonical slot order.
	for _, d := range drivers {
		ctx := componentCtx(opts, cfg, d.Ref)
		if err := d.Comp.Install(ctx); err != nil {
			report.Errors = append(report.Errors, DriverError{Slot: d.Slot, Driver: d.Driver, Err: err})
			return report, fmt.Errorf("install %s/%s: %w", d.Slot, d.Driver, err)
		}
	}

	// Update manifest with component entries + version stamp.
	if err := updateManifestAfterInstall(opts.Paths, cfg, drivers); err != nil {
		return report, err
	}
	return report, nil
}

// RunUninstall calls Uninstall on every selected driver in REVERSE
// order: agent → multiplexer → editor. Continues past errors so the
// user can clean up as much as possible in one pass; the first error
// is returned at the end.
func RunUninstall(opts RunOptions) (RunReport, error) {
	opts, cfg, init, err := resolve(opts)
	if err != nil {
		return RunReport{}, err
	}
	report := RunReport{ConfigInit: init, DryRun: opts.DryRun}

	drivers, err := resolveDrivers(cfg)
	if err != nil {
		return report, err
	}

	if opts.DryRun {
		// Populate plans for visibility; no manifest mutation.
		for _, d := range drivers {
			ctx := componentCtx(opts, cfg, d.Ref)
			plan, _ := d.Comp.Plan(ctx)
			report.Plans = append(report.Plans, plan)
		}
		return report, nil
	}

	// For live runs, populate per-driver entries in the report so the
	// CLI summary reflects how many components were touched.
	for _, d := range drivers {
		report.Plans = append(report.Plans, components.Plan{
			Component: d.Driver,
			Slot:      d.Slot,
			Driver:    d.Driver,
			Status:    components.StatusUpdate,
			Actions:   []string{fmt.Sprintf("uninstall %s/%s", d.Slot, d.Driver)},
		})
	}

	var firstErr error
	for i := len(drivers) - 1; i >= 0; i-- {
		d := drivers[i]
		ctx := componentCtx(opts, cfg, d.Ref)
		if err := d.Comp.Uninstall(ctx); err != nil {
			report.Errors = append(report.Errors, DriverError{Slot: d.Slot, Driver: d.Driver, Err: err})
			if firstErr == nil {
				firstErr = fmt.Errorf("uninstall %s/%s: %w", d.Slot, d.Driver, err)
			}
		}
	}
	return report, firstErr
}

// updateManifestAfterInstall records per-component entries. Called only
// on full install success.
func updateManifestAfterInstall(p Paths, cfg schema.UserConfig, drivers []resolvedDriver) error {
	m, err := LoadManifest(p)
	if err != nil {
		// Fresh install or corrupted manifest: start over.
		m = schema.Manifest{Schema: schema.ManifestSchema}
	}
	if m.Components == nil {
		m.Components = map[string]schema.ComponentEntry{}
	}
	now := time.Now().UTC()
	if m.InstalledAt.IsZero() {
		m.InstalledAt = now
	}
	m.UpdatedAt = now
	m.Channel = cfg.Channel
	for _, d := range drivers {
		entry := m.Components[string(d.Slot)]
		entry.Slot = string(d.Slot)
		entry.Driver = d.Driver
		entry.Ref = d.Ref
		entry.Path = p.Component(d.Driver)
		m.Components[string(d.Slot)] = entry
	}
	return SaveManifest(p, m)
}
