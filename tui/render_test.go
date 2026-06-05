package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/totalizator/sshc/ssh"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func strip(s string) string { return ansiRE.ReplaceAllString(s, "") }

func sampleHosts() []ssh.Host {
	min := func(m int) time.Time { return time.Now().Add(-time.Duration(m) * time.Minute) }
	return []ssh.Host{
		{Alias: "prod-web-01", HostName: "10.0.4.21", User: "deploy", Env: "prod", Fav: true, IdentityFile: "~/.ssh/id_ed25519", ProxyJump: "bastion-acme", Tags: []string{"web", "acme"}, LastUsed: min(120), Editable: true, Source: "~/.ssh/config"},
		{Alias: "prod-web-02", HostName: "10.0.4.22", User: "deploy", Env: "prod", IdentityFile: "~/.ssh/id_ed25519", ProxyJump: "bastion-acme", Tags: []string{"web", "acme"}, LastUsed: min(130), Editable: true, Source: "~/.ssh/config"},
		{Alias: "prod-db-01", HostName: "10.0.4.40", User: "deploy", Env: "prod", Tags: []string{"db"}, LastUsed: min(1440), Editable: true, Source: "~/.ssh/config"},
		{Alias: "bastion-acme", HostName: "bastion.acme.io", User: "jump", Env: "prod", Fav: true, IdentityFile: "~/.ssh/id_ed25519", Tags: []string{"jump"}, LastUsed: min(300), Editable: true, Source: "~/.ssh/config"},
		{Alias: "gpu-trainer", HostName: "gpu.ml.io", User: "paperspace", Env: "cloud", Fav: true, IdentityFile: "~/.ssh/id_ml", ProxyJump: "bastion-acme", Tags: []string{"ml", "gpu"}, LastUsed: min(30), Editable: true, Source: "~/.ssh/config"},
		{Alias: "vps-hetzner", HostName: "65.108.74.12", User: "root", Env: "edge", Tags: []string{"edge"}, LastUsed: min(360), Editable: true, Source: "~/.ssh/config"},
		{Alias: "dev-sandbox", HostName: "dev.local", User: "ubuntu", Port: "2222", Env: "dev", Tags: []string{"scratch"}, LastUsed: min(20), Editable: true, Source: "~/.ssh/config"},
		{Alias: "proxmox-krk-01", HostName: "192.168.1.34", User: "root", Env: "home", Fav: true, Tags: []string{"hypervisor"}, LastUsed: min(5760), Editable: true, Source: "~/.ssh/config"},
		{Alias: "staging-api", HostName: "staging.acme.io", User: "deploy", Env: "staging", Tags: []string{"api"}, LastUsed: min(4320), Editable: true, Source: "~/.ssh/config"},
		{Alias: "ci-runner-02", HostName: "10.0.9.12", User: "runner", Env: "dev", Tags: []string{"ci"}, LastUsed: min(2880), Editable: true, Source: "~/.ssh/config"},
	}
}

func render(t *testing.T, m Model, w, h int) string {
	t.Helper()
	model, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return model.(Model).View()
}

func newSampleModel() Model {
	opts := Options{Hosts: sampleHosts(), Version: "v0.3.0", Settings: DefaultSettings(), ConfigPaths: []string{"~/.ssh/config"}}
	return newModel(opts)
}

func TestRenderList(t *testing.T) {
	out := strip(render(t, newSampleModel(), 120, 40))
	for _, want := range []string{"sshc", "v0.3.0", "10 hosts", "PINNED", "ALL HOSTS", "prod-web-01", "deploy@10.0.4.21:22", "prod", "30m ago"} {
		if !strings.Contains(out, want) {
			t.Errorf("list view missing %q", want)
		}
	}
	t.Log("\n" + out)
}

func TestRenderDetail(t *testing.T) {
	m := newSampleModel()
	m.showDetail = true
	out := strip(render(t, m, 120, 40))
	for _, want := range []string{"CONNECTION", "RESOLVED COMMAND", "$ ssh"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail view missing %q", want)
		}
	}
	t.Log("\n" + out)
}

func TestRenderVerbose(t *testing.T) {
	m := newSampleModel()
	m.verbose = true
	out := strip(render(t, m, 120, 40))
	if !strings.Contains(out, "key id_ed25519") || !strings.Contains(out, "via bastion-acme") {
		t.Errorf("verbose bits missing:\n%s", out)
	}
	t.Log("\n" + out)
}

func TestRenderSearch(t *testing.T) {
	m := newSampleModel()
	m.search.SetValue("prod")
	m.applyFilter()
	out := strip(render(t, m, 120, 40))
	if !strings.Contains(out, "matches") || !strings.Contains(out, "prod-web-01") {
		t.Errorf("search view wrong:\n%s", out)
	}
	if strings.Contains(out, "dev-sandbox") {
		t.Errorf("search should have filtered out dev-sandbox:\n%s", out)
	}
	t.Log("\n" + out)
}

