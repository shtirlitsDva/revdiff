package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/revdiff/diff"
)

// ctx builds a slice of ChangeContext diff lines from raw content strings.
func ctx(lines ...string) []diff.DiffLine {
	out := make([]diff.DiffLine, len(lines))
	for i, s := range lines {
		out[i] = diff.DiffLine{ChangeType: diff.ChangeContext, Content: s, NewNum: i + 1, OldNum: i + 1}
	}
	return out
}

// firstNonEmpty returns the first non-empty string in the slice, or "".
func firstNonEmpty(s []string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

func TestBuildTableFormatted_basicTable(t *testing.T) {
	lines := ctx(
		"Some prose before.",
		"",
		"| A | B |",
		"|---|---|",
		"| 1 | 2 |",
		"",
		"More prose.",
	)
	out := BuildTableFormatted(lines, "")
	require.Len(t, out, len(lines))

	// non-table lines untouched
	assert.Empty(t, out[0])
	assert.Empty(t, out[1])
	assert.Empty(t, out[5])
	assert.Empty(t, out[6])

	// table lines formatted
	assert.NotEmpty(t, out[2])
	assert.NotEmpty(t, out[3])
	assert.NotEmpty(t, out[4])

	assert.Contains(t, out[2], "│", "data row should use │ separator")
	assert.Contains(t, out[3], "├", "separator row should start with ├")
	assert.Contains(t, out[3], "┤", "separator row should end with ┤")
	assert.Contains(t, out[3], "┼", "separator row should use ┼ for column joins")
	assert.Contains(t, out[3], "─", "separator row should use ─ for fill")

	// rows align: visible width of all data and separator rows should match
	w2 := ansi.StringWidth(out[2])
	w3 := ansi.StringWidth(out[3])
	w4 := ansi.StringWidth(out[4])
	assert.Equal(t, w2, w3, "header and separator row widths must match")
	assert.Equal(t, w2, w4, "header and data row widths must match")
}

func TestBuildTableFormatted_skipsBlockWithoutSeparator(t *testing.T) {
	// no separator row → not a real table; leave indices empty.
	lines := ctx(
		"| A | B |",
		"| 1 | 2 |",
		"| 3 | 4 |",
	)
	out := BuildTableFormatted(lines, "")
	for i, s := range out {
		assert.Empty(t, s, "line %d should not be formatted (no separator row)", i)
	}
}

func TestBuildTableFormatted_pipesInsideCodeSpans(t *testing.T) {
	lines := ctx(
		"| Code | Note |",
		"|------|------|",
		"| `a | b` | inline pipe |",
	)
	out := BuildTableFormatted(lines, "")
	require.NotEmpty(t, out[2])
	// the inline-code cell content `a | b` must survive as a single cell —
	// stripped of ANSI, it should still contain "a | b" as one logical group.
	plain := ansi.Strip(out[2])
	assert.Contains(t, plain, "a | b", "pipe inside `…` must not split")
}

func TestBuildTableFormatted_escapedPipes(t *testing.T) {
	lines := ctx(
		"| A | B |",
		"|---|---|",
		`| x \| y | z |`,
	)
	out := BuildTableFormatted(lines, "")
	require.NotEmpty(t, out[2])
	plain := ansi.Strip(out[2])
	assert.Contains(t, plain, "x | y", `\| must be unescaped to literal |`)
}

func TestBuildTableFormatted_alignmentMarkers(t *testing.T) {
	lines := ctx(
		"| L | C | R |",
		"|:---|:---:|---:|",
		"| a | b | c |",
		"| 1234 | 5678 | 9012 |",
	)
	out := BuildTableFormatted(lines, "")
	require.NotEmpty(t, out[2])

	// row "a | b | c" — short cells; check that:
	//  - left-aligned cell ends with spaces (pad on right)
	//  - center-aligned cell has roughly equal padding
	//  - right-aligned cell starts with spaces (pad on left)
	plain := ansi.Strip(out[2])
	cells := strings.Split(plain, "│")
	require.GreaterOrEqual(t, len(cells), 5, "expected ≥5 split parts (outer empties + 3 cells)")
	left, center, right := cells[1], cells[2], cells[3]

	assert.True(t, strings.HasPrefix(left, " a "), "left-aligned cell should start with content after leading space: %q", left)
	assert.True(t, strings.HasSuffix(left, " "), "left-aligned cell should end with padding")

	// center cell visible content "b" with padding both sides
	assert.Contains(t, center, "b")
	assert.True(t, strings.HasPrefix(center, " ") && strings.HasSuffix(center, " "))

	assert.True(t, strings.HasSuffix(right, "c "), "right-aligned cell should have content at end: %q", right)
}

func TestBuildTableFormatted_skipsTablesInsideFencedCode(t *testing.T) {
	lines := ctx(
		"```",
		"| A | B |",
		"|---|---|",
		"| 1 | 2 |",
		"```",
		"",
		"| X | Y |",
		"|---|---|",
		"| 3 | 4 |",
	)
	out := BuildTableFormatted(lines, "")
	// inside fenced code: must NOT format
	for i := 1; i <= 3; i++ {
		assert.Empty(t, out[i], "line %d (inside fence) must not be formatted", i)
	}
	// outside fence: must format
	assert.NotEmpty(t, out[6])
	assert.NotEmpty(t, out[7])
	assert.NotEmpty(t, out[8])
}

func TestBuildTableFormatted_skipsTablesInIndentedCode(t *testing.T) {
	lines := ctx(
		"    | A | B |",
		"    |---|---|",
		"    | 1 | 2 |",
	)
	out := BuildTableFormatted(lines, "")
	for i, s := range out {
		assert.Empty(t, s, "line %d (indented code) must not be formatted", i)
	}
}

func TestBuildTableFormatted_singleColumn(t *testing.T) {
	lines := ctx(
		"| Header |",
		"|--------|",
		"| value  |",
	)
	out := BuildTableFormatted(lines, "")
	require.NotEmpty(t, firstNonEmpty(out))
	for _, s := range out {
		require.NotEmpty(t, s)
	}
}

func TestBuildTableFormatted_mismatchedColumnCounts(t *testing.T) {
	// data row has more cells than the separator declared. Must not panic;
	// missing alignments default to alignDefault, extra columns widen the grid.
	lines := ctx(
		"| A | B |",
		"|---|---|",
		"| 1 | 2 | 3 |",
	)
	out := BuildTableFormatted(lines, "")
	require.NotEmpty(t, out[2])
	plain := ansi.Strip(out[2])
	assert.Contains(t, plain, "1")
	assert.Contains(t, plain, "2")
	assert.Contains(t, plain, "3")
}

func TestBuildTableFormatted_widthSpansAllRows(t *testing.T) {
	// later row has a wider cell — earlier rows must pad to that width.
	lines := ctx(
		"| A | B |",
		"|---|---|",
		"| x | y |",
		"| longer-content | y |",
	)
	out := BuildTableFormatted(lines, "")
	require.Len(t, out, 4)
	w := ansi.StringWidth(out[0])
	for i := range out {
		assert.Equal(t, w, ansi.StringWidth(out[i]), "row %d width must match header width", i)
	}
}

func TestBuildTableFormatted_diffModeMixedChangeTypes(t *testing.T) {
	// a contiguous run of |-prefixed rows containing add/remove/context
	// must be treated as one block. Widths span all rows.
	lines := []diff.DiffLine{
		{ChangeType: diff.ChangeContext, Content: "| A | B |", NewNum: 1, OldNum: 1},
		{ChangeType: diff.ChangeContext, Content: "|---|---|", NewNum: 2, OldNum: 2},
		{ChangeType: diff.ChangeContext, Content: "| x | y |", NewNum: 3, OldNum: 3},
		{ChangeType: diff.ChangeRemove, Content: "| old-row | y |", OldNum: 4},
		{ChangeType: diff.ChangeAdd, Content: "| new-row-longer | y |", NewNum: 4},
	}
	out := BuildTableFormatted(lines, "")
	for i, s := range out {
		require.NotEmpty(t, s, "row %d should be formatted", i)
	}
	// all rows must align to the longest cell ("new-row-longer")
	w0 := ansi.StringWidth(out[0])
	for i, s := range out {
		assert.Equal(t, w0, ansi.StringWidth(s), "row %d width must match", i)
	}
}

func TestBuildTableFormatted_dividerBreaksBlock(t *testing.T) {
	// a hunk Divider in the middle of a |-prefixed run must break the block.
	lines := []diff.DiffLine{
		{ChangeType: diff.ChangeContext, Content: "| A | B |", NewNum: 1},
		{ChangeType: diff.ChangeContext, Content: "|---|---|", NewNum: 2},
		{ChangeType: diff.ChangeContext, Content: "| 1 | 2 |", NewNum: 3},
		{ChangeType: diff.ChangeDivider, Content: "..."},
		{ChangeType: diff.ChangeContext, Content: "| 9 | 8 |", NewNum: 50},
	}
	out := BuildTableFormatted(lines, "")
	// first block valid; second is just a stray |-row with no separator → not formatted.
	assert.NotEmpty(t, out[0])
	assert.NotEmpty(t, out[1])
	assert.NotEmpty(t, out[2])
	assert.Empty(t, out[3], "divider should not be formatted")
	assert.Empty(t, out[4], "post-divider stray |-row has no separator → must not format")
}

func TestRenderInlineMD_codeAndBoldAndItalic(t *testing.T) {
	codeOpen := "\033[38;2;100;200;255m"
	tests := []struct {
		name  string
		in    string
		check func(t *testing.T, got string)
	}{
		{
			name: "bold",
			in:   "**hello**",
			check: func(t *testing.T, got string) {
				assert.Contains(t, got, "\033[1m")
				assert.Contains(t, got, "hello")
				assert.Contains(t, got, "\033[22m")
			},
		},
		{
			name: "italic",
			in:   "*hello*",
			check: func(t *testing.T, got string) {
				assert.Contains(t, got, "\033[3m")
				assert.Contains(t, got, "hello")
				assert.Contains(t, got, "\033[23m")
			},
		},
		{
			name: "code",
			in:   "`hello`",
			check: func(t *testing.T, got string) {
				assert.Contains(t, got, codeOpen)
				assert.Contains(t, got, "hello")
				assert.Contains(t, got, "\033[39m")
			},
		},
		{
			name: "link strips url",
			in:   "[click](https://example.com)",
			check: func(t *testing.T, got string) {
				assert.Equal(t, "click", ansi.Strip(got))
			},
		},
		{
			name: "code wins over emphasis",
			in:   "`*not italic*`",
			check: func(t *testing.T, got string) {
				assert.NotContains(t, got, "\033[3m", "italic should not trigger inside code span")
				assert.Contains(t, got, "*not italic*", "code content rendered verbatim")
			},
		},
		{
			name: "unmatched marker passes through",
			in:   "**unclosed",
			check: func(t *testing.T, got string) {
				assert.Equal(t, "**unclosed", ansi.Strip(got))
			},
		},
		{
			name: "escape passthrough",
			in:   `\*literal\*`,
			check: func(t *testing.T, got string) {
				assert.Equal(t, "*literal*", ansi.Strip(got))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderInlineMD(tt.in, codeOpen)
			tt.check(t, got)
		})
	}
}

func TestParseRow_separator(t *testing.T) {
	cells, isSep, aligns := parseRow("|:---|:---:|---:|")
	assert.True(t, isSep)
	require.Len(t, cells, 3)
	require.Len(t, aligns, 3)
	assert.Equal(t, alignLeft, aligns[0])
	assert.Equal(t, alignCenter, aligns[1])
	assert.Equal(t, alignRight, aligns[2])
}

func TestParseRow_dataRow(t *testing.T) {
	cells, isSep, _ := parseRow("| a | b | c |")
	assert.False(t, isSep)
	require.Len(t, cells, 3)
}

// --- integration tests against Model ---

func TestModel_handleFileLoaded_autoEnablesTableModeForMarkdown(t *testing.T) {
	lines := ctx(
		"# title",
		"",
		"| A | B |",
		"|---|---|",
		"| 1 | 2 |",
	)
	m := testModel([]string{"doc.md"}, map[string][]diff.DiffLine{"doc.md": lines})
	m.singleFile = true

	result, _ := m.Update(fileLoadedMsg{file: "doc.md", lines: lines, seq: m.loadSeq})
	model := result.(Model)

	assert.True(t, model.tableMode, "tableMode must auto-enable for .md files")
	require.Len(t, model.tableFormatted, len(lines))
	// the three table-row indices should have non-empty formatted strings
	assert.NotEmpty(t, model.tableFormatted[2])
	assert.NotEmpty(t, model.tableFormatted[3])
	assert.NotEmpty(t, model.tableFormatted[4])
	// non-table indices are empty
	assert.Empty(t, model.tableFormatted[0])
	assert.Empty(t, model.tableFormatted[1])
}

func TestModel_handleFileLoaded_disablesTableModeForNonMarkdown(t *testing.T) {
	lines := ctx("package main", "func main() {}", "")
	m := testModel([]string{"main.go"}, map[string][]diff.DiffLine{"main.go": lines})

	result, _ := m.Update(fileLoadedMsg{file: "main.go", lines: lines, seq: m.loadSeq})
	model := result.(Model)

	assert.False(t, model.tableMode, "tableMode must default off for non-md files")
	assert.Nil(t, model.tableFormatted)
}

func TestModel_toggleTableMode_roundTrips(t *testing.T) {
	lines := ctx(
		"| A | B |",
		"|---|---|",
		"| 1 | 2 |",
	)
	m := testModel([]string{"doc.md"}, map[string][]diff.DiffLine{"doc.md": lines})
	m.singleFile = true

	result, _ := m.Update(fileLoadedMsg{file: "doc.md", lines: lines, seq: m.loadSeq})
	model := result.(Model)
	require.True(t, model.tableMode)
	require.NotNil(t, model.tableFormatted)

	// focus must be paneDiff for the toggle to take effect
	model.focus = paneDiff

	// toggle off
	model.toggleTableMode()
	assert.False(t, model.tableMode)
	assert.Nil(t, model.tableFormatted, "tableFormatted should clear when toggled off")

	// toggle back on
	model.toggleTableMode()
	assert.True(t, model.tableMode)
	require.NotNil(t, model.tableFormatted)
	assert.NotEmpty(t, model.tableFormatted[0])
}

func TestModel_prepareLineContent_tableFormattedWinsOverHighlight(t *testing.T) {
	lines := ctx(
		"| A | B |",
		"|---|---|",
		"| 1 | 2 |",
	)
	m := testModel([]string{"doc.md"}, map[string][]diff.DiffLine{"doc.md": lines})
	m.singleFile = true

	result, _ := m.Update(fileLoadedMsg{file: "doc.md", lines: lines, seq: m.loadSeq})
	model := result.(Model)

	// inject fake highlight content to verify table formatting wins
	model.highlightedLines = []string{"FAKE-HL-0", "FAKE-HL-1", "FAKE-HL-2"}

	_, textContent, hasHL := model.prepareLineContent(0, model.diffLines[0])
	assert.True(t, hasHL)
	assert.Contains(t, textContent, "│", "table-formatted content must override fake highlight")
	assert.NotContains(t, textContent, "FAKE-HL-0")
}

func TestModel_wrapModeDisablesTableFormatting(t *testing.T) {
	lines := ctx(
		"| A | B |",
		"|---|---|",
		"| 1 | 2 |",
	)
	m := testModel([]string{"doc.md"}, map[string][]diff.DiffLine{"doc.md": lines})
	m.singleFile = true
	m.wrapMode = true

	result, _ := m.Update(fileLoadedMsg{file: "doc.md", lines: lines, seq: m.loadSeq})
	model := result.(Model)

	// tableMode auto-enables, but wrapMode suppresses the formatted slice
	assert.True(t, model.tableMode)
	assert.Nil(t, model.tableFormatted, "wrapMode must suppress table formatting")
}

func TestModel_renderDiff_outputsTableSeparators(t *testing.T) {
	lines := ctx(
		"| A | B |",
		"|---|---|",
		"| 1 | 2 |",
	)
	m := testModel([]string{"doc.md"}, map[string][]diff.DiffLine{"doc.md": lines})
	m.singleFile = true
	// give the viewport some room so renderDiff produces output
	m.viewport = viewport.New(80, 20)

	result, _ := m.Update(fileLoadedMsg{file: "doc.md", lines: lines, seq: m.loadSeq})
	model := result.(Model)

	out := model.renderDiff()
	plain := ansi.Strip(out)
	assert.Contains(t, plain, "│", "rendered output must contain table │ separators")
	assert.Contains(t, plain, "├", "rendered output must contain ├ in separator row")
}
