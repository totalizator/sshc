package fuzzy

import (
	"strings"

	"github.com/totalizator/sshc/ssh"
)

// Subseq reports whether query is a subsequence of target (case-insensitive)
// and returns the matched rune indices in target. An empty query matches
// everything with no indices. Mirrors the design's window.SSHC.subseq.
func Subseq(query, target string) (bool, []int) {
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

// Score ranks a host against query (lower is better); ok is false when no
// field matches. Fields are [alias, user@host, host, tags, env], weighted by
// tightest span, then earliest start, then field priority — matching
// window.SSHC.score. A blank query matches every host with score 0.
//
// This is the single ranking used by every sshc front end (the TUI and the
// Android app), so both order search results identically.
func Score(query string, h ssh.Host) (int, bool) {
	q := strings.TrimSpace(query)
	if q == "" {
		return 0, true
	}
	addr := h.User + "@" + h.HostName
	fields := []string{h.Alias, addr, h.HostName, strings.Join(h.Tags, " "), h.Env}
	best := 0
	found := false
	for fi, f := range fields {
		matched, idx := Subseq(q, f)
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
