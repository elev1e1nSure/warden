package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var diffStatsRe = regexp.MustCompile(`(\+\d+)\s+(-\d+)$`)

var (
	diffAddStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D47A")).Background(lipgloss.Color("#0d1f0d"))
	diffRemoveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff4444")).Background(lipgloss.Color("#1f0d0d"))
	diffHunkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#569CD6"))
	diffFileStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Bold(true)
	diffCtxStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
)

func renderUnifiedDiff(diff string) string {
	lines := strings.Split(strings.TrimRight(diff, "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			out = append(out, diffFileStyle.Render("  "+line))
		case strings.HasPrefix(line, "@@"):
			out = append(out, diffHunkStyle.Render("  "+line))
		case strings.HasPrefix(line, "+"):
			out = append(out, diffAddStyle.Render("  "+line))
		case strings.HasPrefix(line, "-"):
			out = append(out, diffRemoveStyle.Render("  "+line))
		default:
			out = append(out, diffCtxStyle.Render("  "+line))
		}
	}
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

const wardenVersion = "v0.1.0"

var toolDisplayNames = map[string]string{
	"google_search":      "Search",
	"youtube_search":     "Search",
	"grep":               "Search",
	"glob":               "Find",
	"browser_read":       "Read",
	"file_read":          "Read",
	"webfetch":           "Fetch",
	"browser_open":       "Open",
	"browser_screenshot": "Screenshot",
	"screenshot":         "Screenshot",
	"file_write":         "Write",
	"file_delete":        "Delete",
	"file_list":          "List",
	"edit":               "Edit",
	"apply_patch":        "Patch",
	"powershell":         "Shell",
	"bash":               "Shell",
	"mouse":              "Mouse",
	"keyboard":           "Type",
	"clipboard":          "Clipboard",
	"question":           "Ask",
	"skill":              "Skill",
	"todowrite":          "Todo",
}

func toolDisplayName(name string) string {
	if d, ok := toolDisplayNames[name]; ok {
		return d
	}
	if len(name) > 0 {
		return strings.ToUpper(name[:1]) + name[1:]
	}
	return name
}

func truncateRunes(text string, limit int) string {
	if limit < 1 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-1]) + "…"
}

func toolResultIsError(result string) bool {
	lower := strings.ToLower(strings.TrimSpace(result))
	// Check for "error:" or "error " with word boundary to avoid false positives like "error123"
	return strings.HasPrefix(lower, "error:") ||
		strings.HasPrefix(lower, "error ") ||
		strings.HasPrefix(lower, "stderr")
}

func toolSummaryLine(name, args, result string) string {
	result = strings.TrimSpace(result)
	if result == "" {
		result = "(empty)"
	}
	isErr := toolResultIsError(result)
	arrow := ToolStyle().Render("  → ")
	display := toolDisplayName(name)

	// Shell tools: show the command, append result only when it has content.
	if (name == "powershell" || name == "bash") && args != "" {
		cmd := truncateRunes(strings.TrimSpace(args), 80)
		var nameRender string
		if isErr {
			nameRender = ErrorStyle().Render(display)
		} else {
			nameRender = ToolStyle().Render(display)
		}
		line := arrow + nameRender + "  " + DimStyle().Render(cmd)
		if result != "(no output)" && result != "(empty)" {
			rlines := strings.Split(result, "\n")
			head := strings.TrimSpace(rlines[0])
			if len(rlines) > 1 {
				head += fmt.Sprintf("  +%d", len(rlines)-1)
			}
			head = truncateRunes(head, 60)
			if isErr {
				line += "  " + ErrorStyle().Render(head)
			} else {
				line += "  " + DimStyle().Render(head)
			}
		}
		return line
	}

	lines := strings.Split(result, "\n")
	head := strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		head += fmt.Sprintf("  +%d lines", len(lines)-1)
	}
	head = truncateRunes(head, 100)

	if isErr {
		return arrow + ErrorStyle().Render(display) + "  " + ErrorStyle().Render(head)
	}

	text, diff := renderDiffStats(head)
	nameRender := ToolStyle().Render(display)
	if diff != "" {
		return arrow + nameRender + "  " + DimStyle().Render(text) + "  " + diff
	}
	return arrow + nameRender + "  " + DimStyle().Render(head)
}

var toolActivityVerbs = map[string]string{
	"google_search":      "searching",
	"youtube_search":     "searching",
	"grep":               "searching",
	"glob":               "searching files",
	"file_read":          "reading",
	"browser_read":       "reading",
	"webfetch":           "fetching",
	"browser_open":       "opening",
	"browser_screenshot": "screenshotting",
	"screenshot":         "screenshotting",
	"file_write":         "writing",
	"edit":               "editing",
	"apply_patch":        "patching",
	"powershell":         "running",
	"bash":               "running",
	"mouse":              "clicking",
	"keyboard":           "typing",
	"clipboard":          "clipboard",
	"file_delete":        "deleting",
	"file_list":          "listing",
}

