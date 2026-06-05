package tui

import (
	"strings"
	"testing"

	"github.com/totalizator/sshc/ssh"
)

func TestConnectArgs(t *testing.T) {
	h := ssh.Host{Alias: "web", HostName: "10.0.0.7"}
	cases := []struct {
		name        string
		host        ssh.Host
		tmpl        string
		sessionUser string
		want        []string
	}{
		{
			name: "default template, no prompt",
			host: h,
			want: []string{"ssh", "web"},
		},
		{
			name:        "prompt injects -l on default ssh template",
			host:        h,
			sessionUser: "deploy",
			want:        []string{"ssh", "-l", "deploy", "web"},
		},
		{
			name:        "blank prompt leaves command untouched",
			host:        h,
			sessionUser: "",
			want:        []string{"ssh", "web"},
		},
		{
			name:        "custom template with {{{user}}} substitutes instead of -l",
			host:        h,
			tmpl:        `ssh {{{user}}}@{{{hostname}}}`,
			sessionUser: "deploy",
			want:        []string{"ssh", "deploy@10.0.0.7"},
		},
		{
			name:        "non-ssh template is not given -l",
			host:        h,
			tmpl:        `mosh {{{name}}}`,
			sessionUser: "deploy",
			want:        []string{"mosh", "web"},
		},
		{
			name: "saved user, no prompt, default template (relies on config)",
			host: ssh.Host{Alias: "web", HostName: "10.0.0.7", User: "root"},
			want: []string{"ssh", "web"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := connectArgs(c.host, c.tmpl, c.sessionUser)
			if err != nil {
				t.Fatalf("connectArgs: %v", err)
			}
			if strings.Join(got, " ") != strings.Join(c.want, " ") {
				t.Fatalf("args = %v, want %v", got, c.want)
			}
		})
	}
}

func TestLooksLikeSSH(t *testing.T) {
	yes := []string{"ssh", "/usr/bin/ssh", `C:\Windows\System32\OpenSSH\ssh.exe`, "SSH.EXE"}
	no := []string{"mosh", "sshpass", "ssh-copy-id", "telnet"}
	for _, c := range yes {
		if !looksLikeSSH(c) {
			t.Errorf("looksLikeSSH(%q) = false, want true", c)
		}
	}
	for _, c := range no {
		if looksLikeSSH(c) {
			t.Errorf("looksLikeSSH(%q) = true, want false", c)
		}
	}
}
