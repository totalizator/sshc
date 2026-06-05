# How sshc writes `ssh_config`

This note documents how sshc serializes changes back to an SSH config, the
guarantees it makes, the third-party library constraints that shape the design,
and the known limitations / follow-up work. It exists because this area is
safety-critical (a bad write can break every `ssh` the user runs) and is likely
to need further development.

All writes go through `ssh/write.go`. The public entry points are `Add`,
`Update`, `Delete`, `TouchLastUsed`, `SetFav`, and `StripMeta`.

## The write pipeline (`mutate`)

Every mutation runs through `mutate`, a safe read-modify-write cycle:

1. **Re-read** the file from disk each time (never a stale in-memory snapshot),
   so a concurrent external edit is picked up rather than clobbered.
2. **Parse** it with `kevinburke/ssh_config`.
3. **Apply** the caller's closure, which returns the set of blocks it *owns*
   (the ones sshc built or rewrote — see below). A closure may return
   `errNoChange` to abort cleanly with no write and no backup.
4. **Prune** empty directives (`pruneEmptyDirectives`) — a bare `User` with no
   argument is re-emitted by the serializer as `User ` and rejected by OpenSSH
   (`no argument after keyword "user"`); pruning heals that on the next save.
5. **Render** (`render`) and **strip control bytes** (`stripControlBytes`) — a
   stray NUL (e.g. from a `Ctrl+Space` keystroke) must never reach disk.
6. **Re-parse guard**: the rendered bytes are parsed again; if that fails, the
   write is refused. This is the backstop against producing a corrupt file.
7. **Back up** the original to `<path>.sshc.bak`.
8. **Write** the new content (`0o600`, creating the dir if needed).

The re-parse guard + backup are the two properties that make everything else
safe: a logic bug downstream can at worst refuse to write, and the previous
content is always recoverable from `.sshc.bak`.

## The ownership model (`render` / `canonicalBlock`)

The library renders a `Config` by concatenating each block's `String()`. sshc's
`render` reproduces that loop with one difference: blocks sshc **owns** (built or
rewrote this cycle) are emitted by `canonicalBlock` in conventional OpenSSH style
— directives indented four spaces (`indentUnit`) under the `Host` line, blocks
separated by a blank line — while every **other** block is emitted by the
library's faithful `block.String()`, so untouched blocks are preserved
byte-for-byte.

- `Add` / clone owns the new block (it has no prior formatting to preserve).
- `Update` owns the edited block.
- `Delete`, `TouchLastUsed`, `SetFav`, `StripMeta` own nothing — they return an
  empty owned set, so `render` is byte-identical to the library's plain
  `Config.String()` (only the targeted EOL comment / a removed block differs).

`render` also inserts a blank-line separator at an owned block's boundaries. This
is **additive only** (it never removes blank lines), and it skips the boundary
after the implicit global block at index 0 so a header comment directly above the
first host stays attached to it.

`canonicalBlock` emits the `Host` line itself, then each child node via the
node's own `String()` with its leading whitespace normalized to `indentUnit`.
Going through the node's `String()` (rather than re-printing `Key + Value`) is
what preserves quoting, `=` syntax, and trailing comments on directives sshc does
not model.

## Editing in place (`Update` / `applyFields` / `setDirective`)

`Update` does **not** rebuild the block from the modeled fields. It mutates the
existing block in place:

- The alias pattern is replaced only if it changed.
- The sshc metadata comment (`#sshc …`) is rewritten from the host's metadata.
- `applyFields` updates the six modeled directives
  (`HostName`, `User`, `Port`, `IdentityFile`, `ProxyJump`, `ProxyCommand`) via
  `setDirective`; every other node — unmodeled directives (`ForwardAgent`,
  `LocalForward`, `SetEnv`, …), standalone comments, and blank lines — is left
  exactly where it was.

`setDirective` treats the modeled keys as single-valued (sshc owns them): it
replaces the first occurrence in place (keeping the original key casing and any
trailing comment), removes any duplicate occurrences, and inserts a brand-new
directive just after the last existing directive (so it stays grouped above
trailing comments/blank lines). An empty value removes the directive.

## Why a custom renderer at all — library constraints

Two `kevinburke/ssh_config` (v1.6.0) internals force this design:

- **`KV.leadingSpace` is unexported** and there is no setter, so a freshly
  constructed `KV` always renders flush-left. We cannot ask the library to indent
  a block we build; hence `canonicalBlock` post-processes indentation itself.
- **`KV.rawValue` is set for every parsed directive** (parser.go) and
  `KV.String()` always prefers it over `Value`. Consequently:
  - mutating an existing `KV.Value` in place has **no effect** on output — to
    change a value we must replace the node with a fresh `KV` (which has an empty
    `rawValue`);
  - re-printing `Key + " " + Value` for a *preserved* directive would drop quotes
    (the parser stores `Value` unquoted), so `canonicalBlock` must print via the
    node's own `String()` to stay lossless.

## Guarantees

- Untouched blocks are preserved byte-for-byte (modulo the tab caveat below).
- Editing a host never drops unmodeled directives, comments, or quoted values.
- A write is never committed unless it re-parses cleanly; the prior file is
  always backed up to `.sshc.bak`.
- The file sshc writes is always free of control bytes other than tab/newline/CR.

## Known limitations & follow-up work

- **Leading tabs are normalized to spaces** in *untouched* blocks. The library
  stores indentation as an integer space count, so a leading tab round-trips as a
  single space on any write — independent of sshc. Faithfully preserving tabs
  would require replacing the library's serializer.
- **CRLF line endings can become mixed.** `canonicalBlock` / `buildBlock` emit
  `\n`; a rewritten block in an otherwise CRLF file would gain `\n` line endings
  while untouched blocks keep `\r\n`. Not currently normalized.
- **Multiple values for a modeled key collapse to one.** A host with two
  `IdentityFile` lines is modeled as a single field; editing it keeps one. This
  matches the form's single-value model but loses an untouched second value.
- **A trailing comment on a *changed* modeled field is dropped.** `setDirective`
  preserves the trailing comment when a field's value is replaced, but a future
  refactor should confirm this holds for the rename/clear paths too.
- **New fields are appended after the last directive.** Insertion ordering is
  positional, not semantic; there is no attempt to order directives canonically.
- **Possible future option: a `--reformat` / whole-file normalize.** Deliberately
  *not* implemented — it would touch untouched blocks (Match, multiline, quoted
  values) and break the byte-for-byte guarantee. If added, it should be explicit
  and opt-in, never the default.

## Reverting sshc's changes (`StripMeta` / `sshc clean`)

`sshc clean` (→ `StripMeta`) removes the `#sshc …` metadata comments, returning
the file to plain-ssh form. It strips a Host-line comment **only** when it parses
as real sshc metadata (≥1 recognized `key=value`, exactly what `formatMeta`
writes), so a user comment that merely begins with the word `sshc` is left alone.
`clean --dry-run` previews the count using the identical predicate, so preview and
apply always agree. It does **not** undo indentation — sshc cannot tell its own
indentation from the user's, and de-indenting would re-touch every block.
