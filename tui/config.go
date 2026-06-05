package tui

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// fileConfig is the on-disk representation of the UI settings (handoff §7).
// Pointers distinguish "absent" from a meaningful false/zero so a partial file
// only overrides the keys it actually sets.
type fileConfig struct {
	UI uiConfig `toml:"ui"`
}

type uiConfig struct {
	Theme          string            `toml:"theme,omitempty"`
	Variant        string            `toml:"variant,omitempty"`
	Density        string            `toml:"density,omitempty"`
	DetailDefault  *bool             `toml:"detail_default,omitempty"`
	PinFavorites   *bool             `toml:"pin_favorites,omitempty"`
	VerboseDefault *bool             `toml:"verbose_default,omitempty"`
	EnvColors      map[string]string `toml:"env_colors,omitempty"`
}

// ConfigPath returns the path to the sshc config file
// (e.g. ~/.config/sshc/config.toml). It is empty only if the user config dir
// cannot be determined.
func ConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "sshc", "config.toml")
}

// LoadSettings returns the shipped defaults overlaid with any values found in
// the config file. A missing file is not an error (defaults are returned).
func LoadSettings() (Settings, error) {
	s := DefaultSettings()
	path := ConfigPath()
	if path == "" {
		return s, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}

	return unmarshalSettings(data)
}

// unmarshalSettings overlays a TOML document onto the shipped defaults.
func unmarshalSettings(data []byte) (Settings, error) {
	s := DefaultSettings()
	var fc fileConfig
	if err := toml.Unmarshal(data, &fc); err != nil {
		return s, err
	}
	applyFileConfig(&s, fc.UI)
	return s, nil
}

func applyFileConfig(s *Settings, ui uiConfig) {
	if ui.Theme != "" {
		s.Theme = ui.Theme
	}
	if ui.Variant != "" {
		s.Variant = ParseVariant(ui.Variant)
	}
	if ui.Density != "" {
		s.Density = ParseDensity(ui.Density)
	}
	if ui.DetailDefault != nil {
		s.DetailDefault = *ui.DetailDefault
	}
	if ui.PinFavorites != nil {
		s.PinFavorites = *ui.PinFavorites
	}
	if ui.VerboseDefault != nil {
		s.VerboseDefault = *ui.VerboseDefault
	}
	if len(ui.EnvColors) > 0 {
		s.EnvColors = ui.EnvColors
	}
}

// SaveSettings writes s to the config file, creating the directory as needed.
// It preserves any user env-colour overrides already carried on s.
func SaveSettings(s Settings) error {
	path := ConfigPath()
	if path == "" {
		return os.ErrInvalid
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := marshalSettings(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// marshalSettings renders s as a TOML document.
func marshalSettings(s Settings) ([]byte, error) {
	detail, pin, verbose := s.DetailDefault, s.PinFavorites, s.VerboseDefault
	fc := fileConfig{UI: uiConfig{
		Theme:          s.Theme,
		Variant:        string(s.Variant),
		Density:        string(s.Density),
		DetailDefault:  &detail,
		PinFavorites:   &pin,
		VerboseDefault: &verbose,
		EnvColors:      s.EnvColors,
	}}

	var buf bytes.Buffer
	buf.WriteString("# sshc UI configuration — see `sshc --help` and the README.\n")
	if err := toml.NewEncoder(&buf).Encode(fc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
