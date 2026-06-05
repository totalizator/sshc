# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **GitHub Actions CI/CD.** `.github/workflows/ci.yaml` runs gofmt / `go vet` /
  `go test` on pushes and PRs; `.github/workflows/release.yaml` cross-compiles
  Linux/macOS/Windows (amd64 + arm64) on each `v*` tag and attaches the archives
  plus a sha256 checksums file to a GitHub Release.

### Fixed

- **`looksLikeSSH` now recognises the ssh client regardless of path separator.**
  It used `filepath.Base`, which only splits on the host OS's separator, so a
  Windows-style `…\ssh.exe` path was not parsed on non-Windows platforms (the
  `TestLooksLikeSSH` case failed under Linux CI). It now splits on both `/` and
  `\` everywhere.

### Documentation

- **Regenerated the README screenshots without the row gap.** Two causes: the
  gutter bar `▌` and box borders `│` didn't fill a 9×19 ansisvg cell (tightened to
  9×17 so the glyphs tile), and `rasterize.ps1` baked the 1152px-wide SVG into a
  990px window, so Chrome rescaled by a non-integer factor and reopened a ~1px seam
  (now rasterized at the SVG's natural 1152×578 size). The shots are also captured
  at a 128-column terminal so the footer hint bar no longer wraps.
- **README now leads with a dynamic preview.** The hero is a single auto-looping
  animated WebP (`demo/preview.webp`) that cycles list → search → detail, built
  from the PNG frames by `demo/animate.py` (`make preview`) — replacing the old
  stack of three stills. A forge README can't host a live click-to-switch gallery
  (GitHub strips the JS/scoped CSS it would need), but an animated `<img>`
  plays with no scripting. WebP (not GIF) keeps the amber accent and chips
  full-colour at ~150 KB.
- **Trimmed and reorganised the README.** The Android/Termux build notes moved to
  [`docs/android-termux.md`](docs/android-termux.md) (the README keeps a one-line
  pointer), and the "word of caution" about in-place `~/.ssh/config` writes moved
  up under Features so it's seen before install.

### Changed

- **Linux release binaries are now built as PIE** (`-buildmode=pie`, ET_DYN),
  enabling ASLR and matching the modern-distro default. macOS is PIE by default;
  Windows is unchanged. The published v0.4.1 linux assets were rebuilt
  accordingly. (PIE is *necessary but not sufficient* to run under Android/Termux,
  which also trips over TLS alignment and `argv` handling - see the README
  "Android / Termux" section; build from source there.)

## [0.4.1] — 2026-06-03

### Fixed

- **Selection now follows a host when you pin/unpin it.** Pressing `Ctrl+F` kept
  the cursor at the old list row, so after a host floated into (or out of) the
  `★ PINNED` group the highlight landed on a different host. The selection now
  follows the toggled host. Saving an edit likewise keeps the edited host
  selected.

### Changed

- **Search now matches a host's environment.** Typing in the list filters on the
  `env` label as well as alias, address, user, and tags — so the value shown in a
  host's env chip is now type-to-filter, instead of being visible but unsearchable.
  Env ranks below the other fields, so an alias or tag hit still wins.

### Removed

- Dropped the `github.com/sahilm/fuzzy` dependency — interactive matching now
  lives in the TUI, leaving only an alphabetical sort that needs no third party.

## [0.4.0] — 2026-06-01

### Added

- **Canonical formatting for blocks sshc writes.** A host block that sshc adds,
  edits, or clones is now emitted in conventional OpenSSH style — directives
  indented by four spaces under the `Host` line, and a blank line separating the
  block from its neighbours — instead of flush-left with no separation. Blocks
  sshc does not touch are still rendered byte-for-byte (space indentation is
  preserved exactly; note the underlying parser normalises a leading *tab* to a
  space on any round-trip, independent of sshc).
- **`sshc clean` command** (with `--dry-run`) strips the sshc-managed
  `#sshc env=… tags=… fav=1 used=…` metadata comments from the writable config,
  returning it to plain-ssh form. Directives, other comments, indentation, and
  ordering are left untouched, and the original is backed up to
  `<config>.sshc.bak` first. `--config` is now a persistent flag, shared with
  subcommands.
- **Connect-time username prompt.** Pressing `↵` to connect to a host that has
  no saved **User** now opens a small prompt for a username instead of silently
  using ssh's default. Leave it blank to fall back to that default; the typed
  name is applied for that session only (passed to `ssh` as `-l <user>`, or via
  a `{{{user}}}` placeholder in a custom connect template). `esc` cancels.

