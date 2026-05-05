package install

import "testing"

func TestLoadCompatHasV020Pins(t *testing.T) {
	c, err := LoadCompat()
	if err != nil {
		t.Fatalf("LoadCompat: %v", err)
	}
	if got := c.Schema; got != "guyide/compat/v1" {
		t.Errorf("schema = %q", got)
	}
	if ref, ok := c.PinFor("v0.2.0", "nvguy"); !ok || ref != "v0.1.0" {
		t.Errorf("nvguy pin = (%q,%v) want (v0.1.0,true)", ref, ok)
	}
	if ref, ok := c.PinFor("v0.2.0", "vscodium.nvim"); !ok || ref != "v0.1.0" {
		t.Errorf("vscodium.nvim pin = (%q,%v) want (v0.1.0,true)", ref, ok)
	}
	if _, ok := c.PinFor("v9.9.9", "nvguy"); ok {
		t.Errorf("unknown version should not resolve")
	}
}

func TestLoadSupportMatrixTriples(t *testing.T) {
	s, err := LoadSupport()
	if err != nil {
		t.Fatalf("LoadSupport: %v", err)
	}

	if status, ok := s.IsSupported("nvim", "tmux", "opencode"); !ok || status != "supported" {
		t.Errorf("nvim+tmux+opencode = (%q,%v) want (supported,true)", status, ok)
	}
	if status, ok := s.IsSupported("nvim", "tmux", "claude-code"); !ok || status != "stub" {
		t.Errorf("nvim+tmux+claude-code = (%q,%v) want (stub,true)", status, ok)
	}
	if _, ok := s.IsSupported("vscode", "tmux", "opencode"); ok {
		t.Errorf("unsupported triple should not match")
	}

	// Sanity: every declared triple must reference a known driver.
	known := map[string]map[string]bool{
		"editor":      {},
		"multiplexer": {},
		"agent":       {},
	}
	for _, d := range s.Drivers.Editor {
		known["editor"][d.Name] = true
	}
	for _, d := range s.Drivers.Multiplexer {
		known["multiplexer"][d.Name] = true
	}
	for _, d := range s.Drivers.Agent {
		known["agent"][d.Name] = true
	}
	for _, t3 := range s.Triples {
		if !known["editor"][t3.Editor] {
			t.Errorf("triple references unknown editor %q", t3.Editor)
		}
		if !known["multiplexer"][t3.Multiplexer] {
			t.Errorf("triple references unknown multiplexer %q", t3.Multiplexer)
		}
		if !known["agent"][t3.Agent] {
			t.Errorf("triple references unknown agent %q", t3.Agent)
		}
	}
}
