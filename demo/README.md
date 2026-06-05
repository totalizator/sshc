# Demo assets

Imagery for the project README, generated from the **live TUI** (not hand-captured)
against the bundled 100-host sample config (`ssh_config`).

## Screenshots — `01-list.png`, `02-search.png`, `03-detail.png`

Two-step pipeline:

1. `make shots` — a gated test (`tui/shot_test.go`, run with `SSHC_SHOT=1`) dumps
   the real `View()` output (at a 128×34 terminal, so the footer hint bar stays on
   one line) to `demo/_ansi/`, and `ansisvg --grid` converts each to a fixed-grid
   SVG (9×17 px cells → a 1152×578 canvas).
2. `powershell -NoProfile -File demo/rasterize.ps1` — rasterizes those SVGs to PNG
   with headless Chrome/Edge **at the SVG's natural 1152×578 size**. (On
   Linux/macOS, use `rsvg-convert` or `chromium --headless --screenshot` instead —
   same canvas.)

**Two things make the rows tile seamlessly** (both learned the hard way):

- **Cell height must match the glyph height.** The gutter bar `▌` and the box
  borders `│` are drawn as fontsize-16 *glyphs*; in a 9×19 cell they don't fill the
  cell, leaving a hairline seam between each host's two rows. A **9×17** cell makes
  them tile (9×18/19 reopen the seam). The background bands tile at any height — it's
  only the block/box glyphs that need this.
- **Rasterize at the SVG's native size.** If Chrome renders into a window narrower
  than the SVG (the old script forced a 1152px SVG into a 990px window), it rescales
  by a non-integer factor and reopens a ~1px gap. Window == SVG keeps the grid crisp.

We ship **PNG**, not SVG: GitHub sanitize an SVG's `<style>` block at
render time, which strips the embedded font sizing and chip colours, so an SVG
looks broken on the forge even though the file itself is correct. PNG renders
verbatim. The intermediate `*.svg` and `_ansi/` are gitignored.

## Dynamic preview — `preview.webp`

The README hero is an **auto-looping animated WebP** that cycles
list → search → detail. A forge README can't host a live click-to-switch gallery
(GitHub strips the JS and scoped CSS a `:target`/script switcher needs), but
an animated `<img>` plays in the browser with no scripting at all. Build it from
the three PNGs after rasterizing:

```sh
make preview          # or: python demo/animate.py  ->  demo/preview.webp
```

`demo/animate.py` (Pillow) downscales the 2304-wide frames to 1400 px and writes
a 3-frame loop with per-frame dwell times. **WebP, not GIF:** GIF's 256-colour
palette bands the amber accent, env chips, and anti-aliased text; WebP keeps them
full-colour at a fraction of a GIF's size (~150 KB).

## GIF demo — `sshc.linux.tape`, `sshc.windows.tape`

[VHS](https://github.com/charmbracelet/vhs) tapes for a looping demo GIF. Render
with `vhs demo/sshc.linux.tape` on Linux/macOS (or the official VHS Docker image);
the result is written to `demo/sshc.gif`. Native-Windows VHS currently hangs at the
headless-browser stage (charmbracelet/vhs#437), so `sshc.windows.tape` is kept
ready but not yet usable — see the header comment in each tape for details.
