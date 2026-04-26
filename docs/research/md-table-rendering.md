<title>Markdown table rendering — research</title>

<problem>
GFM-style markdown tables in `.md` files currently render as raw source:

```
| Field | Type | Default | Notes |
|-------|------|---------|-------|
| `--theme` | string | `""` | overrides individual `--color-*` flags |
| `--no-colors` | bool | `false` | strips all ANSI; mutually-exclusive with `--theme` |
```

The `|`, the dashes, and the un-padded cells make these blocks hard to scan in
the TUI. A reader has to mentally re-align columns. This affects both diff view
(when an `.md` file is changed) and full-file view (`--view`, `--all-files`,
single-file md flow with the mdTOC pane).
</problem>

<architectural-constraints>

Whatever solution we pick must respect the existing rendering pipeline; ignoring
this is the fastest way to break annotations, blame, line numbers, and search.

<one-source-line-equals-one-rendered-line>
The whole pipeline assumes a 1:1 mapping between `diff.DiffLine` and a rendered
visual line (wrap mode is the only existing exception, and even that adds
*continuation* lines, never replaces the source mapping).

Things keyed off this mapping:

- `m.diffLines[i]` ↔ `m.highlightedLines[i]` parallel array — see
  `highlight.HighlightLines()` and `prepareLineContent()` in `ui/diffview.go:287`.
- Line-number gutter (`L`) and blame gutter (`B`) — `lineGutters(dl)` reads
  `dl.OldNum` / `dl.NewNum` per line.
- Annotations attach to `(filepath, lineNum, changeType)` —
  `buildAnnotationMap()` in `ui/diffview.go:168`. Collapsing rows into a
  multi-line table block would orphan their annotations.
- Cursor position (`m.diffCursor`) is an index into `diffLines`. `j`/`k`
  navigation, hunk navigation, search hits, and TOC sync all use it.
- mdTOC line jumps (`pendingAnnotJump`, `centerViewportOnCursor`) use line
  indices.
</one-source-line-equals-one-rendered-line>

<chroma-already-tokenises-markdown>
`highlight.New()` runs chroma's markdown lexer on `.md` files, so the source
already comes back with foreground ANSI applied. Any table reformatter has to
operate on already-highlighted strings — which means measuring widths with
`lipgloss.Width` / `ansi.StringWidth`, never `len()`. Failing this would push
columns out of alignment by exactly the byte-count of the ANSI escapes.
</chroma-already-tokenises-markdown>

<full-context-vs-diff-mode>
`isFullContext()` in `ui/mdtoc.go:311` is true when every visible line is
`ChangeContext` (the `--view` and full-file paths). In that mode, table
formatting is unambiguous. In diff mode, individual rows can be `+` / `-` /
context, and a single row's content might span an add+remove pair. Table
formatting in diff mode is doable but the corner cases multiply.
</full-context-vs-diff-mode>

<no-existing-block-rendering>
There is currently no concept of a "block" that spans multiple `DiffLine`s.
Adding one would touch viewport scrolling math (`cursorViewportY`,
`wrappedLineCount`), search-match positioning, annotation injection, and the
collapsed/expanded hunk model. That is a large surface area for a feature
labelled "make tables prettier".
</no-existing-block-rendering>

</architectural-constraints>

<approach-options>

<option-a-column-aligned-rewrite>
**Reformat each table line in place; keep 1:1 line mapping.**

Pre-pass over `diffLines` finds contiguous table blocks (a header row, then a
separator row matching `^\s*\|?(\s*:?-{3,}:?\s*\|)+\s*$`, then ≥1 data rows;
break on any non-`|` line). For each block, compute the max visible width of
each column across all its rows. Then, at render time, replace each row's
content with padded cells joined by `│` (U+2502); the separator row becomes
`├───┼───┤` using `─` and `┼`.

Output is stored in a parallel `tableFormatted []string` slice, mirroring
`highlightedLines`. `prepareLineContent()` picks `tableFormatted[i]` over
`highlightedLines[i]` when set.

Pros:
- Zero changes to viewport, cursor, annotations, blame, line numbers, search.
- Works in diff mode trivially: an added row aligns with the rest of the table
  and still gets `LineAdd` background via `styleDiffContent`.
- Horizontal scroll (`scrollX`) already handles tables wider than the viewport
  for free.
- Can be a toggle (e.g. `T`) for parity with `v` / `w` / `L` / `B`.

Cons:
- No top/bottom border (would require inserting "phantom" rows that don't map
  to source — breaks the 1:1 invariant). Acceptable: the separator row already
  gives a strong visual divider.
- Inline cell markdown (`**bold**`, backtick code, links) stays as raw
  characters inside cells unless we add a small inline-MD→ANSI step *per cell*.
  Recommend: ship without it first, add later if needed.
- Need to respect chroma highlighting *inside* cells. Easiest: pre-strip ANSI
  from a cell to measure width, then re-apply chroma's already-emitted
  sequences at the cell boundaries. The chroma output uses `\033[39m` resets
  (no full reset), so it composes cleanly with cell padding.
