// Package fuzzy holds the host search and ordering logic shared by every sshc
// front end: subsequence matching and ranking (match.go) and the alphabetical
// ordering that backs the --sort-name flag (sort.go). It has no UI
// dependencies; rendering of matches (e.g. highlighting) lives in the caller.
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
