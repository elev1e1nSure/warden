package tui

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var presenceRng = rand.New(rand.NewSource(time.Now().UnixNano()))

var wardenPresencePhrases = []string{
	"here",
	"ready",
	"on it",
	"present",
	"nearby",
	"online",
	"alive",
	"on duty",
	"standing by",
	"on watch",
	"here",
	"at hand",
	"inside",
	"on line",
	"working",
	"close by",
	"didn't leave",
	"on",
	"alert",
	"watching",
	"on course",
	"on track",
	"on point",
	"right here",
	"in zone",
	"in network",
	"right here",
	"awake",
	"ready",
	"in order",
	"calm",
	"in shadow",
	"on guard",
	"covering",
	"on standby",
	"don't panic",
	"listening",
	"holding",
	"still here",
	"didn't move",
	"standing",
	"waiting",
	"attentive",
	"on guard",
	"with you",
	"at helm",
	"aware",
	"standing by",
	"right here",
	"alive here",
}

func randomWardenPresence() string {
	return wardenPresencePhrases[presenceRng.Intn(len(wardenPresencePhrases))]
}

func stickyTool(name string) bool {
	switch name {
	case "browser_open", "browser_read", "browser_screenshot", "youtube_search", "google_search":
		return true
	default:
		return false
	}
}

func toolPendingLine() string {
	return DimStyle().Render("  …")
}

func truncateRunes(text string, limit int) string {
	if limit < 1 {
		limit = 1
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-1]) + "…"
}

func toolResultIsError(result string) bool {
	lower := strings.ToLower(strings.TrimSpace(result))
	return strings.HasPrefix(lower, "error") ||
		strings.HasPrefix(lower, "stderr")
}

func toolSummaryLine(name string, result string) string {
	result = strings.TrimSpace(result)
	if result == "" {
		result = "(empty)"
	}
	lines := strings.Split(result, "\n")
	head := strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		head += fmt.Sprintf(" · +%d lines", len(lines)-1)
	}
	head = truncateRunes(head, 120)
	prefix := "  ✓ "
	style := DimStyle()
	if toolResultIsError(result) {
		prefix = "  ! "
		style = ErrorStyle()
	}
	return style.Render(prefix + name + " → " + head)
}

func toolResultBlock(result string) string {
	result = strings.TrimSpace(result)
	if result == "" {
		return DimStyle().Render("  (empty)")
	}

	lines := strings.Split(result, "\n")
	hidden := 0
	if len(lines) > 10 {
		hidden = len(lines) - 10
		lines = lines[:10]
	}
	for i, line := range lines {
		lines[i] = "  " + truncateRunes(strings.TrimRight(line, " \t"), 160)
	}
	if hidden > 0 {
		lines = append(lines, fmt.Sprintf("  … +%d lines", hidden))
	}
	if toolResultIsError(result) {
		return ErrorStyle().Render(strings.Join(lines, "\n"))
	}
	return DimStyle().Render(strings.Join(lines, "\n"))
}

func toolStartLine(name, args string) string {
	if args == "" {
		return ToolStyle().Render("▶ " + name)
	}
	return ToolStyle().Render("▶ "+name) + "  " + DimStyle().Render(truncateRunes(args, 160))
}

// ts returns a rendered timestamp in a unified format.
func (m model) ts() string {
	return DimStyle().Render("[" + m.wardenTS + "]")
}

// wardenLine builds the warden header line with an optional suffix.
func (m model) wardenLine(suffix string) string {
	return m.ts() + "  " + WardenStyle().Render("warden:") + "  " + suffix
}

