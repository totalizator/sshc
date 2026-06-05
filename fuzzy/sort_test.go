package fuzzy

import (
	"testing"

	"github.com/totalizator/sshc/ssh"
)

func sample() []ssh.Host {
	return []ssh.Host{
		{Alias: "web-prod", HostName: "10.0.0.1", User: "deploy"},
		{Alias: "db-prod", HostName: "10.0.0.2", User: "postgres"},
		{Alias: "web-stage", HostName: "10.0.1.1", User: "deploy"},
	}
}

func TestSortByName(t *testing.T) {
	got := SortByName(sample())
	want := []string{"db-prod", "web-prod", "web-stage"}
	for i, w := range want {
		if got[i].Alias != w {
			t.Errorf("position %d: want %s, got %s", i, w, got[i].Alias)
		}
	}
}

func TestSortByNameDoesNotMutateInput(t *testing.T) {
	in := sample()
	_ = SortByName(in)
	if in[0].Alias != "web-prod" {
		t.Errorf("input order changed: got %s at index 0", in[0].Alias)
	}
}
