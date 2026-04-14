package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/umputun/revdiff/diff"
)

// xmlOpenTagRe matches an opening XML-style structural heading that lives alone
// on its own trimmed line, e.g. `<overview>` or `<example type="good">`.
// Group 1 is the tag name (attributes are ignored — see design notes in
// CUSTOMIZATIONS.md). Self-closing forms like `<br/>` are
// excluded by the `HasSuffix("/>")` pre-check in parseTOC, not by this regex.
// Comments (`<!--`), doctypes (`<!DOCTYPE>`), and PIs (`<?xml?>`) are excluded
// because the first-char class is `[a-zA-Z]`.
var xmlOpenTagRe = regexp.MustCompile(`^<([a-zA-Z][a-zA-Z0-9_-]*)(?:\s[^>]*)?>$`)

// xmlCloseTagRe matches a closing XML-style structural heading that lives alone
// on its own trimmed line, e.g. `</overview>`. Group 1 is the tag name.
var xmlCloseTagRe = regexp.MustCompile(`^</([a-zA-Z][a-zA-Z0-9_-]*)\s*>$`)

// tocEntry represents a single markdown header in the table of contents.
type tocEntry struct {
	title   string // header text without # prefix
	level   int    // header level 1-6
	lineIdx int    // index into diffLines
}

// mdTOC manages the markdown table-of-contents navigation pane.
type mdTOC struct {
	entries       []tocEntry
	cursor        int // currently highlighted entry index
	offset        int // first visible entry index for viewport scrolling
	activeSection int // index of entry matching current diff cursor position (-1 if none)
}

// parseTOC scans diff lines for markdown headers and builds a TOC.
// Headers inside fenced code blocks (``` / ~~~) and indented code blocks
// (4+ leading spaces or a leading tab, per CommonMark) are excluded.
// Fence tracking is CommonMark-compliant: closing fence must use the same
// character with at least the same length as the opening fence.
//
// Two heading syntaxes are recognized and may coexist in the same document:
//
//  1. Standard CommonMark ATX headers: `^#{1,6} Title` — level = prefix count.
//  2. Fork-specific XML-style structural headings: `<tag>` alone on its own
//     trimmed line. Level is determined by XML nesting depth via a tag stack.
//     On `</tag>` the stack is popped tolerantly (pops down to the first
//     matching open tag; a bogus close with no match in the stack is silently
//     ignored — mirrors HTML5's philosophy of always producing a usable doc).
//     Self-closing forms (`<br/>`), comments, PIs, and doctypes are ignored.
//     See CUSTOMIZATIONS.md for full rationale and the industry-standard
//     algorithms this was modeled on.
func parseTOC(lines []diff.DiffLine, filename string) *mdTOC {
	entries := make([]tocEntry, 0, len(lines))
	var fenceChar rune // 0 when outside code block, '`' or '~' when inside
	var fenceLen int   // length of the opening fence sequence
	var xmlStack []string

	for i, line := range lines {
		if line.ChangeType == diff.ChangeDivider {
			continue
		}

		content := line.Content

		// track fenced code block state per CommonMark spec.
		// opening fence: 3+ consecutive backticks or tildes (after optional indent).
		// closing fence: same char, at least same length, only whitespace after.
		trimmed := strings.TrimSpace(content)
		if fenceChar == 0 {
			if ch, n := fencePrefix(trimmed); n >= 3 {
				fenceChar = ch
				fenceLen = n
				continue
			}
		} else if ch, n := fencePrefix(trimmed); ch == fenceChar && n >= fenceLen {
			// closing fence must have no non-whitespace after the fence chars
			rest := strings.TrimSpace(trimmed[n:])
			if rest == "" {
				fenceChar = 0
				fenceLen = 0
				continue
			}
		}
		if fenceChar != 0 {
			continue
		}

		// CommonMark indented code block guard: a line with 4+ leading spaces
		// (or a leading tab) outside any fence is a code block — its content
		// must not count as a heading. This protects against false-positive
		// XML tags in pasted code examples that weren't placed inside fences.
		if isIndentedCodeLine(content) {
			continue
		}

		// XML close tag: pop stack tolerantly. If the closing tag name doesn't
		// appear in the stack at all, silently ignore — keeps mis-structured
		// docs producing a best-effort TOC rather than failing.
		if m := xmlCloseTagRe.FindStringSubmatch(trimmed); m != nil {
			tagName := m[1]
			for j := len(xmlStack) - 1; j >= 0; j-- {
				if xmlStack[j] == tagName {
					xmlStack = xmlStack[:j]
					break
				}
			}
			continue
		}

		// XML open tag (excluding self-closing forms): emit TOC entry at the
		// current nesting depth + 1, then push onto the stack. Level is capped
		// at 6 for display consistency with #-style headers.
		if !strings.HasSuffix(trimmed, "/>") {
			if m := xmlOpenTagRe.FindStringSubmatch(trimmed); m != nil {
				tagName := m[1]
				level := len(xmlStack) + 1
				if level > 6 {
					level = 6
				}
				entries = append(entries, tocEntry{title: tagName, level: level, lineIdx: i})
				xmlStack = append(xmlStack, tagName)
				continue
			}
		}

		// check for markdown header: ^#{1,6} (space required after last #)
		if !strings.HasPrefix(content, "#") {
			continue
		}

		level := 0
		for _, ch := range content {
			if ch != '#' {
				break
			}
			level++
		}
		if level < 1 || level > 6 {
			continue
		}
		if len(content) <= level || content[level] != ' ' {
			continue
		}

		title := strings.TrimSpace(content[level+1:])
		if title == "" {
			continue
		}

		entries = append(entries, tocEntry{title: title, level: level, lineIdx: i})
	}

	if len(entries) == 0 {
		return nil
	}

	// prepend a synthetic filename entry so user can jump back to the beginning
	name := filepath.Base(filename)
	entries = append([]tocEntry{{title: name, level: 1, lineIdx: 0}}, entries...)

	return &mdTOC{entries: entries, activeSection: -1}
}

