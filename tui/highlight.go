package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	fz "github.com/totalizator/sshc/fuzzy"
)

// highlight renders text with the subsequence matched against query painted in
// hl and the rest in base. When the row is selected both styles already carry
// the selection background so the band stays continuous.
func highlight(text, query string, base, hl lipgloss.Style) string {
	matched, idx := fz.Subseq(query, text)
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
