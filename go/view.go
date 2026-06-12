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
	diffCtxStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Background(lipgloss.Color("#161616"))
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
	"google_search":      "Web Search",
	"youtube_search":     "Web Search",
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
	ms := d.Round(10 * time.Millisecond).Milliseconds()
	if ms < 10 {
		ms = 10
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	// 1s and up: regular seconds (one decimal under 10s, whole seconds after)
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

func toolPastTense(name string) string {
	switch name {
	case "Web Search":
		return "Searched"
	case "Search":
		return "Searched"
	case "Read":
		return "Read"
	case "Write":
		return "Wrote"
	case "Grep":
		return "Searched"
	case "Glob", "Find":
		return "Found"
	case "Edit":
		return "Edited"
	case "Patch":
		return "Patched"
	case "Browser":
		return "Browsed"
	case "Fetch":
		return "Fetched"
	case "Screenshot":
		return "Screenshot"
	case "Keyboard", "Type":
		return "Typed"
	case "Todo":
		return "Listed"
	case "Shell":
		return "Ran"
	case "Skill":
		return "Used"
	case "Delete":
		return "Deleted"
	case "List":
		return "Listed"
	case "Mouse":
		return "Clicked"
	case "Clipboard":
		return "Copied"
	case "Ask":
		return "Asked"
	}
	return "Ran " + strings.ToLower(name)
}

// toolPresentTenseNames maps display names to present-tense verbs for the live
// action line.
var toolPresentTenseNames = map[string]string{
	"Web Search": "Searching",
	"Search":     "Searching",
	"Find":       "Finding",
	"Read":       "Reading",
	"Fetch":      "Fetching",
	"Open":       "Opening",
	"Screenshot": "Capturing",
	"Write":      "Writing",
	"Delete":     "Deleting",
	"List":       "Listing",
	"Edit":       "Editing",
	"Patch":      "Patching",
	"Shell":      "Running",
	"Mouse":      "Clicking",
	"Type":       "Typing",
	"Clipboard":  "Clipboard",
	"Ask":        "Asking",
	"Skill":      "Loading",
	"Todo":       "Updating todo",
}

func toolPresentTense(display string) string {
	if v, ok := toolPresentTenseNames[display]; ok {
		return v
	}
	return "Running " + strings.ToLower(display)
}

// actionDetail extracts the tool detail for the live action line, stripping any
// trailing ellipsis/dots so URLs render clean (no "…" or running dots).
func actionDetail(display, args string) string {
	return strings.TrimRight(extractToolDetail(display, args), "… ")
}

func extractToolDetail(name, args string) string {
	if args == "" {
		return ""
	}
	// Fetch: extract only the URL
	if name == "Fetch" {
		for _, part := range strings.Split(args, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "url=") {
				v := strings.TrimSpace(part[4:])
				v = strings.Trim(v, `"'`)
				return truncateRunes(v, 60)
			}
		}
		return ""
	}
	// Edit/Patch: show only filename, not old_string/new_string
	if name == "Edit" || name == "Patch" {
		for _, part := range strings.Split(args, ", ") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "file_path=") {
				v := strings.TrimSpace(part[10:])
				v = strings.Trim(v, `"'`)
				return truncateRunes(pathBase(v), 50)
			}
		}
	}
	// default: take the first key=value, strip the key
	parts := strings.SplitN(args, "=", 2)
	if len(parts) == 2 {
		v := strings.TrimSpace(parts[1])
		// drop any subsequent key=value pairs
		if comma := strings.Index(v, ", "); comma >= 0 {
			v = v[:comma]
		}
		v = strings.Trim(v, `"'`)
		if v != "" {
			return truncateRunes(v, 60)
		}
	}
	return truncateRunes(args, 60)
}

