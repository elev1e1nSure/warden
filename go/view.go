package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const wardenVersion = "v0.1.0"
const wardenModel = "qwen3:8b"

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
	return m.ts() + "  " + WardenStyle().Render("Warden:") + "  " + suffix
}

func compactThinkText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func formatThinkDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Second {
		ms := d.Round(10 * time.Millisecond)
		if ms < 10*time.Millisecond {
			ms = 10 * time.Millisecond
		}
		return fmt.Sprintf("%dms", ms/time.Millisecond)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	mins := int(d / time.Minute)
	secs := int((d % time.Minute) / time.Second)
	if secs == 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dm%02ds", mins, secs)
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

func (m model) renderThinkEntry(entry messageEntry) string {
	duration := entry.duration
	if duration <= 0 && !entry.startedAt.IsZero() {
		duration = time.Since(entry.startedAt)
	}
	summary := m.ts() + "  " + WardenStyle().Render("Warden:") + "  " + DimStyle().Render("Thought "+formatThinkDuration(duration))
	if !m.thinkingExpanded {
		return summary
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

func (m *model) renderMessages() []string {
	m.ensureMarkdownRenderer()
	out := make([]string, 0, len(m.messages))
	for _, entry := range m.messages {
		switch entry.kind {
		case messageThink:
			out = append(out, m.renderThinkEntry(entry))
		case messageAssistant:
			out = append(out, m.renderMarkdown(entry.text))
		default:
			out = append(out, entry.text)
		}
	}
	return out
}

func (m *model) syncViewport() {
	m.viewport = setContent(m.viewport, m.renderMessages(), m.streaming || m.loading)
}

func renderConfirmBlock(inner confirmMsg, width int) string {
	var b strings.Builder

	// ⚠  <action title>
	b.WriteString(ErrorStyle().Bold(true).Render("⚠  ") + TitleStyle().Render(inner.title))
	b.WriteString("\n")

	//   <tool>  ·  <filename>
	toolPart := "   " + ToolStyle().Bold(true).Render(inner.tool)
	if inner.preview != "" {
		sep := DimStyle().Render("  ·  ")
		filename := filepath.Base(inner.preview)
		limit := width - lipgloss.Width(toolPart) - lipgloss.Width(sep) - 2
		preview := truncateRunes(filename, limit)
		toolPart += sep + preview
	}
	b.WriteString(toolPart)
	b.WriteString("\n")

	// details: flat, dim, no bullets, no label
	for _, d := range inner.details {
		// Replace "path: <filename>" with "path: <full path>" if preview is available
		detail := d
		if strings.HasPrefix(d, "path: ") && inner.preview != "" {
			detail = "path: " + inner.preview
		}
		b.WriteString(DimStyle().Render("   " + detail))
		b.WriteString("\n")
	}

	// Confirmation buttons: larger, closer together, under the warning
	b.WriteString("\n")
	yBtn := ConfirmYStyle().Render("  Y  run  ")
	nBtn := ConfirmNStyle().Render("  N  cancel  ")
	b.WriteString(yBtn + nBtn)

	return b.String()
}

func (m model) renderHeader() string {
	var b strings.Builder

	b.WriteString("\n\n")

	prefix := "    "

	b.WriteString(prefix)
	b.WriteString(HeaderStyle().Render("Warden CLI"))
	b.WriteString(DimStyle().Render(" " + wardenVersion))
	b.WriteString("\n")

	mode := "Leashed"
	if m.autoMode {
		mode = "Unleashed"
	}
	reasoning := "On"
	if !m.thinkingEnabled {
		reasoning = "Off"
	}
	b.WriteString(prefix)
	b.WriteString(DimStyle().Render(wardenModel + " · " + mode + " · Thinking " + reasoning))
	b.WriteString("\n")

	b.WriteString(prefix)
	b.WriteString(DimStyle().Render(m.cwd))
	b.WriteString("\n")

	sepWidth := m.width - 8
	if sepWidth < 1 {
		sepWidth = 1
	}
	b.WriteString(prefix)
	b.WriteString(FaintStyle().Render(strings.Repeat("─", sepWidth)))
	b.WriteString("\n")

	return b.String()
}

func (m model) View() string {
	if m.height == 0 {
		return ""
	}

	var footer string
	if m.confirming {
		footer = DimStyle().Render("Press Y to run, N to cancel")
	} else {
		footer = KeyStyle().Render("[F2]") +
			DimStyle().Render(" Thoughts")
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
	sep1 := FaintStyle().Render(strings.Repeat("─", sepWidth) + scrollTag)
	sep2 := FaintStyle().Render(strings.Repeat("─", m.width))

	layers := []string{m.renderHeader(), m.viewport.View(), sep1}
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

	reasoning := ThinkingOnStyle().Render("On")
	if !m.thinkingEnabled {
		reasoning = ThinkingOffStyle().Render("Off")
	}

	status := StatusStyle().Render("Status: ") + mode +
		StatusStyle().Render("  Thinking: ") + reasoning

	gap := m.width - lipgloss.Width(footer) - lipgloss.Width(status)
	if gap < 2 {
		gap = 2
	}
	return footer + strings.Repeat(" ", gap) + status
}
