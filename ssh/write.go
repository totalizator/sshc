package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/kevinburke/ssh_config"
)

// backupSuffix is appended to the config path for the pre-write backup.
const backupSuffix = ".sshc.bak"

// Add appends a new host block to the primary config at path.
func Add(path string, h Host) error {
	return mutate(path, func(cfg *ssh_config.Config) ([]*ssh_config.Host, error) {
		if findBlock(cfg, h.Alias) != -1 {
			return nil, fmt.Errorf("host %q already exists", h.Alias)
		}
		block, err := buildBlock(h)
		if err != nil {
			return nil, err
		}
		cfg.Hosts = append(cfg.Hosts, block)
		return []*ssh_config.Host{block}, nil
	})
}

// Update replaces the block identified by oldAlias with h. The block is rebuilt
// from scratch (the underlying parser does not allow mutating values in place
// without losing round-trip fidelity), so the edited block is reformatted while
// every untouched block is preserved byte-for-byte.
func Update(path, oldAlias string, h Host) error {
	return mutate(path, func(cfg *ssh_config.Config) ([]*ssh_config.Host, error) {
		idx := findBlock(cfg, oldAlias)
		if idx == -1 {
			return nil, fmt.Errorf("host %q not found", oldAlias)
		}
		if h.Alias != oldAlias && findBlock(cfg, h.Alias) != -1 {
			return nil, fmt.Errorf("host %q already exists", h.Alias)
		}
		block := cfg.Hosts[idx]
		if h.Alias != oldAlias {
			alias := strings.TrimSpace(h.Alias)
			if err := validateAlias(alias); err != nil {
				return nil, err
			}
			pat, err := ssh_config.NewPattern(alias)
			if err != nil {
				return nil, fmt.Errorf("invalid host name %q: %w", h.Alias, err)
			}
			block.Patterns = []*ssh_config.Pattern{pat}
		}
		block.EOLComment = formatMeta(h)
		applyFields(block, h)
		return []*ssh_config.Host{block}, nil
	})
}

// Delete removes the block identified by alias.
func Delete(path, alias string) error {
	return mutate(path, func(cfg *ssh_config.Config) ([]*ssh_config.Host, error) {
		idx := findBlock(cfg, alias)
		if idx == -1 {
			return nil, fmt.Errorf("host %q not found", alias)
		}
		cfg.Hosts = append(cfg.Hosts[:idx], cfg.Hosts[idx+1:]...)
		return nil, nil
	})
}

// TouchLastUsed stamps the sshc "used=" metadata on the block for alias to now,
// leaving the rest of the block byte-for-byte intact. It is best-effort: a
// missing alias (e.g. a read-only or wildcard host) is a no-op, not an error,
// so a successful connect is never blocked by a bookkeeping write.
func TouchLastUsed(path, alias string, now time.Time) error {
	if path == "" {
		return nil
	}
	return mutate(path, func(cfg *ssh_config.Config) ([]*ssh_config.Host, error) {
		idx := findBlock(cfg, alias)
		if idx == -1 {
			return nil, errNoChange
		}
		block := cfg.Hosts[idx]
		tags, env, fav, _ := parseMeta(block.EOLComment)
		block.EOLComment = formatMeta(Host{Tags: tags, Env: env, Fav: fav, LastUsed: now})
		return nil, nil
	})
}

// SetFav flips the sshc "fav" flag for alias, touching only the metadata
// comment so the block body (and any directives sshc does not model) is left
// byte-for-byte intact. A missing alias is an error so the caller can surface it.
func SetFav(path, alias string, fav bool) error {
	if path == "" {
		return fmt.Errorf("no writable config configured")
	}
	return mutate(path, func(cfg *ssh_config.Config) ([]*ssh_config.Host, error) {
		idx := findBlock(cfg, alias)
		if idx == -1 {
			return nil, fmt.Errorf("host %q not found", alias)
		}
		block := cfg.Hosts[idx]
		tags, env, _, used := parseMeta(block.EOLComment)
		block.EOLComment = formatMeta(Host{Tags: tags, Env: env, Fav: fav, LastUsed: used})
		return nil, nil
	})
}

