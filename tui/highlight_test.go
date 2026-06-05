package tui

import (
	"testing"

	"github.com/totalizator/sshc/ssh"
)

func TestScoreHostMatchesEnv(t *testing.T) {
	// "cloud" appears only in the env, not in the alias, address, or tags —
	// so a match proves env is part of the search haystack.
	h := ssh.Host{Alias: "gpu-trainer", HostName: "gpu.ml.io", User: "paperspace", Env: "cloud", Tags: []string{"ml", "gpu"}}
	if _, ok := scoreHost("cloud", h); !ok {
		t.Errorf("expected env %q to match query %q", h.Env, "cloud")
	}
}

func TestScoreHostEnvRanksBelowAlias(t *testing.T) {
	// A host matched on its alias should outrank one matched only on its env
	// (lower score is better), since env is the lowest-priority field.
	aliasHit := ssh.Host{Alias: "prod-web", HostName: "10.0.0.1", User: "deploy"}
	envHit := ssh.Host{Alias: "gpu-trainer", HostName: "gpu.ml.io", User: "deploy", Env: "prod"}
	as, aok := scoreHost("prod", aliasHit)
	es, eok := scoreHost("prod", envHit)
	if !aok || !eok {
		t.Fatalf("both hosts should match %q (alias ok=%v, env ok=%v)", "prod", aok, eok)
	}
	if as >= es {
		t.Errorf("alias match (%d) should rank better than env match (%d)", as, es)
	}
}
