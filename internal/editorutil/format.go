package editorutil

import (
	"go/format"
	"strings"
)

// FormatCode performs lightweight, zero-dependency code formatting respecting tabSize.
// Go uses the standard library go/format when syntactically valid; other languages
// normalize whitespace and (for brace-delimited languages) re-indent blocks cleanly.
func FormatCode(lang, src string, tabSize int) string {
	if strings.TrimSpace(src) == "" {
		return src
	}
	if tabSize != 2 && tabSize != 4 {
		tabSize = 4
	}
	normalized := normalizeNewlines(src)
	if lang == "golang" {
		if out, err := format.Source([]byte(normalized)); err == nil {
			return string(out)
		}
		return formatBraceCode(normalized, tabSize)
	}
	switch lang {
	case "cpp", "c", "java", "javascript", "typescript", "csharp", "kotlin", "swift", "rust", "scala", "php", "dart":
		return formatBraceCode(normalized, tabSize)
	default:
		return formatWhitespaceOnly(normalized, tabSize)
	}
}

func normalizeNewlines(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	return src
}

func formatWhitespaceOnly(src string, tabSize int) string {
	indentUnit := strings.Repeat(" ", tabSize)
	rawLines := strings.Split(src, "\n")
	out := make([]string, 0, len(rawLines))
	blankRun := 0
	for _, line := range rawLines {
		trimmedRight := strings.TrimRight(line, " \t")
		if trimmedRight == "" {
			if len(out) == 0 || blankRun >= 1 {
				continue
			}
			blankRun++
			out = append(out, "")
			continue
		}
		blankRun = 0
		// Expand leading tabs to spaces while keeping semantic indentation intact.
		i := 0
		for i < len(trimmedRight) && (trimmedRight[i] == ' ' || trimmedRight[i] == '\t') {
			i++
		}
		lead := strings.ReplaceAll(trimmedRight[:i], "\t", indentUnit)
		out = append(out, lead+trimmedRight[i:])
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

func formatBraceCode(src string, tabSize int) string {
	indentUnit := strings.Repeat(" ", tabSize)
	rawLines := strings.Split(src, "\n")
	out := make([]string, 0, len(rawLines))
	depth := 0
	inBlockComment := false
	blankRun := 0

	for _, raw := range rawLines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			if len(out) == 0 || blankRun >= 1 {
				continue
			}
			blankRun++
			out = append(out, "")
			continue
		}
		blankRun = 0

		if inBlockComment {
			prefix := strings.Repeat(indentUnit, depth)
			if strings.HasPrefix(trimmed, "*") {
				out = append(out, prefix+" "+trimmed)
			} else {
				out = append(out, prefix+trimmed)
			}
			if idx := strings.Index(trimmed, "*/"); idx >= 0 {
				inBlockComment = false
				openDelta, closeLead := scanBraceDelta(trimmed[idx+2:], &inBlockComment)
				depth = max(0, depth-closeLead+openDelta)
			}
			continue
		}

		openDelta, closeLead := scanBraceDelta(trimmed, &inBlockComment)
		lineDepth := max(0, depth-closeLead)
		if isLabelLine(trimmed) && lineDepth > 0 {
			lineDepth--
		}
		out = append(out, strings.Repeat(indentUnit, lineDepth)+trimmed)
		depth = max(0, depth-closeLead+openDelta)
	}

	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

func isLabelLine(trimmed string) bool {
	if strings.HasPrefix(trimmed, "case ") && strings.HasSuffix(trimmed, ":") {
		return true
	}
	switch trimmed {
	case "default:", "public:", "private:", "protected:":
		return true
	}
	return false
}

// scanBraceDelta computes leading closing-brace dedent and net remaining brace delta for one line,
// ignoring braces inside string/char literals and comments.
func scanBraceDelta(line string, inBlockComment *bool) (netAfterLead, leadClose int) {
	seenCode := false
	inSingle := false
	inDouble := false
	inBacktick := false
	escaped := false

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if *inBlockComment {
			if ch == '*' && i+1 < len(line) && line[i+1] == '/' {
				*inBlockComment = false
				i++
			}
			continue
		}
		if inSingle {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inDouble = false
			}
			continue
		}
		if inBacktick {
			if ch == '`' {
				inBacktick = false
			}
			continue
		}
		if ch == '/' && i+1 < len(line) {
			if line[i+1] == '/' {
				break
			}
			if line[i+1] == '*' {
				*inBlockComment = true
				i++
				continue
			}
		}
		if ch == '\'' {
			seenCode = true
			inSingle = true
			continue
		}
		if ch == '"' {
			seenCode = true
			inDouble = true
			continue
		}
		if ch == '`' {
			seenCode = true
			inBacktick = true
			continue
		}
		if ch == '}' {
			if !seenCode {
				leadClose++
			} else {
				netAfterLead--
			}
			continue
		}
		if ch == '{' {
			seenCode = true
			netAfterLead++
			continue
		}
		if ch != ' ' && ch != '\t' && ch != ';' && ch != ')' {
			seenCode = true
		}
	}
	return netAfterLead, leadClose
}