func toolActivityLine(name string) string {
	verb, ok := toolActivityVerbs[name]
	if !ok {
		verb = "working"
	}
	return DimStyle().Render("  " + verb + "...")
}

func toolStartLine(name, args string) string {
	arrow := ToolStyle().Render("  → ")
	display := ToolStyle().Render(toolDisplayName(name))
	if args == "" {
		return arrow + display
	}
	return arrow + display + "  " + DimStyle().Render(truncateRunes(args, 140))
}

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

	if !m.verboseMode {
		if entry.duration > 0 {
			return ""
		}
		if m.loading {
			frame := brailleFrames[m.spinner%len(brailleFrames)]
			verb := "Thinking"
			if entry.activity != "" {
				verb = entry.activity
			}
			return DimStyle().Render("  " + frame + "  " + verb + "...")
		}
		return ""
	}

	var summary string
	if entry.duration == 0 && m.loading {
		frame := brailleFrames[m.spinner%len(brailleFrames)]
		summary = DimStyle().Render("  " + frame + "  Thinking")
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

func (m *model) renderMessages() []string {
	m.ensureMarkdownRenderer()
	out := make([]string, 0, len(m.messages))
	for _, entry := range m.messages {
		var rendered string
		switch entry.kind {
		case messageThink:
			rendered = m.renderThinkEntry(entry)
		case messageAssistant:
			rendered = WardenBgStyle().Render(indentLines(m.renderMarkdown(entry.text), "  "))
		case messageToolActivity:
			rendered = entry.text
		case messageToolDiff:
			rendered = renderUnifiedDiff(entry.text)
		default:
			rendered = entry.text
		}
		// always keep messageText (blank lines serve as turn separators)
		if rendered != "" || entry.kind == messageText {
			out = append(out, rendered)
		}
	}
	return out
}

func (m *model) syncViewport() {
	followTail := !m.userScrolled && (m.streaming || m.loading || m.viewport.AtBottom())
	m.viewport = setContent(m.viewport, m.renderMessages())
	if followTail {
		m.viewport.GotoBottom()
	}
}

func renderConfirmBlock(inner confirmMsg, width int) string {
	var b strings.Builder

	// Tool name line — same style as chat tool lines
	b.WriteString("  " + AccentStyle().Render("▶") + "  " + ToolStyle().Bold(true).Render(inner.tool))
	b.WriteString("\n")

	// Command / path preview — split into lines, show up to 4, dim + indented
	if inner.preview != "" {
		limit := width - 6
		if limit < 10 {
			limit = 10
		}
		shown := 0
		for _, line := range strings.Split(inner.preview, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			b.WriteString(DimStyle().Render("    " + truncateRunes(line, limit)))
			b.WriteString("\n")
			shown++
			if shown >= 4 {
				break
			}
		}
	}

	// Details / reason
	details := inner.details
	if len(details) == 0 && inner.summary != "" {
		details = []string{inner.summary}
	}
	if len(details) > 0 {
		b.WriteString("\n")
		for _, d := range details {
			b.WriteString(DimStyle().Render("  " + d))
			b.WriteString("\n")
		}
	}

	// Buttons
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

// renderWaveSpinner renders a smooth bouncing wave with pulsing background dots.
// Triangle-wave pos: -2..8 (overflows both edges for soft bounce).
// Background dots (outside wave tail) slowly pulse for a "breathing" effect.
func (m model) renderWaveSpinner() string {
	const n = 7
	const lo = -2
	const hi = n + 1       // =8
	const span = hi - lo   // 10
	const cycle = span * 2 // 20
	if !m.loading {
		return FaintStyle().Render(strings.Repeat("░", n))
	}
	s := m.spinner % cycle
	var pos int
	if s < span {
		pos = lo + s
	} else {
		pos = hi - (s - span)
	}
	var b strings.Builder
	peak := Green
	mid := GreenMid
	faint := GreenFaint
	if m.autoMode {
		peak = Amber
		mid = AmberMid
		faint = AmberFaint
	}
	for i := 0; i < n; i++ {
		dist := i - pos
		if dist < 0 {
			dist = -dist
		}
		switch {
		case dist == 0:
			b.WriteString(lipgloss.NewStyle().Foreground(peak).Render("█"))
		case dist == 1:
			b.WriteString(lipgloss.NewStyle().Foreground(mid).Render("▓"))
		case dist == 2:
			b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("▒"))
		case dist == 3:
			b.WriteString(FaintStyle().Render("░"))
		default:
			b.WriteString(FaintStyle().Render("░"))
		}
	}
	return b.String()
}

// renderStatusBar renders the 2-line bottom status bar.
func (m model) renderStatusBar() string {
	mode := AccentStyle().Render("Ask")
	if m.autoMode {
		mode = lipgloss.NewStyle().Foreground(Amber).Bold(true).Render("Auto")
	}
	dot := FaintStyle().Render(" · ")
	modelPart := lipgloss.NewStyle().Foreground(White).Render(m.modelName)
	left := mode + dot + modelPart

	line1 := left
	if m.tokenLimit > 0 && m.tokenCount > 0 {
		pct := m.tokenCount * 100 / m.tokenLimit
		k := float64(m.tokenCount) / 1000.0
		tokenStr := DimStyle().Render(fmt.Sprintf("%.1fK (%d%%)", k, pct))
		leftWidth := lipgloss.Width(left)
		tokenWidth := lipgloss.Width(tokenStr)
		padding := m.width - leftWidth - tokenWidth
		if padding > 1 {
			line1 = left + strings.Repeat(" ", padding) + tokenStr
		}
	}

	// Line 2: confirmation or wave spinner + hint
	if m.escPending {
		return line1 + "\n" + ErrorStyle().Render("  Esc") + DimStyle().Render(" cancel · ") + DimStyle().Render("ctrl+c quit")
	}
	if m.quitPending {
		return line1 + "\n" + ErrorStyle().Render("  ctrl+c") + DimStyle().Render(" quit · ") + DimStyle().Render("any key abort")
	}
	if m.selectMode {
		line2 := m.renderWaveSpinner() + DimStyle().Render("  select mode · ") + lipgloss.NewStyle().Foreground(Amber).Bold(true).Render("Esc") + DimStyle().Render(" to exit")
		return line1 + "\n" + line2
	}
	var line2suffix string
	switch {
	case m.confirming:
		line2suffix = DimStyle().Render("  Y run  N cancel")
	case m.streaming:
		keyColor := Amber
		if !m.autoMode {
			keyColor = Green
		}
		line2suffix = "  " + lipgloss.NewStyle().Foreground(keyColor).Bold(true).Render("Esc") + DimStyle().Render(" interrupt")
	default:
		keyColor := Amber
		if !m.autoMode {
			keyColor = Green
		}
		key := lipgloss.NewStyle().Foreground(keyColor).Bold(true).Render("Shift Tab")
		if m.autoMode {
			line2suffix = "  " + key + DimStyle().Render("  to Ask mode")
		} else {
			line2suffix = "  " + key + DimStyle().Render("  to Auto mode")
		}
	}
	line2 := m.renderWaveSpinner() + line2suffix

	return line1 + "\n" + line2
}

// renderInput renders the bordered text input.
func (m model) renderInput() string {
	borderColor := GreenMid
	if m.autoMode {
		borderColor = AmberMid
	}
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

	questionHeight := 0
	if m.questioning && len(m.questionsData) > 0 {
		questionHeight = lipgloss.Height(renderQuestionBlock(
			m.questionsData[m.questionIdx], m.questionIdx, len(m.questionsData), m.width,
		))
	}

	modelPickerHeight := 0
	if m.modelPicking {
		modelPickerHeight = lipgloss.Height(renderModelPicker(m.modelFiltered, m.modelPickIdx, m.modelScrollTop, m.autoMode))
	}

	cwHeight := 0
	if m.cwOpen {
		cwHeight = lipgloss.Height(m.renderConnectWizard())
	}

	// input: 3 (border top + content + border bottom)
	// status bar: 2 lines
	reserved := hintHeight + confirmHeight + questionHeight + modelPickerHeight + cwHeight + 3 + 2
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
			title:   m.confirmTitle,
			tool:    m.confirmTool,
			risk:    m.confirmRisk,
			summary: m.confirmSummary,
			details: m.confirmDetails,
			preview: m.confirmPreview,
		}, m.width))
	}

	if m.questioning && len(m.questionsData) > 0 {
		layers = append(layers, renderQuestionBlock(
			m.questionsData[m.questionIdx], m.questionIdx, len(m.questionsData), m.width,
		))
	}

	if m.modelPicking {
		layers = append(layers, renderModelPicker(m.modelFiltered, m.modelPickIdx, m.modelScrollTop, m.autoMode))
	}

	if m.cwOpen {
		layers = append(layers, m.renderConnectWizard())
	}

	if m.hintVisible {
		layers = append(layers, m.renderHint())
	}

	layers = append(layers, m.renderInput(), m.renderStatusBar())
	return lipgloss.JoinVertical(lipgloss.Left, layers...)
}

