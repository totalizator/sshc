package ssh

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/kevinburke/ssh_config"
)

// Forward is a TCP port forward from a host block's LocalForward or
// RemoteForward directive. Plain ssh honours them on the desktop by itself;
// front ends that make connections in-process (the Android app) start them on
// connect. Host.Forwards is written back on Add and Update (setForwards) when
// it isn't nil: the lines are rewritten only when the list changed, and forward
// lines it can't represent (Host.OtherForwards) are always kept as written.
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

// Directive is f as a config line: "LocalForward" or "RemoteForward" and its
// value, e.g. "8080 localhost:80".
func (f Forward) Directive() (key, value string) {
	key = "LocalForward"
	if f.Remote {
		key = "RemoteForward"
	}
	listen := strconv.Itoa(f.Port)
	if f.Bind != "" {
		listen = bracket(f.Bind) + ":" + listen
	}
	return key, listen + " " + bracket(f.Host) + ":" + strconv.Itoa(f.HostPort)
}

// ParseForwardFlag reads a forward in ssh flag form, as String writes it:
// "-L [bind:]port:host:hostport" or "-R …". IPv6 addresses go in brackets.
func ParseForwardFlag(s string) (Forward, error) {
	s = strings.TrimSpace(s)
	var remote bool
	switch {
	case strings.HasPrefix(s, "-L"):
	case strings.HasPrefix(s, "-R"):
		remote = true
	default:
		return Forward{}, fmt.Errorf("want -L or -R, got %q", s)
	}
	parts := splitOutsideBrackets(strings.TrimSpace(s[2:]))
	var listen, dest string
	switch len(parts) {
	case 3:
		listen, dest = parts[0], parts[1]+":"+parts[2]
	case 4:
		listen, dest = parts[0]+":"+parts[1], parts[2]+":"+parts[3]
	default:
		return Forward{}, fmt.Errorf("want [bind:]port:host:hostport, got %q", s)
	}
	if strings.ContainsAny(listen, " \t") || strings.ContainsAny(dest, " \t") {
		return Forward{}, fmt.Errorf("no spaces in a forward: %q", s)
	}
	return ParseForward(remote, listen+" "+dest)
}

// splitOutsideBrackets splits s at colons that aren't inside [ ].
func splitOutsideBrackets(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
		case ':':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
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

// setForwards makes block's TCP forward lines fwd, in order. Lines are left
// as they are when they already say the same; otherwise the ones that parse
// are removed and fwd goes where the first of them was (or after the last
// directive). Forward lines that don't parse (unix sockets, the SOCKS form of
// RemoteForward, typos) and DynamicForward are never touched.
func setForwards(block *ssh_config.Host, fwd []Forward) {
	have, _ := forwardsOf(block)
	if sameForwards(have, fwd) {
		return
	}
	at := -1
	kept := make([]ssh_config.Node, 0, len(block.Nodes)+len(fwd))
	for _, node := range block.Nodes {
		if kv, ok := node.(*ssh_config.KV); ok {
			remote := strings.EqualFold(kv.Key, "RemoteForward")
			if remote || strings.EqualFold(kv.Key, "LocalForward") {
				if _, err := ParseForward(remote, kv.Value); err == nil {
					if at == -1 {
						at = len(kept)
					}
					continue
				}
			}
		}
		kept = append(kept, node)
	}
	block.Nodes = kept
	if len(fwd) == 0 {
		return
	}
	if at == -1 {
		for _, f := range fwd {
			key, value := f.Directive()
			block.Nodes = insertAfterLastKV(block.Nodes, &ssh_config.KV{Key: key, Value: value})
		}
		return
	}
	nodes := make([]ssh_config.Node, 0, len(block.Nodes)+len(fwd))
	nodes = append(nodes, block.Nodes[:at]...)
	for _, f := range fwd {
		key, value := f.Directive()
		nodes = append(nodes, &ssh_config.KV{Key: key, Value: value})
	}
	block.Nodes = append(nodes, block.Nodes[at:]...)
}

func sameForwards(a, b []Forward) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
