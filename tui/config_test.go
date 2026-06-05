package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestSaveLoadFileRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	// Cover the per-OS roots os.UserConfigDir consults.
	t.Setenv("APPDATA", tmp)         // Windows
	t.Setenv("XDG_CONFIG_HOME", tmp) // Linux/BSD
	t.Setenv("HOME", tmp)            // macOS ($HOME/Library/Application Support)
	if ConfigPath() == "" {
		t.Skip("no user config dir on this platform")
	}

	want := DefaultSettings()
	want.Theme = "magenta"
	want.Variant = VariantMinimal
	if err := SaveSettings(want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "magenta" || got.Variant != VariantMinimal {
		t.Errorf("file did not round-trip through disk: %+v", got)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	in := Settings{
		Theme: "teal", Variant: VariantRich, Density: DensityCompact,
		DetailDefault: true, PinFavorites: false, VerboseDefault: true,
		EnvColors: map[string]string{"edge": "#a78bfa"},
	}
	data, err := marshalSettings(in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := unmarshalSettings(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "teal" || got.Variant != VariantRich || got.Density != DensityCompact {
		t.Errorf("look not round-tripped: %+v", got)
	}
	if !got.DetailDefault || got.PinFavorites || !got.VerboseDefault {
		t.Errorf("toggles not round-tripped: %+v", got)
	}
	if got.EnvColors["edge"] != "#a78bfa" {
		t.Errorf("env colours not round-tripped: %+v", got.EnvColors)
	}
}

func TestPartialConfigKeepsDefaults(t *testing.T) {
	// A file that only sets the theme must leave every other default intact.
	got, err := unmarshalSettings([]byte("[ui]\ntheme = \"green\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	def := DefaultSettings()
	if got.Theme != "green" {
		t.Errorf("theme not applied: %q", got.Theme)
	}
	if got.Variant != def.Variant || got.Density != def.Density || got.PinFavorites != def.PinFavorites {
		t.Errorf("partial config clobbered defaults: %+v", got)
	}
}

func TestRenderSettingsOverlay(t *testing.T) {
	m := newSampleModel()
	m.showSettings = true
	out := strip(render(t, m, 120, 40))
	for _, want := range []string{"settings", "Style", "Theme", "Density", "Pin favourites", "esc save"} {
		if !strings.Contains(out, want) {
			t.Errorf("settings overlay missing %q:\n%s", want, out)
		}
	}
	t.Log("\n" + out)
}

func TestSettingsBoxWidthStable(t *testing.T) {
	// The overlay must not resize as the focused value changes width
	// (e.g. "compact" vs "comfortable").
	width := func(density Density) int {
		m := newSampleModel()
		m.settings.Density = density
		m.showSettings = true
		return lipgloss.Width(m.settingsBox())
	}
	if w1, w2 := width(DensityCompact), width(DensityComfortable); w1 != w2 {
		t.Errorf("settings box width changed with value: %d vs %d", w1, w2)
	}
}

func TestSettingsCycleLiveRetheme(t *testing.T) {
	m := newSampleModel()
	m.showSettings = true
	amber := m.styles.Accent
	// Focus the Theme row and cycle once; the live theme must change.
	m.settingsFocus = 1
	m.changeSetting(1)
	if m.styles.Accent == amber {
		t.Errorf("cycling theme did not re-theme live (still %s)", amber)
	}
}
