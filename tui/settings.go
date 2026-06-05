package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// settingItem is one editable row in the live settings overlay. get renders the
// current value; set cycles it by a delta; sync (optional) mirrors the change
// into live session state so the preview updates immediately.
type settingItem struct {
	label string
	get   func(Settings) string
	set   func(*Settings, int)
	sync  func(*Model)
}

var (
	variantOpts = []string{"minimal", "framed", "rich"}
	themeOpts   = []string{"amber", "teal", "green", "magenta"}
	densityOpts = []string{"comfortable", "compact"}
)

var settingItems = []settingItem{
	{
		label: "Style",
		get:   func(s Settings) string { return string(s.Variant) },
		set:   func(s *Settings, d int) { s.Variant = ParseVariant(cycleStr(string(s.Variant), variantOpts, d)) },
	},
	{
		label: "Theme",
		get:   func(s Settings) string { return s.Theme },
		set:   func(s *Settings, d int) { s.Theme = cycleStr(s.Theme, themeOpts, d) },
	},
	{
		label: "Density",
		get:   func(s Settings) string { return string(s.Density) },
		set:   func(s *Settings, d int) { s.Density = ParseDensity(cycleStr(string(s.Density), densityOpts, d)) },
	},
	{
		label: "Detail by default",
		get:   func(s Settings) string { return onOff(s.DetailDefault) },
		set:   func(s *Settings, _ int) { s.DetailDefault = !s.DetailDefault },
		sync:  func(m *Model) { m.showDetail = m.settings.DetailDefault },
	},
	{
		label: "Pin favourites",
		get:   func(s Settings) string { return onOff(s.PinFavorites) },
		set:   func(s *Settings, _ int) { s.PinFavorites = !s.PinFavorites },
		sync:  func(m *Model) { m.applyFilter() },
	},
	{
		label: "Verbose rows",
		get:   func(s Settings) string { return onOff(s.VerboseDefault) },
		set:   func(s *Settings, _ int) { s.VerboseDefault = !s.VerboseDefault },
		sync:  func(m *Model) { m.verbose = m.settings.VerboseDefault },
	},
}

// updateSettings handles keys while the settings overlay is open. Esc (or ^t)
// closes and persists; arrows move/cycle and re-theme live.
func (m Model) updateSettings(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch km.String() {
	case "esc", "ctrl+t", "ctrl+c":
		m.showSettings = false
		if err := SaveSettings(m.settings); err != nil {
			return m.flash("settings not saved: "+err.Error(), "warn")
		}
		return m.flash("settings saved → "+ConfigPath(), "ok")
	case "up", "ctrl+k":
		if m.settingsFocus > 0 {
			m.settingsFocus--
		}
	case "down", "ctrl+j":
		if m.settingsFocus < len(settingItems)-1 {
			m.settingsFocus++
		}
	case "left", "h":
		m.changeSetting(-1)
	case "right", "l", "enter", "tab":
		m.changeSetting(1)
	}
	return m, nil
}

func (m *Model) changeSetting(d int) {
	item := settingItems[m.settingsFocus]
	item.set(&m.settings, d)
	m.rebuildStyles()
	if item.sync != nil {
		item.sync(m)
	}
}

// settingsRowW is the fixed width of a settings row, so the overlay box keeps a
// stable size as values change width (e.g. "compact" vs "comfortable").
const settingsRowW = 40

// settingsBox renders the centred settings overlay (placed by the model).
func (m Model) settingsBox() string {
	st := m.styles
	rows := make([]string, len(settingItems))
	for i, it := range settingItems {
		focused := i == m.settingsFocus

		marker := "  "
		labelStyle := st.HelpDesc
		valText := "‹ " + it.get(m.settings) + " ›"
		valStyle := st.KVVal
		if focused {
			marker = lipgloss.NewStyle().Foreground(color(st.Accent)).Render("› ")
			labelStyle = lipgloss.NewStyle().Foreground(color(st.Accent))
			valStyle = lipgloss.NewStyle().Foreground(color(st.Accent)).Bold(true)
		}
		label := lipgloss.NewStyle().Width(20).Render(labelStyle.Render(it.label))
		row := marker + label + valStyle.Render(valText)
		rows[i] = lipgloss.NewStyle().Width(settingsRowW).MaxWidth(settingsRowW).Render(row)
	}

	body := st.HelpTitle.Render("settings") + "\n\n" +
		strings.Join(rows, "\n") + "\n\n" +
		st.HelpFoot.Render("↑↓ move · ←→ change · esc save")
	return st.HelpBox.Render(body)
}

// cycleStr returns the option after cur in opts, wrapping; an unknown cur starts
// the cycle from the first option.
func cycleStr(cur string, opts []string, d int) string {
	idx := 0
	for i, o := range opts {
		if o == cur {
			idx = i
			break
		}
	}
	n := len(opts)
	return opts[((idx+d)%n+n)%n]
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
