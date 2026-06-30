# Unit 04: log rendering palette — level tags + rich plain-line highlighting

## Goal

Make command output in the OUTPUT panel (and the expanded/fullscreen
popup) more readable through a three-tier coloring model:

1. **Native ANSI is respected.** Lines that already carry escape
   sequences (docker, kubectl, pytest, etc.) pass through untouched.
   _Already implemented_ via `hasANSI` — this unit must not regress it.
2. **Recognized log levels get a colored tag.** A level word at the
   start of the line (`trace/debug/info/warn/warning/error/err/fatal`)
   is rendered as a compact **4-letter** uppercase tag in level color:
   `TRAC DEBU INFO WARN ERRO FATA`. Text-only color (foreground + bold),
   **no background chip**, consistent with cast's transparent aesthetic.
3. **Plain lines get a defined palette.** Lines with neither ANSI nor a
   level word are tokenized and colored (numbers, quoted strings, paths,
   URLs, `key=value`, IPs, timestamps), instead of being dimmed wholesale.

This is a refinement of the existing `internal/tui/views/logfmt.go` +
`ColorOutputLine` in `common.go`, not a new subsystem. Both the inline
OUTPUT panel (`views.Output` → `renderTermRows`) and the expanded popup
(`views.ExpandedOutput`) already funnel through `colorizeLogLine`, so a
single change covers every surface.

## Design

No new UI tokens, views, config, or keybindings. All colors come from
the existing `views.Palette` (`Cyan/Green/Yellow/Orange/Red/Fg/FgDim/
FgMuted/Accent`), so every theme + env combination stays in tune.

### Tier 2 — level tags (4-char, text-only)

`padLevel` is replaced so it normalizes the level word to a fixed
**4-char** abbreviation and uppercases it. Because all six abbreviations
are exactly 4 chars, the message text aligns without extra padding:

| input (any case)         | tag    |
| ------------------------ | ------ |
| `trace`                  | `TRAC` |
| `debug`                  | `DEBU` |
| `info`                   | `INFO` |
| `warn`, `warning`        | `WARN` |
| `error`, `err`           | `ERRO` |
| `fatal`                  | `FATA` |

Render is `levelStyle(p, tag).Bold(true).Render(tag)` followed by a
single space, then the rest of the line. The leading prefix (timestamp
etc.) stays `FgDim`; the message body stays `Fg`. `levelStyle` is
unchanged — it already returns a foreground-only style with theme
fallbacks (no background), which is exactly "solo texto en color".

### Tier 3 — rich plain-line palette

`ColorOutputLine` keeps its special whole-line prefixes (`✓ ✗ $
--- PASS --- FAIL`) and otherwise delegates to a new `richColorLine`
tokenizer. Because `colorizeLogLine` already returns early when
`hasANSI` is true, `richColorLine` is guaranteed an ANSI-free input, so
a single-pass regex tokenizer is safe (no escape-code re-matching).

A single combined `regexp` with named alternatives is walked with
`FindAllStringSubmatchIndex`; each match is colored by which named group
fired, and the gaps between matches render in the base `Fg` color
(brighter than today's `FgDim`). Token → color:

| token       | pattern (sketch)                                   | color     |
| ----------- | -------------------------------------------------- | --------- |
| url         | `https?://…`                                       | `Cyan`    |
| timestamp   | ISO 8601 / `HH:MM:SS`                              | `FgMuted` |
| ip          | `d.d.d.d[:port]`                                    | `Orange`  |
| key=value   | `key=…`                                             | key `Cyan` `=` `FgDim`, value by type |
| string      | `"…"` / `'…'`                                       | `Green`   |
| path        | `./x`, `../x`, `/abs/x`                             | `Cyan`    |
| bool/null   | `true/false/null/nil/none/yes/no`                  | `Orange`  |
| number      | `\d+(\.\d+)?`                                       | `Yellow`  |

Ordering in the alternation matters (url/timestamp/ip/kv before bare
number) so the most specific token wins. Patterns are conservative and
word-boundary anchored to avoid over-coloring prose.

## Implementation

All edits in `internal/tui/views/`:

- **`logfmt.go`**
  - Replace `padLevel` with a 4-char abbreviation map (table above);
    drop the surrounding-space padding — emit just the 4-char tag.
  - In `colorizeLogLine`, render `tag + " "` (single space separator)
    instead of the old ` TAG ` block.
- **`common.go`**
  - Keep `ColorOutputLine`'s special prefixes; route the `default`
    branch to `richColorLine`.
  - Add `richColorLine(p, line)` + the package-level compiled
    `richTokenRe` and a `colorForGroup` helper. `key=value` is split on
    the first `=` so key and value color independently.

### Testing

Add `logfmt_test.go` (table-driven) asserting:

- ANSI passthrough unchanged (input with `\x1b[` returned verbatim).
- Each level word maps to its 4-char tag and the tag carries an escape
  sequence (colored).
- `richColorLine` colors a representative line (number, quoted string,
  path, `key=value`) — assert via `ansi.Strip` round-trip that the
  visible text is preserved and that escapes were injected.
- Prose without tokens is returned with base color, not mangled.

## Out of scope

- Configurable palettes / user-defined level words.
- Multi-line / stack-trace aware grouping.
- Coloring inside lines that already have ANSI (respected as-is).