// pathBase returns the last component of a file path (handles both / and \).
func pathBase(p string) string {
	p = strings.TrimRight(p, "/\\")
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func (m model) renderToolFlowEntry(idx int, entry messageEntry) string {
	prefix := "  "
	detail := extractToolDetail(entry.toolName, entry.toolArgs)
	if entry.toolDone {
		past := toolPastTense(entry.toolName)
		if detail != "" {
			past += " " + detail
		}
		return DimStyle().Render(prefix + past)
	}
	if detail != "" {
		detail = " -> " + detail
	}
	// Only the currently running tool gets the animated ellipsis
	if idx == m.runningToolIdx {
		dots := []string{".", "..", "..."}
		dotIdx := ((m.spinner / 3) + idx) % 3
		return DimStyle().Render(prefix + entry.toolName + detail + dots[dotIdx])
	}
	return DimStyle().Render(prefix + entry.toolName + detail)
}

// renderChainCounter renders the grouped tool tally: "Searched —2 · Fetched —6 · 18s".
// While live the time ticks; once duration is set the line is frozen.
func (m model) renderChainCounter(entry messageEntry) string {
	if len(m.chainOrder) == 0 {
		return ""
	}
	parts := make([]string, 0, len(m.chainOrder)+1)
	for _, name := range m.chainOrder {
		label := toolPastTense(name)
		if c := m.chainCounts[name]; c > 1 {
			label += fmt.Sprintf(" —%d", c)
		}
		parts = append(parts, label)
	}
	dur := entry.duration
	if dur == 0 {
		dur = time.Since(m.chainStart)
	}
	parts = append(parts, formatThinkDuration(dur))
	return DimStyle().Render("  " + strings.Join(parts, " · "))
}

// renderChainAction renders the single live "what's happening now" line.
func (m model) renderChainAction(entry messageEntry) string {
	if !m.loading {
		return ""
	}
	if entry.thinking {
		dots := []string{".", "..", "..."}
		return DimStyle().Render("  " + entry.activity + dots[(m.spinner/3)%3])
	}
	line := entry.activity
	if entry.toolArgs != "" {
		line += " " + entry.toolArgs
	}
	return DimStyle().Render("  " + line)
}

func (m model) renderThinkEntry(entry messageEntry, active bool) string {
	duration := entry.duration
	if duration <= 0 && !entry.startedAt.IsZero() {
		duration = time.Since(entry.startedAt)
	}

	// only the active (latest) think animates; finished or orphaned thinks freeze
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

func (m *model) renderMessages() []string {
	m.ensureMarkdownRenderer()
	// index of the latest think entry — only it may animate
	lastThinkIdx := -1
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].kind == messageThink {
			lastThinkIdx = i
			break
		}
	}
	out := make([]string, 0, len(m.messages))
	for i, entry := range m.messages {
		var rendered string
		switch entry.kind {
		case messageUser:
			rendered = m.renderUserMsg(entry.text)
		case messageWarden:
			// label removed, skip
		case messageThink:
			rendered = m.renderThinkEntry(entry, i == lastThinkIdx)
		case messageAssistant:
			rendered = indentLines(m.renderMarkdown(entry.text), "  ")
		case messageToolActivity:
			rendered = entry.text
		case messageToolFlow:
			rendered = m.renderToolFlowEntry(i, entry)
		case messageChainCounter:
			rendered = m.renderChainCounter(entry)
		case messageChainAction:
			rendered = m.renderChainAction(entry)
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
	b.WriteString("  " + AccentStyle().Render("¶") + "  " + ToolStyle().Bold(true).Render(inner.tool))
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
		return FaintStyle().Render(strings.Repeat("·", n))
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
		peak = Blue
		mid = BlueMid
		faint = BlueFaint
	}
	for i := 0; i < n; i++ {
		dist := i - pos
		if dist < 0 {
			dist = -dist
		}
		switch {
		case dist == 0:
			b.WriteString(lipgloss.NewStyle().Foreground(peak).Render("â–ˆ"))
		case dist == 1:
			b.WriteString(lipgloss.NewStyle().Foreground(mid).Render("—"))
		case dist == 2:
			b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("–"))
		case dist == 3:
			b.WriteString(FaintStyle().Render("·"))
		default:
			b.WriteString(FaintStyle().Render("·"))
		}
	}
	return b.String()
}

