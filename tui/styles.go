package tui

import "github.com/charmbracelet/lipgloss"

// Layout constants shared across TUI components.
const (
	detailWidthCols = 44 // detail pane inner width (handoff: 44ch)
	detailMinWidth  = 32
	chipFG          = "#0c0c0e" // text on the brand badge / filled chips
)

// Styles is the resolved lipgloss theme for one run: every style is derived
// from the chosen accent, variant, and density up front so render code just
// reaches for a field. Raw colours used for per-row dynamic styling (bars,
// chips, selection fills) are kept alongside.
type Styles struct {
	Variant      Variant
	Density      Density
	Accent       string            // resolved accent hex
	SelBG        string            // selected-row / focused-field background
	EnvOverrides map[string]string // user env→colour overrides

	// brand + search
	Brand       lipgloss.Style
	BrandVer    lipgloss.Style
	BrandCount  lipgloss.Style
	SearchGlyph lipgloss.Style
	SearchText  lipgloss.Style
	SearchPH    lipgloss.Style
	SearchCount lipgloss.Style

	// groups
	GroupGlyph lipgloss.Style
	GroupLabel lipgloss.Style
	GroupRule  lipgloss.Style

	// rows
	Alias     lipgloss.Style
	AliasSel  lipgloss.Style
	Tags      lipgloss.Style
	Addr      lipgloss.Style
	Vbits     lipgloss.Style
	Last      lipgloss.Style
	Highlight lipgloss.Style
	Star      lipgloss.Style
	StarFav   lipgloss.Style
	Empty     lipgloss.Style

	// panes
	ListPane   lipgloss.Style
	DetailPane lipgloss.Style

	// detail
	DetailTitle lipgloss.Style
	DetailSec   lipgloss.Style
	KVKey       lipgloss.Style
	KVVal       lipgloss.Style
	KVMut       lipgloss.Style
	CmdBox      lipgloss.Style
	CmdPrompt   lipgloss.Style
	CmdBody     lipgloss.Style
	DetailHint  lipgloss.Style

	// form
	FormTitle       lipgloss.Style
	FieldLabel      lipgloss.Style
	FieldLabelFocus lipgloss.Style
	FieldPH         lipgloss.Style
	FieldVal        lipgloss.Style
	FieldRow        lipgloss.Style
	FieldRowFocus   lipgloss.Style
	FormHint        lipgloss.Style
	FormErr         lipgloss.Style

	// overlays
	HelpBox   lipgloss.Style
	HelpTitle lipgloss.Style
	HelpKey   lipgloss.Style
	HelpDesc  lipgloss.Style
	HelpFoot  lipgloss.Style

	ConfirmBox  lipgloss.Style
	ConfirmQ    lipgloss.Style
	ConfirmSub  lipgloss.Style
	ConfirmKeys lipgloss.Style
	ConfirmY    lipgloss.Style
	ConfirmN    lipgloss.Style

	// footer
	Footer    lipgloss.Style
	HintKey   lipgloss.Style
	HintDsc   lipgloss.Style
	ToastOK   lipgloss.Style
	ToastGo   lipgloss.Style
	ToastWarn lipgloss.Style
	ToastMut  lipgloss.Style
}

