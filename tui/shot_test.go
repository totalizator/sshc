package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/totalizator/sshc/ssh"
)

// TestGenerateShots renders a few TUI states to ANSI files under demo/_ansi when
// SSHC_SHOT=1. It is a generator (not an assertion test) and is skipped by
// default. Convert the ANSI dumps to SVG/PNG for the README.
//
//	SSHC_SHOT=1 go test ./tui -run TestGenerateShots
func TestGenerateShots(t *testing.T) {
	if os.Getenv("SSHC_SHOT") == "" {
		t.Skip("set SSHC_SHOT=1 to generate screenshots")
	}
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)

	hosts, err := ssh.Load([]string{filepath.Join("..", "demo", "ssh_config")})
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join("..", "demo", "_ansi")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}

	// Width 128 keeps the footer hint bar (123 cols) on a single line; height
	// 34 shows a full list/detail. Both feed the freeze -> PNG demo pipeline.
	const w, h = 128, 34

	mk := func() Model {
		return newModel(Options{
			Hosts:       hosts,
			Version:     "v0.4.1",
			Settings:    DefaultSettings(),
			ConfigPaths: []string{"demo/ssh_config"},
		})
	}
	dump := func(name string, m Model) {
		model, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
		if err := os.WriteFile(filepath.Join(out, name), []byte(model.(Model).View()), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	dump("01-list.ansi", mk())

	ms := mk()
	ms.search.SetValue("edge")
	ms.applyFilter()
	dump("02-search.ansi", ms)

	md := mk()
	md.showDetail = true
	dump("03-detail.ansi", md)
}
