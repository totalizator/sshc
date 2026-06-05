package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/totalizator/sshc/ssh"
)

// viewRow is one entry in the rendered list: either a group header or a host.
type viewRow struct {
	header bool
	label  string // header text (rendered uppercase)
	glyph  string // header glyph ("★" for pinned, "" otherwise)
	host   ssh.Host
	selIdx int // index into m.visible for a host row (-1 for headers)
}

// rowBlock is a rendered row as one or more terminal lines, tagged when it is
// the current selection so the viewport can keep it on screen.
type rowBlock struct {
	lines    []string
	selected bool
}

// listPane renders the framed list region: it builds every row, windows them to
// h lines around the selection, and wraps the result in the variant's pane.
func (m Model) listPane(w, h int) string {
	contentW := w
	frame := m.styles.ListPane.GetHorizontalFrameSize()
	contentW -= frame
	if contentW < 10 {
		contentW = 10
	}
	innerH := h - m.styles.ListPane.GetVerticalFrameSize()
	if innerH < 1 {
		innerH = 1
	}

	// Lines are built to the text-area width (contentW); the pane adds its own
	// padding+border around them, so we must NOT also force .Width() — lipgloss
	// counts horizontal padding *inside* Width, which would re-wrap every row.
	body := m.renderRowsWindowed(contentW, innerH)
	return m.styles.ListPane.Render(body)
}

// renderRowsWindowed builds all rows, flattens them to lines, and returns the
// slice of lines visible given the selection and pane height (padded to h).
func (m Model) renderRowsWindowed(contentW, h int) string {
	rows := m.renderRows(contentW)
	if len(rows) == 0 {
		// Force the message block to the full content width so the pane border
		// spans the list area instead of shrinking to the text.
		empty := lipgloss.NewStyle().Width(contentW).Render(m.styles.Empty.Render(m.emptyMessage()))
		return strings.Join(padTo(strings.Split(empty, "\n"), h), "\n")
	}

	var lines []string
	selStart, selEnd := -1, -1
	for _, rb := range rows {
		if rb.selected {
			selStart = len(lines)
			selEnd = len(lines) + len(rb.lines) - 1
		}
		lines = append(lines, rb.lines...)
	}

	offset := 0
	if len(lines) > h {
		if selEnd >= h {
			offset = selEnd - h + 1
		}
		if max := len(lines) - h; offset > max {
			offset = max
		}
		if offset < 0 {
			offset = 0
		}
		_ = selStart
		lines = lines[offset : offset+h]
	}
	return strings.Join(padTo(lines, h), "\n")
}

// renderRows turns the current filtered/grouped view into rendered row blocks.
func (m Model) renderRows(contentW int) []rowBlock {
	blocks := make([]rowBlock, 0, len(m.rows))
	for _, r := range m.rows {
		if r.header {
			blocks = append(blocks, rowBlock{lines: []string{m.groupHeader(r, contentW)}})
			continue
		}
		sel := r.selIdx == m.sel
		blocks = append(blocks, rowBlock{lines: m.hostBlock(r.host, sel, contentW), selected: sel})
	}
	return blocks
}

// groupHeader renders a "★ PINNED ────" / "ALL HOSTS ────" divider line.
func (m Model) groupHeader(r viewRow, contentW int) string {
	left := ""
	if r.glyph != "" {
		left += m.styles.GroupGlyph.Render(r.glyph) + " "
	}
	left += m.styles.GroupLabel.Render(strings.ToUpper(r.label)) + " "
	fill := contentW - lipgloss.Width(left)
	if fill < 0 {
		fill = 0
	}
	return left + m.styles.GroupRule.Render(strings.Repeat("─", fill))
}

