package schema

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultUserConfigShape(t *testing.T) {
	c := DefaultUserConfig()
	if c.Schema != UserConfigSchema {
		t.Errorf("schema = %q want %q", c.Schema, UserConfigSchema)
	}
	if c.Channel != "stable" {
		t.Errorf("channel = %q want stable", c.Channel)
	}
	if c.Components.Editor.Driver != "nvim" {
		t.Errorf("editor driver = %q", c.Components.Editor.Driver)
	}
	if c.Components.Multiplexer.Driver != "tmux" {
		t.Errorf("multiplexer driver = %q", c.Components.Multiplexer.Driver)
	}
	if c.Components.Agent.Driver != "opencode" {
		t.Errorf("agent driver = %q", c.Components.Agent.Driver)
	}
	if !c.Tmux.OwnConf {
		t.Errorf("tmux.own_conf must default true")
	}
}

func TestUserConfigYAMLAndJSONRoundTrip(t *testing.T) {
	in := DefaultUserConfig()

	yb, err := yaml.Marshal(&in)
	if err != nil {
		t.Fatalf("yaml marshal: %v", err)
	}
	var fromYAML UserConfig
	if err := yaml.Unmarshal(yb, &fromYAML); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	if fromYAML.Components.Agent.Driver != "opencode" {
		t.Errorf("yaml roundtrip lost agent driver: %+v", fromYAML)
	}

	jb, err := json.Marshal(&in)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	var fromJSON UserConfig
	if err := json.Unmarshal(jb, &fromJSON); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if fromJSON.Components.Editor.Driver != "nvim" {
		t.Errorf("json roundtrip lost editor driver: %+v", fromJSON)
	}
}
