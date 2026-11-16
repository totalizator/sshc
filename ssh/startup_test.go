package ssh

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseStartup(t *testing.T) {
	cases := []struct {
		spec    string
		want    Startup
		wantErr bool
	}{
		{spec: "", want: Startup{}},
		{spec: "   ", want: Startup{}},
		{spec: "tmux:main", want: Startup{Kind: StartTmux, Name: "main"}},
		{spec: "tmux:main:share", want: Startup{Kind: StartTmux, Name: "main", Share: true}},
		{spec: "screen:work_1", want: Startup{Kind: StartScreen, Name: "work_1"}},
		{spec: "screen:w:share", want: Startup{Kind: StartScreen, Name: "w", Share: true}},
		{spec: "zellij:dev-box", want: Startup{Kind: StartZellij, Name: "dev-box"}},
		{spec: "cmd:htop -d 5", want: Startup{Kind: StartCmd, Cmd: "htop -d 5"}},
		{spec: "cmd:a:b c", want: Startup{Kind: StartCmd, Cmd: "a:b c"}},
		{spec: "tmux:", wantErr: true},
		{spec: "tmux:has space", wantErr: true},
		{spec: "tmux:a.b", wantErr: true},
		{spec: "tmux:x;rm", wantErr: true},
		{spec: "tmux:" + strings.Repeat("a", 65), wantErr: true},
		{spec: "tmux:main:keep", wantErr: true},
		{spec: "zellij:main:share", wantErr: true},
		{spec: "cmd:", wantErr: true},
		{spec: "cmd:a\nb", wantErr: true},
		{spec: "mosh:main", wantErr: true},
		{spec: "main", wantErr: true},
	}
	for _, c := range cases {
		got, err := ParseStartup(c.spec)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseStartup(%q) = %+v, want an error", c.spec, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseStartup(%q): %v", c.spec, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseStartup(%q) = %+v, want %+v", c.spec, got, c.want)
		}
		// String gives back a spec that parses to the same value.
		again, err := ParseStartup(got.String())
		if err != nil || again != got {
			t.Errorf("round trip of %q via %q = %+v, %v", c.spec, got.String(), again, err)
		}
	}
}

func TestStartupCommand(t *testing.T) {
	cases := []struct {
		name    string
		s       Startup
		grouped bool
		want    []string // substrings, in order
		not     []string
	}{
		{name: "login shell", s: Startup{}},
		{name: "custom command runs as given", s: Startup{Kind: StartCmd, Cmd: "htop -d 5"}, want: []string{"htop -d 5"}},
		{
			name: "tmux takes over by default",
			s:    Startup{Kind: StartTmux, Name: "main"},
			want: []string{"sh -c '", "command -v tmux", `exec "${SHELL:-/bin/sh}" -l`, "exec tmux new-session -A -D -s main'"},
			not:  []string{"has-session"},
		},
		{
			name: "tmux share keeps other clients",
			s:    Startup{Kind: StartTmux, Name: "main", Share: true},
			want: []string{"exec tmux new-session -A -s main'"},
			not:  []string{"-D"},
		},
		{
			name:    "tmux grouped joins the existing session without detaching",
			s:       Startup{Kind: StartTmux, Name: "main"},
			grouped: true,
			want: []string{
				"if tmux has-session -t =main",
				`exec tmux new-session -t =main \; set-option destroy-unattached on`,
				"else exec tmux new-session -A -D -s main; fi'",
			},
		},
		{name: "screen takes over", s: Startup{Kind: StartScreen, Name: "w"}, want: []string{"command -v screen", "exec screen -D -R -S w'"}},
		{name: "screen share", s: Startup{Kind: StartScreen, Name: "w", Share: true}, want: []string{"exec screen -x -R -S w'"}},
		{name: "zellij", s: Startup{Kind: StartZellij, Name: "z"}, grouped: true, want: []string{"command -v zellij", "exec zellij attach --create z'"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.s.Command(c.grouped)
			if len(c.want) == 0 && got != "" {
				t.Fatalf("Command = %q, want empty", got)
			}
			rest := got
			for _, w := range c.want {
				i := strings.Index(rest, w)
				if i < 0 {
					t.Fatalf("Command = %q, missing %q (in order)", got, w)
				}
				rest = rest[i+len(w):]
			}
			for _, n := range c.not {
				if strings.Contains(got, n) {
					t.Fatalf("Command = %q, must not contain %q", got, n)
				}
			}
			// The script sits in single quotes, so it must not contain any.
			if body := strings.TrimSuffix(strings.TrimPrefix(got, "sh -c '"), "'"); body != got && strings.Contains(body, "'") {
				t.Fatalf("Command = %q has a single quote inside the sh -c script", got)
			}
		})
	}
}