var pickerKeyStyle = lipgloss.NewStyle().Foreground(Amber).Bold(true)
var pickerTabActive = lipgloss.NewStyle().Foreground(Amber).Bold(true)

func (m model) renderConnectWizard() string {
	acc := AccentStyle()
	dim := DimStyle()
	err := ErrorStyle()
	bold := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	key := func(s string) string { return pickerKeyStyle.Render(s) }

	var lines []string

	if m.cwErr != "" {
		lines = append(lines, "  "+err.Render("✕  "+m.cwErr))
		lines = append(lines, "  "+dim.Render(key("esc")+" dismiss"))
		return strings.Join(lines, "\n")
	}

	if m.cwLoading {
		lines = append(lines, "  "+dim.Render("connecting..."))
		return strings.Join(lines, "\n")
	}

	switch m.cwStep {
	case 0:
		lines = append(lines, "  "+bold.Render("connect"), "")
		providers := []string{"openrouter", "ollama"}
		for i, p := range providers {
			if i == m.cwPickIdx {
				lines = append(lines, "  "+acc.Render("› "+p))
			} else {
				lines = append(lines, "  "+dim.Render("  "+p))
			}
		}
		lines = append(lines, "")
		hint := key("↑↓") + dim.Render(" navigate   ") + key("Enter") + dim.Render(" select   ") + key("Esc") + dim.Render(" cancel")
		lines = append(lines, "  "+hint)

	case 1:
		lines = append(lines, "  "+bold.Render("api key"), "")
		lines = append(lines, "  "+m.cwInput.View())
		lines = append(lines, "  "+dim.Render("get one at openrouter.ai/keys"))
		lines = append(lines, "")
		hint := key("Enter") + dim.Render(" confirm   ") + key("Esc") + dim.Render(" back")
		lines = append(lines, "  "+hint)

	case 2:
		lines = append(lines, "  "+bold.Render("model"), "")
		if m.cwCustom {
			lines = append(lines, "  "+m.cwInput.View())
			lines = append(lines, "")
			hint := key("Enter") + dim.Render(" confirm   ") + key("Esc") + dim.Render(" back")
			lines = append(lines, "  "+hint)
		} else {
			const maxVis = 7
			start := m.cwScroll
			end := start + maxVis
			if end > len(m.cwModels) {
				end = len(m.cwModels)
			}
			for i := start; i < end; i++ {
				name := m.cwModels[i]
				if i == m.cwPickIdx {
					lines = append(lines, "  "+acc.Render("› "+name))
				} else {
					lines = append(lines, "  "+dim.Render("  "+name))
				}
			}
			lines = append(lines, "")
			hint := key("↑↓") + dim.Render(" navigate   ") + key("Enter") + dim.Render(" select   ") + key("Esc") + dim.Render(" back")
			lines = append(lines, "  "+hint)
		}
	}

	return strings.Join(lines, "\n")
}

