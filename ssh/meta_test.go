package ssh

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestMetaRoundTrip(t *testing.T) {
	path := writeTemp(t, "config", "")

	used := time.Unix(1717000000, 0)
	in := Host{
		Alias: "web", HostName: "10.0.0.1", User: "deploy",
		Tags: []string{"web", "acme"}, Env: "prod", Fav: true, LastUsed: used,
	}
	if err := Add(path, in); err != nil {
		t.Fatal(err)
	}

	hosts, _ := Load([]string{path})
	got, ok := findHost(hosts, "web")
	if !ok {
		t.Fatal("host not found after reload")
	}
	if got.Env != "prod" || !got.Fav {
		t.Errorf("env/fav not round-tripped: %+v", got)
	}
	if strings.Join(got.Tags, ",") != "web,acme" {
		t.Errorf("tags not round-tripped: %v", got.Tags)
	}
	if !got.LastUsed.Equal(used) {
		t.Errorf("lastUsed not round-tripped: got %v want %v", got.LastUsed, used)
	}
}

func TestParseMetaStripsControlBytes(t *testing.T) {
	// A stray NUL in env/tags (from a corrupted write) must be stripped on read
	// so chip colours stay stable and the value heals on the next save.
	tags, env, _, _ := parseMeta("sshc env=PROD\x00 tags=web\x00,db")
	if env != "PROD" {
		t.Errorf("env = %q, want %q", env, "PROD")
	}
	if strings.Join(tags, ",") != "web,db" {
		t.Errorf("tags = %v, want [web db]", tags)
	}

	// A value that is only a control byte must read as empty (no chip).
	_, env, _, _ = parseMeta("sshc env=\x00")
	if env != "" {
		t.Errorf("control-only env = %q, want empty", env)
	}
}

func TestFormatMetaStripsControlBytes(t *testing.T) {
	if got := formatMeta(Host{Env: "PROD\x00"}); got != "sshc env=PROD" {
		t.Errorf("formatMeta = %q, want %q", got, "sshc env=PROD")
	}
	if got := formatMeta(Host{Env: "\x00"}); got != "" {
		t.Errorf("control-only env should yield no metadata, got %q", got)
	}
}

func TestMetaHealsControlBytesOnWrite(t *testing.T) {
	// A config whose env carries a NUL is healed once the host is rewritten.
	src := "Host a #sshc env=PROD\x00\n  HostName 1.1.1.1\n"
	path := writeTemp(t, "config", src)
	if err := TouchLastUsed(path, "a", time.Unix(1717000000, 0)); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if strings.ContainsRune(string(out), 0) {
		t.Fatalf("NUL survived write:\n%q", out)
	}
	if !strings.Contains(string(out), "env=PROD") {
		t.Errorf("env value lost during heal:\n%q", out)
	}
}

func TestMetaOnHostLineIsValidConfig(t *testing.T) {
	path := writeTemp(t, "config", "")
	if err := Add(path, Host{Alias: "h", HostName: "1.1.1.1", Env: "prod", Fav: true}); err != nil {
		t.Fatal(err)
	}
	// The metadata must be a comment on the Host line so plain ssh ignores it.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(raw)
	if !strings.Contains(out, "#sshc") {
		t.Errorf("expected #sshc metadata comment, got:\n%s", out)
	}
	if !strings.Contains(out, "Host h #sshc") {
		t.Errorf("metadata should ride the Host line, got:\n%s", out)
	}
}

func TestTouchLastUsedPreservesFields(t *testing.T) {
	path := writeTemp(t, "config", "")
	if err := Add(path, Host{Alias: "h", HostName: "1.1.1.1", Env: "prod", Tags: []string{"db"}, Fav: true}); err != nil {
		t.Fatal(err)
	}

	now := time.Unix(1717000123, 0)
	if err := TouchLastUsed(path, "h", now); err != nil {
		t.Fatal(err)
	}

	hosts, _ := Load([]string{path})
	got, _ := findHost(hosts, "h")
	if !got.LastUsed.Equal(now) {
		t.Errorf("touch did not stamp lastUsed: %v", got.LastUsed)
	}
	if got.Env != "prod" || !got.Fav || strings.Join(got.Tags, ",") != "db" {
		t.Errorf("touch clobbered metadata: %+v", got)
	}
	// HostName lives in the block body and must survive a touch.
	if got.HostName != "1.1.1.1" {
		t.Errorf("touch clobbered body: %+v", got)
	}
}

func TestSetFavPreservesBody(t *testing.T) {
	path := writeTemp(t, "config", "")
	if err := Add(path, Host{Alias: "h", HostName: "1.1.1.1", Env: "prod", Tags: []string{"db"}}); err != nil {
		t.Fatal(err)
	}
	if err := SetFav(path, "h", true); err != nil {
		t.Fatal(err)
	}
	hosts, _ := Load([]string{path})
	got, _ := findHost(hosts, "h")
	if !got.Fav {
		t.Error("fav not set")
	}
	if got.HostName != "1.1.1.1" || got.Env != "prod" || strings.Join(got.Tags, ",") != "db" {
		t.Errorf("SetFav clobbered fields: %+v", got)
	}
}

func TestTouchMissingAliasIsNoop(t *testing.T) {
	path := writeTemp(t, "config", "Host a\n  HostName 1.1.1.1\n")
	if err := TouchLastUsed(path, "nope", time.Now()); err != nil {
		t.Errorf("touch of missing alias should be a no-op, got %v", err)
	}
}

func TestLastUsedLabel(t *testing.T) {
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{0, "just now"},
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{2 * time.Hour, "2h ago"},
		{30 * time.Hour, "1d ago"},
		{4 * 24 * time.Hour, "4d ago"},
		{14 * 24 * time.Hour, "2w ago"},
	}
	for _, c := range cases {
		if got := LastUsedLabel(time.Now().Add(-c.ago)); got != c.want {
			t.Errorf("LastUsedLabel(%v) = %q, want %q", c.ago, got, c.want)
		}
	}
	if got := LastUsedLabel(time.Time{}); got != "never" {
		t.Errorf("zero time label = %q, want never", got)
	}
}
