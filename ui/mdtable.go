package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/umputun/revdiff/diff"
)

// cellAlign captures GFM column alignment from the separator row.
type cellAlign int

const (
	alignDefault cellAlign = iota // no alignment marker — render left-padded
	alignLeft
	alignCenter
	alignRight
)

// tableBlock describes a contiguous run of `|`-prefixed lines that contains
// a separator row, with computed per-column widths and alignments.
type tableBlock struct {
	startIdx int         // first diffLines index in this block
	endIdx   int         // inclusive last diffLines index
	widths   []int       // visible cell width per column
	aligns   []cellAlign // alignment per column
	rows     [][]string  // pre-rendered (inline-MD applied) cells per row, parallel to startIdx..endIdx
	sepRow   int         // local row index of the separator (relative to startIdx)
}

// separatorCellRe matches a separator-row cell: optional leading colon, 1+
// dashes, optional trailing colon. Surrounding whitespace is trimmed before match.
var separatorCellRe = regexp.MustCompile(`^:?-+:?$`)

// BuildTableFormatted scans diff lines for markdown table blocks and returns a
// slice parallel to lines. Indices that belong to a recognized table block hold
// the reformatted display string; other indices are empty (signaling "no
// override" — the caller falls back to chroma-highlighted or raw content).
//
// codeFgOpen is the ANSI fg-set escape applied to inline `code` spans
// (typically m.ansiFg(colors.TableCode)). Pass "" to skip code coloring; bold
// and italic still render.
func BuildTableFormatted(lines []diff.DiffLine, codeFgOpen string) []string {
	out := make([]string, len(lines))
	blocks := detectTableBlocks(lines, codeFgOpen)
	for _, blk := range blocks {
		for j, cells := range blk.rows {
			isSep := j == blk.sepRow
			out[blk.startIdx+j] = formatRow(cells, blk.widths, blk.aligns, isSep)
		}
	}
	return out
}

// detectTableBlocks walks lines, skipping fenced and indented code, identifying
// contiguous table-row runs that contain at least one separator row.
// It pre-renders each cell's inline markdown so width math counts visible cells
// of the final output.
func detectTableBlocks(lines []diff.DiffLine, codeFgOpen string) []tableBlock {
	var blocks []tableBlock
	var fenceChar rune
	var fenceLen int

	i := 0
	for i < len(lines) {
		line := lines[i]
		if line.ChangeType == diff.ChangeDivider {
			i++
			continue
		}
		content := line.Content

		// fence tracking: opening / closing fenced code blocks per CommonMark.
		trimmed := strings.TrimSpace(content)
		if fenceChar == 0 {
			if ch, n := fencePrefix(trimmed); n >= 3 {
				fenceChar = ch
				fenceLen = n
				i++
				continue
			}
		} else if ch, n := fencePrefix(trimmed); ch == fenceChar && n >= fenceLen {
			rest := strings.TrimSpace(trimmed[n:])
			if rest == "" {
				fenceChar = 0
				fenceLen = 0
				i++
				continue
			}
		}
		if fenceChar != 0 {
			i++
			continue
		}

		// indented code blocks shadow tables — must not format.
		if isIndentedCodeLine(content) {
			i++
			continue
		}

		if !isTableCandidate(content) {
			i++
			continue
		}

		// scan forward through contiguous candidate rows (within fence/indent guards).
		j := i
		for j < len(lines) {
			ln := lines[j]
			if ln.ChangeType == diff.ChangeDivider {
				break
			}
			if isIndentedCodeLine(ln.Content) {
				break
			}
			if !isTableCandidate(ln.Content) {
				break
			}
			j++
		}
		// j is now exclusive end. block spans [i, j-1].
		if blk, ok := buildBlock(lines, i, j-1, codeFgOpen); ok {
			blocks = append(blocks, blk)
		}
		i = j
	}
	return blocks
}

// isTableCandidate reports whether a line shape qualifies as a possible table
// row: non-empty trimmed content beginning with `|`. Lines with leading `|` are
// the strict GFM form we recognize. Tables without the leading pipe are not
// detected (avoids false positives in prose containing inline `|`).
func isTableCandidate(content string) bool {
	t := strings.TrimLeft(content, " \t")
	return strings.HasPrefix(t, "|")
}

