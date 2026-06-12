package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// wardenLine builds a labeled response line (used for slash command output).
func (m model) wardenLine(suffix string) string {
	return "  " + WardenStyleAuto(m.autoMode).Render("warden") + "\n    " + suffix
}

func compactThinkText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func formatThinkDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	ms := d.Round(10 * time.Millisecond).Milliseconds()
	if ms < 10 {
		ms = 10
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	secs := d.Seconds()
	if secs < 10 {
		return fmt.Sprintf("%.1fs", secs)
	}
	if secs < 60 {
		return fmt.Sprintf("%.0fs", secs)
	}
	mins := int(d / time.Minute)
	sec := int((d % time.Minute) / time.Second)
	if sec == 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dm%02ds", mins, sec)
}

func wrapWords(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	lines := make([]string, 0, len(words))
	current := words[0]
	currentWidth := lipgloss.Width(current)

	for _, word := range words[1:] {
		wordWidth := lipgloss.Width(word)
		if currentWidth+1+wordWidth <= width {
			current += " " + word
			currentWidth += 1 + wordWidth
			continue
		}
		lines = append(lines, current)
		current = word
		currentWidth = wordWidth
	}

	lines = append(lines, current)
	return lines
}

func (m model) renderThinkEntry(entry messageEntry, active bool) string {
	duration := entry.duration
	if duration <= 0 && !entry.startedAt.IsZero() {
		duration = time.Since(entry.startedAt)
	}

	animating := active && entry.duration == 0 && m.loading

	if !m.verboseMode {
		if !animating {
			return DimStyle().Render("  Thought: " + formatThinkDuration(duration))
		}
		dots := []string{".", "..", "..."}
		dotIdx := ((m.spinner / 3) + 1) % 3
		verb := "Thinking"
		if entry.activity != "" {
			verb = entry.activity
		}
		return DimStyle().Render("  " + verb + dots[dotIdx])
	}

	var summary string
	if animating {
		dots := []string{".", "..", "..."}
		dotIdx := ((m.spinner / 3) + 1) % 3
		summary = DimStyle().Render("  Thinking" + dots[dotIdx])
	} else {
		summary = DimStyle().Render("  + Thought: " + formatThinkDuration(duration))
	}
	body := compactThinkText(entry.text)
	if body == "" {
		return summary
	}

	prefix := "    "
	firstWidth := m.width - lipgloss.Width(prefix)
	if firstWidth < 1 {
		firstWidth = 1
	}

	parts := wrapWords(body, firstWidth)
	lines := make([]string, 0, len(parts)+1)
	lines = append(lines, summary)
	for _, part := range parts {
		lines = append(lines, DimStyle().Render(prefix+part))
	}
	return strings.Join(lines, "\n")
}

func indentLines(text string, prefix string) string {
	if text == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

var (
	userMsgBg   = lipgloss.NewStyle().Background(lipgloss.Color("#242424"))
	assistantBg = lipgloss.NewStyle().Background(lipgloss.Color("#1a1a1a"))
)

// bgLine pads a single pre-rendered string to full terminal width with a background.
func (m *model) bgLine(style lipgloss.Style, content string) string {
	return style.Width(m.width).Render(content)
}

// applyBgLines applies background to each line of a multi-line string.
func (m *model) applyBgLines(style lipgloss.Style, content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = style.Width(m.width).Render(l)
	}
	return strings.Join(out, "\n")
}

func (m *model) renderUserMsg(text string) string {
	bgColor := lipgloss.Color("#242424")
	plainOnBg := lipgloss.NewStyle().Background(bgColor)
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)+2)
	out = append(out, plainOnBg.Width(m.width).Render(""))
	for _, l := range lines {
		out = append(out, plainOnBg.Width(m.width).Render("  "+l))
	}
	out = append(out, plainOnBg.Width(m.width).Render(""))
	return strings.Join(out, "\n")
}
