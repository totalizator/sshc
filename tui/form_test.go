package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/totalizator/sshc/ssh"
)

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
		{"space in alias", map[string]string{"alias": "my server"}, true},
		{"space in fallback host-as-alias", map[string]string{"host": "my host"}, true},
		{"space in host but explicit alias ok", map[string]string{"alias": "web", "host": "10.0.0.7"}, false},
		{"startup tmux", map[string]string{"alias": "web", "start": "tmux:main"}, false},
		{"startup blank", map[string]string{"alias": "web", "start": "  "}, false},
		{"startup bad name", map[string]string{"alias": "web", "start": "tmux:my session"}, true},
		{"startup unknown kind", map[string]string{"alias": "web", "start": "mosh:x"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := newForm(c.vals).validate() != ""; got != c.wantErr {
				t.Fatalf("validate() err=%v, want err=%v", got, c.wantErr)
			}
		})
	}
}

func TestStartupRoundTripsThroughForm(t *testing.T) {
	in := ssh.Host{Alias: "web", HostName: "10.0.0.7", Start: ssh.Startup{Kind: ssh.StartTmux, Name: "main", Share: true}}
	f := formFromHost(in, "edit", in.Alias, true)
	if got := f.values["start"]; got != "tmux:main:share" {
		t.Fatalf("start field = %q", got)
	}
	if out := f.toHost("config"); out.Start != in.Start {
		t.Fatalf("toHost start = %+v, want %+v", out.Start, in.Start)
	}
}

// Editing a host on the desktop must keep what the form doesn't show: the
// favourite, last-used and the app's after-login commands.
func TestEditKeepsMetadataTheFormDoesntShow(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config"
	if err := os.WriteFile(path, []byte("Host web #sshc fav=1 used=1717000000\n  HostName 10.0.0.7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hosts, _ := ssh.Load([]string{path})
	f := formFromHost(hosts[0], "edit", "web", true)
	f.values["user"] = "ops"
	if err := ssh.Update(path, "web", f.toHost(path)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	for _, want := range []string{"fav=1", "used=1717000000", "User ops"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("after edit, config lacks %q:\n%s", want, raw)
		}
	}

	// A clone starts unpinned and unused.
	c := formFromHost(hosts[0], "clone", "web-2", false).toHost(path)
	if c.Fav || !c.LastUsed.IsZero() {
		t.Errorf("clone kept fav/used: %+v", c)
	}
}

func TestRunStepsSurviveEditAndClone(t *testing.T) {
	in := ssh.Host{Alias: "web", HostName: "10.0.0.7", Run: []ssh.RunStep{{Cmd: "cd /srv"}, {Cmd: "htop", Off: true}}}
	if out := formFromHost(in, "edit", "web", true).toHost("config"); len(out.Run) != 2 || !out.Run[1].Off {
		t.Fatalf("edit Run = %+v", out.Run)
	}
	if out := formFromHost(in, "clone", "web-2", false).toHost("config"); len(out.Run) != 2 {
		t.Fatalf("clone Run = %+v", out.Run)
	}
}

func TestForwardsSurviveEditAndClone(t *testing.T) {
	f, err := ssh.ParseForwardFlag("-L 8080:localhost:80")
	if err != nil {
		t.Fatal(err)
	}
	in := ssh.Host{Alias: "web", HostName: "10.0.0.7", Forwards: []ssh.Forward{f}}
	if out := formFromHost(in, "edit", "web", true).toHost("config"); len(out.Forwards) != 1 {
		t.Fatalf("edit Forwards = %+v", out.Forwards)
	}
	if out := formFromHost(in, "clone", "web-2", false).toHost("config"); len(out.Forwards) != 1 {
		t.Fatalf("clone Forwards = %+v", out.Forwards)
	}
}
