package schema

// UserConfigSchema is the stable identifier for ~/.guyide/config.yaml.
const UserConfigSchema = "guyide/config/v1"

// UserConfig is the user-facing configuration written to
// ~/.guyide/config.yaml. It selects which driver fills each slot and
// supplies per-driver tuning.
//
// Only the active driver's config block is validated; inactive blocks
// are preserved verbatim so users can swap drivers without losing
// settings.
type UserConfig struct {
	Schema     string             `yaml:"schema" json:"schema"`
	Channel    string             `yaml:"channel" json:"channel"` // stable|dev
	Components ComponentSelection `yaml:"components" json:"components"`
	Nvim       NvimConfig         `yaml:"nvim,omitempty" json:"nvim,omitempty"`
	Tmux       TmuxConfig         `yaml:"tmux,omitempty" json:"tmux,omitempty"`
	OpenCode   OpenCodeConfig     `yaml:"opencode,omitempty" json:"opencode,omitempty"`
	ClaudeCode ClaudeCodeConfig   `yaml:"claude-code,omitempty" json:"claude-code,omitempty"`
}

// ComponentSelection chooses which driver fills each slot.
type ComponentSelection struct {
	Editor      DriverRef `yaml:"editor" json:"editor"`
	Multiplexer DriverRef `yaml:"multiplexer" json:"multiplexer"`
	Agent       DriverRef `yaml:"agent" json:"agent"`
}

// DriverRef names a driver and optionally pins a ref overriding the
// channel default.
type DriverRef struct {
	Driver string `yaml:"driver" json:"driver"`
	Ref    string `yaml:"ref,omitempty" json:"ref,omitempty"`
}

// NvimConfig tunes the nvim editor driver.
type NvimConfig struct {
	DebugpyPython string `yaml:"debugpy_python,omitempty" json:"debugpy_python,omitempty"`
	HeadlessSync  bool   `yaml:"headless_sync,omitempty" json:"headless_sync,omitempty"`
}

// TmuxConfig tunes the tmux multiplexer driver.
type TmuxConfig struct {
	OwnConf      bool `yaml:"own_conf,omitempty" json:"own_conf,omitempty"`           // replace ~/.tmux.conf (default true)
	ReloadOnInstall bool `yaml:"reload_on_install,omitempty" json:"reload_on_install,omitempty"`
}

// OpenCodeConfig tunes the opencode agent driver.
type OpenCodeConfig struct {
	SkillRoot string `yaml:"skill_root,omitempty" json:"skill_root,omitempty"`
}

// ClaudeCodeConfig tunes the (stubbed) claude-code agent driver.
type ClaudeCodeConfig struct {
	// Reserved for v0.3.
}

// DefaultUserConfig returns the config shipped on first install.
func DefaultUserConfig() UserConfig {
	return UserConfig{
		Schema:  UserConfigSchema,
		Channel: "stable",
		Components: ComponentSelection{
			Editor:      DriverRef{Driver: "nvim"},
			Multiplexer: DriverRef{Driver: "tmux"},
			Agent:       DriverRef{Driver: "opencode"},
		},
		Tmux: TmuxConfig{OwnConf: true, ReloadOnInstall: true},
		Nvim: NvimConfig{HeadlessSync: true},
	}
}
