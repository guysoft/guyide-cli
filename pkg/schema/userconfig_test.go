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

func TestClaudeCodeConfigYAMLRoundTrip(t *testing.T) {
	input := `
schema: guyide/config/v1
channel: stable
components:
  editor:
    driver: nvim
  multiplexer:
    driver: tmux
  agent:
    driver: claude-code
claude-code:
  cli: claude
  extra_args:
    - "--model"
    - "opus"
  skill_root: "~/.claude/skills"
`
	var cfg UserConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	if cfg.Components.Agent.Driver != "claude-code" {
		t.Errorf("agent driver = %q want claude-code", cfg.Components.Agent.Driver)
	}
	if cfg.ClaudeCode.CLI != "claude" {
		t.Errorf("claude-code.cli = %q want claude", cfg.ClaudeCode.CLI)
	}
	if len(cfg.ClaudeCode.ExtraArgs) != 2 || cfg.ClaudeCode.ExtraArgs[1] != "opus" {
		t.Errorf("claude-code.extra_args = %v", cfg.ClaudeCode.ExtraArgs)
	}
	if cfg.ClaudeCode.SkillRoot != "~/.claude/skills" {
		t.Errorf("claude-code.skill_root = %q", cfg.ClaudeCode.SkillRoot)
	}
}
