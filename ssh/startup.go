package ssh

import (
	"fmt"
	"regexp"
	"strings"
)

// Startup is what an interactive session runs in place of the login shell,
// stored per host as the "start=" metadata token (see meta.go):
//
//	Host box #sshc start=tmux:main
//
// It lives in sshc metadata rather than as an OpenSSH RemoteCommand because
// plain ssh would honour RemoteCommand too, and OpenSSH refuses a command line
// while one is set ("Cannot execute command-line and remote command"), which
// breaks `ssh box uptime`, rsync and git over ssh. Only sshc front ends read
// the token, and they run it like `ssh -t box '<command>'`, so leaving the
// program (tmux detach, exit) ends the connection.
//
// Spec forms (String / ParseStartup):
//
//	tmux:<name>[:share]     attach to tmux session <name>, creating it if needed
//	screen:<name>[:share]   the same with GNU screen
//	zellij:<name>           the same with zellij
//	cmd:<command>           any command line, run as given
//
// By default a multiplexer takes the session over: other attached clients are
// detached, so the window isn't held to the smallest one. ":share" keeps them
// attached.
type Startup struct {
	Kind  string // StartTmux, StartScreen, StartZellij, StartCmd; "" for the login shell
	Name  string // session name (multiplexers)
	Cmd   string // command line (StartCmd)
	Share bool   // multiplexers: keep other clients attached
}

// Startup kinds.
const (
	StartTmux   = "tmux"
	StartScreen = "screen"
	StartZellij = "zellij"
	StartCmd    = "cmd"
)

// startNameRE limits multiplexer session names to characters that need no
// quoting in a shell and that tmux keeps as is (it rewrites '.' and ':').
var startNameRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// IsZero reports whether s is the default: the plain login shell.
func (s Startup) IsZero() bool { return s.Kind == "" }

// ParseStartup reads a spec as written by String. A blank spec is the login
// shell.
func ParseStartup(spec string) (Startup, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return Startup{}, nil
	}
	kind, rest, _ := strings.Cut(spec, ":")
	switch kind {
	case StartCmd:
		cmd := strings.TrimSpace(rest)
		if cmd == "" {
			return Startup{}, fmt.Errorf("startup command is empty")
		}
		if strings.ContainsAny(cmd, "\r\n") {
			return Startup{}, fmt.Errorf("startup command must be one line")
		}
		return Startup{Kind: StartCmd, Cmd: cmd}, nil
	case StartTmux, StartScreen, StartZellij:
		name, opt, _ := strings.Cut(rest, ":")
		if !startNameRE.MatchString(name) {
			return Startup{}, fmt.Errorf("session name %q: use 1-64 letters, digits, '-' or '_'", name)
		}
		s := Startup{Kind: kind, Name: name}
		switch opt {
		case "":
		case "share":
			if kind == StartZellij {
				return Startup{}, fmt.Errorf("zellij always shares its sessions; drop \":share\"")
			}
			s.Share = true
		default:
			return Startup{}, fmt.Errorf("unknown option %q (only \"share\")", opt)
		}
		return s, nil
	default:
		return Startup{}, fmt.Errorf("unknown startup %q: use tmux:<name>, screen:<name>, zellij:<name> or cmd:<command>", kind)
	}
}

// String renders s as a spec ParseStartup accepts; "" for the login shell.
func (s Startup) String() string {
	switch s.Kind {
	case "":
		return ""
	case StartCmd:
		return StartCmd + ":" + s.Cmd
	default:
		spec := s.Kind + ":" + s.Name
		if s.Share {
			spec += ":share"
		}
		return spec
	}
}

// Command returns the remote command line that starts s, or "" for the login
// shell. A multiplexer that isn't installed falls back to the login shell with
// a note on stderr, so a host without tmux still gives a usable session.
//
// grouped asks for a second, independent view of a tmux session another
// client of the same front end is already showing: a grouped session shares
// the windows but has its own current window, and is removed once it's
// detached. It doesn't detach anyone. Other kinds ignore it.
//
// The command is wrapped in `sh -c` so it works whatever the login shell is
// (fish, for one, doesn't take "${SHELL:-…}").
func (s Startup) Command(grouped bool) string {
	var tool, run string
	switch s.Kind {
	case "":
		return ""
	case StartCmd:
		return s.Cmd
	case StartTmux:
		tool = "tmux"
		attach := "tmux new-session -A -s " + s.Name
		if !s.Share {
			attach = "tmux new-session -A -D -s " + s.Name
		}
		run = "exec " + attach
		if grouped {
			run = "if tmux has-session -t =" + s.Name + " 2>/dev/null; then " +
				"exec tmux new-session -t =" + s.Name + " \\; set-option destroy-unattached on; " +
				"else exec " + attach + "; fi"
		}
	case StartScreen:
		tool = "screen"
		if s.Share {
			run = "exec screen -x -R -S " + s.Name
		} else {
			run = "exec screen -D -R -S " + s.Name
		}
	case StartZellij:
		tool = "zellij"
		run = "exec zellij attach --create " + s.Name
	default:
		return ""
	}
	script := "command -v " + tool + " >/dev/null 2>&1 || { echo \"sshc: " + tool +
		" not found, starting a shell\" >&2; exec \"${SHELL:-/bin/sh}\" -l; }; " + run
	return "sh -c '" + script + "'"
}