// StripMeta removes sshc-managed metadata comments (the "#sshc …" markers) from
// every Host line in the config at path, returning the file to plain-ssh form.
// It edits only a comment that actually parses as sshc metadata (at least one
// recognised key — exactly what formatMeta writes), so a user comment that
// merely begins with the word "sshc" is left alone, as are other comments,
// directives, indentation, and ordering. Returns the number of hosts stripped;
// a file with no sshc metadata is left untouched (no write, no backup).
func StripMeta(path string) (int, error) {
	n := 0
	err := mutate(path, func(cfg *ssh_config.Config) ([]*ssh_config.Host, error) {
		for _, host := range cfg.Hosts {
			if tags, env, fav, used := parseMeta(host.EOLComment); len(tags) > 0 || env != "" || fav || !used.IsZero() {
				host.EOLComment = ""
				n++
			}
		}
		if n == 0 {
			return nil, errNoChange
		}
		return nil, nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

// errNoChange signals mutate to skip the write (and backup) entirely.
var errNoChange = fmt.Errorf("no change")

// mutate performs a safe read-modify-write cycle on path:
//  1. read the current file (re-read each time, so external edits are picked up
//     rather than clobbered from a stale snapshot)
//  2. parse it and apply fn
//  3. re-parse the rendered result to guard against producing a corrupt file
//  4. back up the original to <path>.sshc.bak
//  5. write the new content
func mutate(path string, fn func(*ssh_config.Config) ([]*ssh_config.Host, error)) error {
	if path == "" {
		return fmt.Errorf("no writable config configured")
	}

	original, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	cfg, err := ssh_config.DecodeBytes(original)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	owned, err := fn(cfg)
	if err != nil {
		if err == errNoChange {
			return nil
		}
		return err
	}

	pruneEmptyDirectives(cfg)

	ownedSet := make(map[*ssh_config.Host]bool, len(owned))
	for _, block := range owned {
		ownedSet[block] = true
	}

	// Final guard: never emit control bytes (a NUL can reach a value from a
	// Ctrl+Space keystroke or an already-corrupt file). Only tab/newline/CR are
	// legal control characters in a config; strip the rest so the written file
	// is always clean regardless of how the byte got in.
	out := stripControlBytes(render(cfg, ownedSet))
	if _, err := ssh_config.DecodeBytes([]byte(out)); err != nil {
		return fmt.Errorf("refusing to write corrupt config: %w", err)
	}

	if original != nil {
		if err := os.WriteFile(path+backupSuffix, original, 0o600); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
	}

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(out), 0o600)
}

// validateAlias rejects aliases the SSH config grammar can't represent as a
// single host. Whitespace is the critical case: `Host my server` is two
// patterns to ssh — and to the parser on the next Load, which would silently
// fan the block out into multiple read-only entries. The underlying library
// does not honour double-quoting for Host patterns either, so a space cannot be
// carried at all; reject it rather than write a block that corrupts on reload.
func validateAlias(alias string) error {
	if alias == "" {
		return fmt.Errorf("host alias is empty")
	}
	if strings.ContainsAny(alias, " \t") {
		return fmt.Errorf("host alias %q contains whitespace; ssh reads each word as a separate host pattern", alias)
	}
	return nil
}

// buildBlock constructs a fresh Host AST node from h. Only non-empty fields are
// emitted as directives.
func buildBlock(h Host) (*ssh_config.Host, error) {
	alias := strings.TrimSpace(h.Alias)
	if err := validateAlias(alias); err != nil {
		return nil, err
	}
	pat, err := ssh_config.NewPattern(alias)
	if err != nil {
		return nil, fmt.Errorf("invalid host name %q: %w", alias, err)
	}
	block := &ssh_config.Host{
		Patterns:   []*ssh_config.Pattern{pat},
		EOLComment: formatMeta(h),
	}
	add := func(key, val string) {
		val = strings.TrimSpace(val)
		if val == "" {
			return
		}
		block.Nodes = append(block.Nodes, &ssh_config.KV{Key: key, Value: val})
	}
	add("HostName", h.HostName)
	add("User", h.User)
	add("Port", h.Port)
	add("IdentityFile", h.IdentityFile)
	add("ProxyJump", h.ProxyJump)
	add("ProxyCommand", h.ProxyCommand)
	return block, nil
}

// modeledKeys are the directives sshc represents as form fields. Everything else
// in a host block (ForwardAgent, LocalForward, standalone comments, …) is opaque
// to sshc and must survive an edit untouched.
var modeledKeys = []string{"HostName", "User", "Port", "IdentityFile", "ProxyJump", "ProxyCommand"}

// applyFields updates an existing block's modeled directives in place to match
// h, leaving every other node (unmodeled directives, comments, blank lines)
// exactly where it was. This is what lets Update preserve a hand-written
// directive sshc doesn't model instead of dropping it when rebuilding.
func applyFields(block *ssh_config.Host, h Host) {
	vals := map[string]string{
		"HostName":     h.HostName,
		"User":         h.User,
		"Port":         h.Port,
		"IdentityFile": h.IdentityFile,
		"ProxyJump":    h.ProxyJump,
		"ProxyCommand": h.ProxyCommand,
	}
	for _, key := range modeledKeys {
		setDirective(block, key, vals[key])
	}
}

// setDirective sets the (single) value of a modeled directive on block. sshc
// owns the modeled keys, so it collapses them to one occurrence: the first match
// is replaced in place (keeping the original key casing and any trailing
// comment), any further duplicates are removed, and a brand-new directive is
// inserted just after the last existing directive (so it stays grouped above
// trailing comments/blank lines). An empty value removes the directive entirely.
func setDirective(block *ssh_config.Host, key, val string) {
	val = strings.TrimSpace(val)
	replaced := false
	kept := block.Nodes[:0]
	for _, node := range block.Nodes {
		if kv, ok := node.(*ssh_config.KV); ok && strings.EqualFold(kv.Key, key) {
			if !replaced && val != "" {
				kept = append(kept, &ssh_config.KV{Key: kv.Key, Value: val, Comment: kv.Comment})
				replaced = true
			}
			continue // drop the original / any duplicate occurrence
		}
		kept = append(kept, node)
	}
	block.Nodes = kept
	if val != "" && !replaced {
		block.Nodes = insertAfterLastKV(block.Nodes, &ssh_config.KV{Key: key, Value: val})
	}
}

// insertAfterLastKV places kv immediately after the last KV directive in nodes,
// keeping directives grouped above any trailing comment or blank-line nodes. If
// there is no directive yet, kv is appended.
func insertAfterLastKV(nodes []ssh_config.Node, kv *ssh_config.KV) []ssh_config.Node {
	last := -1
	for i, node := range nodes {
		if _, ok := node.(*ssh_config.KV); ok {
			last = i
		}
	}
	if last == -1 {
		return append(nodes, kv)
	}
	out := make([]ssh_config.Node, 0, len(nodes)+1)
	out = append(out, nodes[:last+1]...)
	out = append(out, kv)
	out = append(out, nodes[last+1:]...)
	return out
}

// indentUnit is the per-directive indentation for blocks sshc writes: four
// spaces, the conventional OpenSSH style. ssh_config indentation is cosmetic, so
// this only affects readability, never resolution.
const indentUnit = "    "

// render serializes cfg the way the library's Config.String does (concatenating
// each block's text in order), except that blocks in owned — the ones sshc just
// built — are rendered in canonical style (indented directives) instead of
// flush-left. Every other block is emitted by the library's faithful
// block.String(), so untouched blocks stay byte-for-byte intact.
func render(cfg *ssh_config.Config, owned map[*ssh_config.Host]bool) string {
	var b strings.Builder
	for i, host := range cfg.Hosts {
		// Guarantee a blank-line separator at the boundary of an owned block: a
		// rebuilt block loses the trailing Empty (blank-line) node it had when
		// parsed, so without this an edited block would abut its neighbour. This
		// is additive — it only inserts a separator when one is missing and never
		// removes blank lines elsewhere, so untouched regions keep their spacing.
		//
		// i > 1 skips the boundary after the implicit global block (always index
		// 0): a comment directly above the first host is usually its label, so we
		// don't want to detach it with a blank line.
		if i > 1 && (owned[host] || owned[cfg.Hosts[i-1]]) {
			if s := b.String(); s != "" && !strings.HasSuffix(s, "\n\n") {
				if strings.HasSuffix(s, "\n") {
					b.WriteByte('\n')
				} else {
					b.WriteString("\n\n")
				}
			}
		}
		if owned[host] {
			b.WriteString(canonicalBlock(host))
		} else {
			b.WriteString(host.String())
		}
	}
	return b.String()
}

// canonicalBlock renders a host block sshc wrote in conventional OpenSSH style:
// the Host line (plus any sshc metadata comment) at column 0, then every node
// indented by indentUnit. Each node is emitted through its own faithful
// String() — which preserves quoting, "=" syntax, and trailing comments on
// directives sshc does not model — with only its leading whitespace normalised
// to the canonical indent. Blank lines stay blank, so comments and unmodeled
// directives carried over from an edited block survive intact.
func canonicalBlock(h *ssh_config.Host) string {
	var b strings.Builder
	b.WriteString("Host")
	for _, pat := range h.Patterns {
		b.WriteByte(' ')
		b.WriteString(pat.String())
	}
	if h.EOLComment != "" {
		b.WriteString(" #")
		b.WriteString(h.EOLComment)
	}
	b.WriteByte('\n')
	for _, node := range h.Nodes {
		line := strings.TrimLeft(node.String(), " \t")
		if line == "" {
			b.WriteByte('\n')
			continue
		}
		b.WriteString(indentUnit)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// pruneEmptyDirectives drops any directive whose value is blank from every
// block. The parser accepts a bare keyword (e.g. `User` with no argument) and
// the serializer re-emits it as `User ` (a trailing space), which OpenSSH then
// rejects with `no argument after keyword "user"` — and our re-parse guard
// misses it because this parser is lenient. Pruning keeps every write valid and
// heals such breakage (from a manual edit or an older version) on the next save.
// No ssh_config keyword is meaningful with an empty argument, so this is safe.
func pruneEmptyDirectives(cfg *ssh_config.Config) {
	for _, host := range cfg.Hosts {
		kept := host.Nodes[:0]
		for _, node := range host.Nodes {
			if kv, ok := node.(*ssh_config.KV); ok && isBlankArg(kv.Value) {
				continue
			}
			kept = append(kept, node)
		}
		host.Nodes = kept
	}
}

// stripControlBytes removes control runes from the serialized config, keeping
// only the legal whitespace controls (tab, newline, carriage return). This is
// the last line of defence: whatever path a NUL took to get into a value, the
// file written to disk is guaranteed free of control bytes.
func stripControlBytes(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// isBlankArg reports whether v carries no usable argument: nothing but
// whitespace or control bytes (e.g. a stray NUL from a corrupted write).
// strings.TrimSpace alone is not enough — it leaves control runes like NUL,
// which ssh still rejects as a missing argument.
func isBlankArg(v string) bool {
	return strings.TrimFunc(v, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) == ""
}

// findBlock returns the index of the single-pattern host block whose alias
// matches, or -1. Multi-pattern, wildcard, and Match blocks never match.
func findBlock(cfg *ssh_config.Config, alias string) int {
	for i, block := range cfg.Hosts {
		if len(block.Patterns) != 1 {
			continue
		}
		if block.Patterns[0].String() == alias {
			return i
		}
	}
	return -1
}
