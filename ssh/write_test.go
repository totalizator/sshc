package ssh

import (
	"os"
	"strings"
	"testing"
)

func TestAddCreatesHost(t *testing.T) {
	path := writeTemp(t, "config", "Host existing\n  HostName 1.1.1.1\n")

	err := Add(path, Host{Alias: "new", HostName: "2.2.2.2", User: "bob", Port: "2200"})
	if err != nil {
		t.Fatal(err)
	}

	hosts, _ := Load([]string{path})
	got, ok := findHost(hosts, "new")
	if !ok {
		t.Fatal("added host not found after reload")
	}
	if got.HostName != "2.2.2.2" || got.User != "bob" || got.Port != "2200" {
		t.Errorf("added host fields wrong: %+v", got)
	}
	// The untouched block must survive.
	if _, ok := findHost(hosts, "existing"); !ok {
		t.Error("existing host disappeared")
	}
}

func TestAddRejectsDuplicate(t *testing.T) {
	path := writeTemp(t, "config", "Host dup\n  HostName 1.1.1.1\n")
	if err := Add(path, Host{Alias: "dup", HostName: "9.9.9.9"}); err == nil {
		t.Error("expected duplicate alias to be rejected")
	}
}

func TestUpdateReplacesFields(t *testing.T) {
	path := writeTemp(t, "config", "Host a\n  HostName 1.1.1.1\n  User old\n\nHost keep\n  HostName 8.8.8.8\n")

	err := Update(path, "a", Host{Alias: "a", HostName: "1.1.1.2", User: "new"})
	if err != nil {
		t.Fatal(err)
	}
	hosts, _ := Load([]string{path})
	a, _ := findHost(hosts, "a")
	if a.HostName != "1.1.1.2" || a.User != "new" {
		t.Errorf("update did not apply: %+v", a)
	}
	if _, ok := findHost(hosts, "keep"); !ok {
		t.Error("untouched host was lost on update")
	}
}

func TestDeleteRemovesHost(t *testing.T) {
	path := writeTemp(t, "config", "Host gone\n  HostName 1.1.1.1\n\nHost stay\n  HostName 2.2.2.2\n")

	if err := Delete(path, "gone"); err != nil {
		t.Fatal(err)
	}
	hosts, _ := Load([]string{path})
	if _, ok := findHost(hosts, "gone"); ok {
		t.Error("deleted host still present")
	}
	if _, ok := findHost(hosts, "stay"); !ok {
		t.Error("non-target host was deleted")
	}
}

func TestWriteCreatesBackup(t *testing.T) {
	path := writeTemp(t, "config", "Host a\n  HostName 1.1.1.1\n")
	original, _ := os.ReadFile(path)

	if err := Add(path, Host{Alias: "b", HostName: "2.2.2.2"}); err != nil {
		t.Fatal(err)
	}

	backup, err := os.ReadFile(path + backupSuffix)
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if string(backup) != string(original) {
		t.Error("backup does not match pre-write content")
	}
}

func TestWriteHealsBareDirective(t *testing.T) {
	// A value-less `User` — from a manual edit, an older version, or a stray
	// NUL byte from a corrupted write — must be pruned on the next write so ssh
	// does not reject the file with `no argument after keyword "user"`. Editing
	// a *different* block heals it.
	for _, bad := range []string{"User", "  User  ", "User \x00"} {
		t.Run(strings.ReplaceAll(bad, "\x00", "NUL"), func(t *testing.T) {
			src := "Host a\n  HostName 1.1.1.1\n  " + bad + "\n\nHost b\n  HostName 2.2.2.2\n"
			path := writeTemp(t, "config", src)

			if err := Update(path, "b", Host{Alias: "b", HostName: "3.3.3.3"}); err != nil {
				t.Fatal(err)
			}
			out, _ := os.ReadFile(path)
			for _, line := range strings.Split(string(out), "\n") {
				if strings.EqualFold(strings.TrimSpace(strings.Trim(line, "\x00")), "user") {
					t.Fatalf("value-less User directive survived write:\n%q", out)
				}
			}
			// The healed host must still be present.
			hosts, _ := Load([]string{path})
			if _, ok := findHost(hosts, "a"); !ok {
				t.Error("host a disappeared after healing")
			}
		})
	}
}

