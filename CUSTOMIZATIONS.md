<customizations>

<overview>

This fork of `umputun/revdiff` carries **feature-level customizations**
that are not specific to any platform — they reflect how this fork's
maintainer uses revdiff day-to-day and do not belong in either the
upstream codebase (author has declined) or in WINDOWS.md (which is
reserved for OS-specific divergences).

See WINDOWS.md for Windows/WezTerm-specific divergences.
See README.md for user-facing documentation.

**Guiding principles** (mirror WINDOWS.md):
- Keep divergence from upstream small and localized.
- Prefer **new files** (additive divergence) over edits to
  upstream-tracked files (merge-conflict surface).
- When we must edit an upstream-tracked file, isolate the fork-only
  behavior so the merge diff stays tiny and reviewable.
- Document every divergence here.

</overview>

<fork-features>

<overview>

Feature additions that exist only in this fork. Each entry names
the files it touches and whether those files are upstream-tracked
(merge-conflict-prone) or fork-only.

</overview>

<xml-style-toc-headings>

<summary>

The markdown TOC parser in `ui/mdtoc.go` recognizes **two** heading
syntaxes, where upstream recognizes one:

1. **Standard CommonMark ATX headers**: `#` through `######` followed
   by a space. Upstream behavior, unchanged.
2. **XML-style structural headings**: an opening tag alone on its own
   trimmed line, e.g. `<overview>` or `<example type="good">`.
   Fork-specific addition.

This exists because the maintainer writes long-form documentation
using XML-like structural tags (`<section-name>…</section-name>`)
instead of `#` headings, as a convention that makes section
boundaries unambiguous for LLM context windows. Standard markdown
tooling (mdbook, Pandoc, GitHub rendering, upstream revdiff) ignores
these as plain text, which meant the TOC pane stayed empty for the
exact files the maintainer most wanted to navigate.

</summary>

<algorithm>

