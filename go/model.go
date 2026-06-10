package main

import (
	"fmt"
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
	modelInfo       string
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
		modelInfo:       "qwen3:8b",
		thinkingEnabled: true,
	}
}

func (m model) Init() tea.Cmd {
	return m.checkBackend()
}

// ── slash-команды ─────────────────────────────────────────────────────────────

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

// ── вспомогательные методы ─────────────────────────────────────────────────

// ts возвращает отрендеренную метку времени в едином формате.
func (m model) ts() string {
	return DimStyle().Render("[" + m.wardenTS + "]")
}

// wardenLine строит строку-хедер warden с опциональным суффиксом.
func (m model) wardenLine(suffix string) string {
	return m.ts() + "  " + WardenStyle().Render("warden:") + "  " + suffix
}

// finalizeThink возвращает строку-итог думания (пустую если не думал).
func (m model) finalizeThink() string {
	if m.thinkBuf != "" {
		words := len(strings.Fields(m.thinkBuf))
		return DimStyle().Render(fmt.Sprintf("  думал %d слов", words))
	}
	return ""
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
			m.messages[len(m.messages)-1] = m.wardenLine(m.thinkIndicator())
			m.viewport = setContent(m.viewport, m.messages)
			cmds = append(cmds, readNext(msg.ch))

		case tokenMsg:
			if !m.thinkDone {
				if summary := m.finalizeThink(); summary != "" {
					m.messages[len(m.messages)-1] = summary
					m.messages = append(m.messages, m.wardenLine(""))
				} else {
					m.messages[len(m.messages)-1] = m.wardenLine("")
				}
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
				if summary := m.finalizeThink(); summary != "" {
					m.messages[len(m.messages)-1] = summary
				} else {
					m.messages = m.messages[:len(m.messages)-1]
				}
				m.thinkDone = true
				m.thinkBuf = ""
			}
			m.messages = append(m.messages,
				ToolStyle().Render("▸ "+inner.name+" ")+DimStyle().Render(inner.args),
				DimStyle().Render("  ..."),
			)
			m.viewport = setContent(m.viewport, m.messages)
			cmds = append(cmds, readNext(msg.ch))

		case toolMsg:
			if len(m.messages) > 0 && m.messages[len(m.messages)-1] == DimStyle().Render("  ...") {
				m.messages[len(m.messages)-1] = DimStyle().Render("  " + inner.tool.Result)
			} else {
				m.messages = append(m.messages, DimStyle().Render("  "+inner.tool.Result))
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
			m.thinkBuf = ""
			m.thinkDone = false
			m.messages = append(m.messages, "")
			m.viewport = setContent(m.viewport, m.messages)
		}

	case doneMsg:
		if m.streaming {
			m.streaming = false
			m.loading = false
			m.thinkBuf = ""
			m.thinkDone = false
			m.messages = append(m.messages, "")
			m.viewport = setContent(m.viewport, m.messages)
		}

	case tickMsg:
		if m.loading {
			m.spinner = (m.spinner + 1) % 24
			if !m.thinkDone && m.streaming && !m.confirming && !m.toolRunning && len(m.messages) > 0 {
				m.messages[len(m.messages)-1] = m.wardenLine(m.thinkIndicator())
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
		m.messages = append(m.messages, m.wardenLine("Ready"))
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

	// синхронизируем видимость подсказки и высоту вьюпорта
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

	// статус бар
	var modeBadge string
	if m.autoMode {
		modeBadge = AutoStyle().Render(" AUTO")
	} else {
		modeBadge = SafeStyle().Render(" SAFE")
	}

	var thinkingBadge string
	if m.thinkingEnabled {
		thinkingBadge = ThinkingOnStyle().Render(" THINK")
	} else {
		thinkingBadge = ThinkingOffStyle().Render(" -think")
	}

	statusBar := modeBadge + " " + thinkingBadge

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

	// выравнивание плашки режима справа
	statusBar = lipgloss.NewStyle().Width(m.width).Align(lipgloss.Right).Render(modeBadge)

	layers := []string{m.viewport.View(), sep1}
	if m.hintVisible {
		layers = append(layers, m.renderHint())
	}
	layers = append(layers, m.textinput.View(), sep2, statusBar, footer)
	return lipgloss.JoinVertical(lipgloss.Left, layers...)
}

// handleSlash обрабатывает /команды перед отправкой.
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
		m.messages = append(m.messages, m.wardenLine("Сессия сброшена"))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
		return true, func() tea.Msg {
			m.client.ResetSession()
			return noopMsg{}
		}
	case "/thinking":
		m.thinkingEnabled = !m.thinkingEnabled
		status := "включены"
		if !m.thinkingEnabled {
			status = "выключены"
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

// setContent обновляет viewport и скроллит вниз только если пользователь уже был внизу.
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

func (m model) thinkIndicator() string {
	dots := []string{".", "..", "..."}
	dot := dots[(m.spinner/2)%3]
	if m.thinkBuf == "" {
		return DimStyle().Render(dot)
	}
	words := len(strings.Fields(m.thinkBuf))
	return DimStyle().Render(fmt.Sprintf("%s  %d сл", dot, words))
}
