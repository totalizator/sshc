package tui

import (
	"strings"

	"github.com/totalizator/sshc/ssh"
)

// detailView renders the master/detail side panel for the selected host, in the
// variant's framed (or minimal-divider) pane sized to w columns.
func (m Model) detailView(h ssh.Host, w int) string {
	st := m.styles
	innerW := w - st.DetailPane.GetHorizontalFrameSize()
	if innerW < detailMinWidth {
		innerW = detailMinWidth
	}

	var b strings.Builder

	// head: ★ alias  ……  env chip
	title := h.Alias
	if h.Fav {
		title = "★ " + h.Alias
	}
	head := joinLR(st.DetailTitle.Render(title), st.chip(h.Env, ""), innerW, "")
	b.WriteString(head)
	b.WriteString("\n\n")

	b.WriteString(m.section("connection"))
	b.WriteString(m.kv("HostName", h.HostName, false))
	b.WriteString(m.kv("User", orDash(h.User), h.User == ""))
	b.WriteString(m.kv("Port", h.DisplayPort(), false))

	b.WriteString(m.section("auth & routing"))
	if h.IdentityFile != "" {
		b.WriteString(m.kv("IdentityFile", h.IdentityFile, false))
	} else {
		b.WriteString(m.kv("IdentityFile", "(ssh-agent / default keys)", true))
	}
	proxy := h.Proxy()
	b.WriteString(m.kv("ProxyJump", orPlaceholder(proxy, "(direct)"), proxy == ""))

	b.WriteString(m.section("meta"))
	b.WriteString(m.kv("Tags", tagList(h.Tags), len(h.Tags) == 0))
	b.WriteString(m.kv("Last used", ssh.LastUsedLabel(h.LastUsed), h.LastUsed.IsZero()))
	access := "editable"
	if !h.Editable {
		access = "read-only"
	}
	b.WriteString(m.kv("Access", access, !h.Editable))

	b.WriteString(m.section("resolved command"))
	cmd := st.CmdPrompt.Render("$ ") + st.CmdBody.Render(resolvedCommand(h))
	b.WriteString(st.CmdBox.Width(innerW - st.CmdBox.GetHorizontalBorderSize()).Render(cmd))
	b.WriteString("\n\n")
	b.WriteString(st.DetailHint.Render("enter ↵ connect · ^e edit · ^l clone"))

	// Content lines are built to innerW; the pane wraps its own frame around
	// them (see the note in listPane on lipgloss Width semantics).
	return st.DetailPane.Render(b.String())
}

func (m Model) section(label string) string {
	return "\n" + m.styles.DetailSec.Render(strings.ToUpper(label)) + "\n"
}

func (m Model) kv(key, val string, mut bool) string {
	v := m.styles.KVVal
	if mut {
		v = m.styles.KVMut
	}
	return m.styles.KVKey.Render(key) + " " + v.Render(val) + "\n"
}

// resolvedCommand mirrors the design's buildCmd: ssh [-i id] [-J proxy]
// user@host [-p port], emitting only the non-default flags.
func resolvedCommand(h ssh.Host) string {
	parts := []string{"ssh"}
	if h.IdentityFile != "" {
		parts = append(parts, "-i "+h.IdentityFile)
	}
	if h.ProxyJump != "" {
		parts = append(parts, "-J "+h.ProxyJump)
	}
	target := h.HostName
	if u := strings.TrimSpace(h.User); u != "" {
		target = u + "@" + h.HostName
	}
	parts = append(parts, target)
	if p := h.DisplayPort(); p != "22" {
		parts = append(parts, "-p "+p)
	}
	return strings.Join(parts, " ")
}

func tagList(tags []string) string {
	if len(tags) == 0 {
		return "—"
	}
	return hashTags(tags)
}

// hashTags renders tags in stored order as "#a #b" (the design preserves order).
func hashTags(tags []string) string {
	hashed := make([]string, len(tags))
	for i, t := range tags {
		hashed[i] = "#" + t
	}
	return strings.Join(hashed, " ")
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func orPlaceholder(s, ph string) string {
	if strings.TrimSpace(s) == "" {
		return ph
	}
	return s
}
