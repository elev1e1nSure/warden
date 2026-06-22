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

var (
	diffAddStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8AE0A0"))
	diffRemoveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0908F"))
	diffCtxStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E6E6E"))
	diffFileStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#CFCFCF")).Bold(true)
	diffFrameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3A3A3A"))
	diffAddGutter  = lipgloss.NewStyle().Foreground(lipgloss.Color("#2D8A5A"))
	diffDelGutter  = lipgloss.NewStyle().Foreground(lipgloss.Color("#9A4343"))
	diffAddStat    = lipgloss.NewStyle().Foreground(lipgloss.Color("#8AE0A0"))
	diffDelStat    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0908F"))
)

type diffBodyLine struct {
	sign    byte // '+', '-', or ' '
	content string
	num     int
}

// renderUnifiedDiff renders a unified diff with line numbers and a framed header.
// filenameHint overrides the filename extracted from the diff header (optional).
func renderUnifiedDiff(diff string, width int, filenameHint ...string) string {
	hint := ""
	if len(filenameHint) > 0 {
		hint = filenameHint[0]
	}

	raw := strings.Split(strings.TrimRight(diff, "\n"), "\n")

	var body []diffBodyLine
	adds, dels := 0, 0
	file := hint
	maxNum := 1
	oldLine, newLine := 1, 1

	for _, l := range raw {
		l = strings.TrimSuffix(l, "\r")
		switch {
		case strings.HasPrefix(l, "diff --git"), strings.HasPrefix(l, "index "),
			strings.HasPrefix(l, "new file mode"), strings.HasPrefix(l, "deleted file mode"):
			// skip git metadata

		case strings.HasPrefix(l, "+++"):
			if file == "" {
				if p := strings.TrimSpace(strings.TrimPrefix(l, "+++")); p != "" && p != "/dev/null" {
					file = pathBase(strings.TrimPrefix(p, "b/"))
				}
			}

		case strings.HasPrefix(l, "---"):
			if file == "" {
				if p := strings.TrimSpace(strings.TrimPrefix(l, "---")); p != "" && p != "/dev/null" {
					file = pathBase(strings.TrimPrefix(p, "a/"))
				}
			}

		case strings.HasPrefix(l, "@@"):
			if m := hunkRe.FindStringSubmatch(l); m != nil {
				o, _ := strconv.Atoi(m[1])
				n, _ := strconv.Atoi(m[2])
				oldLine = o
				newLine = n
			}

		case strings.HasPrefix(l, "+"):
			adds++
			if newLine > maxNum {
				maxNum = newLine
			}
			body = append(body, diffBodyLine{'+', l[1:], newLine})
			newLine++

		case strings.HasPrefix(l, "-"):
			dels++
			if oldLine > maxNum {
				maxNum = oldLine
			}
			body = append(body, diffBodyLine{'-', l[1:], oldLine})
			oldLine++

		default:
			// context line (starts with space in unified diff)
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

	if file == "" {
		file = "diff"
	}

	numWidth := len(fmt.Sprintf("%d", maxNum))

	// Visible layout per body line:
	//   rail "  │ " = 4 chars
	//   linenum = numWidth chars (right-aligned, dim)
	//   " " = 1 char
	//   sign = 1 char (colored)
	//   "    " = 4 chars (colored with content)
	//   content = remaining
	overhead := 4 + numWidth + 1 + 1 + 4
	contentW := width - overhead
	if contentW < 4 {
		contentW = 4
	}

	rail := "  " + diffFrameStyle.Render("│") + " "

	out := make([]string, 0, len(body)+3)

	// Header
	stats := ""
	if adds > 0 {
		stats += diffAddStat.Render(fmt.Sprintf("+%d", adds))
	}
	if dels > 0 {
		if stats != "" {
			stats += " "
		}
		stats += diffDelStat.Render(fmt.Sprintf("-%d", dels))
	}
	header := "  " + diffFrameStyle.Render("╭ ") + diffFileStyle.Render(file)
	if stats != "" {
		header += "  " + stats
	}
	out = append(out, header)

	// Body
	for _, bl := range body {
		numStr := DimStyle().Render(fmt.Sprintf("%*d", numWidth, bl.num))
		content := truncateRunes(bl.content, contentW)

		var signStr string
		var lineStyle lipgloss.Style
		switch bl.sign {
		case '+':
			signStr = diffAddGutter.Render("+")
			lineStyle = diffAddStyle
		case '-':
			signStr = diffDelGutter.Render("-")
			lineStyle = diffRemoveStyle
		default:
			signStr = " "
			lineStyle = diffCtxStyle
		}

		out = append(out, rail+numStr+" "+signStr+lineStyle.Render("    "+content))
	}

	out = append(out, "  "+diffFrameStyle.Render("╰"))
	return strings.Join(out, "\n")
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
