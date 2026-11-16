package ssh

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/kevinburke/ssh_config"
)

// Forward is a TCP port forward from a host block's LocalForward or
// RemoteForward directive. sshc reads these but doesn't write them: they stay
// hand-edited directives, kept as they are when the block is rewritten (see
// modeledKeys), and plain ssh honours them on the desktop by itself. Front ends
// that make connections in-process (the Android app) start them on connect.
//
//	LocalForward  [bind_address:]port  host:hostport   # -L
//	RemoteForward [bind_address:]port  host:hostport   # -R
//
// IPv6 addresses go in brackets ("[::1]:8080"), as OpenSSH accepts them.
type Forward struct {
	Remote   bool   // RemoteForward (-R); LocalForward (-L) otherwise
	Bind     string // listen address as written: "" for the default (loopback), "*" for every interface
	Port     int    // listen port; 0 lets the listening side pick one
	Host     string // destination host, without brackets
	HostPort int    // destination port
}

// String renders f as the matching ssh flag, e.g. "-L 8080:localhost:80".
func (f Forward) String() string {
	flag := "-L "
	if f.Remote {
		flag = "-R "
	}
	listen := strconv.Itoa(f.Port)
	if f.Bind != "" {
		listen = bracket(f.Bind) + ":" + listen
	}
	return flag + listen + ":" + bracket(f.Host) + ":" + strconv.Itoa(f.HostPort)
}

// ListenAddr is the address to listen on, in Go's host:port form, with
// OpenSSH's defaults made explicit: no bind address listens on loopback only
// (127.0.0.1), and "*" on every interface (0.0.0.0).
func (f Forward) ListenAddr() string {
	bind := f.Bind
	switch bind {
	case "", "localhost":
		bind = "127.0.0.1"
	case "*":
		bind = "0.0.0.0"
	}
	return net.JoinHostPort(bind, strconv.Itoa(f.Port))
}

// TargetAddr is the destination in Go's host:port form.
func (f Forward) TargetAddr() string {
	return net.JoinHostPort(f.Host, strconv.Itoa(f.HostPort))
}

// ParseForward reads the value of a LocalForward (remote false) or
// RemoteForward (remote true) directive: two space-separated arguments,
// listener then destination. Unix socket paths and the one-argument
// RemoteForward (a SOCKS proxy on the server) aren't TCP forwards and are
// rejected, as is anything malformed.
func ParseForward(remote bool, value string) (Forward, error) {
	args := strings.Fields(value)
	if len(args) == 1 && remote {
		return Forward{}, fmt.Errorf("a RemoteForward with one argument is a SOCKS proxy, not a port forward")
	}
	if len(args) != 2 {
		return Forward{}, fmt.Errorf("want a listener and a destination, got %q", value)
	}
	if isSocketPath(args[0]) || isSocketPath(args[1]) {
		return Forward{}, fmt.Errorf("unix socket forwards aren't supported")
	}

	f := Forward{Remote: remote}
	var err error
	if strings.ContainsAny(args[0], ":[") {
		var port string
		if f.Bind, port, err = splitHostPort(args[0]); err != nil {
			return Forward{}, fmt.Errorf("listener %q: %w", args[0], err)
		}
		if f.Port, err = parsePort(port, true); err != nil {
			return Forward{}, fmt.Errorf("listener %q: %w", args[0], err)
		}
	} else if f.Port, err = parsePort(args[0], true); err != nil {
		return Forward{}, fmt.Errorf("listener %q: %w", args[0], err)
	}

	var port string
	if f.Host, port, err = splitHostPort(args[1]); err != nil {
		return Forward{}, fmt.Errorf("destination %q: %w", args[1], err)
	}
	if f.Host == "" {
		return Forward{}, fmt.Errorf("destination %q has no host", args[1])
	}
	if f.HostPort, err = parsePort(port, false); err != nil {
		return Forward{}, fmt.Errorf("destination %q: %w", args[1], err)
	}
	return f, nil
}

// forwardsOf collects a block's forward directives in file order: the ones
// that parse go to fwd, the rest (unix sockets, DynamicForward, typos) to
// other as the directive text, so a front end can say what it didn't start.
func forwardsOf(block *ssh_config.Host) (fwd []Forward, other []string) {
	for _, node := range block.Nodes {
		kv, ok := node.(*ssh_config.KV)
		if !ok {
			continue
		}
		var remote bool
		switch {
		case strings.EqualFold(kv.Key, "LocalForward"):
		case strings.EqualFold(kv.Key, "RemoteForward"):
			remote = true
		case strings.EqualFold(kv.Key, "DynamicForward"):
			other = append(other, "DynamicForward "+strings.Join(strings.Fields(kv.Value), " "))
			continue
		default:
			continue
		}
		f, err := ParseForward(remote, kv.Value)
		if err != nil {
			key := "LocalForward "
			if remote {
				key = "RemoteForward "
			}
			other = append(other, key+strings.Join(strings.Fields(kv.Value), " "))
			continue
		}
		fwd = append(fwd, f)
	}
	return fwd, other
}

func isSocketPath(s string) bool {
	return strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~") || strings.HasPrefix(s, "./")
}

// splitHostPort splits "host:port" or "[v6]:port". Unlike net.SplitHostPort it
// takes an empty host (":8080") as the default.
func splitHostPort(s string) (host, port string, err error) {
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end < 0 || !strings.HasPrefix(s[end+1:], ":") {
			return "", "", fmt.Errorf("want [address]:port")
		}
		return s[1:end], s[end+2:], nil
	}
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return "", "", fmt.Errorf("want host:port")
	}
	host = s[:i]
	if strings.Contains(host, ":") {
		return "", "", fmt.Errorf("put an IPv6 address in brackets")
	}
	return host, s[i+1:], nil
}

func parsePort(s string, allowZero bool) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 65535 || (n == 0 && !allowZero) {
		return 0, fmt.Errorf("bad port %q", s)
	}
	return n, nil
}

// bracket puts an IPv6 address in brackets for host:port text.
func bracket(host string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}
