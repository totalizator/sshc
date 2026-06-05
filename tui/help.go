package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// helpKeys is the keybinding cheatsheet shown in the help overlay (handoff §2).
var helpKeys = [][2]string{
	{"type", "filter name / addr / tags"},
	{"↑ / ↓", "move selection"},
	{"enter", "connect (asks user if unset)"},
	{"tab", "toggle detail panel"},
	{"^v", "toggle verbose rows"},
	{"^n", "new connection"},
	{"^e", "edit selected"},
	{"^l", "clone selected"},
	{"^f", "toggle favourite"},
	{"^d", "delete selected"},
	{"^t", "settings"},
	{"?", "toggle this help"},
	{"^c", "quit"},
}

// helpBox renders the help overlay content box (centred over the screen by the
// model). Keys are laid out in two columns.
func (m Model) helpBox() string {
	st := m.styles
	rows := make([]string, 0, (len(helpKeys)+1)/2)
	for i := 0; i < len(helpKeys); i += 2 {
		left := m.helpEntry(helpKeys[i])
		right := ""
		if i+1 < len(helpKeys) {
			right = m.helpEntry(helpKeys[i+1])
		}
		col := lipgloss.NewStyle().Width(32)
		rows = append(rows, col.Render(left)+col.Render(right))
	}

	body := st.HelpTitle.Render("keybindings") + "\n\n" +
		strings.Join(rows, "\n") + "\n\n" +
		st.HelpFoot.Render("press ? or esc to close")
	return st.HelpBox.Render(body)
}

func (m Model) helpEntry(kv [2]string) string {
	return m.styles.HelpKey.Render(kv[0]) + "  " + m.styles.HelpDesc.Render(kv[1])
}
