package fuzzy

import (
	"reflect"
	"testing"

	"github.com/totalizator/sshc/ssh"
)

func TestSubseq(t *testing.T) {
	cases := []struct {
		query, target string
		ok            bool
		idx           []int
	}{
		{"", "anything", true, nil},
		{"pw", "prod-web", true, []int{0, 5}},
		{"PW", "prod-web", true, []int{0, 5}}, // case-insensitive
		{"wp", "prod-web", false, []int{5}},   // order matters
		{"é", "café", true, []int{3}},         // indices are runes, not bytes
		{"x", "", false, []int{}},
	}
	for _, c := range cases {
		ok, idx := Subseq(c.query, c.target)
		if ok != c.ok || !reflect.DeepEqual(idx, c.idx) {
			t.Errorf("Subseq(%q, %q) = %v, %v; want %v, %v", c.query, c.target, ok, idx, c.ok, c.idx)
		}
	}
}

func TestScoreBlankQueryMatchesAll(t *testing.T) {
	if s, ok := Score("   ", ssh.Host{Alias: "x"}); !ok || s != 0 {
		t.Errorf("blank query: got (%d, %v), want (0, true)", s, ok)
	}
}

func TestScoreNoMatch(t *testing.T) {
	if _, ok := Score("zzz", ssh.Host{Alias: "prod-web", HostName: "10.0.0.1"}); ok {
		t.Error("expected no match")
	}
}

func TestScoreMatchesEnv(t *testing.T) {
	// "cloud" appears only in the env, not in the alias, address, or tags —
	// so a match proves env is part of the search haystack.
	h := ssh.Host{Alias: "gpu-trainer", HostName: "gpu.ml.io", User: "paperspace", Env: "cloud", Tags: []string{"ml", "gpu"}}
	if _, ok := Score("cloud", h); !ok {
		t.Errorf("expected env %q to match query %q", h.Env, "cloud")
	}
}

func TestScoreEnvRanksBelowAlias(t *testing.T) {
	// A host matched on its alias should outrank one matched only on its env
	// (lower score is better), since env is the lowest-priority field.
	aliasHit := ssh.Host{Alias: "prod-web", HostName: "10.0.0.1", User: "deploy"}
	envHit := ssh.Host{Alias: "gpu-trainer", HostName: "gpu.ml.io", User: "deploy", Env: "prod"}
	as, aok := Score("prod", aliasHit)
	es, eok := Score("prod", envHit)
	if !aok || !eok {
		t.Fatalf("both hosts should match %q (alias ok=%v, env ok=%v)", "prod", aok, eok)
	}
	if as >= es {
		t.Errorf("alias match (%d) should rank better than env match (%d)", as, es)
	}
}

func TestScoreTighterSpanWins(t *testing.T) {
	tight := ssh.Host{Alias: "web"}
	loose := ssh.Host{Alias: "w-e-b"}
	ts, _ := Score("web", tight)
	ls, _ := Score("web", loose)
	if ts >= ls {
		t.Errorf("tight span (%d) should rank better than loose (%d)", ts, ls)
	}
}
