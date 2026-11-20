package ssh

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRunStepsRoundTripThroughTheConfig(t *testing.T) {
	path := writeTemp(t, "config", "")
	steps := []RunStep{
		{Cmd: "cd /srv/app && . venv/bin/activate"},
		{Cmd: "htop -d 5", Off: true},
		{Cmd: "-literal dash 100%"},
	}
	if err := Add(path, Host{Alias: "box", HostName: "10.0.0.9", Run: steps}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	line := strings.SplitN(string(raw), "\n", 2)[0]
	for _, want := range []string{"run=cd%20/srv/app%20&&%20.%20venv/bin/activate", "run=-htop%20-d%205", "run=%2Dliteral%20dash%20100%25"} {
		if !strings.Contains(line, want) {
			t.Errorf("Host line %q lacks %q", line, want)
		}
	}
	hosts, _ := Load([]string{path})
	h, _ := findHost(hosts, "box")
	if !reflect.DeepEqual(h.Run, steps) {
		t.Fatalf("Run = %+v, want %+v", h.Run, steps)
	}

	// Metadata writes and an edit keep them, in order.
	if err := TouchLastUsed(path, "box", time.Unix(1717000000, 0)); err != nil {
		t.Fatal(err)
	}
	if err := SetFav(path, "box", true); err != nil {
		t.Fatal(err)
	}
	h.User = "ops"
	h.Fav = true
	if err := Update(path, "box", h); err != nil {
		t.Fatal(err)
	}
	hosts, _ = Load([]string{path})
	h, _ = findHost(hosts, "box")
	if !reflect.DeepEqual(h.Run, steps) || h.User != "ops" {
		t.Fatalf("after writes: %+v", h)
	}
}

func TestRunOnlyLineIsSshcMetadata(t *testing.T) {
	path := writeTemp(t, "config", "Host box #sshc run=uptime\n  HostName 10.0.0.9\n")
	n, err := StripMeta(path)
	if err != nil || n != 1 {
		t.Fatalf("StripMeta = %d, %v", n, err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "run=") {
		t.Fatalf("run= left after clean:\n%s", raw)
	}
}

func TestBadRunTokenIsKeptVerbatim(t *testing.T) {
	// An empty step (or one from a newer sshc this one can't read) isn't
	// dropped by a write.
	path := writeTemp(t, "config", "Host box #sshc run= run=ok\n  HostName 10.0.0.9\n")
	if err := SetFav(path, "box", true); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	line := strings.SplitN(string(raw), "\n", 2)[0]
	if !strings.Contains(line+" ", " run= ") {
		t.Fatalf("bad token dropped: %q", line)
	}
	hosts, _ := Load([]string{path})
	h, _ := findHost(hosts, "box")
	if len(h.Run) != 1 || h.Run[0].Cmd != "ok" {
		t.Fatalf("Run = %+v", h.Run)
	}
}

func TestValidateRunStep(t *testing.T) {
	if ValidateRunStep("cd /srv") != nil {
		t.Error("a plain command was rejected")
	}
	for _, bad := range []string{"", "   ", "a\nb", "a\rb"} {
		if ValidateRunStep(bad) == nil {
			t.Errorf("ValidateRunStep(%q) = nil", bad)
		}
	}
}
