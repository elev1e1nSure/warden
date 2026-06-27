package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var hunkRe = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)
var diffStatsRe = regexp.MustCompile(`(\+\d+)\s+(-\d+)$`)

type diffBodyLine struct {
	sign    byte // '+', '-', or ' '
	content string
	num     int
}

// renderUnifiedDiff renders a unified diff body with line numbers.
// The filename is NOT shown here — it's already visible in the tool summary line.
// The variadic filenameHint is accepted for call-site compatibility but ignored.
func renderUnifiedDiff(diff string, width int, filenameHint ...string) string {
	_ = filenameHint
	raw := strings.Split(strings.TrimRight(diff, "\n"), "\n")

	var body []diffBodyLine
	maxNum := 1
	oldLine, newLine := 1, 1

	for _, l := range raw {
		l = strings.TrimSuffix(l, "\r")
		switch {
		case strings.HasPrefix(l, "diff --git"), strings.HasPrefix(l, "index "),
			strings.HasPrefix(l, "new file mode"), strings.HasPrefix(l, "deleted file mode"),
			strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"):
			// skip git/diff metadata

		case strings.HasPrefix(l, "@@"):
			if m := hunkRe.FindStringSubmatch(l); m != nil {
				o, _ := strconv.Atoi(m[1])
				n, _ := strconv.Atoi(m[2])
				oldLine = o
				newLine = n
			}

		case strings.HasPrefix(l, "+"):
			if newLine > maxNum {
				maxNum = newLine
			}
			body = append(body, diffBodyLine{'+', l[1:], newLine})
			newLine++

		case strings.HasPrefix(l, "-"):
			if oldLine > maxNum {
				maxNum = oldLine
			}
			body = append(body, diffBodyLine{'-', l[1:], oldLine})
			oldLine++

		default:
			content := ""
			if len(l) > 0 {
				content = l[1:]
			}
			if newLine > maxNum {
				maxNum = newLine
			}
			body = append(body, diffBodyLine{' ', content, newLine})
			oldLine++
			newLine++
		}
	}

	numWidth := len(fmt.Sprintf("%d", maxNum))
	// per-line visible layout: numWidth + " " + sign(1) + "    "(4) + content
	contentW := width - numWidth - 6
	contentW = max(contentW, 4)

	var sb strings.Builder
	for i, bl := range body {
		if i > 0 {
			sb.WriteByte('\n')
		}

		// line number — dim grey
		numStr := fmt.Sprintf("%*d", numWidth, bl.num)
		sb.WriteString("\x1b[38;2;102;102;102m")
		sb.WriteString(numStr)
		sb.WriteString("\x1b[0m ")

		content := truncateRunes(bl.content, contentW)

		switch bl.sign {
		case '+':
			sb.WriteString("\x1b[38;2;45;138;90m+\x1b[0m")
			sb.WriteString("\x1b[38;2;138;224;160m    ")
			sb.WriteString(content)
			sb.WriteString("\x1b[0m")
		case '-':
			sb.WriteString("\x1b[38;2;154;67;67m-\x1b[0m")
			sb.WriteString("\x1b[38;2;240;144;143m    ")
			sb.WriteString(content)
			sb.WriteString("\x1b[0m")
		default:
			sb.WriteString(" \x1b[38;2;110;110;110m    ")
			sb.WriteString(content)
			sb.WriteString("\x1b[0m")
		}
	}
	return sb.String()
}

// renderDiffStats finds "+N -N" at the end of s, returns (prefix, colored stats).
func renderDiffStats(s string) (string, string) {
	loc := diffStatsRe.FindStringIndex(s)
	if loc == nil {
		return s, ""
	}
	match := diffStatsRe.FindStringSubmatch(s)
	prefix := strings.TrimRight(s[:loc[0]], " ")
	add := lipgloss.NewStyle().Foreground(lipgloss.Color("#00D47A")).Render(match[1])
	del := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff4444")).Render(match[2])
	return prefix, add + "  " + del
}
