package ssh

import (
	"os"
	"strings"
	"testing"
)

func TestParseForward(t *testing.T) {
	cases := []struct {
		remote  bool
		value   string
		want    Forward
		listen  string
		target  string
		flag    string
		wantErr bool
	}{
		{value: "8080 localhost:80", want: Forward{Port: 8080, Host: "localhost", HostPort: 80},
			listen: "127.0.0.1:8080", target: "localhost:80", flag: "-L 8080:localhost:80"},
		{value: "  0.0.0.0:5432\tdb.internal:5432 ", want: Forward{Bind: "0.0.0.0", Port: 5432, Host: "db.internal", HostPort: 5432},
			listen: "0.0.0.0:5432", target: "db.internal:5432", flag: "-L 0.0.0.0:5432:db.internal:5432"},
		{value: "*:3000 127.0.0.1:3000", want: Forward{Bind: "*", Port: 3000, Host: "127.0.0.1", HostPort: 3000},
			listen: "0.0.0.0:3000", target: "127.0.0.1:3000", flag: "-L *:3000:127.0.0.1:3000"},
		{value: ":7000 h:7", want: Forward{Port: 7000, Host: "h", HostPort: 7},
			listen: "127.0.0.1:7000", target: "h:7", flag: "-L 7000:h:7"},
		{value: "[::1]:8080 [fe80::1]:80", want: Forward{Bind: "::1", Port: 8080, Host: "fe80::1", HostPort: 80},
			listen: "[::1]:8080", target: "[fe80::1]:80", flag: "-L [::1]:8080:[fe80::1]:80"},
		{remote: true, value: "9000 localhost:9000", want: Forward{Remote: true, Port: 9000, Host: "localhost", HostPort: 9000},
			listen: "127.0.0.1:9000", target: "localhost:9000", flag: "-R 9000:localhost:9000"},
		{remote: true, value: "0 localhost:22", want: Forward{Remote: true, Port: 0, Host: "localhost", HostPort: 22},
			listen: "127.0.0.1:0", target: "localhost:22", flag: "-R 0:localhost:22"},
		{remote: true, value: "1080", wantErr: true},
		{value: "8080", wantErr: true},
		{value: "8080 localhost:80 extra", wantErr: true},
		{value: "/tmp/a.sock localhost:80", wantErr: true},
		{value: "8080 /run/docker.sock", wantErr: true},
		{value: "8080 localhost", wantErr: true},
		{value: "8080 :80", wantErr: true},
		{value: "8080 localhost:0", wantErr: true},
		{value: "70000 localhost:80", wantErr: true},
		{value: "http localhost:80", wantErr: true},
		{value: "::1:8080 localhost:80", wantErr: true},
		{value: "[::1 localhost:80", wantErr: true},
	}
	for _, c := range cases {
		got, err := ParseForward(c.remote, c.value)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseForward(%v, %q) = %+v, want an error", c.remote, c.value, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseForward(%v, %q): %v", c.remote, c.value, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseForward(%v, %q) = %+v, want %+v", c.remote, c.value, got, c.want)
		}
		if a := got.ListenAddr(); a != c.listen {
			t.Errorf("%q ListenAddr = %q, want %q", c.value, a, c.listen)
		}
		if a := got.TargetAddr(); a != c.target {
			t.Errorf("%q TargetAddr = %q, want %q", c.value, a, c.target)
		}
		if s := got.String(); s != c.flag {
			t.Errorf("%q String = %q, want %q", c.value, s, c.flag)
		}
	}
}

func TestLoadReadsForwards(t *testing.T) {
	path := writeTemp(t, "config", `Host box
  HostName 10.0.0.9
  LocalForward 8080 localhost:80
  DynamicForward 1080
  remoteforward=9000 localhost:9000
  LocalForward /tmp/a.sock /tmp/b.sock
  LocalForward 5432  db:5432
`)
	hosts, err := Load([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	h, _ := findHost(hosts, "box")
	want := []string{"-L 8080:localhost:80", "-R 9000:localhost:9000", "-L 5432:db:5432"}
	if len(h.Forwards) != len(want) {
		t.Fatalf("Forwards = %+v, want %v", h.Forwards, want)
	}
	for i, f := range h.Forwards {
		if f.String() != want[i] {
			t.Errorf("Forwards[%d] = %q, want %q", i, f.String(), want[i])
		}
	}
	other := []string{"DynamicForward 1080", "LocalForward /tmp/a.sock /tmp/b.sock"}
	if strings.Join(h.OtherForwards, "|") != strings.Join(other, "|") {
		t.Errorf("OtherForwards = %q, want %q", h.OtherForwards, other)
	}
}

func TestUpdateKeepsForwardDirectives(t *testing.T) {
	path := writeTemp(t, "config", "Host box\n  HostName 10.0.0.9\n  LocalForward 8080 localhost:80\n  RemoteForward 9000 localhost:9000\n")
	hosts, _ := Load([]string{path})
	h, _ := findHost(hosts, "box")
	h.User = "ops"
	h.Forwards = nil // sshc never writes forwards; the directives stay as written
	if err := Update(path, "box", h); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	for _, want := range []string{"LocalForward 8080 localhost:80", "RemoteForward 9000 localhost:9000", "User ops"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("config lacks %q after Update:\n%s", want, raw)
		}
	}
}
