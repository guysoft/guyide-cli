package components

import (
	"context"
	"testing"

	"github.com/guysoft/guyide-cli/pkg/schema"
)

type fakeComponent struct {
	name, driver string
	slot         Slot
}

func (f *fakeComponent) Name() string                              { return f.name }
func (f *fakeComponent) Slot() Slot                                { return f.slot }
func (f *fakeComponent) Driver() string                            { return f.driver }
func (f *fakeComponent) Plan(*Context) (Plan, error)               { return Plan{Component: f.name}, nil }
func (f *fakeComponent) Install(*Context) error                    { return nil }
func (f *fakeComponent) Update(*Context) error                     { return nil }
func (f *fakeComponent) Uninstall(*Context) error                  { return nil }
func (f *fakeComponent) Doctor(*Context) []schema.DoctorCheck      { return nil }
func (f *fakeComponent) OwnedPaths(*Context) []string              { return nil }

func TestRegisterAndGet(t *testing.T) {
	reset()
	Register(SlotEditor, "fake", func() Component {
		return &fakeComponent{name: "fake", driver: "fake", slot: SlotEditor}
	})

	c, err := Get(SlotEditor, "fake")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if c.Name() != "fake" || c.Driver() != "fake" || c.Slot() != SlotEditor {
		t.Fatalf("unexpected component: %+v", c)
	}
}

func TestGetMissing(t *testing.T) {
	reset()
	if _, err := Get(SlotEditor, "nope"); err == nil {
		t.Fatal("expected error for missing driver")
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	reset()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate register")
		}
	}()
	f := func() Component { return &fakeComponent{slot: SlotEditor, driver: "x"} }
	Register(SlotEditor, "x", f)
	Register(SlotEditor, "x", f)
}

func TestDriversSorted(t *testing.T) {
	reset()
	Register(SlotAgent, "zeta", func() Component { return &fakeComponent{slot: SlotAgent, driver: "zeta"} })
	Register(SlotAgent, "alpha", func() Component { return &fakeComponent{slot: SlotAgent, driver: "alpha"} })
	Register(SlotAgent, "mike", func() Component { return &fakeComponent{slot: SlotAgent, driver: "mike"} })
	got := Drivers(SlotAgent)
	want := []string{"alpha", "mike", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("Drivers returned %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("Drivers[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestDriversEmptySlot(t *testing.T) {
	reset()
	if got := Drivers(SlotMultiplexer); got != nil {
		t.Fatalf("expected nil for empty slot, got %v", got)
	}
}

func TestContextZeroValueUsable(t *testing.T) {
	// Just confirm Context is a value type we can construct piecemeal.
	c := &Context{Ctx: context.Background(), Channel: "stable"}
	if c.Channel != "stable" {
		t.Fatal("Context fields not assignable")
	}
}

func TestAllSlotsOrder(t *testing.T) {
	got := AllSlots()
	want := []Slot{SlotEditor, SlotMultiplexer, SlotAgent}
	if len(got) != len(want) {
		t.Fatalf("AllSlots length: got %d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("AllSlots[%d]=%q want %q", i, got[i], want[i])
		}
	}
}
