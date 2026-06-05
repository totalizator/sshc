package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	colorful "github.com/lucasb-eyer/go-colorful"
)

// Variant selects the overall list/detail chrome. See the redesign handoff §4.
type Variant string

const (
	VariantMinimal Variant = "minimal" // borderless, faint selection wash
	VariantFramed  Variant = "framed"  // rounded panes, accent-tinted selection
	VariantRich    Variant = "rich"    // per-env left bars, filled chips
)

// Density selects row height: two-line (comfortable) or single-line (compact).
type Density string

const (
	DensityComfortable Density = "comfortable"
	DensityCompact     Density = "compact"
)

// Accent themes, keyed by name. amber is the shipped default.
var accentThemes = map[string]string{
	"amber":   "#f5a524",
	"teal":    "#2dd4bf",
	"green":   "#4ade80",
	"magenta": "#d946ef",
}

// Core UI palette (truecolor). See handoff §3.
const (
	colorBG     = "#0d0d10" // screen background
	colorFG     = "#d7dadf" // primary text
	colorDim    = "#9aa0a8" // secondary text (address)
	colorFaint  = "#676c75" // meta text (tags, last-used, labels)
	colorBorder = "#2a2c33" // pane borders, rules, dividers

	colorOverlayBG = "#131319" // help / confirm box background
	colorCmdBG     = "#08080b" // resolved-command box background
	colorCmdBody   = "#cfd6dd"
	colorStar      = "#4d5159" // default favourite-star colour
	colorEnvBar    = "#3a3d44" // rich left bar when env is empty
	colorHintKey   = "#c7ccd4"

	colorOK   = "#54c47a" // toast: success
	colorWarn = "#f87171" // toast: destructive / error
)

// Preset env chip colours (handoff §3). Any other non-empty env gets a stable
// hue derived from its characters; an empty env renders no chip at all.
var envPresets = map[string]string{
	"prod":    "#f87171",
	"staging": "#fbbf24",
	"dev":     "#4ade80",
	"home":    "#60a5fa",
	"cloud":   "#22d3ee",
}

// ParseVariant resolves a variant name, falling back to the framed default.
func ParseVariant(s string) Variant {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "minimal":
		return VariantMinimal
	case "rich":
		return VariantRich
	default:
		return VariantFramed
	}
}

// ParseDensity resolves a density name, falling back to comfortable.
func ParseDensity(s string) Density {
	if strings.EqualFold(strings.TrimSpace(s), "compact") {
		return DensityCompact
	}
	return DensityComfortable
}

// accentHex resolves a theme name to its hex; an unknown name (or a literal hex
// the user supplied) is returned as-is when it looks like a colour, else amber.
func accentHex(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if hex, ok := accentThemes[name]; ok {
		return hex
	}
	if strings.HasPrefix(name, "#") {
		return name
	}
	return accentThemes["amber"]
}

// envColor resolves a chip colour for an env string. Presets get their fixed
// colour; any other non-empty value gets a stable derived hue so user-defined
// envs read consistently across runs. Empty env returns ("", false) — no chip.
// Mirrors the design's window.SSHC.envColor exactly.
func envColor(env string, overrides map[string]string) (string, bool) {
	// Strip control bytes (e.g. a stray NUL) before keying so a corrupt value
	// neither tints a chip nor derives a different hue than its clean twin.
	key := strings.TrimSpace(stripControl(env))
	if key == "" {
		return "", false
	}
	if hex, ok := overrides[key]; ok {
		return hex, true
	}
	if hex, ok := envPresets[key]; ok {
		return hex, true
	}
	var h uint32
	for _, c := range key {
		h = h*31 + uint32(c)
	}
	hue := float64(h % 360)
	return colorful.Hsl(hue, 0.58, 0.64).Clamped().Hex(), true
}

// blend mixes overlay into base at the given fraction (0..1), e.g. the
// accent-at-11%-over-bg selection tint. Both inputs are hex strings.
func blend(base, overlay string, t float64) string {
	b, err1 := colorful.Hex(base)
	o, err2 := colorful.Hex(overlay)
	if err1 != nil || err2 != nil {
		return base
	}
	return b.BlendRgb(o, t).Clamped().Hex()
}

func color(hex string) lipgloss.Color { return lipgloss.Color(hex) }