// buildBlock parses a candidate range [startIdx, endIdx] (inclusive). Returns
// (block, true) when the range contains a separator row, otherwise false.
func buildBlock(lines []diff.DiffLine, startIdx, endIdx int, codeFgOpen string) (tableBlock, bool) {
	rowCount := endIdx - startIdx + 1
	rendered := make([][]string, rowCount)
	rawCells := make([][]string, rowCount)
	sepIdx := -1
	var aligns []cellAlign

	// pass 1: parse cells, find separator
	for k := 0; k < rowCount; k++ {
		cells, isSep, rowAligns := parseRow(lines[startIdx+k].Content)
		rawCells[k] = cells
		if isSep && sepIdx == -1 {
			sepIdx = k
			aligns = rowAligns
		}
	}
	if sepIdx == -1 {
		return tableBlock{}, false
	}

	// pass 2: render inline MD per cell (separator cells skip — they become dashes)
	maxCols := len(aligns)
	for k, cells := range rawCells {
		if k == sepIdx {
			rendered[k] = nil // marker: separator row
			continue
		}
		if len(cells) > maxCols {
			maxCols = len(cells)
		}
		out := make([]string, len(cells))
		for c, cell := range cells {
			out[c] = renderInlineMD(strings.TrimSpace(cell), codeFgOpen)
		}
		rendered[k] = out
	}

	// align slice may be shorter than maxCols if data rows have more cells than
	// the separator declared — pad with default alignment.
	for len(aligns) < maxCols {
		aligns = append(aligns, alignDefault)
	}

	// pass 3: per-column max visible width across all data rows
	widths := make([]int, maxCols)
	for k, cells := range rendered {
		if k == sepIdx {
			continue
		}
		for c, cell := range cells {
			if w := ansi.StringWidth(cell); w > widths[c] {
				widths[c] = w
			}
		}
	}
	// guarantee a minimum visible width of 3 chars per column so the
	// separator dashes still read as a divider even for tiny cells.
	for i := range widths {
		if widths[i] < 3 {
			widths[i] = 3
		}
	}

	return tableBlock{
		startIdx: startIdx,
		endIdx:   endIdx,
		widths:   widths,
		aligns:   aligns,
		rows:     rendered,
		sepRow:   sepIdx,
	}, true
}

// parseRow splits a single table-row line into cells. Honors `\|` escapes and
// backtick code spans (pipes inside `…` do not split). Drops one leading and
// one trailing pipe if present (GFM tolerance — both bracketed and unbracketed
// forms accepted, though we require the leading pipe in isTableCandidate).
//
// Returns isSeparator=true when every cell matches the separator pattern, in
// which case alignments are populated from the cell shapes.
func parseRow(content string) (cells []string, isSeparator bool, aligns []cellAlign) {
	t := strings.TrimRight(strings.TrimLeft(content, " \t"), " \t")
	if t == "" {
		return nil, false, nil
	}
	// drop one leading pipe
	if strings.HasPrefix(t, "|") {
		t = t[1:]
	}
	// drop one trailing pipe (only if not escaped — the simple HasSuffix check
	// is fine because an escaped pipe at end would be `\|` not `|`)
	if strings.HasSuffix(t, "|") && !strings.HasSuffix(t, `\|`) {
		t = t[:len(t)-1]
	}

	cells = splitCells(t)

	// separator detection
	if len(cells) > 0 {
		all := true
		aligns = make([]cellAlign, len(cells))
		for i, c := range cells {
			tc := strings.TrimSpace(c)
			if !separatorCellRe.MatchString(tc) {
				all = false
				break
			}
			aligns[i] = alignmentOf(tc)
		}
		if all {
			return cells, true, aligns
		}
	}
	return cells, false, nil
}

// splitCells walks runes splitting on unescaped, non-code-span `|`.
func splitCells(s string) []string {
	var cells []string
	var cur strings.Builder
	inCode := false
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\\' && i+1 < len(runes) && runes[i+1] == '|' {
			cur.WriteRune('|')
			i++
			continue
		}
		if r == '`' {
			inCode = !inCode
			cur.WriteRune(r)
			continue
		}
		if r == '|' && !inCode {
			cells = append(cells, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	cells = append(cells, cur.String())
	return cells
}

// alignmentOf maps a separator-cell pattern to its declared alignment.
func alignmentOf(sep string) cellAlign {
	hasL := strings.HasPrefix(sep, ":")
	hasR := strings.HasSuffix(sep, ":")
	switch {
	case hasL && hasR:
		return alignCenter
	case hasR:
		return alignRight
	case hasL:
		return alignLeft
	default:
		return alignDefault
	}
}

// formatRow renders one table row to a display string. For data rows, cells
// are padded per (width, alignment) and joined with `│`. For separator rows,
// each column becomes a `─` run joined with `┼`, with `├`/`┤` at the ends.
func formatRow(cells []string, widths []int, aligns []cellAlign, isSeparator bool) string {
	if isSeparator {
		var b strings.Builder
		b.WriteRune('├')
		for c, w := range widths {
			if c > 0 {
				b.WriteRune('┼')
			}
			// 1 space pad on each side of the cell, plus the cell width itself
			b.WriteString(strings.Repeat("─", w+2))
		}
		b.WriteRune('┤')
		return b.String()
	}

	var b strings.Builder
	b.WriteRune('│')
	for c, w := range widths {
		if c > 0 {
			b.WriteRune('│')
		}
		var cell string
		if c < len(cells) {
			cell = cells[c]
		}
		align := alignDefault
		if c < len(aligns) {
			align = aligns[c]
		}
		b.WriteRune(' ')
		b.WriteString(padCell(cell, w, align))
		b.WriteRune(' ')
	}
	b.WriteRune('│')
	return b.String()
}

// padCell pads a (possibly ANSI-styled) cell to the target visible width using
// the requested alignment. Padding spaces are emitted as plain (no color), so
// add/remove diff backgrounds applied later by extendLineBg paint cleanly over
// them.
func padCell(s string, width int, align cellAlign) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	pad := width - w
	switch align {
	case alignRight:
		return strings.Repeat(" ", pad) + s
	case alignCenter:
		left := pad / 2
		right := pad - left
		return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
	default: // alignLeft, alignDefault
		return s + strings.Repeat(" ", pad)
	}
}

