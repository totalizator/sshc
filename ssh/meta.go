package ssh

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// metaPrefix marks a Host-line comment that carries sshc-managed metadata, e.g.
//
//	Host prod-web-01 #sshc env=prod tags=web,acme fav=1 used=1717000000
//
// Storing it as a comment keeps the file valid for plain ssh while letting
// fav/tags/env/last-used travel with the host when the config is shared.
const metaPrefix = "sshc"

// meta is the sshc metadata carried on a Host line.
type meta struct {
	tags     []string
	env      string
	fav      bool
	lastUsed time.Time
	start    Startup
	// extra holds key=value tokens this version doesn't know (written by a
	// newer sshc), kept verbatim so rewriting the line doesn't drop them.
	extra []string
}

// recognised reports whether m has at least one key this version knows.
func (m meta) recognised() bool {
	return len(m.tags) > 0 || m.env != "" || m.fav || !m.lastUsed.IsZero() || !m.start.IsZero()
}

// metaOf takes h's metadata, keeping the unknown tokens of prev.
func metaOf(h Host, prev meta) meta {
	return meta{tags: h.Tags, env: h.Env, fav: h.Fav, lastUsed: h.LastUsed, start: h.Start, extra: prev.extra}
}

// parseMeta reads the sshc metadata carried in a Host line's EOL comment. The
// comment string is the text after '#' (as exposed by the parser); a comment
// that does not start with the sshc marker yields a zero meta and leaves the
// host's regular fields untouched.
func parseMeta(comment string) meta {
	var m meta
	fields := strings.Fields(strings.TrimSpace(comment))
	if len(fields) == 0 || fields[0] != metaPrefix {
		return m
	}
	for _, tok := range fields[1:] {
		key, val, ok := strings.Cut(tok, "=")
		if !ok {
			continue
		}
		switch key {
		case "tags":
			for _, t := range strings.Split(val, ",") {
				if t = sanitizeToken(t); t != "" {
					m.tags = append(m.tags, t)
				}
			}
		case "env":
			m.env = sanitizeToken(val)
		case "fav":
			m.fav = val == "1" || strings.EqualFold(val, "true")
		case "used":
			if secs, err := strconv.ParseInt(val, 10, 64); err == nil && secs > 0 {
				m.lastUsed = time.Unix(secs, 0)
			}
		case "start":
			// A spec this version can't read is kept as is rather than lost.
			if st, err := ParseStartup(unescapeToken(val)); err == nil {
				m.start = st
			} else {
				m.extra = append(m.extra, tok)
			}
		default:
			if key != "" {
				m.extra = append(m.extra, tok)
			}
		}
	}
	return m
}

// formatMeta renders m as a Host-line comment body (the text that follows
// '#'), or "" when there is no metadata worth recording. Tag and env tokens
// are space-free so they round-trip through the whitespace-delimited comment;
// the startup spec is percent-escaped for the same reason.
func formatMeta(m meta) string {
	var parts []string
	if env := sanitizeToken(m.env); env != "" {
		parts = append(parts, "env="+env)
	}
	if tags := sanitizeTags(m.tags); len(tags) > 0 {
		parts = append(parts, "tags="+strings.Join(tags, ","))
	}
	if m.fav {
		parts = append(parts, "fav=1")
	}
	if !m.lastUsed.IsZero() {
		parts = append(parts, "used="+strconv.FormatInt(m.lastUsed.Unix(), 10))
	}
	if spec := m.start.String(); spec != "" {
		parts = append(parts, "start="+escapeToken(spec))
	}
	parts = append(parts, m.extra...)
	if len(parts) == 0 {
		return ""
	}
	return metaPrefix + " " + strings.Join(parts, " ")
}

// escapeToken percent-escapes the bytes that can't sit in a whitespace-
// delimited comment token: whitespace, control bytes and '%' itself.
func escapeToken(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= ' ' || c == 0x7f || c == '%' {
			fmt.Fprintf(&b, "%%%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// unescapeToken reverses escapeToken. A malformed escape is kept literally.
func unescapeToken(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// sanitizeToken normalises a metadata value for the space-delimited comment
// encoding: it drops control bytes (e.g. a stray NUL from a corrupted write,
// which strings.Fields does not treat as whitespace) and collapses internal
// whitespace to '-'. Applied on both read and write so a corrupt value heals on
// the next load and never re-persists — otherwise a NUL round-trips forever and
// breaks env-chip colours (a derived hue keyed off "prod\x00" ≠ "prod").
func sanitizeToken(s string) string {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(cleaned), "-")
}

// sanitizeTags trims, de-spaces, and de-duplicates tags, preserving order.
func sanitizeTags(tags []string) []string {
	seen := make(map[string]bool, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = sanitizeToken(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// LastUsedLabel renders a human "… ago" string for t relative to now, matching
// the design's recency labels. A zero time reads as "never".
func LastUsedLabel(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	mins := time.Since(t).Minutes()
	switch {
	case mins < 1:
		return "just now"
	case mins < 60:
		return fmt.Sprintf("%dm ago", int(mins))
	}
	h := int(math.Round(mins / 60))
	if h < 24 {
		return fmt.Sprintf("%dh ago", h)
	}
	d := int(math.Round(float64(h) / 24))
	if d < 7 {
		return fmt.Sprintf("%dd ago", d)
	}
	w := int(math.Round(float64(d) / 7))
	return fmt.Sprintf("%dw ago", w)
}