func compactThinkText(text string) string {
	return strings.Join(strings.Fields(text), " ")
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

func (m model) renderThinkLine() string {
	think := compactThinkText(m.thinkBuf)
	if think == "" {
		think = "..."
	}

	prefix := m.ts() + "  " + WardenStyle().Render("warden:") + "  "
	firstWidth := m.width - lipgloss.Width(prefix)
	if firstWidth < 1 {
		firstWidth = 1
	}

	parts := wrapWords(think, firstWidth)
	if len(parts) == 0 {
		return m.wardenLine(DimStyle().Render(think))
	}

	lines := make([]string, 0, len(parts))
	lines = append(lines, prefix+DimStyle().Render(parts[0]))
	for _, part := range parts[1:] {
		lines = append(lines, DimStyle().Render(part))
	}
	return strings.Join(lines, "\n")
}

func (m *model) clearThinkLine() {
	if len(m.messages) == 0 {
		return
	}
	last := len(m.messages) - 1
	if strings.HasPrefix(m.messages[last], m.ts()+"  "+WardenStyle().Render("warden:")) {
		m.messages = append(m.messages[:last], m.messages[last+1:]...)
	}
}

func (m *model) syncViewport() {
	m.viewport = setContent(m.viewport, m.messages, m.streaming || m.loading)
}

func renderConfirmBlock(inner confirmMsg, width int) string {
	var b strings.Builder
	b.WriteString(ErrorStyle().Render("⚠ " + inner.title))
	b.WriteString("\n")
	b.WriteString(ToolStyle().Render("  " + inner.tool))
	b.WriteString("\n")
	if inner.preview != "" {
		b.WriteString(DimStyle().Render("  will run:"))
		b.WriteString("\n")
		preview := inner.preview
		if len(preview) > width-6 {
			preview = preview[:width-7] + "…"
		}
		b.WriteString("    " + preview)
		b.WriteString("\n")
	}
	if len(inner.details) > 0 {
		b.WriteString(DimStyle().Render("  why:"))
		b.WriteString("\n")
		for _, d := range inner.details {
			b.WriteString("    • " + d)
			b.WriteString("\n")
		}
	}
	b.WriteString(DimStyle().Render("  [Y] run    [Enter/Esc/N] cancel"))
	return b.String()
}

func (m model) View() string {
	if m.height == 0 {
		return ""
	}

	var footer string
	if m.confirming {
		footer = KeyStyle().Render("[Y]") +
			DimStyle().Render(" Run  ") +
			KeyStyle().Render("[Enter/Esc/N]") +
			DimStyle().Render(" Cancel")
	} else {
		footer = KeyStyle().Render("[Enter]") +
			DimStyle().Render(" Send  ") +
			KeyStyle().Render("[Esc]") +
			DimStyle().Render(" Clear  ") +
			KeyStyle().Render("[Ctrl+C]") +
			DimStyle().Render(" Exit")
	}

	var scrollTag string
	if m.viewport.TotalLineCount() > m.viewport.Height {
		if m.viewport.AtBottom() {
			scrollTag = " end "
		} else {
			scrollTag = fmt.Sprintf(" %d%% ", int(m.viewport.ScrollPercent()*100))
		}
	}
	sepWidth := m.width - len(scrollTag)
	if sepWidth < 0 {
		sepWidth = 0
	}
	sep1 := DimStyle().Render(strings.Repeat("─", sepWidth) + scrollTag)
	sep2 := DimStyle().Render(strings.Repeat("─", m.width))

	footer = m.renderFooterStatus(footer)

	layers := []string{m.viewport.View(), sep1}
	if m.hintVisible {
		layers = append(layers, m.renderHint())
	}
	layers = append(layers, m.textinput.View(), sep2, footer)
	return lipgloss.JoinVertical(lipgloss.Left, layers...)
}

func (m model) renderHint() string {
	matches := matchSlash(m.textinput.Value())
	lines := make([]string, 0, len(matches))
	for _, cmd := range matches {
		lines = append(lines,
			"  "+ToolStyle().Render(cmd.name)+"  "+DimStyle().Render(cmd.desc),
		)
	}
	return strings.Join(lines, "\n")
}

func (m model) renderFooterStatus(footer string) string {
	mode := SafeStyle().Render("Leashed")
	if m.autoMode {
		mode = AutoStyle().Render("Unleashed")
	}

	thinking := ThinkingOnStyle().Render("On")
	if !m.thinkingEnabled {
		thinking = ThinkingOffStyle().Render("Off")
	}

	status := StatusStyle().Render("Status: ") + mode +
		StatusStyle().Render("  Thinking: ") + thinking

	gap := m.width - lipgloss.Width(footer) - lipgloss.Width(status)
	if gap < 2 {
		gap = 2
	}
	return footer + strings.Repeat(" ", gap) + status
}