func renderModelPicker(filtered []string, idx, scrollTop int, autoMode bool) string {
	const maxVisible = 8
	start := scrollTop
	end := start + maxVisible
	if end > len(filtered) {
		end = len(filtered)
	}
	lines := make([]string, 0, maxVisible+4)

	accent := WardenStyleAuto(autoMode)
	key := func(s string) string { return pickerKeyStyle.Render(s) }
	hint := key("↑↓") + DimStyle().Render(" navigate   ") +
		key("Enter") + DimStyle().Render(" select   ") +
		key("Esc") + DimStyle().Render(" cancel")
	lines = append(lines, "  "+hint)
	lines = append(lines, "")

	for i := start; i < end; i++ {
		name := filtered[i]
		if i == idx {
			lines = append(lines, accent.Render("  › "+name))
		} else {
			lines = append(lines, DimStyle().Render("    "+name))
		}
	}

	return strings.Join(lines, "\n")
}

func (m model) renderHint() string {
	val := m.textinput.Value()
	accent := WardenStyleAuto(m.autoMode)
	if strings.HasPrefix(val, "/") {
		matches := matchSlash(val)
		lines := make([]string, 0, len(matches))
		for _, cmd := range matches {
			name := fmt.Sprintf("/%-13s", cmd.name[1:])
			lines = append(lines,
				accent.Render(name)+"  "+DimStyle().Render(cmd.desc),
			)
		}
		return strings.Join(lines, "\n")
	}
	if strings.HasPrefix(val, "!") {
		skills := matchBang(val, m.skills)
		lines := make([]string, 0, len(skills))
		for _, s := range skills {
			name := fmt.Sprintf("!%-13s", s.Name)
			lines = append(lines,
				accent.Render(name)+"  "+DimStyle().Render(s.Description),
			)
		}
		return strings.Join(lines, "\n")
	}
	return ""
}
