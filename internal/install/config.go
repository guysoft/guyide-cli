package install

import (
	"errors"
	"io/fs"
	"os"

	"github.com/guysoft/guyide-cli/pkg/schema"
	"gopkg.in/yaml.v3"
)

// LoadUserConfig reads ~/.guyide/config.yaml. Returns os.ErrNotExist
// (wrapped) if the file is absent. Validates the schema string.
func LoadUserConfig(p Paths) (schema.UserConfig, error) {
	var cfg schema.UserConfig
	b, err := os.ReadFile(p.UserConfig())
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Schema != schema.UserConfigSchema {
		return cfg, errors.New("install: user config schema mismatch: " + cfg.Schema)
	}
	return cfg, nil
}

// SaveUserConfig writes config.yaml atomically (tmp + rename).
func SaveUserConfig(p Paths, cfg schema.UserConfig) error {
	if cfg.Schema == "" {
		cfg.Schema = schema.UserConfigSchema
	}
	if err := p.EnsureLayout(); err != nil {
		return err
	}
	b, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	tmp := p.UserConfig() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p.UserConfig())
}

// LoadOrInitUserConfig loads ~/.guyide/config.yaml, or writes the
// default config and returns it if the file does not yet exist.
// The returned bool reports whether the config was freshly initialised.
func LoadOrInitUserConfig(p Paths) (schema.UserConfig, bool, error) {
	cfg, err := LoadUserConfig(p)
	if err == nil {
		return cfg, false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return cfg, false, err
	}
	cfg = schema.DefaultUserConfig()
	if err := SaveUserConfig(p, cfg); err != nil {
		return cfg, false, err
	}
	return cfg, true, nil
}