// isIndentedCodeLine reports whether a line's leading whitespace makes it a
// CommonMark indented code block: 4+ leading spaces, or a leading tab. Lines
// that are purely whitespace return false so blank lines don't mask real
// content ahead.
func isIndentedCodeLine(s string) bool {
	spaces := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case ' ':
			spaces++
			if spaces >= 4 {
				return true
			}
		case '\t':
			return true
		default:
			return false
		}
	}
	return false
}

// moveUp moves cursor to the previous entry, clamped to first entry.
func (toc *mdTOC) moveUp() {
	if toc.cursor > 0 {
		toc.cursor--
	}
}

// moveDown moves cursor to the next entry, clamped to last entry.
func (toc *mdTOC) moveDown() {
	if toc.cursor < len(toc.entries)-1 {
		toc.cursor++
	}
}

// ensureVisible adjusts offset so the cursor is within the visible range of given height.
func (toc *mdTOC) ensureVisible(height int) {
	if height <= 0 {
		return
	}
	if toc.cursor < toc.offset {
		toc.offset = toc.cursor
	} else if toc.cursor >= toc.offset+height {
		toc.offset = toc.cursor - height + 1
	}
	if toc.offset < 0 {
		toc.offset = 0
	}
	if maxOff := max(len(toc.entries)-height, 0); toc.offset > maxOff {
		toc.offset = maxOff
	}
}

// updateActiveSection finds the nearest entry with lineIdx <= diffCursor and sets activeSection.
func (toc *mdTOC) updateActiveSection(diffCursor int) {
	toc.activeSection = -1
	for i, e := range toc.entries {
		if e.lineIdx > diffCursor {
			break
		}
		toc.activeSection = i
	}
}

// render produces the TOC display string with indentation by level.
// the highlighted entry uses FileSelected style in both modes:
// when TOC is focused it highlights the cursor, when diff is focused it highlights the active section.
func (toc *mdTOC) render(width, height int, focusedPane pane, s styles) string {
	if len(toc.entries) == 0 {
		return "  no headers"
	}

	// determine which entry to highlight — cursor when TOC focused, active section when diff focused
	highlighted := toc.cursor
	if focusedPane == paneDiff && toc.activeSection >= 0 {
		highlighted = toc.activeSection
	}

	// ensure the highlighted entry is visible in the viewport
	savedCursor := toc.cursor
	toc.cursor = highlighted
	toc.ensureVisible(height)
	toc.cursor = savedCursor
	end := min(toc.offset+height, len(toc.entries))

	var b strings.Builder
	for idx := toc.offset; idx < end; idx++ {
		e := toc.entries[idx]
		indent := strings.Repeat("  ", e.level-1)                // h1=0 indent, h2=2, h3=4, etc.
		title := toc.truncateTitle(e.title, width-len(indent)-4) // 2 prefix + 2 padding (matches FileSelected width-2)
		line := fmt.Sprintf("  %s%s", indent, title)

		if idx == highlighted {
			line = s.FileSelected.Width(max(width-2, 1)).Render(line)
		}

		b.WriteString(line)
		if idx < end-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// truncateTitle trims a title to fit maxWidth, appending ellipsis when truncated.
func (toc *mdTOC) truncateTitle(title string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(title)
	if len(runes) <= maxWidth {
		return title
	}
	if maxWidth <= 1 {
		return "…"
	}
	return string(runes[:maxWidth-1]) + "…"
}

// fencePrefix returns the fence character ('`' or '~') and count of leading consecutive
// occurrences. Returns (0, 0) if the string doesn't start with backticks or tildes.
func fencePrefix(s string) (rune, int) {
	if s == "" {
		return 0, 0
	}
	ch := rune(s[0])
	if ch != '`' && ch != '~' {
		return 0, 0
	}
	n := 0
	for _, r := range s {
		if r != ch {
			break
		}
		n++
	}
	return ch, n
}

// isFullContext returns true when all lines are ChangeContext (skips ChangeDivider).
func (m Model) isFullContext(lines []diff.DiffLine) bool {
	hasContext := false
	for _, line := range lines {
		if line.ChangeType == diff.ChangeDivider {
			continue
		}
		if line.ChangeType != diff.ChangeContext {
			return false
		}
		hasContext = true
	}
	return hasContext
}

// isTOCEligibleFile checks whether the filename extension signals a document
// format that could contain structural headings worth a TOC pane. Markdown
// (.md, .markdown) uses ATX `#`-style headings; XML-family documents (.xml,
// .xhtml) use tag nesting. Source code files are deliberately excluded —
// their `#` characters are comment prefixes, not headings.
// See CUSTOMIZATIONS.md for the rationale behind the extension gate.
func (m Model) isTOCEligibleFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown", ".xml", ".xhtml":
		return true
	default:
		return false
	}
}