// hostBlock renders one host as 1 (compact) or 2 (comfortable) lines of exactly
// contentW columns. When selected, every cell carries the selection background
// so the tinted band spans the full row width.
func (m Model) hostBlock(h ssh.Host, sel bool, contentW int) []string {
	bg := ""
	if sel {
		bg = m.styles.SelBG
	}
	st := m.styles

	// gutter: left bar + favourite star + separator. Its width is measured
	// rather than assumed: ★ and ▌ can render as double-width glyphs, so a
	// fixed count would drift the right-hand columns on favourite rows.
	bar := m.barCell(h, sel)
	star := m.starCell(h, sel)
	sep := bgSpace(1, bg)
	gutter := bar + star + sep
	gutterW := lipgloss.Width(gutter)
	bodyW := contentW - gutterW
	if bodyW < 4 {
		bodyW = 4
	}

	aliasBase := withBG(st.Alias, bg, sel)
	if sel {
		aliasBase = withBG(st.AliasSel, bg, true)
	}
	aliasHL := withBG(st.Highlight, bg, sel)
	alias := highlight(h.Alias, m.query, aliasBase, aliasHL)

	if st.Density == DensityCompact {
		return []string{gutter + m.compactBody(h, sel, bg, bodyW, alias)}
	}

	// comfortable: line 1 = alias + tags/env, line 2 = addr + last-used.
	right1 := m.tagsAndChip(h, sel, bg)
	line1 := gutter + joinLR(alias, right1, bodyW, bg)

	addr := highlight(h.Target(), m.query, withBG(st.Addr, bg, sel), withBG(st.Highlight, bg, sel))
	if vb := verboseBits(h, m.verbose); vb != "" {
		addr += withBG(st.Vbits, bg, sel).Render("  ·  " + vb)
	}
	last := withBG(st.Last, bg, sel).Render(ssh.LastUsedLabel(h.LastUsed))
	// Indent line 2 to the same gutter width so the address aligns under the
	// alias regardless of the star/bar glyph widths.
	line2 := bar + bgSpace(gutterW-lipgloss.Width(bar), bg) + joinLR(addr, last, bodyW, bg)

	return []string{line1, line2}
}

// compactBody renders the single-line compact row body as aligned columns:
// alias | addr | env-chip | last-used. The chip and last-used each sit in a
// fixed-width right-aligned column so they line up vertically regardless of
// label width (the design's `22ch 1fr auto auto` grid).
func (m Model) compactBody(h ssh.Host, sel bool, bg string, bodyW int, alias string) string {
	st := m.styles
	addr := highlight(h.Target(), m.query, withBG(st.Addr, bg, sel), withBG(st.Highlight, bg, sel))
	left := colLeft(alias, 22, bg) + bgSpace(1, bg) + addr

	// Right cluster mirrors comfortable: tags · env chip butt together, then a
	// fixed-width last-used column. Only last-used is a fixed column, so the
	// chip's right edge (and thus the chip) still lines up vertically while the
	// tags sit immediately to its left.
	last := colRight(withBG(st.Last, bg, sel).Render(ssh.LastUsedLabel(h.LastUsed)), compactLastCol, bg)
	cluster := st.chip(h.Env, bg)
	if len(h.Tags) > 0 {
		tags := withBG(st.Tags, bg, sel).Render(hashTags(h.Tags))
		if cluster != "" {
			cluster = tags + bgSpace(1, bg) + cluster
		} else {
			cluster = tags
		}
	}
	right := last
	if cluster != "" {
		right = cluster + bgSpace(1, bg) + last
	}
	return joinLR(left, right, bodyW, bg)
}

// compactLastCol is the fixed width of the compact last-used column.
const compactLastCol = 8

// colRight right-aligns a (possibly styled) cell within width w, padding the
// left with the selection background when set. Cells wider than w are left as-is.
func colRight(s string, w int, bg string) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return bgSpace(w-vis, bg) + s
}

// colLeft left-aligns a (possibly styled) cell within width w, padding the right
// with the selection background when set. Cells wider than w are truncated.
func colLeft(s string, w int, bg string) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return lipgloss.NewStyle().MaxWidth(w).Render(s)
	}
	return s + bgSpace(w-vis, bg)
}

