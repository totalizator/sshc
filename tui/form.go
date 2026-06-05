package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/totalizator/sshc/ssh"
)

// formField describes one editable row of the add/edit form.
type formField struct {
	key, label, placeholder string
}

// formFields is the field order shown in the form (handoff §6 / ui.jsx). The
// User placeholder reflects what ssh uses when the field is left blank (the
// local login name) rather than suggesting root.
var formFields = []formField{
	{"alias", "Alias", "my-server"},
	{"host", "HostName", "10.0.0.1 or example.com"},
	{"user", "User", userPlaceholder()},
	{"port", "Port", "22"},
	{"identity", "IdentityFile", "~/.ssh/id_ed25519"},
	{"proxy", "ProxyJump", "bastion (optional)"},
	{"tags", "Tags", "web, prod (comma-sep)"},
	{"env", "Env", "prod, dev, edge…  (optional, free-form)"},
}

// userPlaceholder hints the ssh default (the local username) for the blank User
// field, so it's clear leaving it empty connects as that user — not root.
func userPlaceholder() string {
	if u := currentUsername(); u != "" {
		return u + " (default if blank)"
	}
	return "blank → ssh default"
}

// formModel is the state for the new/edit/clone form. Editing is append/
// backspace at the end of the focused field, mirroring the design mockup.
type formModel struct {
	title  string
	values map[string]string
	focus  int

	editAlias        string // non-empty only when editing in place
	origProxyCommand string // preserved when the form leaves ProxyJump empty
	err              string
}

// newFormModel seeds a blank "new connection" form.
func newFormModel() formModel {
	return formModel{
		title: "new connection",
		values: map[string]string{
			"port": "22",
		},
	}
}

// formFromHost seeds a form from an existing host. When clone is true the entry
// is treated as new (editAlias stays empty) and the alias is pre-suffixed.
func formFromHost(h ssh.Host, title, alias string, editing bool) formModel {
	f := formModel{
		title:            title,
		origProxyCommand: h.ProxyCommand,
		values: map[string]string{
			"alias":    alias,
			"host":     h.HostName,
			"user":     h.User,
			"port":     h.Port,
			"identity": h.IdentityFile,
			"proxy":    h.ProxyJump,
			"tags":     strings.Join(h.Tags, ", "),
			"env":      h.Env,
		},
	}
	if editing {
		f.editAlias = h.Alias
	}
	return f
}

func (f *formModel) moveFocus(d int) {
	f.focus += d
	if f.focus < 0 {
		f.focus = 0
	}
	if f.focus > len(formFields)-1 {
		f.focus = len(formFields) - 1
	}
}

func (f *formModel) curKey() string { return formFields[f.focus].key }

// insert appends typed text to the focused field, dropping control runes. Some
// terminals deliver Ctrl+Space (and similar) as a NUL rune; left in, it would
// be persisted to the config and corrupt env-chip colours / ssh parsing.
func (f *formModel) insert(s string) {
	if s = stripControl(s); s != "" {
		f.values[f.curKey()] += s
	}
}

func (f *formModel) backspace() {
	k := f.curKey()
	v := []rune(f.values[k])
	if len(v) > 0 {
		f.values[k] = string(v[:len(v)-1])
	}
}

// validate returns a human error when the form can't yield a usable host. The
// alias is derived from HostName when blank, so a host needs at least one of
// the two; everything else has a sensible default.
func (f formModel) validate() string {
	if strings.TrimSpace(f.values["alias"]) == "" && strings.TrimSpace(f.values["host"]) == "" {
		return "enter an alias or a hostname"
	}
	return ""
}

// toHost applies the design's defaulting rules and returns the resulting host
// plus the alias to write. Alias and HostName fill in for each other when one
// is blank (idiomatic ssh: with no HostName, ssh resolves the alias itself; the
// reverse makes the Host line double as the connection target). A blank User
// is left unset so ssh applies its own default (the local username) at connect
// time rather than forcing root. bad/empty port→default(22, left unset).
// submitForm rejects the both-empty case before this runs, so at least one of
// alias/host is always present here.
func (f formModel) toHost(source string) ssh.Host {
	val := func(k string) string { return strings.TrimSpace(f.values[k]) }

	alias := val("alias")
	if alias == "" {
		alias = val("host") // e.g. "10.0.0.7" → `Host 10.0.0.7` works as-is
	}
	host := val("host")
	if host == "" {
		host = alias // mirror ssh's implicit fallback so the list row resolves
	}
	user := val("user")
	port := val("port")
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		port = "" // unset → DisplayPort renders 22
	}

	var tags []string
	for _, t := range strings.Split(f.values["tags"], ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}

	proxyJump := val("proxy")
	proxyCommand := ""
	if proxyJump == "" {
		proxyCommand = f.origProxyCommand // carry through when ProxyJump unused
	}

	return ssh.Host{
		Alias:        alias,
		HostName:     host,
		User:         user,
		Port:         port,
		IdentityFile: val("identity"),
		ProxyJump:    proxyJump,
		ProxyCommand: proxyCommand,
		Tags:         tags,
		Env:          val("env"),
		Source:       source,
		Editable:     true,
	}
}

// view renders the form within width w.
func (m Model) formView(f formModel, w int) string {
	st := m.styles
	var b strings.Builder
	b.WriteString(st.FormTitle.Render(f.title))
	b.WriteString("\n\n")

	rowW := w
	if rowW > 64 {
		rowW = 64
	}

	for i, fld := range formFields {
		focused := i == f.focus
		label := st.FieldLabel.Render(fld.label)
		if focused {
			label = st.FieldLabelFocus.Render(fld.label)
		}

		val := f.values[fld.key]
		var input string
		if val == "" {
			input = st.FieldPH.Render(fld.placeholder)
		} else {
			input = st.FieldVal.Render(val)
		}
		if focused {
			input += caret(st)
		}

		row := label + " " + input
		rowStyle := st.FieldRow
		if focused {
			rowStyle = st.FieldRowFocus.Width(rowW)
		}
		b.WriteString(rowStyle.Render(row))
		b.WriteByte('\n')
	}

	b.WriteByte('\n')
	if f.err != "" {
		b.WriteString(st.FormErr.Render("⚠ " + f.err))
	} else {
		b.WriteString(st.FormHint.Render("↑↓ / tab move · ^s save · esc cancel"))
	}
	return b.String()
}

// caret renders the blinking-style accent caret used in inputs.
func caret(st Styles) string {
	return lipgloss.NewStyle().Foreground(color(st.Accent)).Render("▏")
}