### Fixed

- **Editing a host no longer drops directives sshc doesn't model, or its
  comments.** Previously `Update` rebuilt the block from the six modeled fields
  (HostName/User/Port/IdentityFile/ProxyJump/ProxyCommand), silently discarding
  any other directive (`ForwardAgent`, `LocalForward`, …) and any inline comment
  on the edited host. The edit now applies in place: modeled fields are updated
  (or removed when cleared) while every other node — unmodeled directives,
  standalone comments, blank lines, and quoted/`=`-style values — is preserved
  verbatim.
- **Control bytes can no longer enter or persist in the config.** Some
  terminals deliver Ctrl+Space (and similar) as a NUL rune; typed into a form
  field it was written to the config, corrupting env-chip colours (same label,
  two hues — `prod` vs `prod\x00`), leaving an empty coloured tile for a
  cleared env, and (for a directive value) breaking `ssh` parsing. Fixed in
  depth: form input now drops control runes, metadata strips them on read and
  write (healing existing files on the next save), and every write makes a
  final pass that removes any control byte except tab/newline/CR — so the file
  sshc writes is guaranteed clean regardless of how the byte got in. `envColor`
  also ignores control bytes when keying a chip colour.
- **Never write a config `ssh` rejects.** A bare directive with no argument
  (e.g. `User` with an empty value, from a manual edit or an older version) was
  re-emitted on save as `User ` and broke every connection with
  `no argument after keyword "user"`. Writes now prune directives with no usable
  argument (whitespace- or control-only, including a stray NUL byte) from all
  blocks, so such breakage is healed on the next save (a `.sshc.bak` backup is
  still kept).
- The connect-time username prompt no longer shows a leading `@` glyph (which
  read as if it prefixed the username). It now shows a live `→ user@host`
  preview, dimming the local-default username until you type one.
- The **User** form field now hints the ssh default (your local username) when
  blank instead of suggesting `root`.
- Detail/“resolved command” and list rows no longer render a stray `@` for a
  whitespace-only user.
- New-connection form (`Ctrl+N`) no longer pre-fills **User** with `root`; the
  field now starts blank like the others, with only **Port** seeded to `22`.
- A blank **User** is now left unset in the saved config (no `User` directive)
  instead of being defaulted to `root`, so ssh applies its own default (the
  local username) at connect time.

### Changed

