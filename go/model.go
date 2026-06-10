package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	viewport  viewport.Model
	textinput textinput.Model
	client    *Client
	messages  []string
	streaming bool
	height    int
	width     int
	loading   bool
	spinner   int
	thinkBuf  string
	thinkDone bool
	wardenTS  string
	// tool execution
	toolRunning bool
	// confirmation
	confirming bool
	confirmID  string
	confirmCh  <-chan tea.Msg
	// mode
	autoMode    bool
	hintVisible bool
	hintCount   int
	// status
	thinkingEnabled bool
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 0
	ti.Width = 80
	ti.Focus()
	ti.BlinkSpeed = 0

	vp := viewport.New(80, 20)
	vp.SetContent("")

	return model{
		textinput:       ti,
		viewport:        vp,
		client:          NewClient("http://localhost:8765"),
		messages:        []string{},
		thinkingEnabled: true,
	}
}

func (m model) Init() tea.Cmd {
	return m.checkBackend()
}

// ── slash commands ──────────────────────────────────────────────────────────

type slashCmd struct {
	name string
	desc string
}

var slashCommands = []slashCmd{
	{"/auto", "Авторежим — опасные команды без подтверждения"},
	{"/safe", "Безопасный режим — подтверждение на опасные команды"},
	{"/reset", "Сбросить историю сессии"},
	{"/thinking", "Включить/выключить размышления модели"},
}

func matchSlash(prefix string) []slashCmd {
	if len(prefix) == 0 || prefix[0] != '/' {
		return nil
	}
	var out []slashCmd
	lower := strings.ToLower(prefix)
	for _, cmd := range slashCommands {
		if strings.HasPrefix(cmd.name, lower) {
			out = append(out, cmd)
		}
	}
	return out
}

