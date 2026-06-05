package tui

import "testing"

// newForm builds a form with the given field values, mirroring how the TUI
// seeds f.values during editing.
func newForm(vals map[string]string) formModel {
	return formModel{values: vals}
}

func TestInsertDropsControlRunes(t *testing.T) {
	// A NUL (e.g. from Ctrl+Space on some terminals) must not enter a field.
	f := formModel{values: map[string]string{}, focus: 0} // alias field
	f.insert("pr")
	f.insert("\x00")
	f.insert("od\x00")
	if got := f.values["alias"]; got != "prod" {
		t.Fatalf("alias = %q, want %q", got, "prod")
	}
}

func TestToHostDerivesAliasFromHostName(t *testing.T) {
	// Blank alias falls back to the HostName so the Host line is a usable
	// connection target (e.g. `ssh 10.0.0.7`), not "unnamed".
	h := newForm(map[string]string{"host": "10.0.0.7"}).toHost("config")
	if h.Alias != "10.0.0.7" {
		t.Fatalf("alias = %q, want %q", h.Alias, "10.0.0.7")
	}
	if h.HostName != "10.0.0.7" {
		t.Fatalf("hostname = %q, want %q", h.HostName, "10.0.0.7")
	}
}

func TestToHostInheritsHostNameFromAlias(t *testing.T) {
	// Blank HostName mirrors the alias, matching ssh's implicit fallback and
	// keeping the list row from rendering "(no hostname)".
	h := newForm(map[string]string{"alias": "web"}).toHost("config")
	if h.HostName != "web" {
		t.Fatalf("hostname = %q, want %q", h.HostName, "web")
	}
	if h.Alias != "web" {
		t.Fatalf("alias = %q, want %q", h.Alias, "web")
	}
}

func TestToHostLeavesBlankUserUnset(t *testing.T) {
	// A blank User must stay empty so the writer omits the directive and ssh
	// uses its own default (the local username) instead of forcing root.
	h := newForm(map[string]string{"host": "10.0.0.7"}).toHost("config")
	if h.User != "" {
		t.Fatalf("user = %q, want empty", h.User)
	}
}

func TestToHostKeepsExplicitAlias(t *testing.T) {
	h := newForm(map[string]string{"alias": "web", "host": "10.0.0.7"}).toHost("config")
	if h.Alias != "web" {
		t.Fatalf("alias = %q, want %q", h.Alias, "web")
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		vals    map[string]string
		wantErr bool
	}{
		{"both empty", map[string]string{}, true},
		{"both whitespace", map[string]string{"alias": "  ", "host": " "}, true},
		{"alias only", map[string]string{"alias": "web"}, false},
		{"host only", map[string]string{"host": "10.0.0.7"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := newForm(c.vals).validate() != ""; got != c.wantErr {
				t.Fatalf("validate() err=%v, want err=%v", got, c.wantErr)
			}
		})
	}
}
