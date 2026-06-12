package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var diffStatsRe = regexp.MustCompile(`(\+\d+)\s+(-\d+)$`)

var (
	diffAddStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#8AE0A0")).Background(lipgloss.Color("#10221A"))
	diffRemoveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0908F")).Background(lipgloss.Color("#241313"))
	diffHunkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Background(lipgloss.Color("#0C1B26"))
	diffCtxStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E6E6E")).Background(lipgloss.Color("#151515"))
	diffFileStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#CFCFCF")).Bold(true)
	diffFrameStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#3A3A3A"))
	diffAddGutter   = lipgloss.NewStyle().Foreground(lipgloss.Color("#2D8A5A"))
	diffDelGutter   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9A4343"))
	diffAddStat     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8AE0A0"))
	diffDelStat     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0908F"))
)

// renderUnifiedDiff renders a unified diff as a self-contained block, framed
// with a left rail and a header (file · +adds -dels), so edits stand out
// clearly from surrounding prose. Changed rows get a full-width color band.
func renderUnifiedDiff(diff string, width int) string {
	raw := strings.Split(strings.TrimRight(diff, "\n"), "\n")

	// First pass: count changes and find the target file path.
	adds, dels := 0, 0
	file := ""
	for _, l := range raw {
		l = strings.TrimSuffix(l, "\r")
		switch {
		case strings.HasPrefix(l, "+++"):
			if p := strings.TrimSpace(strings.TrimPrefix(l, "+++")); p != "" && p != "/dev/null" {
				file = strings.TrimPrefix(p, "b/")
			}
		case strings.HasPrefix(l, "---"):
			if file == "" {
				if p := strings.TrimSpace(strings.TrimPrefix(l, "---")); p != "" && p != "/dev/null" {
					file = strings.TrimPrefix(p, "a/")
				}
			}
		case strings.HasPrefix(l, "+"):
			adds++
		case strings.HasPrefix(l, "-"):
			dels++
		}
	}

	contentW := width - 4 // 2-space indent + rail + space
	if contentW < 8 {
		contentW = 8
	}
	pad := func(s string) string {
		if w := lipgloss.Width(s); w < contentW {
			return s + strings.Repeat(" ", contentW-w)
		}
		return truncateRunes(s, contentW)
	}
	rail := func(style lipgloss.Style) string { return "  " + style.Render("│") + " " }

	out := make([]string, 0, len(raw)+3)

	// Header
	if file == "" {
		file = "diff"
	}
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
	header := "  " + diffFrameStyle.Render("╭ ") + diffFileStyle.Render(pathBase(file))
	if stats != "" {
		header += "  " + stats
	}
	out = append(out, header)

	// Body
	for _, l := range raw {
		l = strings.TrimSuffix(l, "\r")
		switch {
		case strings.HasPrefix(l, "diff --git"), strings.HasPrefix(l, "index "),
			strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"):
			continue // metadata folded into the header
		case strings.HasPrefix(l, "@@"):
			out = append(out, rail(diffFrameStyle)+diffHunkStyle.Render(pad(l)))
		case strings.HasPrefix(l, "+"):
			out = append(out, rail(diffAddGutter)+diffAddStyle.Render(pad(l)))
		case strings.HasPrefix(l, "-"):
			out = append(out, rail(diffDelGutter)+diffRemoveStyle.Render(pad(l)))
		default:
			out = append(out, rail(diffFrameStyle)+diffCtxStyle.Render(pad(l)))
		}
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
