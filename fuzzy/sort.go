// Package fuzzy holds list-ordering helpers for the host list. Interactive
// fuzzy matching lives in the TUI (see tui.scoreHost); what remains here is the
// alphabetical ordering that backs the --sort-name flag.
package fuzzy

import (
	"sort"
	"strings"

	"github.com/totalizator/sshc/ssh"
)

// SortByName returns hosts sorted alphabetically by alias (case-insensitive).
// This backs the --sort-name flag, which overrides the default ranking.
func SortByName(hosts []ssh.Host) []ssh.Host {
	out := make([]ssh.Host, len(hosts))
	copy(out, hosts)
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Alias) < strings.ToLower(out[j].Alias)
	})
	return out
}