func TestRenderForm(t *testing.T) {
	m := newSampleModel()
	mm, _ := m.openForm(newFormModel())
	out := strip(render(t, mm.(Model), 120, 40))
	for _, want := range []string{"new connection", "Alias", "HostName", "Tags", "Env", "save"} {
		if !strings.Contains(out, want) {
			t.Errorf("form view missing %q", want)
		}
	}
	t.Log("\n" + out)
}

func TestRenderHelp(t *testing.T) {
	m := newSampleModel()
	m.showHelp = true
	out := strip(render(t, m, 120, 40))
	for _, want := range []string{"keybindings", "connect", "esc to close"} {
		if !strings.Contains(out, want) {
			t.Errorf("help overlay missing %q", want)
		}
	}
	t.Log("\n" + out)
}

func TestRenderConfirm(t *testing.T) {
	m := newSampleModel()
	mm, _ := m.startDelete()
	out := strip(render(t, mm.(Model), 120, 40))
	for _, want := range []string{"Delete", "gpu-trainer", "backup is kept", "cancel"} {
		if !strings.Contains(out, want) {
			t.Errorf("confirm overlay missing %q", want)
		}
	}
	t.Log("\n" + out)
}

func TestRenderEmpty(t *testing.T) {
	m := newSampleModel()
	m.search.SetValue("zzz")
	m.applyFilter()
	out := strip(render(t, m, 120, 40))
	if !strings.Contains(out, "no hosts match") {
		t.Errorf("empty view wrong:\n%s", out)
	}
	t.Log("\n" + out)
}

func TestRenderCompactAlignment(t *testing.T) {
	for _, v := range []Variant{VariantMinimal, VariantFramed} {
		set := DefaultSettings()
		set.Variant, set.Density = v, DensityCompact
		m := newModel(Options{Hosts: sampleHosts(), Version: "v0.3.0", Settings: set})
		out := strip(render(t, m, 110, 30))

		// Every visible last-used label must end at the same display column so
		// the rightmost column lines up across rows. Measure display width (not
		// byte offset) so multi-byte glyphs like ★/▌ don't skew the result.
		var cols []int
		for _, line := range strings.Split(out, "\n") {
			for _, lbl := range []string{"ago", "never"} {
				if i := strings.LastIndex(line, lbl); i >= 0 {
					cols = append(cols, lipgloss.Width(line[:i+len(lbl)]))
				}
			}
		}
		if len(cols) < 3 {
			t.Fatalf("%s/compact: expected several last-used labels, got %d", v, len(cols))
		}
		if !strings.Contains(out, "#web") || !strings.Contains(out, "#gpu") {
			t.Errorf("%s/compact: rows should show tags:\n%s", v, out)
		}
		for _, c := range cols[1:] {
			if c != cols[0] {
				t.Errorf("%s/compact: last-used column not aligned: %v", v, cols)
				break
			}
		}
		t.Logf("%s/compact:\n%s", v, out)
	}
}

func TestSelectedRowBandContinuous(t *testing.T) {
	// On the selected row every cell (separators, chip surrounds, column pads)
	// must carry the selection background. A reset-to-default ("\x1b[0m")
	// immediately followed by a literal space is the signature of an un-themed
	// gap that shows through as a black notch.
	for _, d := range []Density{DensityComfortable, DensityCompact} {
		set := DefaultSettings()
		set.Density = d
		m := newModel(Options{Hosts: sampleHosts(), Version: "v0.3.0", Settings: set, ConfigPaths: []string{"~/.ssh/config"}})
		raw := render(t, m, 120, 40)
		for _, line := range strings.Split(raw, "\n") {
			if !strings.Contains(strip(line), "gpu-trainer") {
				continue
			}
			if strings.Contains(line, "\x1b[0m ") {
				t.Errorf("%s: selected row has an un-themed gap (reset+space):\n%q", d, line)
			}
		}
	}
}

func TestRenderVariants(t *testing.T) {
	for _, v := range []Variant{VariantMinimal, VariantFramed, VariantRich} {
		for _, d := range []Density{DensityComfortable, DensityCompact} {
			set := DefaultSettings()
			set.Variant, set.Density = v, d
			m := newModel(Options{Hosts: sampleHosts(), Version: "v0.3.0", Settings: set})
			out := render(t, m, 120, 40)
			if !strings.Contains(strip(out), "prod-web-01") {
				t.Errorf("%s/%s missing host", v, d)
			}
		}
	}
}
