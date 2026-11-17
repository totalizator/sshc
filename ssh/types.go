package ssh

import (
	"strings"
	"time"
)

// Host is a single resolved entry from an SSH config file.
type Host struct {
	Alias        string // the Host pattern / connection alias
	HostName     string
	User         string
	Port         string
	IdentityFile string
	ProxyJump    string
	ProxyCommand string

	// Port forwards from LocalForward / RemoteForward directives (forward.go).
	// Read-only: sshc keeps the directives as written and never writes these.
	Forwards []Forward
	// OtherForwards holds forward directives Forwards can't represent (unix
	// sockets, DynamicForward, malformed ones) as text, e.g. "DynamicForward 1080".
	OtherForwards []string

	// sshc-managed metadata. These are not native SSH keys; they are persisted
	// as a structured "#sshc …" comment on the Host line (see meta.go) so the
	// config stays valid for plain ssh.
	Tags     []string  // free-form tags for grouping/filtering
	Env      string    // free-form environment label (drives the env chip)
	Fav      bool      // pinned/favourite
	LastUsed time.Time // last connect time; zero when never used
	Start    Startup   // what a session runs instead of the login shell (startup.go)
	Run      []RunStep // commands the Android app types after login (run.go)

	// Source is the path of the config file this block was read from.
	Source string
	// Editable is true only for concrete single-pattern blocks that live in the
	// primary (writable) config. Host */Match/wildcard blocks and blocks from
	// read-only configs are never editable.
	Editable bool
}

// DisplayPort returns the port, defaulting to "22" when unset.
func (h Host) DisplayPort() string {
	if h.Port == "" {
		return "22"
	}
	return h.Port
}

// Proxy returns the proxy descriptor for display: ProxyJump takes precedence
// over ProxyCommand (they are mutually exclusive in practice).
func (h Host) Proxy() string {
	if h.ProxyJump != "" {
		return h.ProxyJump
	}
	return h.ProxyCommand
}

// Target is the "user@host:port" connection string used in list rows.
func (h Host) Target() string {
	host := h.HostName
	if host == "" {
		host = "(no hostname)"
	}
	if u := strings.TrimSpace(h.User); u != "" {
		host = u + "@" + host
	}
	return host + ":" + h.DisplayPort()
}