- **Alias** and **HostName** now fill in for each other when one is left blank:
  a blank Alias derives from HostName (e.g. `10.0.0.7` → `Host 10.0.0.7`, a
  working `ssh` target) instead of the meaningless, collision-prone `unnamed`,
  and a blank HostName mirrors the Alias (matching ssh's own fallback) instead
  of the bogus `0.0.0.0`. Saving with both empty is rejected with "enter an
  alias or a hostname".
- Search placeholder now mentions tags ("…by name, address, user, or tags"),
  matching the fuzzy filter which already scores tags.
- Reworded the README intro's sshs reference to be neutral/appreciative.

## [0.3.0] — 2026-05-31

### Changed

- **Redesigned the TUI** to the *framed · amber · comfortable* look (Bubble Tea +
  Lip Gloss): rounded list/detail panes, two-line rows, a brand bar, an
  accent-tinted selected row with a left bar, `★ PINNED` / `ALL HOSTS` group
  headers, and a footer hint bar with toasts. The list and the add/edit form are
  now custom renderers (replacing the bubbles default delegate and the `huh`
  form) for faithful control over the layout. Switchable via `--variant`
  (`minimal | framed | rich`), `--theme` (`amber | teal | green | magenta`), and
  `--density` (`comfortable | compact`); the locked defaults ship as
  `framed · amber · comfortable`.
- Default (empty-search) ordering is now **most-recently-used first**, with
  favourites floated into a pinned group on top.
- `--no-proxy` no longer hides a column — proxy now appears in verbose rows
  (`Ctrl+V`) and the detail panel instead.
- Reworked the list into a **search-first** experience: typing filters
  immediately (no `/` to enter search mode first), matching alias, address, and
  user. Mutating actions moved to `Ctrl` combos to avoid colliding with typing:
  `Ctrl+N` new, `Ctrl+E` edit, `Ctrl+D` delete, `Ctrl+C` quit. Help is `?`
  (when the search box is empty) or `F1`.
- Search bar is now a dedicated always-focused input (bubbles/textinput) instead
  of the list's built-in `/` filter.
- Rows now always show the port (`user@hostname:port`).

### Added

- **Persistent config file** (TOML) at `os.UserConfigDir()/sshc/config.toml` with
  a `[ui]` section (theme/variant/density/detail/pin/verbose) and `[ui.env_colors]`
  overrides. Precedence is defaults → config file → CLI flags.
- **Live settings overlay** (`Ctrl+T`): cycle style/theme/density and the
  behaviour toggles with the arrow keys, preview live, and persist to the config
  file on close.
- **Per-host metadata** — free-form **tags**, an **env** label (rendered as a
  coloured chip), and **favourites** (`★`) — persisted as a structured
  `#sshc env=… tags=… fav=1 used=…` comment on the `Host` line, so the file stays
  valid for plain `ssh` and the metadata travels with the host. Env chips colour
  by preset (`prod`/`staging`/`dev`/`home`/`cloud`) or a stable hue derived from
  any other value; override or extend via config.
- Toggle a favourite with `Ctrl+F`; pinned hosts float to a `★ PINNED` group at
  the top when the search box is empty.
- **Last-used recency:** connecting stamps `used=` on the host (best-effort,
  primary config only) so the list can sort by most-recent-first and show
  relative "… ago" labels.
- The add/edit form gained **Tags** and **Env** fields; the detail panel now
  shows tags, env, last-used, access, and the resolved `ssh` command.
- New UI flags: `--theme`, `--variant`, `--density`, `--detail` (open the detail
  panel by default), `--no-pin` (do not float favourites), and `--verbose-rows`.
- Clone a host with `Ctrl+L`: opens the form pre-filled from the selected entry
  as a **new** host (named `<original>-copy`), leaving the original untouched
  until saved. Works on read-only hosts too (clone into your config).
- Arrow-key navigation between fields in the add/edit form (`↑`/`↓`, alongside
  `Tab`/`Shift+Tab`).
- `Ctrl+V` toggles verbose rows, appending the identity file and proxy inline
  (extensible for future fields).
- Build-time version stamping: the version is injected via `-ldflags` (derived
  from `git describe` by the Makefile), shown by `--version` and in the TUI
  title. Falls back to the embedded VCS revision for plain `go build` and to the
  module version for `go install ...@vX`.
- `Makefile` with `build` / `install` / `test` / `vet` / `fmt` / `version`
  targets.
- GitHub Actions release workflow (`.github/workflows/release.yaml`): on each `v*`
  tag, cross-compiles Linux/macOS/Windows (amd64 + arm64), stamps the version,
  and attaches `.tar.gz` / `.zip` archives plus a sha256 checksums file to the
  GitHub release.
- `LICENSE` (MIT).
- README "Troubleshooting" section covering the Windows Terminal `Ctrl+V`
  (paste) conflict with the verbose-rows toggle and its workarounds.

### Added (initial implementation)

- SSH config parsing (`ssh` package): loads multiple `--config` files in order,
  tags each host with its source file, and marks `Host *` / `Match` / wildcard
  blocks and non-primary configs as read-only.
- Safe config writer (`ssh` package): add / edit / delete host blocks with a
  pre-write `.sshc.bak` backup, re-parse verification, and byte-for-byte
  preservation of untouched blocks.
- Fuzzy search wrapper (`fuzzy` package) over alias, hostname, and user, plus
  alphabetical sort for `--sort-name`.
- Bubbletea TUI (`tui` package): searchable host list, add/edit form (huh),
  delete confirmation, toggleable detail panel, and help overlay.
- Cobra CLI (`cmd` package) with `--config`, `--filter`, `--template`,
  `--sort-name`, `--no-proxy`, and `-v/--version` flags.
- Connect-on-Enter via a customizable exec template.
- Unit tests for the parser, writer (round-trip, backup, untouched-block
  preservation), and fuzzy search.

### Known limitations

- Newly added and edited host blocks are written without indentation
  (cosmetic; valid and re-parseable). Untouched blocks are unaffected.
  *(Resolved in [Unreleased]: sshc now indents the blocks it writes.)*

## [0.0.0] — scaffold

### Added

- Initial project scaffold: module, package layout, dependency set, and CI-clean
  build.