func slashCommonPrefix(matches []slashCmd) string {
	if len(matches) == 0 {
		return ""
	}
	prefix := matches[0].name
	for _, m := range matches[1:] {
		for !strings.HasPrefix(m.name, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

var presenceRng = rand.New(rand.NewSource(time.Now().UnixNano()))

var wardenPresencePhrases = []string{
	"тут",
	"на месте",
	"в деле",
	"я здесь",
	"рядом",
	"на связи",
	"живой",
	"на посту",
	"в строю",
	"на дежурстве",
	"здесь",
	"под рукой",
	"внутри",
	"на линии",
	"в работе",
	"поблизости",
	"не ушёл",
	"включён",
	"на чеку",
	"смотрю",
	"держу курс",
	"держу ход",
	"на точке",
	"тут как тут",
	"в зоне",
	"в сети",
	"здесь же",
	"не сплю",
	"наготове",
	"в порядке",
	"спокойно",
	"в тени",
	"на ковре",
	"подхвачу",
	"на подхвате",
	"не дергай",
	"слушаю",
	"держусь",
	"ещё тут",
	"не сдвинулся",
	"стоим",
	"ожидаю",
	"внимателен",
	"на страже",
	"с тобой",
	"у штурвала",
	"в курсе",
	"рядом стою",
	"тут и есть",
	"живой тут",
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
	return strings.HasPrefix(lower, "ошибка") ||
		strings.HasPrefix(lower, "error") ||
		strings.HasPrefix(lower, "stderr")
}

func toolSummaryLine(name string, result string) string {
	result = strings.TrimSpace(result)
	if result == "" {
		result = "(пусто)"
	}
	lines := strings.Split(result, "\n")
	head := strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		head += fmt.Sprintf(" · +%d строк", len(lines)-1)
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
		return DimStyle().Render("  (пусто)")
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
		lines = append(lines, fmt.Sprintf("  … +%d строк", hidden))
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

// ── helpers ──────────────────────────────────────────────────────────────────

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

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 4 - m.hintCount
		m.textinput.Width = msg.Width
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyTab:
			if !m.streaming {
				val := m.textinput.Value()
				matches := matchSlash(val)
				if len(matches) == 1 {
					m.textinput.SetValue(matches[0].name)
					m.textinput.CursorEnd()
				} else if len(matches) > 1 {
					m.textinput.SetValue(slashCommonPrefix(matches))
					m.textinput.CursorEnd()
				}
			}

		case tea.KeyEsc:
			if m.confirming {
				m.confirming = false
				ch := m.confirmCh
				id := m.confirmID
				m.confirmID = ""
				m.confirmCh = nil
				m.textinput.Placeholder = ""
				m.textinput.Reset()
				return m, tea.Batch(m.sendConfirm(id, false), readNext(ch))
			}
			m.textinput.Reset()

		case tea.KeyEnter:
			if m.confirming {
				val := strings.TrimSpace(m.textinput.Value())
				ok := val == "y" || val == "Y" || val == "yes"
				ch := m.confirmCh
				id := m.confirmID
				m.confirming = false
				m.confirmID = ""
				m.confirmCh = nil
				m.textinput.Placeholder = ""
				m.textinput.Reset()
				return m, tea.Batch(m.sendConfirm(id, ok), readNext(ch))
			}
			if m.streaming {
				return m, nil
			}
			text := strings.TrimSpace(m.textinput.Value())
			if text == "" {
				return m, nil
			}
			if handled, cmd := m.handleSlash(text); handled {
				m.textinput.Reset()
				return m, cmd
			}
			ts := DimStyle().Render("[" + time.Now().Format("15:04") + "]")
			m.messages = append(m.messages, ts+"  "+UserStyle().Render("you:")+"  "+text)
			m.textinput.Reset()
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			m.streaming = true
			m.loading = true
			m.spinner = 0
			return m, tea.Batch(m.sendMessage(text), m.tick())
		}

	case startStreamMsg:
		cmds = append(cmds, readNext(msg.ch))

	case nextMsg:
		switch inner := msg.inner.(type) {
		case wardenStartMsg:
			m.wardenTS = time.Now().Format("15:04")
			m.thinkBuf = ""
			m.thinkDone = false
			m.toolRunning = false
			m.messages = append(m.messages, m.wardenLine(""))
			m.viewport = setContent(m.viewport, m.messages)
			cmds = append(cmds, readNext(msg.ch))

		case thinkMsg:
			m.thinkBuf += inner.text
			m.messages[len(m.messages)-1] = m.renderThinkLine()
			m.viewport = setContent(m.viewport, m.messages)
			cmds = append(cmds, readNext(msg.ch))

		case tokenMsg:
			if !m.thinkDone {
				m.clearThinkLine()
				m.messages = append(m.messages, m.wardenLine(""))
				m.thinkDone = true
				m.thinkBuf = ""
			}
			if len(m.messages) > 0 {
				m.messages[len(m.messages)-1] += inner.text
				m.viewport = setContent(m.viewport, m.messages)
			}
			cmds = append(cmds, readNext(msg.ch))

		case toolStartMsg:
			m.toolRunning = true
			if !m.thinkDone && len(m.messages) > 0 {
				m.clearThinkLine()
				m.thinkDone = true
				m.thinkBuf = ""
			}
			m.messages = append(m.messages,
				toolStartLine(inner.name, inner.args),
				toolPendingLine(),
			)
			m.viewport = setContent(m.viewport, m.messages)
			cmds = append(cmds, readNext(msg.ch))

		case toolMsg:
			m.toolRunning = false
			sticky := stickyTool(inner.tool.Name)
			if len(m.messages) > 0 && m.messages[len(m.messages)-1] == toolPendingLine() {
				if sticky {
					m.messages[len(m.messages)-1] = toolResultBlock(inner.tool.Result)
				} else {
					m.messages = m.messages[:len(m.messages)-1]
					if len(m.messages) > 0 {
						m.messages[len(m.messages)-1] = toolSummaryLine(inner.tool.Name, inner.tool.Result)
					} else {
						m.messages = append(m.messages, toolSummaryLine(inner.tool.Name, inner.tool.Result))
					}
				}
			} else if sticky {
				m.messages = append(m.messages, toolResultBlock(inner.tool.Result))
			} else {
				m.messages = append(m.messages, toolSummaryLine(inner.tool.Name, inner.tool.Result))
			}
			m.viewport = setContent(m.viewport, m.messages)
			cmds = append(cmds, readNext(msg.ch))

		case confirmMsg:
			m.confirming = true
			m.confirmID = inner.id
			m.confirmCh = msg.ch
			m.messages = append(m.messages,
				ErrorStyle().Render("⚠ опасно")+"  "+ToolStyle().Render(inner.tool)+"  "+DimStyle().Render(inner.args),
			)
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			m.textinput.Placeholder = "y / Enter для отмены..."
			m.textinput.Reset()

		case doneMsg:
			m.streaming = false
			m.loading = false
			m.toolRunning = false
			m.thinkBuf = ""
			m.thinkDone = false
			m.messages = append(m.messages, "")
			m.viewport = setContent(m.viewport, m.messages)
		}

	case doneMsg:
		if m.streaming {
			m.streaming = false
			m.loading = false
			m.toolRunning = false
			m.thinkBuf = ""
			m.thinkDone = false
			m.messages = append(m.messages, "")
			m.viewport = setContent(m.viewport, m.messages)
		}

	case tickMsg:
		if m.loading {
			m.spinner = (m.spinner + 1) % 24
			if !m.thinkDone && m.streaming && !m.confirming && !m.toolRunning && len(m.messages) > 0 {
				m.messages[len(m.messages)-1] = m.renderThinkLine()
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
			}
			return m, m.tick()
		}
		return m, nil

	case modeMsg:
		m.autoMode = msg.auto
		label := "Safe"
		if m.autoMode {
			label = "Auto"
		}
		m.messages = append(m.messages, DimStyle().Render("  Режим: "+label))
		m.viewport = setContent(m.viewport, m.messages)

	case backendReadyMsg:
		m.client.ResetSession()
		m.wardenTS = time.Now().Format("15:04")
		m.messages = append(m.messages, m.wardenLine(randomWardenPresence()))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case backendErrorMsg:
		m.messages = append(m.messages, ErrorStyle().Render("Error: backend unavailable"))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
	}

	var cmd tea.Cmd
	m.textinput, cmd = m.textinput.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	// sync hint visibility and viewport height
	matches := matchSlash(m.textinput.Value())
	newCount := 0
	if !m.streaming {
		newCount = len(matches)
	}
	if newCount != m.hintCount {
		m.hintCount = newCount
		m.hintVisible = newCount > 0
		if m.height > 0 {
			m.viewport.Height = m.height - 4 - newCount
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.height == 0 {
		return ""
	}

	var footer string
	if m.confirming {
		footer = KeyStyle().Render("[Y Enter]") +
			DimStyle().Render(" Подтвердить  ") +
			KeyStyle().Render("[Esc]") +
			DimStyle().Render(" Отменить")
	} else {
		footer = KeyStyle().Render("[Enter]") +
			DimStyle().Render(" Отправить  ") +
			KeyStyle().Render("[Esc]") +
			DimStyle().Render(" Очистить  ") +
			KeyStyle().Render("[Ctrl+C]") +
			DimStyle().Render(" Выйти")
	}

	var scrollTag string
	if m.viewport.TotalLineCount() > m.viewport.Height {
		if m.viewport.AtBottom() {
			scrollTag = " конец "
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

// handleSlash processes /commands before sending.
func (m *model) handleSlash(text string) (bool, tea.Cmd) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "/auto":
		m.autoMode = true
		return true, m.setMode(true)
	case "/safe":
		m.autoMode = false
		return true, m.setMode(false)
	case "/reset":
		m.messages = []string{}
		m.viewport.SetContent("")
		m.wardenTS = time.Now().Format("15:04")
		m.messages = append(m.messages, m.wardenLine("Сброшено"))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
		return true, func() tea.Msg {
			m.client.ResetSession()
			return noopMsg{}
		}
	case "/thinking":
		m.thinkingEnabled = !m.thinkingEnabled
		status := "вкл"
		if !m.thinkingEnabled {
			status = "выкл"
		}
		m.wardenTS = time.Now().Format("15:04")
		m.messages = append(m.messages, m.wardenLine("Размышления "+status))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
		return true, m.setThinking(m.thinkingEnabled)
	}
	return false, nil
}

// message types
type tokenMsg struct{ text string }
type thinkMsg struct{ text string }
type toolMsg struct{ tool ToolMsg }
type toolStartMsg struct {
	name string
	args string
}
type wardenStartMsg struct{ ch <-chan tea.Msg }
type confirmMsg struct {
	id   string
	tool string
	args string
}
type modeMsg struct{ auto bool }
type doneMsg struct{}
type backendReadyMsg struct{}
type backendErrorMsg struct{}
type tickMsg struct{}
type startStreamMsg struct{ ch <-chan tea.Msg }
type nextMsg struct {
	inner tea.Msg
	ch    <-chan tea.Msg
}
type noopMsg struct{}

func (m model) checkBackend() tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(m.client.BaseURL + "/health")
		if err != nil || resp.StatusCode != 200 {
			return backendErrorMsg{}
		}
		return backendReadyMsg{}
	}
}

func (m model) sendMessage(text string) tea.Cmd {
	ch := m.client.SendMessage(text)
	return func() tea.Msg {
		return startStreamMsg{ch: ch}
	}
}

func (m model) sendConfirm(id string, ok bool) tea.Cmd {
	return func() tea.Msg {
		m.client.SendConfirm(id, ok)
		return noopMsg{}
	}
}

func (m model) setMode(auto bool) tea.Cmd {
	return func() tea.Msg {
		m.client.SetMode(auto)
		return modeMsg{auto: auto}
	}
}

func (m model) setThinking(enabled bool) tea.Cmd {
	return func() tea.Msg {
		m.client.SetThinking(enabled)
		return noopMsg{}
	}
}

func readNext(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		inner, ok := <-ch
		if !ok {
			return doneMsg{}
		}
		return nextMsg{inner: inner, ch: ch}
	}
}

// setContent updates viewport and scrolls down only if user was already at the bottom.
func setContent(vp viewport.Model, lines []string) viewport.Model {
	atBottom := vp.AtBottom()
	vp.SetContent(strings.Join(lines, "\n"))
	if atBottom {
		vp.GotoBottom()
	}
	return vp
}

func (m model) tick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
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
	mode := SafeStyle().Render("Safe")
	if m.autoMode {
		mode = AutoStyle().Render("Auto")
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