// renderInlineMD applies a small markdown subset to plain cell text:
//   - `code`     → codeFgOpen + content + \033[39m   (highest precedence)
//   - **bold**   → \033[1m + content + \033[22m      (recurses on inner span)
//   - *italic*   → \033[3m + content + \033[23m      (recurses on inner span)
//   - [text](url) → text                              (URL stripped)
//   - \X         → X                                  (escape passthrough)
//
// Unmatched delimiters fall through as literal characters. Recursion only
// happens between matched delimiters, so it terminates.
func renderInlineMD(s, codeFgOpen string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	runes := []rune(s)
	n := len(runes)
	i := 0
	for i < n {
		r := runes[i]

		// backtick code span — eat to closing backtick (highest precedence)
		if r == '`' {
			j := i + 1
			for j < n && runes[j] != '`' {
				j++
			}
			if j < n {
				if codeFgOpen != "" {
					b.WriteString(codeFgOpen)
				}
				b.WriteString(string(runes[i+1 : j]))
				if codeFgOpen != "" {
					b.WriteString("\033[39m")
				}
				i = j + 1
				continue
			}
			b.WriteRune(r)
			i++
			continue
		}

		// escape: \X → X (skip the backslash, copy the next char literally)
		if r == '\\' && i+1 < n {
			b.WriteRune(runes[i+1])
			i += 2
			continue
		}

		// bold: **text**
		if r == '*' && i+1 < n && runes[i+1] == '*' {
			j := i + 2
			for j+1 < n && !(runes[j] == '*' && runes[j+1] == '*') {
				j++
			}
			if j+1 < n {
				b.WriteString("\033[1m")
				b.WriteString(renderInlineMD(string(runes[i+2:j]), codeFgOpen))
				b.WriteString("\033[22m")
				i = j + 2
				continue
			}
			b.WriteRune(r)
			i++
			continue
		}

		// italic: *text*  (also _text_, but skip underscores in v1 — they're
		// uncommon in cell content and easy to add later if requested)
		if r == '*' {
			j := i + 1
			for j < n && runes[j] != '*' {
				j++
			}
			if j < n {
				b.WriteString("\033[3m")
				b.WriteString(renderInlineMD(string(runes[i+1:j]), codeFgOpen))
				b.WriteString("\033[23m")
				i = j + 1
				continue
			}
			b.WriteRune(r)
			i++
			continue
		}

		// link [text](url) → text only
		if r == '[' {
			if close1, close2, ok := findLinkBounds(runes, i); ok {
				b.WriteString(renderInlineMD(string(runes[i+1:close1]), codeFgOpen))
				i = close2 + 1
				continue
			}
			b.WriteRune(r)
			i++
			continue
		}

		b.WriteRune(r)
		i++
	}
	return b.String()
}

// findLinkBounds locates the `]` and `)` of `[text](url)` starting at runes[i]=='['.
// Returns (closeBracket, closeParen, true) on success. Bails on newlines or
// nested `[` to keep the matcher simple — pathological cases fall through to
// literal `[`.
func findLinkBounds(runes []rune, i int) (int, int, bool) {
	n := len(runes)
	close1 := -1
	for j := i + 1; j < n; j++ {
		if runes[j] == '\n' || runes[j] == '[' {
			return 0, 0, false
		}
		if runes[j] == ']' {
			if j+1 >= n || runes[j+1] != '(' {
				return 0, 0, false
			}
			close1 = j
			break
		}
	}
	if close1 < 0 {
		return 0, 0, false
	}
	for j := close1 + 2; j < n; j++ {
		if runes[j] == '\n' {
			return 0, 0, false
		}
		if runes[j] == ')' {
			return close1, j, true
		}
	}
	return 0, 0, false
}