// tagsAndChip builds the right side of a comfortable line 1: "#tag #tag  chip".
func (m Model) tagsAndChip(h ssh.Host, sel bool, bg string) string {
	var parts []string
	if len(h.Tags) > 0 {
		parts = append(parts, withBG(m.styles.Tags, bg, sel).Render(hashTags(h.Tags)))
	}
	if chip := m.styles.chip(h.Env, bg); chip != "" {
		parts = append(parts, chip)
	}
	return strings.Join(parts, bgSpace(1, bg))
}

// barCell renders the 1-column left bar: env-coloured in rich, accent when
// selected elsewhere, otherwise blank (still carrying the selection bg).
func (m Model) barCell(h ssh.Host, sel bool) string {
	bg := ""
	if sel {
		bg = m.styles.SelBG
	}
	if m.styles.Variant == VariantRich {
		hex, ok := envColor(h.Env, m.styles.EnvOverrides)
		if !ok {
			hex = colorEnvBar
		}
		return withBG(lipgloss.NewStyle().Foreground(color(hex)), bg, sel).Render("▌")
	}
	if sel {
		return lipgloss.NewStyle().Foreground(color(m.styles.Accent)).Background(color(bg)).Render("▌")
	}
	return " "
}

// starCell renders the 1-column favourite marker.
func (m Model) starCell(h ssh.Host, sel bool) string {
	bg := ""
	if sel {
		bg = m.styles.SelBG
	}
	if !h.Fav {
		return bgSpace(1, bg)
	}
	style := m.styles.Star
	if m.styles.Variant == VariantRich {
		style = m.styles.StarFav
	}
	return withBG(style, bg, sel).Render("★")
}

// verboseBits returns the inline "key … · via … · agent" suffix, or "".
func verboseBits(h ssh.Host, verbose bool) string {
	if !verbose {
		return ""
	}
	var bits []string
	if h.IdentityFile != "" {
		key := strings.TrimPrefix(h.IdentityFile, "~/.ssh/")
		bits = append(bits, "key "+key)
	}
	if p := h.Proxy(); p != "" {
		bits = append(bits, "via "+p)
	}
	if len(bits) == 0 {
		bits = append(bits, "agent")
	}
	return strings.Join(bits, "  ·  ")
}

func (m Model) emptyMessage() string {
	if strings.TrimSpace(m.query) != "" {
		return "no hosts match “" + m.query + "” · ^n to create one"
	}
	return "no hosts yet · ^n to create one"
}

// --- small layout helpers ---

// joinLR places left and right on a line of width w, filling the gap (with the
// selection background when bg is set). The left side is truncated if needed.
func joinLR(left, right string, w int, bg string) string {
	rw := lipgloss.Width(right)
	maxLeft := w - rw - 1
	if maxLeft < 1 {
		maxLeft = 1
	}
	if lipgloss.Width(left) > maxLeft {
		left = lipgloss.NewStyle().MaxWidth(maxLeft).Render(left)
	}
	gap := w - lipgloss.Width(left) - rw
	if gap < 1 {
		gap = 1
	}
	return left + bgSpace(gap, bg) + right
}

// bgSpace returns n spaces, carrying bg as a background when set.
func bgSpace(n int, bg string) string {
	s := strings.Repeat(" ", n)
	if bg == "" {
		return s
	}
	return lipgloss.NewStyle().Background(color(bg)).Render(s)
}

// withBG returns st with the selection background applied when on.
func withBG(st lipgloss.Style, bg string, on bool) lipgloss.Style {
	if on && bg != "" {
		return st.Background(color(bg))
	}
	return st
}

// padTo pads a slice of lines to exactly n entries with empty lines.
func padTo(lines []string, n int) []string {
	for len(lines) < n {
		lines = append(lines, "")
	}
	return lines[:n]
}
