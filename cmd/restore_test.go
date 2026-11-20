package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/totalizator/sshc/ssh"
)

func runSshc(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	t.Cleanup(func() { configPaths = nil; rootCmd.SetArgs(nil) })
	err := rootCmd.Execute()
	return out.String(), err
}

func TestRestoreListsAndUndoesADelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	orig := "Host a\n  HostName a.example\n\nHost b\n  HostName b.example\n"
	if err := os.WriteFile(path, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := runSshc(t, "--config", path, "restore")
	if err != nil || !strings.Contains(out, "no backups") {
		t.Fatalf("empty list: %q, %v", out, err)
	}

	if err := ssh.Delete(path, "b"); err != nil {
		t.Fatal(err)
	}
	out, err = runSshc(t, "--config", path, "restore")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "  1  saved ") || !strings.Contains(out, "2 hosts") {
		t.Fatalf("list after delete:\n%s", out)
	}

	out, err = runSshc(t, "--config", path, "restore", "1")
	if err != nil {
		t.Fatalf("restore: %v\n%s", err, out)
	}
	got, _ := os.ReadFile(path)
	if string(got) != orig {
		t.Fatalf("restored config:\n%s\nwant:\n%s", got, orig)
	}
	if !strings.Contains(out, `"sshc restore 1" undoes this`) {
		t.Fatalf("restore output: %q", out)
	}

	// The config the restore replaced (without b) is now backup 1.
	if _, err := runSshc(t, "--config", path, "restore", "1"); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	if strings.Contains(string(got), "Host b") {
		t.Fatalf("undoing the restore should bring back the delete:\n%s", got)
	}
}

func TestRestoreRejectsABadNumber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("Host a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runSshc(t, "--config", path, "restore", "1"); err == nil || !strings.Contains(err.Error(), "no backups") {
		t.Fatalf("no backups: %v", err)
	}
	if err := ssh.Delete(path, "a"); err != nil {
		t.Fatal(err)
	}
	for _, arg := range []string{"0", "2", "x"} {
		if _, err := runSshc(t, "--config", path, "restore", arg); err == nil {
			t.Fatalf("restore %s: want an error", arg)
		}
	}
}