func TestWriteStripsControlBytesInValues(t *testing.T) {
	// A NUL that reaches a directive value or metadata (e.g. via a Ctrl+Space
	// keystroke in a form field) must never reach disk.
	path := writeTemp(t, "config", "Host x\n  HostName 1.1.1.1\n")
	if err := Add(path, Host{Alias: "y", HostName: "2.2.2.2\x00", User: "bob\x00", Env: "prod\x00"}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if strings.ContainsRune(string(out), 0) {
		t.Fatalf("control byte reached disk:\n%q", out)
	}
	hosts, _ := Load([]string{path})
	got, _ := findHost(hosts, "y")
	if got.HostName != "2.2.2.2" || got.User != "bob" || got.Env != "prod" {
		t.Errorf("values not clean after write: %+v", got)
	}
}

func TestAddOmitsEmptyUser(t *testing.T) {
	path := writeTemp(t, "config", "Host x\n  HostName 1.1.1.1\n")
	if err := Add(path, Host{Alias: "y", HostName: "2.2.2.2", User: ""}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if strings.Contains(string(out), "User") {
		t.Errorf("empty User should not be written:\n%q", out)
	}
}

func TestAddIndentsNewBlock(t *testing.T) {
	path := writeTemp(t, "config", "Host existing\n  HostName 1.1.1.1\n")

	if err := Add(path, Host{Alias: "new", HostName: "2.2.2.2", User: "bob"}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "\n    HostName 2.2.2.2") {
		t.Errorf("new block directives not indented by 4 spaces:\n%q", out)
	}
	// A blank line must separate the appended block from the existing one.
	if !strings.Contains(string(out), "\n\nHost new") {
		t.Errorf("appended block not separated by a blank line:\n%q", out)
	}
}

func TestUpdateIndentsAndKeepsSeparation(t *testing.T) {
	path := writeTemp(t, "config", "Host a\n  HostName 1.1.1.1\n  User old\n\nHost b\n  HostName 2.2.2.2\n")

	if err := Update(path, "a", Host{Alias: "a", HostName: "1.1.1.2", User: "new"}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	outS := string(out)
	if !strings.Contains(outS, "\n    HostName 1.1.1.2") || !strings.Contains(outS, "\n    User new") {
		t.Errorf("edited block not indented by 4 spaces:\n%q", outS)
	}
	// The blank line before the untouched neighbour must survive (a rebuilt
	// block loses its trailing Empty node, so render must restore it).
	if !strings.Contains(outS, "\n\nHost b") {
		t.Errorf("separation before untouched block lost:\n%q", outS)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func TestUpdatePreservesUnmodeledDirectives(t *testing.T) {
	// Editing a host must not drop directives sshc does not model, nor inline
	// comments inside the block. Only the modeled fields change.
	src := "Host a\n  HostName 1.1.1.1\n  ForwardAgent yes\n  LocalForward 8080 localhost:80\n  # keep this note\n"
	path := writeTemp(t, "config", src)

	if err := Update(path, "a", Host{Alias: "a", HostName: "1.1.1.2", User: "bob"}); err != nil {
		t.Fatal(err)
	}
	out := string(mustReadFile(t, path))
	for _, want := range []string{"ForwardAgent yes", "LocalForward 8080 localhost:80", "# keep this note", "HostName 1.1.1.2", "User bob"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q to survive the edit:\n%s", want, out)
		}
	}
}

func TestUpdatePreservesQuotedUnmodeledValues(t *testing.T) {
	// A quoted value on an unmodeled directive must round-trip unchanged (the
	// parser keeps the raw text; canonicalBlock renders via the node's own
	// String(), so quotes are not stripped).
	src := "Host a\n  HostName 1.1.1.1\n  SetEnv FOO=\"bar baz\"\n"
	path := writeTemp(t, "config", src)

	if err := Update(path, "a", Host{Alias: "a", HostName: "9.9.9.9"}); err != nil {
		t.Fatal(err)
	}
	out := string(mustReadFile(t, path))
	if !strings.Contains(out, `SetEnv FOO="bar baz"`) {
		t.Errorf("quoted unmodeled value was altered:\n%s", out)
	}
}

func TestUntouchedBlockBytesPreserved(t *testing.T) {
	// An oddly-indented, comment-bearing block we do NOT edit must come back
	// byte-for-byte after editing a different block. (Indentation is space-based
	// here: the underlying parser normalises leading tabs to spaces on any
	// round-trip, independent of sshc, so a tab fixture would fail for reasons
	// unrelated to what this test guards.)
	untouched := "# note\nHost weird\n   HostName 9.9.9.9\n      Port 2222\n"
	src := untouched + "\nHost target\n  HostName 1.1.1.1\n"
	path := writeTemp(t, "config", src)

	if err := Update(path, "target", Host{Alias: "target", HostName: "1.1.1.2"}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	outS := string(out)
	if !strings.HasPrefix(outS, untouched) {
		t.Errorf("untouched block was not preserved byte-for-byte:\ngot:  %q\nwant prefix: %q", outS, untouched)
	}
}

func TestStripMeta(t *testing.T) {
	src := "# user comment\nHost x #sshc env=prod fav=1 used=1717000000\n  HostName 1.1.1.1\n\nHost plain\n  HostName 2.2.2.2\n"
	path := writeTemp(t, "config", src)

	n, err := StripMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 host stripped, got %d", n)
	}
	out, _ := os.ReadFile(path)
	outS := string(out)
	if strings.Contains(outS, "sshc") {
		t.Errorf("sshc metadata comment survived:\n%q", outS)
	}
	if !strings.Contains(outS, "# user comment") {
		t.Errorf("user comment was lost:\n%q", outS)
	}
	// The host (and its directive) must still be present and loadable.
	hosts, _ := Load([]string{path})
	if got, ok := findHost(hosts, "x"); !ok || got.HostName != "1.1.1.1" {
		t.Errorf("host x lost or altered after strip: %+v ok=%v", got, ok)
	}
	// Backup written.
	if _, err := os.ReadFile(path + backupSuffix); err != nil {
		t.Errorf("backup not created: %v", err)
	}
	// A second pass is a no-op (no metadata left).
	if n2, err := StripMeta(path); err != nil || n2 != 0 {
		t.Errorf("second StripMeta should be a no-op, got n=%d err=%v", n2, err)
	}
}

func TestStripMetaLeavesSshcPrefixedUserComment(t *testing.T) {
	// A user comment that merely begins with the word "sshc" but carries no
	// recognised key=value must NOT be mistaken for metadata and deleted.
	src := "Host z #sshc is my favourite box\n  HostName 1.1.1.1\n"
	path := writeTemp(t, "config", src)

	n, err := StripMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected nothing stripped, got %d", n)
	}
	out, _ := os.ReadFile(path)
	if string(out) != src {
		t.Errorf("user comment was altered:\ngot:  %q\nwant: %q", out, src)
	}
}

func TestUpdatePreservesUntouchedFormatting(t *testing.T) {
	// A comment on an untouched block must be retained verbatim.
	src := "# keep me\nHost a\n  HostName 1.1.1.1\n\nHost b\n  HostName 2.2.2.2\n"
	path := writeTemp(t, "config", src)

	if err := Update(path, "b", Host{Alias: "b", HostName: "3.3.3.3"}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "# keep me") {
		t.Error("comment on untouched block was lost")
	}
}
