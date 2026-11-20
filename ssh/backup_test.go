package ssh

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestBackupsRotateAndKeepFive(t *testing.T) {
	path := writeTemp(t, "config", "Host h0\n  HostName 10.0.0.0\n")
	for i := 1; i <= 7; i++ {
		if err := Add(path, Host{Alias: "h" + string(rune('0'+i)), HostName: "10.0.0.1"}); err != nil {
			t.Fatal(err)
		}
	}
	bs := Backups(path)
	if len(bs) != BackupsKept {
		t.Fatalf("got %d backups, want %d", len(bs), BackupsKept)
	}
	// Newest first: the newest backup is the config before the 7th add
	// (h0..h6, 7 hosts); the oldest kept is before the 3rd (h0..h2).
	for i, b := range bs {
		if want := 7 - i; b.Hosts != want {
			t.Errorf("backup %d (%s) has %d hosts, want %d", i, b.Path, b.Hosts, want)
		}
	}
	if bs[0].Path != path+".sshc.bak" || bs[1].Path != path+".sshc.bak.1" {
		t.Errorf("paths = %s, %s", bs[0].Path, bs[1].Path)
	}
}

func TestMetadataWritesDontTouchBackups(t *testing.T) {
	path := writeTemp(t, "config", "Host a\n  HostName 10.0.0.1\n")
	if err := Add(path, Host{Alias: "b", HostName: "10.0.0.2"}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path + ".sshc.bak")
	for i := 0; i < 10; i++ {
		if err := TouchLastUsed(path, "a", time.Unix(1717000000+int64(i), 0)); err != nil {
			t.Fatal(err)
		}
		if err := SetFav(path, "b", i%2 == 0); err != nil {
			t.Fatal(err)
		}
	}
	after, _ := os.ReadFile(path + ".sshc.bak")
	if string(before) != string(after) {
		t.Fatalf("backup changed by used=/fav writes:\nbefore %q\nafter  %q", before, after)
	}
	if n := len(Backups(path)); n != 1 {
		t.Fatalf("%d backups, want 1", n)
	}
}

func TestRestoreAndUndoRestore(t *testing.T) {
	path := writeTemp(t, "config", "Host a\n  HostName 10.0.0.1\n")
	if err := Delete(path, "a"); err != nil {
		t.Fatal(err)
	}
	if err := TouchLastUsed(path, "nobody", time.Now()); err != nil { // a connect in between
		t.Fatal(err)
	}
	bs := Backups(path)
	if len(bs) != 1 || bs[0].Hosts != 1 {
		t.Fatalf("backups = %+v", bs)
	}
	if err := Restore(path, bs[0].Path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "Host a") {
		t.Fatalf("restore didn't bring the host back:\n%s", raw)
	}
	// The emptied config is now the newest backup, so the restore can be undone.
	bs = Backups(path)
	if len(bs) != 2 || bs[0].Hosts != 0 || bs[1].Hosts != 1 {
		t.Fatalf("after restore: %+v", bs)
	}
	if err := Restore(path, path); err == nil {
		t.Error("restoring from a path that isn't a backup must fail")
	}
}

func TestReplaceRefusesAnInvalidConfig(t *testing.T) {
	path := writeTemp(t, "config", "Host a\n  HostName 10.0.0.1\n")
	if err := Replace(path, []byte("Match\n")); err == nil {
		raw, _ := os.ReadFile(path)
		t.Fatalf("Replace accepted an invalid config; now:\n%s", raw)
	}
	if len(Backups(path)) != 0 {
		t.Error("a refused Replace made a backup")
	}
}

func TestWritesLeaveNoTempFiles(t *testing.T) {
	path := writeTemp(t, "config", "")
	if err := Add(path, Host{Alias: "a", HostName: "10.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(strings.TrimSuffix(path, "/config"))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("config mode = %v", fi.Mode().Perm())
	}
}
