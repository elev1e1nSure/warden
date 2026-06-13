package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Indentation rule for everything in the chat viewport:
//   - contentIndent: the text column for assistant/think/chain/tool lines.
//     Decorators (breathing orb, →, +) sit in this same column, so text after a
//     decorator lands one space further — that's intentional and consistent.
//   - The user block matches contentIndent via its accent bar (col 0) + 1 space.
//   - bodyIndent: hanging indent for wrapped sub-text (think body), one level in.
const (
	contentIndent = "  "
	bodyIndent    = "    "
)

// wardenLine builds a labeled response line (used for slash command output).
func (m model) wardenLine(suffix string) string {
	return contentIndent + WardenStyleAuto(m.autoMode).Render("warden") + "\n" + bodyIndent + suffix
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

// pulseFrames is a breathing orb cycle: dim → bright → dim. Paired with the
// mode accent it reads as a live "working" heartbeat, not a blinking dot.
var pulseFrames = []string{"·", "•", "●", "●", "•", "·"}

// pulse returns the breathing accent-colored orb for the current spinner step.
// Occupies one column + trailing space, so text after it aligns at column 2 —
// matching the 2-space indent of frozen log lines.
func (m model) pulse() string {
	peak, mid, faint := Green, GreenMid, GreenFaint
	if m.autoMode {
		peak, mid, faint = Blue, BlueMid, BlueFaint
	}
	cols := []lipgloss.Color{faint, mid, peak, peak, mid, faint}
	i := (m.spinner / 2) % len(pulseFrames)
	orb := lipgloss.NewStyle().Foreground(cols[i]).Render(pulseFrames[i])
	return orb + " "
}

func (m model) renderThinkEntry(entry messageEntry, active bool) string {
	duration := entry.duration
	if duration <= 0 && !entry.startedAt.IsZero() {
		duration = time.Since(entry.startedAt)
	}

	animating := active && entry.duration == 0 && m.loading

	if !m.verboseMode {
		if !animating {
			return DimStyle().Render(contentIndent + "Thought: " + formatThinkDuration(duration))
		}
		verb := "Thinking"
		if entry.activity != "" {
			verb = entry.activity
		}
		return contentIndent + m.pulse() + DimStyle().Render(verb)
	}

	var summary string
	if animating {
		summary = contentIndent + m.pulse() + DimStyle().Render("Thinking")
	} else {
		summary = DimStyle().Render(contentIndent + "+ Thought: " + formatThinkDuration(duration))
	}
	body := compactThinkText(entry.text)
	if body == "" {
		return summary
	}

	prefix := bodyIndent
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

// userBg is the background of the user message block.
const userBg = lipgloss.Color("#191919")

// renderUserMsg renders a user message as an accent-barred block, the same width
// as the input box and centered with equal side margins. Inside the block the
// bar fills column 0, then a single space — so its text lines up with the
// gutter-indented assistant/think/tool lines (see renderMessages).
func (m *model) renderUserMsg(text string) string {
	accentColor := Green
	if m.autoMode {
		accentColor = Blue
	}
	bar := lipgloss.NewStyle().Foreground(accentColor).Background(userBg).Render("▌")
	bg := lipgloss.NewStyle().Background(userBg)
	inner := m.barWidth() - 1 // minus the bar
	if inner < 1 {
		inner = 1
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)+2)
	out = append(out, bar+bg.Width(inner).Render(""))
	for _, l := range lines {
		out = append(out, bar+bg.Width(inner).Render(" "+l))
	}
	out = append(out, bar+bg.Width(inner).Render(""))
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, strings.Join(out, "\n"))
}
