package ssh

import (
	"os"
	"strings"

	"github.com/kevinburke/ssh_config"
)

// Load parses every config file in paths (in order) and returns the combined
// list of hosts. The first path is treated as the primary (writable) config;
// all others are read-only. A path that does not exist yet is skipped silently
// rather than treated as an error, so sshc works on a fresh machine.
func Load(paths []string) ([]Host, error) {
	var hosts []Host
	for i, path := range paths {
		primary := i == 0
		fileHosts, err := loadFile(path, primary)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, fileHosts...)
	}
	return hosts, nil
}

func loadFile(path string, primary bool) ([]Host, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return nil, err
	}

	var hosts []Host
	for _, block := range cfg.Hosts {
		concrete := isConcrete(block)
		// sshc metadata rides on the Host line's trailing comment; it only makes
		// sense for single-pattern blocks (the same ones we treat as editable).
		tags, env, fav, lastUsed := parseMeta(block.EOLComment)
		for _, pat := range block.Patterns {
			alias := pat.String()
			if alias == "" || alias == "*" {
				continue
			}
			hosts = append(hosts, Host{
				Alias:        alias,
				HostName:     nodeValue(block, "HostName"),
				User:         nodeValue(block, "User"),
				Port:         nodeValue(block, "Port"),
				IdentityFile: nodeValue(block, "IdentityFile"),
				ProxyJump:    nodeValue(block, "ProxyJump"),
				ProxyCommand: nodeValue(block, "ProxyCommand"),
				Tags:         tags,
				Env:          env,
				Fav:          fav,
				LastUsed:     lastUsed,
				Source:       path,
				// Only a primary-config block with exactly one concrete pattern
				// maps cleanly to a single editable entry.
				Editable: primary && concrete && len(block.Patterns) == 1,
			})
		}
	}
	return hosts, nil
}

// isConcrete reports whether a host block is a concrete, editable entry: not a
// Match block and free of wildcard ('*', '?') or negated ('!') patterns.
func isConcrete(h *ssh_config.Host) bool {
	if strings.HasPrefix(strings.TrimSpace(h.String()), "Match") {
		return false
	}
	if len(h.Patterns) == 0 {
		return false
	}
	for _, p := range h.Patterns {
		s := p.String()
		if s == "" || strings.ContainsAny(s, "*?!") {
			return false
		}
	}
	return true
}

// nodeValue returns the first value for key (case-insensitive) in the block.
func nodeValue(h *ssh_config.Host, key string) string {
	for _, node := range h.Nodes {
		kv, ok := node.(*ssh_config.KV)
		if !ok {
			continue
		}
		if strings.EqualFold(kv.Key, key) {
			return kv.Value
		}
	}
	return ""
}
