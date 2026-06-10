package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const wardenVersion = "v0.1.0"

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
		return ""
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
		head += fmt.Sprintf("  +%d lines", len(lines)-1)
	}
	head = truncateRunes(head, 100)

	arrow := ToolStyle().Render("  → ")
	if toolResultIsError(result) {
		return arrow + ErrorStyle().Render(name) + "  " + ErrorStyle().Render(head)
	}
	return arrow + ToolStyle().Render(name) + "  " + DimStyle().Render(head)
}

func toolResultBlock(result string) string {
	trimmed := strings.TrimSpace(result)
	if trimmed == "" {
		return DimStyle().Render("  (empty)")
	}

	lines := strings.Split(trimmed, "\n")
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
	arrow := ToolStyle().Render("  → ")
	toolName := ToolStyle().Render(name)
	if args == "" {
		return arrow + toolName
	}
	return arrow + toolName + "  " + DimStyle().Render(truncateRunes(args, 140))
}

// wardenLine builds a labeled response line (used for slash command output).
func (m model) wardenLine(suffix string) string {
	return WardenStyle().Render("warden") + "  " + suffix
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
	brailleFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	var summary string
	if entry.duration == 0 {
		frame := brailleFrames[m.spinner%len(brailleFrames)]
		summary = DimStyle().Render("  " + frame + "  thinking")
	} else {
		summary = DimStyle().Render("  + Thought: " + formatThinkDuration(duration))
	}
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

func (m *model) renderMessages() []string {
	m.ensureMarkdownRenderer()
	out := make([]string, 0, len(m.messages))
	for _, entry := range m.messages {
		switch entry.kind {
		case messageThink:
			out = append(out, m.renderThinkEntry(entry))
		case messageAssistant:
			out = append(out, indentLines(m.renderMarkdown(entry.text), "  "))
		default:
			out = append(out, entry.text)
		}
	}
	return out
}

func (m *model) syncViewport() {
	followTail := m.streaming || m.loading || m.viewport.AtBottom()
	m.viewport = setContent(m.viewport, m.renderMessages())
	if followTail {
		m.viewport.GotoBottom()
	}
}

func renderedLineCount(text string) int {
	return strings.Count(text, "\n") + 1
}

func (m *model) syncViewportToLatestThink() {
	rendered := m.renderMessages()
	target := -1
	line := 0
	for i, entry := range m.messages {
		if i >= len(rendered) {
			break
		}
		if entry.kind == messageThink {
			target = line
		}
		line += renderedLineCount(rendered[i])
	}
	m.viewport = setContent(m.viewport, rendered)
	if target >= 0 {
		m.viewport.SetYOffset(target)
	}
}

func renderConfirmBlock(inner confirmMsg, width int) string {
	var b strings.Builder

	b.WriteString(ErrorStyle().Bold(true).Render("⚠  ") + HeaderStyle().Render(inner.title))
	b.WriteString("\n")

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

	for _, d := range inner.details {
		detail := d
		if strings.HasPrefix(d, "path: ") && inner.preview != "" {
			detail = "path: " + inner.preview
		}
		b.WriteString(DimStyle().Render("   " + detail))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	yBtn := ConfirmYStyle().Render("  Y  run  ")
	nBtn := ConfirmNStyle().Render("  N  cancel  ")
	b.WriteString(yBtn + nBtn)

	return b.String()
}

func renderQuestionBlock(q QuestionItem, idx, total, width int) string {
	var b strings.Builder

	header := q.Header
	if total > 1 {
		header = fmt.Sprintf("%s (%d/%d)", q.Header, idx+1, total)
	}
	b.WriteString(AccentStyle().Render("? ") + HeaderStyle().Render(header))
	b.WriteString("\n")
	b.WriteString("  " + q.Question)
	b.WriteString("\n")

	if len(q.Options) > 0 {
		b.WriteString("\n")
		for i, opt := range q.Options {
			num := AccentStyle().Render(fmt.Sprintf("  %d", i+1))
			label := "  " + opt.Label
			if opt.Description != "" {
				sep := DimStyle().Render("  —  ")
				desc := DimStyle().Render(truncateRunes(opt.Description, width-lipgloss.Width(num)-lipgloss.Width(label)-lipgloss.Width(sep)))
				b.WriteString(num + label + sep + desc)
			} else {
				b.WriteString(num + label)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(DimStyle().Render("  press 1–" + fmt.Sprintf("%d", len(q.Options)) + " to select"))
	} else {
		b.WriteString("\n")
		b.WriteString(DimStyle().Render("  type your answer and press enter"))
	}

	return b.String()
}

// renderWaveSpinner renders a smooth bouncing wave that slightly overflows edges.
func (m model) renderWaveSpinner() string {
	const n = 7
	const cycle = (n + 2) * 2 // 18: pos goes -1..n then back
	if !m.loading {
		return FaintStyle().Render(strings.Repeat("·", n))
	}
	s := m.spinner % cycle
	// forward: s=0..n+1, backward: s=n+2..cycle-1
	var pos int
	half := cycle / 2 // n+1 = 8
	if s <= half {
		pos = s - 1 // -1..n
	} else {
		pos = cycle - s - 1 // n-1..0 going back, then -1 again at s=cycle
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		dist := i - pos
		if dist < 0 {
			dist = -dist
		}
		switch dist {
		case 0:
			b.WriteString(lipgloss.NewStyle().Foreground(Green).Render("█"))
		case 1:
			b.WriteString(lipgloss.NewStyle().Foreground(GreenMid).Render("▓"))
		case 2:
			b.WriteString(lipgloss.NewStyle().Foreground(GreenFaint).Render("▒"))
		default:
			b.WriteString(FaintStyle().Render("░"))
		}
	}
	return b.String()
}

// renderStatusBar renders the 2-line bottom status bar.
func (m model) renderStatusBar() string {
	// Line 1: mode · model · provider
	// Visual hierarchy: green(mode) · white(model) · dim(provider)
	mode := AccentStyle().Render("ask")
	if m.autoMode {
		mode = lipgloss.NewStyle().Foreground(Amber).Bold(true).Render("build")
	}
	dot := FaintStyle().Render(" · ")
	provider := m.providerName
	if provider == "" {
		provider = "ollama"
	}
	modelPart := lipgloss.NewStyle().Foreground(White).Render(m.modelName)
	providerPart := DimStyle().Render(provider)
	line1 := mode + dot + modelPart + dot + providerPart

	// Line 2: wave spinner + hint
	hint := "  esc interrupt"
	if m.confirming {
		hint = "  Y run  N cancel"
	}
	line2 := m.renderWaveSpinner() + DimStyle().Render(hint)

	return line1 + "\n" + line2
}

// renderInput renders the bordered text input.
func (m model) renderInput() string {
	borderColor := GreenMid
	if m.streaming || m.confirming {
		borderColor = Faint
	}
	innerWidth := m.width - 4
	if innerWidth < 1 {
		innerWidth = 1
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		PaddingLeft(1).
		Width(innerWidth)
	return style.Render(m.textinput.View())
}

// renderLiveActivity shows the current tool/think activity as a single updating line.
func (m model) renderLiveActivity() string {
	if m.liveActivity == "" {
		return ""
	}
	return m.liveActivity
}

func (m *model) layoutViewportHeight() int {
	if m.height < 1 {
		return 1
	}

	hintHeight := 0
	if m.hintVisible {
		hintHeight = lipgloss.Height(m.renderHint())
	}

	confirmHeight := 0
	if m.confirming {
		confirmHeight = lipgloss.Height(renderConfirmBlock(confirmMsg{
			title:   "Dangerous action",
			tool:    m.confirmTool,
			details: []string{},
		}, m.width))
	}

	liveHeight := 0
	if m.liveActivity != "" {
		liveHeight = 1
	}

	questionHeight := 0
	if m.questioning && len(m.questionsData) > 0 {
		questionHeight = lipgloss.Height(renderQuestionBlock(
			m.questionsData[m.questionIdx], m.questionIdx, len(m.questionsData), m.width,
		))
	}

	// input: 3 (border top + content + border bottom)
	// status bar: 2 lines
	reserved := hintHeight + confirmHeight + liveHeight + questionHeight + 3 + 2
	height := m.height - reserved
	if height < 1 {
		height = 1
	}
	return height
}

func (m *model) updateViewportHeight() {
	m.viewport.Height = m.layoutViewportHeight()
}

func (m model) View() string {
	if m.height == 0 {
		return ""
	}

	layers := []string{m.viewport.View()}

	if m.confirming {
		layers = append(layers, renderConfirmBlock(confirmMsg{
			title:   "Dangerous action",
			tool:    m.confirmTool,
			details: []string{},
		}, m.width))
	}

	if m.questioning && len(m.questionsData) > 0 {
		layers = append(layers, renderQuestionBlock(
			m.questionsData[m.questionIdx], m.questionIdx, len(m.questionsData), m.width,
		))
	}

	if live := m.renderLiveActivity(); live != "" {
		layers = append(layers, live)
	}

	if m.hintVisible {
		layers = append(layers, m.renderHint())
	}

	layers = append(layers, m.renderInput(), m.renderStatusBar())
	return lipgloss.JoinVertical(lipgloss.Left, layers...)
}

func (m model) renderHint() string {
	matches := matchSlash(m.textinput.Value())
	lines := make([]string, 0, len(matches))
	for _, cmd := range matches {
		name := fmt.Sprintf("%-14s", cmd.name)
		lines = append(lines,
			AccentStyle().Render(name)+"  "+DimStyle().Render(cmd.desc),
		)
	}
	return strings.Join(lines, "\n")
}
