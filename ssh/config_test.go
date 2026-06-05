package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleConfig = `# top of file
Host web
    HostName 10.0.0.1
    User deploy
    Port 2222
    IdentityFile ~/.ssh/web_ed25519

Host db
    HostName db.internal
    ProxyJump web

Host *.example.com
    User wildcard

Host *
    ForwardAgent yes
`

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func findHost(hosts []Host, alias string) (Host, bool) {
	for _, h := range hosts {
		if h.Alias == alias {
			return h, true
		}
	}
	return Host{}, false
}

func TestLoadParsesFields(t *testing.T) {
	path := writeTemp(t, "config", sampleConfig)
	hosts, err := Load([]string{path})
	if err != nil {
		t.Fatal(err)
	}

	web, ok := findHost(hosts, "web")
	if !ok {
		t.Fatal("web host not found")
	}
	if web.HostName != "10.0.0.1" || web.User != "deploy" || web.Port != "2222" {
		t.Errorf("web fields wrong: %+v", web)
	}
	if web.IdentityFile != "~/.ssh/web_ed25519" {
		t.Errorf("web identity wrong: %q", web.IdentityFile)
	}
	if !web.Editable {
		t.Error("web from primary config should be editable")
	}

	db, ok := findHost(hosts, "db")
	if !ok {
		t.Fatal("db host not found")
	}
	if db.ProxyJump != "web" {
		t.Errorf("db proxyjump wrong: %q", db.ProxyJump)
	}
}

func TestLoadExcludesWildcards(t *testing.T) {
	path := writeTemp(t, "config", sampleConfig)
	hosts, _ := Load([]string{path})

	if _, ok := findHost(hosts, "*"); ok {
		t.Error("Host * should not appear in the list")
	}
	if w, ok := findHost(hosts, "*.example.com"); ok && w.Editable {
		t.Error("wildcard host must not be editable")
	}
}

func TestLoadSecondaryReadOnly(t *testing.T) {
	primary := writeTemp(t, "primary", "Host a\n  HostName 1.1.1.1\n")
	secondary := writeTemp(t, "secondary", "Host b\n  HostName 2.2.2.2\n")

	hosts, err := Load([]string{primary, secondary})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := findHost(hosts, "a")
	b, _ := findHost(hosts, "b")
	if !a.Editable {
		t.Error("primary host should be editable")
	}
	if b.Editable {
		t.Error("secondary-config host must be read-only")
	}
}

func TestLoadMissingFileIsSkipped(t *testing.T) {
	hosts, err := Load([]string{filepath.Join(t.TempDir(), "does-not-exist")})
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected no hosts, got %d", len(hosts))
	}
}