// renderFullWave renders a full-terminal-width bouncing wave under the prompt bar.
// Uses a fixed 60-tick cycle (~4.2s at 70ms/tick) regardless of terminal width.
func (m model) renderFullWave() string {
	n := m.width
	if n < 1 {
		n = 1
	}
	if !m.loading {
		return FaintStyle().Render(strings.Repeat("·", n))
	}
	const virtualSpan = 30
	const cycle = virtualSpan * 2
	s := m.spinner % cycle
	var t float64
	if s < virtualSpan {
		t = float64(s) / float64(virtualSpan)
	} else {
		t = float64(cycle-s) / float64(virtualSpan)
	}
	// map to screen pos with slight edge overflow for soft bounce
	pos := int(t*float64(n+3)) - 2
	peak := Green
	mid := GreenMid
	faint := GreenFaint
	if m.autoMode {
		peak = Blue
		mid = BlueMid
		faint = BlueFaint
	}
	// glow radius scales with terminal width
	halo := n / 6
	if halo < 4 {
		halo = 4
	}
	h1 := halo / 3
	h2 := halo * 2 / 3
	var b strings.Builder
	for i := 0; i < n; i++ {
		dist := i - pos
		if dist < 0 {
			dist = -dist
		}
		switch {
		case dist == 0:
			b.WriteString(lipgloss.NewStyle().Foreground(peak).Render("·"))
		case dist <= h1:
			b.WriteString(lipgloss.NewStyle().Foreground(mid).Render("·"))
		case dist <= h2:
			b.WriteString(lipgloss.NewStyle().Foreground(faint).Render("·"))
		case dist <= halo:
			b.WriteString(FaintStyle().Render("·"))
		default:
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render("·"))
		}
	}
	return b.String()
}

// renderStatusBar renders the single-line bottom status bar: mode · model · hint [tokens].
func (m model) renderStatusBar() string {
	mode := AccentStyle().Render("Ask")
	if m.autoMode {
		mode = lipgloss.NewStyle().Foreground(Blue).Bold(true).Render("Auto")
	}
	dot := FaintStyle().Render(" · ")
	modelPart := lipgloss.NewStyle().Foreground(White).Render(m.modelName)

	var hint string
	switch {
	case m.escPending:
		hint = ErrorStyle().Render("Esc") + DimStyle().Render(" cancel · ctrl+c quit")
	case m.quitPending:
		hint = ErrorStyle().Render("ctrl+c") + DimStyle().Render(" quit · any key abort")
	case m.selectMode:
		keyColor := Blue
		if !m.autoMode {
			keyColor = Green
		}
		hint = DimStyle().Render("select mode · ") + lipgloss.NewStyle().Foreground(keyColor).Bold(true).Render("Esc") + DimStyle().Render(" exit")
	case m.confirming:
		hint = DimStyle().Render("Y run  N cancel")
	case m.streaming:
		keyColor := Blue
		if !m.autoMode {
			keyColor = Green
		}
		hint = lipgloss.NewStyle().Foreground(keyColor).Bold(true).Render("Esc") + DimStyle().Render(" interrupt")
	default:
		keyColor := Blue
		if !m.autoMode {
			keyColor = Green
		}
		key := lipgloss.NewStyle().Foreground(keyColor).Bold(true).Render("Shift Tab")
		if m.autoMode {
			hint = key + DimStyle().Render("  to Ask mode")
		} else {
			hint = key + DimStyle().Render("  to Auto mode")
		}
	}

	left := mode + dot + modelPart + dot + hint

	if m.tokenLimit > 0 && m.tokenCount > 0 {
		pct := m.tokenCount * 100 / m.tokenLimit
		k := float64(m.tokenCount) / 1000.0
		tokenStr := DimStyle().Render(fmt.Sprintf("%.1fK (%d%%)", k, pct))
		leftWidth := lipgloss.Width(left)
		tokenWidth := lipgloss.Width(tokenStr)
		padding := m.width - leftWidth - tokenWidth
		if padding > 1 {
			return left + strings.Repeat(" ", padding) + tokenStr
		}
	}
	return left
}