- Wrap mode (`w`) interaction: simplest is to skip table formatting when wrap
  mode is on. Cells rarely wrap usefully anyway.
- Detection edge cases: pipes inside fenced code blocks, escaped `\|`, GFM
  alignment markers (`:---`, `---:`, `:---:`). Reuse the fence-tracking logic
  already in `parseTOC` (`ui/mdtoc.go:57`) so we don't duplicate it.

Effort estimate: medium. ~1 new file (`ui/mdtable.go`) plus 4–6 small hooks in
`diffview.go` / `model.go`. Mock-and-test footprint is low because the
formatter is a pure `[]DiffLine → []string` function.
</option-a-column-aligned-rewrite>

<option-b-block-replacement-with-lipgloss-table>
**Render the whole table block as one multi-line `lipgloss/table` object.**

Detect block, run it through `github.com/charmbracelet/lipgloss/table`, splice
the output back into the rendered stream, hide the original rows.

Pros:
- Prettier output: real Unicode borders top and bottom, native alignment,
  optional row dividers.

Cons:
- `lipgloss/table` is **not currently vendored** — would add a new dep
  (and `lipgloss` itself is at v1.1.0; the `table` subpackage is in v0.x land
  with a separate import path).
- Breaks the 1:1 invariant. To keep cursor / line numbers / annotations
  working, every rendered row of the lipgloss table would have to know which
  source `DiffLine` index it came from. The library doesn't expose that.
- In diff mode, mixing `+` / `-` rows inside a single lipgloss table is awkward
  — the table renderer wants uniform styling per row, but our diff styling
  comes from `extendLineBg` and is computed at the line level.
- Loses horizontal scroll support: `scrollX` works on per-line ANSI strings
  via `ansi.Cut`; a multi-line lipgloss block has to be re-rendered or
  post-processed to scroll.

Recommend against. The cosmetic gain is small relative to the integration
cost.
</option-b-block-replacement-with-lipgloss-table>

<option-c-glamour-rendered-view-mode>
**Add a separate "rendered markdown" mode (toggle key, e.g. `R`) that pipes
the file through `charmbracelet/glamour` and shows the result.**

Pros:
- Tables look great. So do code fences, lists, blockquotes, headings.

Cons:
- Glamour wraps prose, re-flows paragraphs, and emits its own line numbering
  — completely incompatible with the diff-line architecture. It is a separate
  viewer, not a table fix.
- Adds a heavy dependency (glamour pulls in goldmark, chroma's HTML formatter,
  termenv). Expands the binary noticeably.
- Loses every diff-mode capability: annotations, line numbers, blame, search
  by line, collapsed/expanded hunks, all gone in this mode.
- Doesn't address the diff-mode case at all (you can't run glamour on a diff).

Recommend against unless we explicitly want a "preview" feature, which is a
different feature request.
</option-c-glamour-rendered-view-mode>

</approach-options>

<recommendation>

Go with **Option A** (column-aligned in-place rewrite), gated behind a toggle
key (proposed: `T` for "tables"). Default it to **on** when:

- the file extension is `.md` / `.markdown`, AND
- the line is detected as part of a table block.

Off by default for everything else. Users who want raw view can toggle it.

Implementation order if you decide to proceed:

1. Pure formatter in `ui/mdtable.go` — input `[]DiffLine` + filename, output
   `map[int]string` (line index → reformatted content, only entries for table
   rows). No UI deps. Heavy test coverage here is cheap and worth it because
   the parsing edge cases are where bugs hide.
2. Wire into `prepareLineContent()` so a table-formatted line wins over
   `highlightedLines[i]`.
3. Recompute on file load alongside `highlightedLines`. Cache invalidated the
   same way.
4. Toggle key + status icon (similar to `≋` for wrap, `#` for line numbers).
5. Update CLAUDE.md "Data Flow" section, README.md, `site/docs.html`, and the
   plugin reference docs.

Things to **not** do in v1, even if tempted:

- Don't render inline markdown inside cells (bold/italic/code). Ship plain
  text first. It's the single biggest scope creep risk here.
- Don't try to handle multi-line cells (CommonMark doesn't allow them; GFM
  technically allows `<br>` inside a cell, just leave it raw).
- Don't add top/bottom borders via phantom rows. The mid-rule is enough and
  the architecture pushback isn't worth it.

</recommendation>

<questions-for-you>

1. **Scope:** is this for full-context viewing only (`--view`, `--all-files`,
   single-file md), or also for diff mode? Diff-mode support roughly doubles
   the test surface but uses the same code path.
2. **Toggle vs always-on:** are you OK with a `T` toggle, defaulting to on for
   `.md` files? Or always-on with no escape hatch?
3. **Inline cell markdown:** ship v1 without bold/italic/code rendering inside
   cells — agreed?

</questions-for-you>
