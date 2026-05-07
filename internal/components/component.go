// Package components defines the Component interface and the per-slot
// driver registry that backs guyide's pluggable architecture.
//
// A Component represents one installed unit of GuyIDE — an editor, a
// multiplexer, or an AI agent. Each slot accepts exactly one driver
// (e.g. slot=editor, driver=nvim). Drivers register themselves via
// Register() in their package's init() and the install manager looks
// them up by (slot, driver) pair.
package components

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/guysoft/guyide-cli/pkg/schema"
)

// Slot identifies one of GuyIDE's three pluggable slots.
type Slot string

const (
	SlotEditor      Slot = "editor"
	SlotMultiplexer Slot = "multiplexer"
	SlotAgent       Slot = "agent"
)

// AllSlots returns the slots in canonical install order.
func AllSlots() []Slot {
	return []Slot{SlotEditor, SlotMultiplexer, SlotAgent}
}

// ErrNotImplemented is returned by stub drivers (e.g. claude-code in
// v0.2). The install manager treats this as a soft failure and surfaces
// it to the user without aborting the run, unless the driver was
// explicitly selected.
var ErrNotImplemented = errors.New("driver not implemented in this release")

// Status describes a planned action for a component.
type Status string

const (
	StatusInstall   Status = "install"
	StatusUpdate    Status = "update"
	StatusUnchanged Status = "unchanged"
	StatusSkip      Status = "skip"
	StatusError     Status = "error"
)

// Plan describes what Install/Update would do, without doing it. Used
// for --dry-run and for the pre-install summary panel.
type Plan struct {
	Component string   // human name, e.g. "nvim (NvGuy)"
	Slot      Slot     //
	Driver    string   //
	Status    Status   //
	Ref       string   // tag or commit, "" for n/a
	Source    string   // git URL or local override
	Actions   []string // bullet list shown to the user
	Notes     []string // warnings, drift, etc.
}

// Context carries dependencies into driver methods. It exists so we can
// add fields (logger, output writer, channel) without churning every
// driver signature.
type Context struct {
	Ctx       context.Context
	Channel   string // "stable" | "dev"
	DryRun    bool
	UserRef   string // explicit ref override (empty = use channel default)
	HomeDir   string // user $HOME (testable)
	GuyideDir string // ~/.guyide (testable)
	// UserConfig is the parsed ~/.guyide/config.yaml. Drivers that
	// honour user-tunable behaviour (e.g. tmux own_conf) read from
	// here. May be nil; drivers must treat that as "use defaults".
	UserConfig *schema.UserConfig
}

// Component is the contract every driver implements.
type Component interface {
	// Name returns a short human label, e.g. "nvim".
	Name() string
	// Slot returns which pluggable slot this driver fills.
	Slot() Slot
	// Driver returns the registered driver id, e.g. "nvim".
	Driver() string

	// Plan returns what Install/Update would do.
	Plan(c *Context) (Plan, error)

	// Install lays down the component. Idempotent: re-running on a
	// healthy install must be a no-op (StatusUnchanged).
	Install(c *Context) error

	// Update pulls the latest ref permitted by the channel and
	// re-runs any post-install steps.
	Update(c *Context) error

	// Uninstall removes everything in OwnedPaths and clears registry
	// entries.
	Uninstall(c *Context) error

	// Doctor returns health checks for this component.
	Doctor(c *Context) []schema.DoctorCheck

	// OwnedPaths returns absolute paths the driver claims ownership
	// of. The install manager records these in the manifest.
	OwnedPaths(c *Context) []string
}

// Factory builds a fresh Component instance.
type Factory func() Component

// registry is the package-level driver index, keyed by (slot, driver).
var (
	registryMu sync.RWMutex
	registry   = map[Slot]map[string]Factory{}
)

// Register adds a driver to the registry. Drivers call this from their
// package init(). Duplicate registration panics — it is always a bug.
func Register(slot Slot, driver string, f Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := registry[slot]; !ok {
		registry[slot] = map[string]Factory{}
	}
	if _, dup := registry[slot][driver]; dup {
		panic(fmt.Sprintf("components: duplicate driver %q for slot %q", driver, slot))
	}
	registry[slot][driver] = f
}

// Get instantiates the driver registered for (slot, driver).
// Returns an error if no such driver is registered.
func Get(slot Slot, driver string) (Component, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if drivers, ok := registry[slot]; ok {
		if f, ok := drivers[driver]; ok {
			return f(), nil
		}
	}
	return nil, fmt.Errorf("no driver %q registered for slot %q", driver, slot)
}

// Drivers returns the sorted list of driver ids registered for a slot.
// Useful for `guyide config` validation and tab completion.
func Drivers(slot Slot) []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	drivers, ok := registry[slot]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(drivers))
	for d := range drivers {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// reset is for tests only.
func reset() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = map[Slot]map[string]Factory{}
}
