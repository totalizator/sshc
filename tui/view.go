package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// appStyle is the outer screen margin (matches the design's screen padding).
var appStyle = lipgloss.NewStyle().Padding(1, 2)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	innerW := m.width - appStyle.GetHorizontalFrameSize()
	innerH := m.height - appStyle.GetVerticalFrameSize()
	if innerW < 24 {
		innerW = 24
	}
	if innerH < 8 {
		innerH = 8
	}

	// Modal overlays take the whole frame, centred.
	if m.showHelp {
		return m.overlay(m.helpBox(), innerW, innerH)
	}
	if m.showSettings {
		return m.overlay(m.settingsBox(), innerW, innerH)
	}
	if m.state == stateConfirm {
		return m.overlay(m.confirmBox(m.confirmHost), innerW, innerH)
	}
	if m.state == statePromptUser && m.promptHost != nil {
		return m.overlay(m.userPromptBox(), innerW, innerH)
	}

	top := m.topChrome(innerW)
	bottom := m.footer(innerW)
	mainH := innerH - lipgloss.Height(top) - lipgloss.Height(bottom) - 2 // 2 blank separators
	if mainH < 3 {
		mainH = 3
	}

	var main string
	if m.state == stateForm {
		main = lipgloss.NewStyle().Width(innerW).Height(mainH).Render(m.formView(m.form, innerW))
	} else {
		main = m.mainArea(innerW, mainH)
	}

	body := lipgloss.JoinVertical(lipgloss.Left, top, "", main, "", bottom)
	return appStyle.Render(body)
}

// topChrome renders the brand bar and the search bar.
func (m Model) topChrome(w int) string {
	st := m.styles
	brandLeft := st.Brand.Render("sshc")
	if v := strings.TrimSpace(m.opts.Version); v != "" {
		brandLeft += " " + st.BrandVer.Render(v)
	}
	count := fmt.Sprintf("%d hosts", len(m.hosts))
	brand := joinLR(brandLeft, st.BrandCount.Render(count), w, "")

	search := m.search.View()
	if m.query != "" {
		n := len(m.visible)
		label := fmt.Sprintf("%d matches", n)
		if n == 1 {
			label = "1 match"
		}
		search = joinLR(search, st.SearchCount.Render(label), w, "")
	}
	return brand + "\n\n" + search
}

// mainArea renders the list, optionally split with the detail panel.
func (m Model) mainArea(w, h int) string {
	h0, hasSel := m.selectedHost()
	if m.showDetail && hasSel {
		detailOuter := detailWidthCols + m.styles.DetailPane.GetHorizontalFrameSize()
		gap := 2
		listW := w - detailOuter - gap
		if listW < 32 {
			listW = 32
			detailOuter = w - listW - gap
		}
		list := m.listPane(listW, h)
		detail := lipgloss.NewStyle().MaxHeight(h).Render(m.detailView(h0, detailOuter))
		return lipgloss.JoinHorizontal(lipgloss.Top, list, strings.Repeat(" ", gap), detail)
	}
	return m.listPane(w, h)
}

// footer renders the toast line and the hint bar.
func (m Model) footer(w int) string {
	toastLine := " "
	if m.toast.text != "" {
		toastLine = m.toastStyle().Render(m.toast.text)
	}
	hints := []struct{ k, d string }{
		{"type", "search"}, {"↵", "connect"}, {"^n", "new"}, {"^e", "edit"},
		{"^l", "clone"}, {"^f", "fav"}, {"^d", "delete"}, {"^v", "verbose"},
		{"tab", "detail"}, {"^t", "settings"}, {"?", "help"},
	}
	sep := m.styles.HintDsc.Render(" · ")
	segs := make([]string, len(hints))
	for i, hn := range hints {
		segs[i] = m.styles.HintKey.Render(hn.k) + " " + m.styles.HintDsc.Render(hn.d)
	}
	bar := m.styles.Footer.Width(w).Render(strings.Join(segs, sep))
	return toastLine + "\n" + bar
}

func (m Model) toastStyle() lipgloss.Style {
	switch m.toast.kind {
	case "ok":
		return m.styles.ToastOK
	case "go":
		return m.styles.ToastGo
	case "warn":
		return m.styles.ToastWarn
	default:
		return m.styles.ToastMut
	}
}

// overlay centres box within the inner frame and applies the screen margin.
func (m Model) overlay(box string, w, h int) string {
	placed := lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "))
	return appStyle.Render(placed)
}