The parser uses a **line-based scanner with an explicit tag stack**,
the same approach real-world markdown-to-HTML converters use for
HTML-block detection (CommonMark spec "HTML block" rule 6). Full XML
parsers (SAX, DOM, Go's `encoding/xml.Decoder`) were rejected because
they require well-formed XML input and our content is mixed markdown
plus XML-style headings — bare `&` characters, unbalanced stretches,
markdown fences, and `#` prose all routinely appear and would trip
up a real XML parser long before reaching the structural headings.

**Scan loop** (in `parseTOC`):

1. Skip diff-divider lines (unchanged upstream behavior).
2. Apply CommonMark fenced-code-block tracking (`` ``` `` / `~~~`).
   Lines inside a fence are skipped entirely. Fence-state transitions
   use the existing `fencePrefix` helper — unchanged upstream code.
3. Skip CommonMark indented code blocks: any line with 4+ leading
   spaces or a leading tab (see `isIndentedCodeLine`). This is new —
   upstream's parser relied only on fence tracking, so pasted code
   examples using indent-style blocks could falsely trigger headings.
4. Try to match `xmlCloseTagRe` (closing tag alone on its line).
   If matched, pop the tag stack **tolerantly**: scan from top to
   bottom for the first matching open tag and pop everything above
   it. If no match in the stack, silently ignore the bogus close.
5. Try to match `xmlOpenTagRe` (opening tag alone on its line), with
   self-closing forms rejected via `HasSuffix(trimmed, "/>")`.
   If matched, emit a TOC entry at `level = stackDepth + 1` (capped
   at 6 for display parity with ATX headers), then push the tag name
   onto the stack.
6. Fall through to the existing `#` header detection.

**Why tolerant close?** Real XML parsers fail hard on mismatched
closes (Go's `encoding/xml.Decoder` with `Strict=true`). HTML5's
"adoption agency algorithm" takes the opposite stance — always
recover, always produce a usable tree. For our use case (navigation,
not validation), HTML5's philosophy wins: if a doc is already
mis-structured, its TOC is inherently approximate, and we gain
nothing by blanking the pane. Tolerant recovery means the TOC
degrades gracefully on malformed input.

**Why not the full adoption agency algorithm?** That algorithm is
hundreds of lines and handles active-formatting-element stacks,
implicit re-opens, and other tag-soup pathologies that don't occur
in structural-heading usage. Pop-until-match is the simple subset
that handles all realistic cases.

</algorithm>

<semantics>

<opening-tags>

Matched by `xmlOpenTagRe = ^<([a-zA-Z][a-zA-Z0-9_-]*)(?:\s[^>]*)?>$`
against the trimmed line content. Group 1 is the tag name and is
used verbatim as the TOC entry title (no case change, no attribute
inclusion, no prettification — matches what's on disk).

- First char is `[a-zA-Z]`, which excludes:
  - HTML/XML comments (`<!-- … -->` starts with `!`)
  - Processing instructions (`<?xml …?>` starts with `?`)
  - Doctypes (`<!DOCTYPE …>` starts with `!`)
  - CDATA sections (`<![CDATA[…]]>` starts with `!`)
- Hyphens and underscores are allowed in tag names after the first
  char, matching XML Name rules loosely (`<my-section>`, `<sub_part>`).
- Attributes with whitespace-separated `k="v"` pairs are matched by
  `(?:\s[^>]*)?` and silently dropped from the title.
- Tag must live alone on its own trimmed line. Inline forms like
  `use <foo> in your config` do not match.

</opening-tags>

<closing-tags>

Matched by `xmlCloseTagRe = ^</([a-zA-Z][a-zA-Z0-9_-]*)\s*>$`
against the trimmed line content. Group 1 is the tag name. Only
whitespace is permitted between the tag name and the `>`.

</closing-tags>

<self-closing-tags>

Rejected by a `HasSuffix(trimmed, "/>")` pre-check, not by the
regex. `<br/>`, `<img src="x.png"/>`, and similar structurally-empty
elements are not headings; skipping them keeps the TOC clean.

</self-closing-tags>

<nesting>

Level is `len(stack) + 1` at the moment the opening tag is matched.
This is computed BEFORE the push. So:

```
<outer>          ← stack before: []     → level 1, push
  <inner>        ← stack before: [outer] → level 2, push
    <deepest>    ← stack before: [outer, inner] → level 3, push
    </deepest>   ← pop deepest
  </inner>       ← pop inner
</outer>         ← pop outer
```

Levels above 6 are clamped to 6 (display-only cap). Deeper nesting
still tracks correctly in the stack — the cap is purely about how
the entry indents in the TOC render.

</nesting>

<mixed-hash-and-xml>

ATX `#` headers and XML-style headings may appear in the same
document. They are independent: `#` headers use absolute prefix-count
level (`## Foo` is always level 2), XML headings use stack-depth
level. So `## Foo` inside an `<outer>` block renders at level 2
regardless of the fact that the XML stack has depth 1.

This is by design — ATX headers have intrinsic absolute semantics
(from CommonMark) that shouldn't shift based on surrounding XML.

</mixed-hash-and-xml>

<code-blocks>

Both CommonMark code block styles exclude XML (and `#`) content
from TOC consideration:

- **Fenced**: ``` and ~~~ fences, with the existing length-matching
  close rule. XML tags inside a fence are not emitted, and fake
  close tags inside a fence do not pop the outer stack.
- **Indented**: 4+ leading spaces or a leading tab. Lines with 1–3
  leading spaces still count as headings (matches CommonMark's rule
  that only 4+ spaces promotes to a code block).

</code-blocks>

<extension-gate>

The TOC pane activates only when the file's extension is in an
allowlist: `.md`, `.markdown`, `.xml`, `.xhtml`. Upstream allowed
only `.md` and `.markdown`. The XML extensions were added so pure
XML documents (schemas, configs, scratch files) can also get a
navigation pane.

Deliberately **excluded** from the gate:

- `.html` — real HTML uses `<h1>..<h6>` as semantic headings, which
  this parser doesn't handle specially. A generic HTML TOC would
  need different logic and is out of scope.
- `.txt` — no reliable signal that a plain-text file uses
  structural XML-style headings. False positives on source-code-like
  `.txt` files would noise up the TOC.
- Source code extensions (`.go`, `.py`, `.sh`, etc.) — these use
  `#` as comment prefixes in many languages, and a `# TODO` line
  would register as an H1 heading. The extension gate is the
  primary protection against that.

The gate check lives in `isTOCEligibleFile` in `ui/mdtoc.go` (was
`isMarkdownFile` upstream; renamed during this feature to reflect
the broader scope).

</extension-gate>

</semantics>

<files-touched>

Upstream-tracked (merge-conflict surface):

- `ui/mdtoc.go` — main logic. Added `xmlOpenTagRe`, `xmlCloseTagRe`,
  `isIndentedCodeLine`, rewrote `parseTOC` to handle XML headings
  with the stack, renamed `isMarkdownFile` → `isTOCEligibleFile`.
- `ui/mdtoc_test.go` — added `TestParseTOC_XmlHeadings` (22 sub-
  tests), `TestIsIndentedCodeLine` (13 sub-tests), updated existing
  `TestModel_IsMarkdownFile` → `TestModel_IsTOCEligibleFile` with
  added `.xml`/`.xhtml`/`.html`/`.txt` cases.
- `ui/model.go` — one-line change at the single caller: renamed
  `isMarkdownFile` → `isTOCEligibleFile`.
- `CLAUDE.md` — updated the Markdown TOC line to reflect the new
  name and dual-syntax support.

Fork-only (additive, no conflict risk):

- `CUSTOMIZATIONS.md` — this document.

</files-touched>

<merge-strategy>

When pulling upstream changes that touch `ui/mdtoc.go`, `ui/model.go`,
`ui/mdtoc_test.go`, or `CLAUDE.md`:

1. Prefer `git merge` over rebase (see WINDOWS.md's merge strategy).
2. On conflict, keep the fork's XML-heading logic intact and replay
   upstream's changes around it. The fork's regexes, stack tracking,
   indent guard, and renamed `isTOCEligibleFile` must survive.
3. If upstream renames `isMarkdownFile` to something else, rename
   our `isTOCEligibleFile` to match the new upstream name and widen
   its extension list — do not end up with two sibling functions.
4. Run `go test ./ui/...` after resolving — the 22 XML sub-tests
   and 13 indented-code sub-tests cover enough ground to flag a
   broken merge.

</merge-strategy>

</xml-style-toc-headings>

</fork-features>

<known-quirks>

<overview>

Behavioral surprises that aren't bugs in the customization itself but
are worth documenting so future maintainers don't rediscover them.

</overview>

<view-mode-close-hang>

<summary>

When closing a `--view` session in WezTerm on Windows by pressing `q`,
the split pane does not auto-close — it shows a blank line and waits
for any keypress before dismissing. Plan-review sessions (same
launcher, same `.cmd` shape, same WezTerm invocation) auto-close
cleanly. The only behavioral difference between the two flows is that
`--view` uses revdiff's `--stdin` mode while plan-review uses
`--only`.

</summary>

<investigation>

Ruled out during a debug session:

- **Pipe vs redirection**: `type "<file>" | revdiff --stdin` and
  `revdiff --stdin < "<file>"` both hang identically — the pipe
  subshell is not the cause.
- **Script exit code**: adding an explicit `exit 0` to the `.cmd`
  after the sentinel touch did not change the behavior.
- **Launcher shape**: the `.cmd` generation, WezTerm split-pane
  invocation, sentinel polling, and cleanup are byte-identical to
  the plan-review launcher, which works fine.

The remaining common factor is `--stdin` itself. revdiff's stdin mode
opens `CONIN$` with `O_RDWR` (see `cmd/revdiff/tty_windows.go`) so
Bubble Tea can enter raw mode, and Bubble Tea is wired via
`tea.WithInput(tty)` in `cmd/revdiff/main.go:402-410`. After revdiff
exits, something about the console-mode restoration leaves the pane
in a state that WezTerm interprets as "hold for keypress" — even
though the .cmd script's subsequent `break > sentinel` + `exit 0`
run normally (the sentinel IS written, so the launcher poll loop
returns cleanly and captures annotations).

A proper fix needs a deeper Go-level investigation of Bubble Tea's
Windows raw-mode teardown when stdin was file-redirected but input
was read from `CONIN$`. Out of scope for this customization.

</investigation>

<workaround>

Press any key after `q` to dismiss the pane. Annotations are already
captured at the moment `q` is pressed, so no data is lost.

</workaround>

</view-mode-close-hang>

</known-quirks>

</customizations>