// NewStyles builds the theme for the given accent name, variant, and density.
func NewStyles(accentName string, variant Variant, density Density, envOverrides map[string]string) Styles {
	accent := accentHex(accentName)
	selBG := blend(colorBG, accent, 0.11)

	faint := color(colorFaint)
	dim := color(colorDim)
	fg := color(colorFG)
	border := color(colorBorder)
	acc := color(accent)

	s := Styles{
		Variant:      variant,
		Density:      density,
		Accent:       accent,
		SelBG:        selBG,
		EnvOverrides: envOverrides,

		Brand:       lipgloss.NewStyle().Background(acc).Foreground(color(chipFG)).Bold(true).Padding(0, 1),
		BrandVer:    lipgloss.NewStyle().Foreground(faint),
		BrandCount:  lipgloss.NewStyle().Foreground(faint),
		SearchGlyph: lipgloss.NewStyle().Foreground(acc).Bold(true),
		SearchText:  lipgloss.NewStyle().Foreground(fg),
		SearchPH:    lipgloss.NewStyle().Foreground(faint),
		SearchCount: lipgloss.NewStyle().Foreground(faint),

		GroupGlyph: lipgloss.NewStyle().Foreground(acc),
		GroupLabel: lipgloss.NewStyle().Foreground(faint),
		GroupRule:  lipgloss.NewStyle().Foreground(border),

		Alias:     lipgloss.NewStyle().Foreground(fg).Bold(true),
		AliasSel:  lipgloss.NewStyle().Foreground(acc).Bold(true),
		Tags:      lipgloss.NewStyle().Foreground(faint),
		Addr:      lipgloss.NewStyle().Foreground(dim),
		Vbits:     lipgloss.NewStyle().Foreground(faint),
		Last:      lipgloss.NewStyle().Foreground(faint),
		Highlight: lipgloss.NewStyle().Foreground(acc).Bold(true),
		Star:      lipgloss.NewStyle().Foreground(color(colorStar)),
		StarFav:   lipgloss.NewStyle().Foreground(acc),
		Empty:     lipgloss.NewStyle().Foreground(faint).Padding(1, 1),

		DetailTitle: lipgloss.NewStyle().Foreground(acc).Bold(true),
		DetailSec:   lipgloss.NewStyle().Foreground(faint),
		KVKey:       lipgloss.NewStyle().Foreground(faint).Width(13),
		KVVal:       lipgloss.NewStyle().Foreground(fg),
		KVMut:       lipgloss.NewStyle().Foreground(faint),
		CmdBox: lipgloss.NewStyle().
			Background(color(colorCmdBG)).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(0, 1),
		CmdPrompt:  lipgloss.NewStyle().Foreground(acc),
		CmdBody:    lipgloss.NewStyle().Foreground(color(colorCmdBody)),
		DetailHint: lipgloss.NewStyle().Foreground(faint),

		FormTitle:       lipgloss.NewStyle().Foreground(acc).Bold(true),
		FieldLabel:      lipgloss.NewStyle().Foreground(faint).Width(15),
		FieldLabelFocus: lipgloss.NewStyle().Foreground(acc).Width(15),
		FieldPH:         lipgloss.NewStyle().Foreground(color("#565b63")),
		FieldVal:        lipgloss.NewStyle().Foreground(fg),
		FieldRow:        lipgloss.NewStyle().Padding(0, 1),
		FieldRowFocus:   lipgloss.NewStyle().Padding(0, 1).Background(color(selBG)),
		FormHint:        lipgloss.NewStyle().Foreground(faint).PaddingLeft(1),
		FormErr:         lipgloss.NewStyle().Foreground(color(colorWarn)).PaddingLeft(1),

		HelpBox: lipgloss.NewStyle().
			Background(color(colorOverlayBG)).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(1, 3),
		HelpTitle: lipgloss.NewStyle().Foreground(acc).Bold(true),
		HelpKey:   lipgloss.NewStyle().Foreground(acc).Width(6).Align(lipgloss.Right),
		HelpDesc:  lipgloss.NewStyle().Foreground(dim),
		HelpFoot:  lipgloss.NewStyle().Foreground(faint),

		ConfirmBox: lipgloss.NewStyle().
			Background(color(colorOverlayBG)).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(color(blend(colorBorder, colorWarn, 0.55))).
			Padding(1, 3).
			Align(lipgloss.Center),
		ConfirmQ:    lipgloss.NewStyle().Foreground(fg),
		ConfirmSub:  lipgloss.NewStyle().Foreground(faint),
		ConfirmKeys: lipgloss.NewStyle().Foreground(dim),
		ConfirmY:    lipgloss.NewStyle().Foreground(color(colorWarn)).Bold(true),
		ConfirmN:    lipgloss.NewStyle().Foreground(faint).Bold(true),

		Footer:    lipgloss.NewStyle().Foreground(border).BorderTop(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(border).PaddingTop(0),
		HintKey:   lipgloss.NewStyle().Foreground(color(colorHintKey)),
		HintDsc:   lipgloss.NewStyle().Foreground(faint),
		ToastOK:   lipgloss.NewStyle().Foreground(color(colorOK)),
		ToastGo:   lipgloss.NewStyle().Foreground(acc),
		ToastWarn: lipgloss.NewStyle().Foreground(color(colorWarn)),
		ToastMut:  lipgloss.NewStyle().Foreground(faint),
	}

	// Selected alias colour differs by variant: rich highlights to white.
	if variant == VariantRich {
		s.AliasSel = lipgloss.NewStyle().Foreground(color("#ffffff")).Bold(true)
	}

	// List/detail panes: framed and rich sit in rounded borders; minimal is
	// borderless (the detail pane gets a single left divider, handled in view).
	switch variant {
	case VariantFramed:
		s.ListPane = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1)
		s.DetailPane = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1)
	case VariantRich:
		s.ListPane = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1)
		s.DetailPane = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(color(blend(colorBorder, accent, 0.35))).Padding(0, 1)
	default: // minimal
		s.ListPane = lipgloss.NewStyle()
		s.DetailPane = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(border).PaddingLeft(2)
	}
	return s
}

// chip renders an env chip for the given env using this theme. Filled (rich)
// chips get a solid tinted background; outline chips colour the text on a faint
// wash blended over the row background (bg, or the screen colour when bg is
// empty) so the chip integrates with a selected row's band instead of reading
// as a black box. An empty/unset env yields "" (no chip at all).
func (s Styles) chip(env, bg string) string {
	hex, ok := envColor(env, s.EnvOverrides)
	if !ok {
		return ""
	}
	label := " " + env + " "
	if s.Variant == VariantRich {
		return lipgloss.NewStyle().
			Foreground(color(chipFG)).
			Background(color(hex)).
			Render(label)
	}
	base := colorBG
	if bg != "" {
		base = bg
	}
	return lipgloss.NewStyle().
		Foreground(color(hex)).
		Background(color(blend(base, hex, 0.18))).
		Render(label)
}