// renderInput renders the bordered text input.
func (m model) renderInput() string {
	borderColor := Green
	if m.autoMode {
		borderColor = BlueMid
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

	// input: border top + N content lines + border bottom
	// status bar: 2 lines
	inputHeight := m.inputLineCount() + 2
	reserved := hintHeight + confirmHeight + questionHeight + modelPickerHeight + cwHeight + inputHeight + 2
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

	layers = append(layers, m.renderFullWave(), m.renderInput(), m.renderStatusBar())
	return lipgloss.JoinVertical(lipgloss.Left, layers...)
}

func (m model) renderConnectWizard() string {
	acc := WardenStyleAuto(m.autoMode)
	dim := DimStyle()
	err := ErrorStyle()
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(Green)
	if m.autoMode {
		keyStyle = lipgloss.NewStyle().Bold(true).Foreground(Blue)
	}
	key := func(s string) string { return keyStyle.Render(s) }

	var lines []string

	if m.cwErr != "" {
		lines = append(lines, "  "+err.Render("•  "+m.cwErr))
		lines = append(lines, "  "+dim.Render(key("esc")+" dismiss"))
		return strings.Join(lines, "\n")
	}

	if m.cwLoading {
		lines = append(lines, "  "+dim.Render("connecting..."))
		return strings.Join(lines, "\n")
	}

	switch m.cwStep {
	case 0:
		lines = append(lines, "  "+acc.Render("connect"), "")
		providers := []string{"openrouter", "ollama"}
		for i, p := range providers {
			if i == m.cwPickIdx {
				lines = append(lines, "  "+acc.Render("→ "+p))
			} else {
				lines = append(lines, "  "+dim.Render("  "+p))
			}
		}
		lines = append(lines, "")
		hint := key("←→") + dim.Render(" navigate   ") + key("Enter") + dim.Render(" select   ") + key("Esc") + dim.Render(" cancel")
		lines = append(lines, "  "+hint)

	case 1:
		lines = append(lines, "  "+acc.Render("api key"), "")
		lines = append(lines, "  "+m.cwInput.View())
		lines = append(lines, "  "+dim.Render("get one at openrouter.ai/keys"))
		lines = append(lines, "")
		hint := key("Enter") + dim.Render(" confirm   ") + key("Esc") + dim.Render(" back")
		lines = append(lines, "  "+hint)

	case 2:
		lines = append(lines, "  "+acc.Render("model"), "")
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
					lines = append(lines, "  "+acc.Render("→ "+name))
				} else {
					lines = append(lines, "  "+dim.Render("  "+name))
				}
			}
			lines = append(lines, "")
			hint := key("←→") + dim.Render(" navigate   ") + key("Enter") + dim.Render(" select   ") + key("Esc") + dim.Render(" back")
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
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(Green)
	if autoMode {
		keyStyle = lipgloss.NewStyle().Bold(true).Foreground(Blue)
	}
	key := func(s string) string { return keyStyle.Render(s) }
	hint := key("←→") + DimStyle().Render(" navigate   ") +
		key("Enter") + DimStyle().Render(" select   ") +
		key("Esc") + DimStyle().Render(" cancel")
	lines = append(lines, "  "+hint)
	lines = append(lines, "")

	for i := start; i < end; i++ {
		name := filtered[i]
		if i == idx {
			lines = append(lines, accent.Render("  → "+name))
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
		if len(matches) == 0 {
			return ""
		}
		selected := m.slashIdx
		if selected < 0 || selected >= len(matches) {
			selected = 0
		}
		const maxVisible = 5
		start := 0
		if len(matches) > maxVisible {
			start = selected - maxVisible/2
			if start < 0 {
				start = 0
			}
			if start > len(matches)-maxVisible {
				start = len(matches) - maxVisible
			}
		}
		end := start + maxVisible
		if end > len(matches) {
			end = len(matches)
		}
		lines := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			cmd := matches[i]
			active := i == selected
			name := fmt.Sprintf("/%-13s", cmd.name[1:])
			nameStyle := SlashNameStyle(active, m.autoMode)
			descStyle := SlashDescStyle(active)
			descLimit := m.width - lipgloss.Width(name) - 6
			if active {
				descLimit = m.width - lipgloss.Width(name) - 4
			}
			if descLimit < 0 {
				descLimit = 0
			}
			if active {
				lines = append(lines,
					"  "+accent.Render(">")+" "+nameStyle.Render(name)+"  "+descStyle.Render(truncateRunes(cmd.desc, descLimit)),
				)
				continue
			}
			lines = append(lines,
				"    "+nameStyle.Render(name)+"  "+descStyle.Render(truncateRunes(cmd.desc, descLimit)),
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
