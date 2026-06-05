package ssh

import (
	"strings"
	"testing"
)

// A Host line is whitespace-separated patterns, and the parser does not honour
// quoting for them, so an alias with a space cannot round-trip as one host —
// it must be rejected at write time rather than fan out into read-only entries
// on the next Load.
func TestAddRejectsWhitespaceAlias(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config"

	// Internal whitespace is the bug; leading/trailing space is trimmed, not rejected.
	for _, alias := range []string{"my server", "tab\there", "a b c"} {
		err := Add(path, Host{Alias: alias, HostName: "10.0.0.9"})
		if err == nil {
			t.Errorf("Add(alias=%q) = nil error, want rejection", alias)
			continue
		}
		if !strings.Contains(err.Error(), "whitespace") {
			t.Errorf("Add(alias=%q) error = %v, want a whitespace complaint", alias, err)
		}
	}

	// A normal single-token alias still works.
	if err := Add(path, Host{Alias: "my-server", HostName: "10.0.0.9"}); err != nil {
		t.Fatalf("Add(alias=%q) = %v, want success", "my-server", err)
	}
}

// Renaming an existing host to a whitespace alias must be rejected too, and the
// on-disk config must be left untouched (the mutate() backup/replace is atomic).
func TestUpdateRejectsWhitespaceAlias(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config"
	if err := Add(path, Host{Alias: "web", HostName: "10.0.0.1"}); err != nil {
		t.Fatalf("seed Add: %v", err)
	}
	if err := Update(path, "web", Host{Alias: "web prod", HostName: "10.0.0.1"}); err == nil {
		t.Fatalf("Update to %q = nil error, want rejection", "web prod")
	}

	hosts, err := Load([]string{path})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(hosts) != 1 || hosts[0].Alias != "web" {
		t.Fatalf("config changed after rejected rename: got %+v", hosts)
	}
}
