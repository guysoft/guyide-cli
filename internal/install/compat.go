package install

import (
	"encoding/json"
	"errors"
	"fmt"

	embedfs "github.com/guysoft/guyide-cli/embed"
	"gopkg.in/yaml.v3"
)

// CompatMatrix is the parsed embed/compat.json.
type CompatMatrix struct {
	Schema   string                       `json:"schema"`
	Versions map[string]map[string]string `json:"versions"`
}

// LoadCompat parses the embedded compat matrix.
func LoadCompat() (CompatMatrix, error) {
	var c CompatMatrix
	if err := json.Unmarshal(embedfs.CompatJSON(), &c); err != nil {
		return c, fmt.Errorf("compat: %w", err)
	}
	if c.Schema != "guyide/compat/v1" {
		return c, errors.New("compat: unexpected schema " + c.Schema)
	}
	return c, nil
}

// PinFor returns the pinned ref for component `name` at guyide-cli
// version `cliVersion`. ok=false when no pin exists (caller may fall
// back to "latest" or refuse).
func (c CompatMatrix) PinFor(cliVersion, component string) (ref string, ok bool) {
	row, hasRow := c.Versions[cliVersion]
	if !hasRow {
		return "", false
	}
	ref, ok = row[component]
	return ref, ok
}

// SupportMatrix is the parsed embed/support_matrix.yaml.
type SupportMatrix struct {
	Schema  string `yaml:"schema"`
	Drivers struct {
		Editor      []DriverSpec `yaml:"editor"`
		Multiplexer []DriverSpec `yaml:"multiplexer"`
		Agent       []DriverSpec `yaml:"agent"`
	} `yaml:"drivers"`
	Triples []TripleSpec `yaml:"triples"`
}

// DriverSpec is one supported (or stubbed) driver in a slot.
type DriverSpec struct {
	Name       string            `yaml:"name"`
	Status     string            `yaml:"status"` // supported|stub|deprecated
	MinVersion string            `yaml:"min_version,omitempty"`
	Preferred  string            `yaml:"preferred,omitempty"`
	Planned    string            `yaml:"planned,omitempty"`
	Requires   map[string]string `yaml:"requires,omitempty"`
}

// TripleSpec is one validated combination.
type TripleSpec struct {
	Editor      string `yaml:"editor"`
	Multiplexer string `yaml:"multiplexer"`
	Agent       string `yaml:"agent"`
	Status      string `yaml:"status"`
	Since       string `yaml:"since,omitempty"`
	Planned     string `yaml:"planned,omitempty"`
}

// LoadSupport parses the embedded support matrix.
func LoadSupport() (SupportMatrix, error) {
	var s SupportMatrix
	if err := yaml.Unmarshal(embedfs.SupportMatrixYAML(), &s); err != nil {
		return s, fmt.Errorf("support: %w", err)
	}
	if s.Schema != "guyide/support_matrix/v1" {
		return s, errors.New("support: unexpected schema " + s.Schema)
	}
	return s, nil
}

// IsSupported returns the status of (editor, multiplexer, agent) and
// true if a row matches. A missing row returns ("", false).
func (s SupportMatrix) IsSupported(editor, multiplexer, agent string) (status string, found bool) {
	for _, t := range s.Triples {
		if t.Editor == editor && t.Multiplexer == multiplexer && t.Agent == agent {
			return t.Status, true
		}
	}
	return "", false
}
