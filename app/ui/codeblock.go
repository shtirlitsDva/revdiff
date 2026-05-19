package ui

// shared CommonMark fenced/indented code-block detection helpers.
// used by parseTOC (ui/mdtoc.go) and the markdown table detector (ui/mdtable.go).

// fencePrefix returns the fence character ('`' or '~') and count of leading
// consecutive occurrences. Returns (0, 0) if the string doesn't start with
// backticks or tildes.
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
