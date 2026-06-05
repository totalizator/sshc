package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/totalizator/sshc/ssh"
)

// subseq reports whether query is a subsequence of target (case-insensitive)
// and returns the matched rune indices in target. Mirrors the design's
// window.SSHC.subseq.
func subseq(query, target string) (bool, []int) {
	q := []rune(strings.ToLower(query))
	t := []rune(strings.ToLower(target))
	if len(q) == 0 {
		return true, nil
	}
	idx := make([]int, 0, len(q))
	j := 0
	for i := 0; i < len(t) && j < len(q); i++ {
		if t[i] == q[j] {
			idx = append(idx, i)
			j++
		}
	}
	return j == len(q), idx
}

// scoreHost ranks a host against query (lower is better); ok is false when no
// field matches. Fields are [alias, user@host, host, tags, env], weighted by
// tightest span, then earliest start, then field priority — matching
// window.SSHC.score.
func scoreHost(query string, h ssh.Host) (int, bool) {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0, true
	}
	addr := h.User + "@" + h.HostName
	fields := []string{h.Alias, addr, h.HostName, strings.Join(h.Tags, " "), h.Env}
	best := 0
	found := false
	for fi, f := range fields {
		matched, idx := subseq(q, f)
		if !matched || len(idx) == 0 {
			continue
		}
		span := idx[len(idx)-1] - idx[0]
		start := idx[0]
		s := span*4 + start*2 + fi
		if !found || s < best {
			best = s
			found = true
		}
	}
	return best, found
}

// highlight renders text with the subsequence matched against query painted in
// hl and the rest in base. When the row is selected both styles already carry
// the selection background so the band stays continuous.
func highlight(text, query string, base, hl lipgloss.Style) string {
	matched, idx := subseq(query, text)
	runes := []rune(text)
	if query == "" || !matched || len(idx) == 0 {
		return base.Render(text)
	}
	set := make(map[int]bool, len(idx))
	for _, i := range idx {
		set[i] = true
	}
	var b strings.Builder
	i := 0
	for i < len(runes) {
		on := set[i]
		j := i
		for j < len(runes) && set[j] == on {
			j++
		}
		seg := string(runes[i:j])
		if on {
			b.WriteString(hl.Render(seg))
		} else {
			b.WriteString(base.Render(seg))
		}
		i = j
	}
	return b.String()
}