func TestStartRoundTripsThroughConfig(t *testing.T) {
	path := writeTemp(t, "config", "")
	in := Host{Alias: "box", HostName: "10.0.0.2", Start: Startup{Kind: StartCmd, Cmd: "htop -d 5 # 100%"}}
	if err := Add(path, in); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "start=cmd:htop%20-d%205%20#%20100%25") {
		t.Fatalf("start token not escaped as expected:\n%s", raw)
	}
	hosts, _ := Load([]string{path})
	got, ok := findHost(hosts, "box")
	if !ok || got.Start != in.Start {
		t.Fatalf("start = %+v, want %+v", got.Start, in.Start)
	}
}

func TestMetaWritesKeepStartAndUnknownTokens(t *testing.T) {
	path := writeTemp(t, "config", "Host box #sshc env=dev start=tmux:main future=x start2=y\n  HostName 10.0.0.3\n")

	if err := TouchLastUsed(path, "box", time.Unix(1717000000, 0)); err != nil {
		t.Fatal(err)
	}
	if err := SetFav(path, "box", true); err != nil {
		t.Fatal(err)
	}
	hosts, _ := Load([]string{path})
	h, _ := findHost(hosts, "box")
	if h.Start != (Startup{Kind: StartTmux, Name: "main"}) || h.Env != "dev" || !h.Fav {
		t.Fatalf("after touch + fav: %+v", h)
	}

	// An edit through Update keeps tokens this version doesn't know.
	h.Start = Startup{Kind: StartTmux, Name: "main", Share: true}
	if err := Update(path, "box", h); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	line := strings.SplitN(string(raw), "\n", 2)[0]
	for _, want := range []string{"start=tmux:main:share", "future=x", "start2=y", "fav=1", "used=1717000000"} {
		if !strings.Contains(line, want) {
			t.Errorf("Host line %q lacks %q", line, want)
		}
	}
}

func TestUnreadableStartIsKept(t *testing.T) {
	// A spec from a newer sshc that this version can't parse survives a write.
	path := writeTemp(t, "config", "Host box #sshc start=mosh:main\n  HostName 10.0.0.4\n")
	if err := SetFav(path, "box", true); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "start=mosh:main") {
		t.Fatalf("unknown start spec dropped:\n%s", raw)
	}
	hosts, _ := Load([]string{path})
	if h, _ := findHost(hosts, "box"); !h.Start.IsZero() {
		t.Fatalf("unknown spec parsed as %+v", h.Start)
	}
}

func TestStripMetaRemovesStart(t *testing.T) {
	path := writeTemp(t, "config", "Host box #sshc start=tmux:main\n  HostName 10.0.0.5\n")
	n, err := StripMeta(path)
	if err != nil || n != 1 {
		t.Fatalf("StripMeta = %d, %v", n, err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "sshc") {
		t.Fatalf("metadata left behind:\n%s", raw)
	}
}

func TestTokenEscaping(t *testing.T) {
	for _, s := range []string{"", "plain", "a b", "100%", "%41", "tab\there", "x\x00y", "ünï cødé"} {
		esc := escapeToken(s)
		if strings.ContainsAny(esc, " \t\x00") {
			t.Errorf("escapeToken(%q) = %q still has whitespace or NUL", s, esc)
		}
		if got := unescapeToken(esc); got != s {
			t.Errorf("unescapeToken(escapeToken(%q)) = %q", s, got)
		}
	}
	// A malformed escape is kept literally instead of failing.
	if got := unescapeToken("50%zz%4"); got != "50%zz%4" {
		t.Errorf("unescapeToken malformed = %q", got)
	}
}
