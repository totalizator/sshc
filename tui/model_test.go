package tui

import "testing"

// TestSelectionFollowsFavToggle mirrors what reload(..., focusAlias) does: after
// a host's fav state flips and the list is re-grouped, the cursor should land
// back on that same host (now in the ★ PINNED group), not on whatever host
// happens to occupy its old index.
func TestSelectionFollowsFavToggle(t *testing.T) {
	m := newSampleModel()
	m.settings.PinFavorites = true
	m.applyFilter()

	const target = "dev-sandbox" // a non-favourite in the sample set
	m.selectAlias(target)
	if h, _ := m.selectedHost(); h.Alias != target {
		t.Fatalf("setup: selected %q, want %q", h.Alias, target)
	}

	// Pin it and re-group, as reload would after ssh.SetFav.
	for i := range m.hosts {
		if m.hosts[i].Alias == target {
			m.hosts[i].Fav = true
		}
	}
	m.applyFilter()

	// Without following, the stale index points elsewhere; selectAlias fixes it.
	m.selectAlias(target)
	if h, ok := m.selectedHost(); !ok || h.Alias != target {
		t.Errorf("after pin, selected %q (ok=%v), want %q", h.Alias, ok, target)
	}
}

// TestSelectAliasAbsentIsNoOp guards the delete path: focusAlias on a host that
// no longer exists must not move (or crash) the cursor.
func TestSelectAliasAbsentIsNoOp(t *testing.T) {
	m := newSampleModel()
	m.applyFilter()
	m.sel = 3
	m.selectAlias("no-such-host")
	if m.sel != 3 {
		t.Errorf("selectAlias on absent host moved cursor to %d, want 3", m.sel)
	}
}
