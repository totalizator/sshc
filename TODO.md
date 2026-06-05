# TODO

## Per-host metadata — persistence decision (landed in the TUI redesign)

The TUI redesign shipped favourites, tags, and env labels. The open
"state file vs. structured comment" question is now **decided: structured
comment on the `Host` line**, e.g.

```
Host prod-web-01 #sshc env=prod tags=web,acme fav=1 used=1717000000
```

Rationale: the metadata travels with the host when the config is shared/synced,
and plain `ssh` ignores the comment. See `ssh/meta.go` (`parseMeta`/`formatMeta`)
and the README "Appearance & metadata" section. `TouchLastUsed`/`SetFav` edit
only the comment so the block body is preserved byte-for-byte.

## Feature ideas

- **Starring / pinning favourite hosts.** ✅ **Done.** `Ctrl+F` toggles a `★`
  favourite; pinned hosts float into a `★ PINNED` group above `ALL HOSTS` when
  the search is empty. Persisted in the `#sshc fav=1` comment (not a state file
  as originally sketched). Open question still stands: a star is keyed by alias,
  so an *external* rename outside sshc loses it — an in-app edit re-keys fine.
  - ✅ **Done.** Toggling fav (`Ctrl+F`) now keeps the selection on the entry
    that was just (un)favourited — it follows the host as it moves between the
    `★ PINNED` and `ALL HOSTS` groups, instead of the cursor staying at the old
    list position. Implemented via `reload(..., focusAlias)` / `selectAlias` in
    `tui/model.go`; edit-save follows the host too, while delete intentionally
    keeps the clamped position.

- **Per-host colours (optional).** ◑ **Partially done.** Each host shows a
  coloured **env chip**, and the `rich` variant paints a per-env left gutter bar;
  env colours use fixed presets or a stable hue derived from the env string
  (hash → hue), matching the "derive from a seed" idea. Still **TODO**:
  - a *colour independent of env* (arbitrary per-host colour) with a form field
    and live preview as you type;
  - an explicit pick (cycle a palette / accept hex/256);
  - a global `--no-color` / `NO_COLOR` kill switch (lipgloss already degrades by
    terminal capability, but an explicit opt-out is not wired yet).

- **Tagging hosts.** ◑ **Mostly done.** A comma-separated **Tags** form field;
  tags show inline on rows and in the detail panel; fuzzy search matches tags.
  Persisted in the `#sshc tags=…` comment. Still **TODO**:
  - a dedicated `tag:prod` filter token (and/or a tag-filter mode);
  - optional grouping of the list by tag;
  - tag autocomplete from tags already in use.

## Nice-to-have (deferred)

- **Shrink release binaries with `-s -w`.** ✅ **Done** (release CI). The release
  `-ldflags` in `.github/workflows/release.yaml` already strip the symbol table and
  DWARF debug info — ~28% smaller for free (measured: 6.78 MB → 4.87 MB on
  linux/amd64), no runtime cost. Tradeoff: no symbolized panic stacks / symbol
  debugging on the shipped binary (fine for a release artifact). `make build` is
  intentionally left **unstripped** for local debugging. UPX was deliberately
  skipped: the extra ~50% isn't worth the Windows AV/SmartScreen false positives
  and macOS arm64 signing/notarization breakage for a binary that's only a few
  MB to begin with.
- **Indent edited/added blocks.** ✅ **Done.** Blocks sshc writes are now
  rendered in canonical OpenSSH style (four-space-indented directives + a blank
  line between blocks) by `render`/`canonicalBlock` in `ssh/write.go`, while
  untouched blocks stay byte-for-byte. A `sshc clean [--dry-run]` command strips
  sshc's `#sshc …` metadata comments to return the file to plain-ssh form.

## Android / Termux support

Cross-compiled `linux/*` release binaries do **not** run under Termux; Android
rejects them in three successive stages:

1. **`e_type: 2`** — default Go static builds are `ET_EXEC`; Android's loader only
   accepts PIE (`ET_DYN`). Fixed by `-buildmode=pie` (now in the release build).
2. **TLS underalignment** — bionic on arm64 needs the TLS segment aligned to ≥64
   bytes; Go's `GOOS=linux` linker emits 8. `termux-elf-cleaner` patches this
   in place, but it shouldn't be required.
3. **Doubled `argv`** — even after the above, the binary receives its own path as
   `argv[1]` under Android's loader, so cobra treats it as an unknown subcommand
   (`unknown command "/data/.../sshc"`). Not patchable from outside the binary.

A cross-compiled `GOOS=linux` binary is the wrong target for Android. **Current
guidance: build from source in Termux** (`pkg install golang`, clone, `go build`),
where Go targets `android/arm64` natively — documented in README "Android /
Termux".

**TODO (deferred):** decide whether to ship a prebuilt Android binary via a
`GOOS=android` CI artifact. That needs the Android NDK + `CGO_ENABLED=1` in the
release workflow — so build-from-source is the pragmatic answer until demand
justifies the NDK
toolchain. The `-buildmode=pie` linux change stays regardless (ASLR / modern
default), it's just not sufficient for Android on its own.

## Demo screenshots — row gap ✅ Done

The README screenshots used to show a visible **vertical gap between each host's
two rows** (alias vs. `user@host`), with the gutter bar `▌` / frame borders `│`
not tiling. The original diagnosis (ansisvg's grid cell taller than the glyphs) was
on the right track but the fix was mis-stated. Two distinct causes:

1. **Block/box glyphs don't fill a 9×19 cell.** ansisvg draws `▌` and `│` as
   fontsize-16 glyphs; in a 19px-tall cell they leave a ~2px seam between stacked
   rows (the background *bands* tile fine — only these glyphs gap). Fixed by
   tightening the cell to **9×17** (`--charboxsize`), the tallest cell at which the
   glyphs still touch. 9×18/19 reopen the seam.
2. **Non-integer rasterizer rescale.** `rasterize.ps1` baked the 1152px-wide SVG
   into a 990px window, so Chrome downscaled by a fractional factor and reopened a
   ~1px seam even where the SVG was clean. Fixed by rendering at the SVG's natural
   size (**1152×578** at 9×17; window == SVG).

The shot terminal also went to **128×34** so the footer hint bar stays on one line.
`charmbracelet/freeze` was evaluated as an alternative and rejected: it tiles rows
fine but mis-sizes the chip background rects (they bleed left over the tags) — a
freeze bug present through v0.2.2/@main. See `tui/shot_test.go`, the Makefile
`shots` target, `demo/rasterize.ps1`, and `demo/README.md`.

## SSH config write model — follow-ups

The write model and its guarantees are documented in `docs/config-writes.md`.
Open items tracked there (not yet scheduled):

- Faithful **tab** preservation in untouched blocks (the parser normalizes a
  leading tab to a single space on any round-trip).
- **CRLF** handling: a rewritten block emits `\n`, so it can end up mixed with
  `\r\n` untouched blocks in a Windows-authored config.
- **Multiple values for a modeled key** (e.g. two `IdentityFile` lines) collapse
  to one on edit, matching the single-field form model.
- A possible explicit, opt-in **`--reformat`** (whole-file normalize) — never the
  default, since it would touch untouched blocks.
- Consider an aligned proxy *column* in the list (currently proxy is inline in
  the description / verbose row).
