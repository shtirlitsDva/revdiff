package sidepane

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/shtirlitsDva/revdiff/app/diff"
	"github.com/shtirlitsDva/revdiff/app/ui/style"
)

// xmlOpenTagRe matches an opening XML-style structural heading that lives
// alone on its own trimmed line, e.g. `<overview>` or `<example type="good">`.
// Group 1 is the tag name (attributes are ignored). Self-closing forms like
// `<br/>` are excluded by the `HasSuffix("/>")` pre-check in ParseTOC, not by
// this regex. Comments (`<!--`), doctypes, and PIs are excluded because the
// first-char class is `[a-zA-Z]`.
var xmlOpenTagRe = regexp.MustCompile(`^<([a-zA-Z][a-zA-Z0-9_-]*)(?:\s[^>]*)?>$`)

// xmlCloseTagRe matches a closing XML-style structural heading that lives
// alone on its own trimmed line, e.g. `</overview>`. Group 1 is the tag name.
var xmlCloseTagRe = regexp.MustCompile(`^</([a-zA-Z][a-zA-Z0-9_-]*)\s*>$`)

// TOC manages the markdown table-of-contents navigation pane.
type TOC struct {
	entries       []tocEntry
	cursor        int // currently highlighted entry index
	offset        int // first visible entry index for viewport scrolling
	activeSection int // index of entry matching current diff cursor position, -1 means no active section
}

// tocEntry represents a single markdown header in the table of contents.
type tocEntry struct {
	title   string // header text without # prefix
	level   int    // header level 1-6
	lineIdx int    // index into diffLines
}

// ParseTOC scans diff lines for markdown headers and builds a TOC.
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
//     ignored). Self-closing forms (`<br/>`), comments, PIs, and doctypes are
//     ignored. See CUSTOMIZATIONS.md for rationale.
//
// Returns nil when no headers are found.
func ParseTOC(lines []diff.DiffLine, filename string) *TOC {
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
		ch, n := fencePrefix(trimmed)
		switch {
		case fenceChar == 0 && n >= 3:
			fenceChar = ch
			fenceLen = n
			continue
		case fenceChar != 0 && ch == fenceChar && n >= fenceLen:
			// closing fence must have no non-whitespace after the fence chars
			if rest := strings.TrimSpace(trimmed[n:]); rest == "" {
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

	return &TOC{entries: entries, activeSection: -1}
}

// NumEntries returns the number of TOC entries.
func (t *TOC) NumEntries() int {
	return len(t.entries)
}

// CurrentLineIdx returns the diff line index of the entry at the current cursor position.
// returns ok=false when entries are empty or cursor is out of range.
func (t *TOC) CurrentLineIdx() (int, bool) {
	if len(t.entries) == 0 || t.cursor < 0 || t.cursor >= len(t.entries) {
		return 0, false
	}
	return t.entries[t.cursor].lineIdx, true
}

// Move navigates the cursor according to the given motion.
// count is variadic: page motions use count[0] for the page size,
// non-page motions ignore count entirely. Missing count for page motions
// defaults to 1 (single step), which is harmless.
func (t *TOC) Move(m Motion, count ...int) {
	switch m {
	case MotionUp:
		if t.cursor > 0 {
			t.cursor--
		}
	case MotionDown:
		if t.cursor < len(t.entries)-1 {
			t.cursor++
		}
	case MotionPageUp:
		n := 1
		if len(count) > 0 {
			n = count[0]
		}
		t.cursor = max(0, t.cursor-n)
	case MotionPageDown:
		n := 1
		if len(count) > 0 {
			n = count[0]
		}
		t.cursor = min(len(t.entries)-1, t.cursor+n)
	case MotionFirst:
		t.cursor = 0
	case MotionLast:
		t.cursor = max(0, len(t.entries)-1)
	}
}

// EnsureVisible adjusts offset so the cursor is within the visible range of given height.
func (t *TOC) EnsureVisible(height int) {
	ensureVisible(&t.cursor, &t.offset, len(t.entries), height)
}

// SelectByVisibleRow sets the cursor to the entry at the given visible row.
// row is 0-based relative to the first visible TOC line (t.offset).
// returns true if the row maps to a valid entry, false otherwise.
// does not modify the cursor when returning false.
func (t *TOC) SelectByVisibleRow(row int) bool {
	if row < 0 {
		return false
	}
	idx := t.offset + row
	if idx >= len(t.entries) {
		return false
	}
	t.cursor = idx
	return true
}

// UpdateActiveSection finds the nearest entry with lineIdx <= diffCursor and sets activeSection.
// sets activeSection back to -1 when no entry matches, preserving the sentinel contract.
func (t *TOC) UpdateActiveSection(diffCursor int) {
	t.activeSection = -1
	for i, e := range t.entries {
		if e.lineIdx > diffCursor {
			break
		}
		t.activeSection = i
	}
}

// SyncCursorToActiveSection sets cursor to activeSection when activeSection >= 0.
// no-op when activeSection == -1 (no active section).
func (t *TOC) SyncCursorToActiveSection() {
	if t.activeSection >= 0 {
		t.cursor = t.activeSection
	}
}

// ScrollState returns the TOC's current visible window state.
// call after Render or EnsureVisible so Offset reflects active-section visibility.
func (t *TOC) ScrollState() ScrollState {
	return ScrollState{Total: len(t.entries), Offset: t.offset}
}

// Render produces the TOC display string with indentation by level.
// the highlighted entry uses FileSelected style in both modes:
// when TOC is focused it highlights the cursor, when diff is focused it highlights the active section.
func (t *TOC) Render(r TOCRender) string {
	if len(t.entries) == 0 {
		return "  no headers"
	}

	// determine which entry to highlight — cursor when TOC focused, active section when diff focused
	highlighted := t.cursor
	if !r.Focused && t.activeSection >= 0 {
		highlighted = t.activeSection
	}

	// ensure the highlighted entry is visible in the viewport
	savedCursor := t.cursor
	t.cursor = highlighted
	t.EnsureVisible(r.Height)
	t.cursor = savedCursor
	end := min(t.offset+r.Height, len(t.entries))

	var b strings.Builder
	for idx := t.offset; idx < end; idx++ {
		e := t.entries[idx]
		indent := strings.Repeat("  ", e.level-1)                // h1=0 indent, h2=2, h3=4, etc.
		title := t.truncateTitle(e.title, r.Width-len(indent)-4) // 2 prefix + 2 padding (matches FileSelected width-2)
		line := fmt.Sprintf("  %s%s", indent, title)

		if idx == highlighted {
			line = r.Resolver.Style(style.StyleKeyFileSelected).Width(max(r.Width-2, 1)).Render(line)
		}

		b.WriteString(line)
		if idx < end-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// truncateTitle trims a title to fit maxWidth display cells, appending ellipsis when truncated.
// uses runewidth for correct CJK/wide-character handling, consistent with FileTree.truncateDirName.
func (t *TOC) truncateTitle(title string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if runewidth.StringWidth(title) <= maxWidth {
		return title
	}
	if maxWidth <= 1 {
		return "…"
	}
	runes := []rune(title)
	w := 0
	end := 0
	for i, r := range runes {
		rw := runewidth.RuneWidth(r)
		if w+rw > maxWidth-1 { // reserve 1 cell for "…"
			break
		}
		w += rw
		end = i + 1
	}
	return string(runes[:end]) + "…"
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

// fencePrefix returns the fence character ('`' or '~') and count of leading consecutive
// occurrences. returns (0, 0) if the string doesn't start with backticks or tildes.
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
