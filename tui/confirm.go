package tui

import (
	"strings"

	"github.com/totalizator/sshc/ssh"
)

// confirmBox renders the centred delete-confirmation dialog content (the box
// itself; the model centres it over the screen).
func (m Model) confirmBox(h ssh.Host) string {
	st := m.styles
	q := st.ConfirmQ.Render("Delete ") + st.DetailTitle.Render(h.Alias) + st.ConfirmQ.Render("?")
	sub := st.ConfirmSub.Render("This rewrites " + h.Source + " (a .sshc.bak backup is kept).")
	keys := st.ConfirmKeys.Render("") +
		st.ConfirmY.Render("y") + st.ConfirmKeys.Render(" delete   ·   ") +
		st.ConfirmN.Render("n") + st.ConfirmKeys.Render(" / esc cancel")
	return st.ConfirmBox.Render(q + "\n" + sub + "\n\n" + keys)
}

// userPromptBox renders the centred connect-time username prompt shown when the
// selected host has no saved User. A live "→ user@host" preview shows the
// resulting target, dimming the local-default username until one is typed.
func (m Model) userPromptBox() string {
	st := m.styles
	h := m.promptHost
	title := st.ConfirmQ.Render("Connect to ") + st.DetailTitle.Render(h.Alias)
	sub := st.ConfirmSub.Render("No user is set — type one, or leave blank for the ssh default.")

	input := m.userInput.View()

	// Live target preview: typed username in full colour, else the local
	// default dimmed so it's clear what an empty value resolves to.
	userPart := st.DetailTitle.Render(strings.TrimSpace(m.userInput.Value()))
	if strings.TrimSpace(m.userInput.Value()) == "" {
		def := currentUsername()
		if def == "" {
			def = "default user"
		}
		userPart = st.ConfirmSub.Render(def)
	}
	preview := st.CmdPrompt.Render("→ ") + userPart + st.ConfirmSub.Render("@"+h.HostName)

	keys := st.ConfirmKeys.Render("") +
		st.ConfirmY.Render("↵") + st.ConfirmKeys.Render(" connect   ·   ") +
		st.ConfirmN.Render("esc") + st.ConfirmKeys.Render(" cancel")
	return st.ConfirmBox.Render(title + "\n" + sub + "\n\n" + input + "\n" + preview + "\n\n" + keys)
}
