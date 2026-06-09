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
	viewport   viewport.Model
	textinput  textinput.Model
	client     *Client
	messages   []string
	streaming  bool
	height     int
	width      int
	loading    bool
	spinner    int
	thinkBuf   string
	thinkDone  bool
	wardenTS   string
	// tool execution
	toolRunning bool
	// confirmation
	confirming bool
	confirmID  string
	confirmCh  <-chan tea.Msg
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "prompt..."
	ti.CharLimit = 0
	ti.Width = 80
	ti.Focus()
	ti.BlinkSpeed = 0

	vp := viewport.New(80, 20)
	vp.SetContent("")

	return model{
		textinput: ti,
		viewport:  vp,
		client:    NewClient("http://localhost:8765"),
		messages:  []string{},
	}
}

func (m model) Init() tea.Cmd {
	return m.checkBackend()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 4
		m.textinput.Width = msg.Width
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.confirming {
				// отмена подтверждения
				m.confirming = false
				ch := m.confirmCh
				id := m.confirmID
				m.confirmID = ""
				m.confirmCh = nil
				m.textinput.Placeholder = "prompt..."
				m.textinput.Reset()
				return m, tea.Batch(
					m.sendConfirm(id, false),
					readNext(ch),
				)
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
				m.textinput.Placeholder = "prompt..."
				m.textinput.Reset()
				return m, tea.Batch(
					m.sendConfirm(id, ok),
					readNext(ch),
				)
			}
			if m.streaming {
				return m, nil
			}
			text := strings.TrimSpace(m.textinput.Value())
			if text == "" {
				return m, nil
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
			ts := DimStyle().Render("[" + m.wardenTS + "]")
			m.thinkBuf = ""
			m.thinkDone = false
			m.toolRunning = false
			m.messages = append(m.messages, ts+"  "+WardenStyle().Render("warden:")+"  ")
			m.viewport = setContent(m.viewport, m.messages)
			cmds = append(cmds, readNext(msg.ch))
		case thinkMsg:
			m.thinkBuf += inner.text
			ts := DimStyle().Render("[" + m.wardenTS + "]")
			m.messages[len(m.messages)-1] = ts + "  " + WardenStyle().Render("warden:") + "  " + m.thinkIndicator()
			m.viewport = setContent(m.viewport, m.messages)
			cmds = append(cmds, readNext(msg.ch))
		case tokenMsg:
			if !m.thinkDone {
				ts := DimStyle().Render("[" + m.wardenTS + "]")
				if m.thinkBuf != "" {
					words := len(strings.Fields(m.thinkBuf))
					m.messages[len(m.messages)-1] = DimStyle().Render(fmt.Sprintf("  думал %d слов", words))
					m.messages = append(m.messages, ts+"  "+WardenStyle().Render("warden:")+"  ")
				} else {
					m.messages[len(m.messages)-1] = ts + "  " + WardenStyle().Render("warden:") + "  "
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
			m.messages = append(m.messages,
				ToolStyle().Render("▸ "+inner.name+" ")+DimStyle().Render(inner.args),
				DimStyle().Render("  ..."),
			)
			m.viewport = setContent(m.viewport, m.messages)
			cmds = append(cmds, readNext(msg.ch))
		case toolMsg:
			// заменяем "..." на результат
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
			m.textinput.Placeholder = "y / enter для отмены..."
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
				ts := DimStyle().Render("[" + m.wardenTS + "]")
				m.messages[len(m.messages)-1] = ts + "  " + WardenStyle().Render("warden:") + "  " + m.thinkIndicator()
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
			}
			return m, m.tick()
		}
		return m, nil

	case backendReadyMsg:
		m.client.ResetSession()
		m.messages = append(m.messages, WardenStyle().Render("warden:")+"  ready")
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case backendErrorMsg:
		m.messages = append(m.messages, ErrorStyle().Render("error: backend unavailable"))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
	}

	var cmd tea.Cmd
	m.textinput, cmd = m.textinput.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

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
		pct := int(m.viewport.ScrollPercent() * 100)
		if m.viewport.AtBottom() {
			scrollTag = " конец "
		} else {
			scrollTag = fmt.Sprintf(" %d%% ", pct)
		}
	}
	sepWidth := m.width - len(scrollTag)
	if sepWidth < 0 {
		sepWidth = 0
	}
	sep1 := DimStyle().Render(strings.Repeat("─", sepWidth) + scrollTag)
	sep2 := DimStyle().Render(strings.Repeat("─", m.width))
	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		sep1,
		m.textinput.View(),
		sep2,
		footer,
	)
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
type doneMsg struct{}
type backendReadyMsg struct{}
type backendErrorMsg struct{}
type tickMsg struct{}
type startStreamMsg struct{ ch <-chan tea.Msg }
type nextMsg struct {
	inner tea.Msg
	ch    <-chan tea.Msg
}

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

type noopMsg struct{}

// setContent обновляет viewport и скроллит вниз только если пользователь уже был внизу.
func setContent(vp viewport.Model, lines []string) viewport.Model {
	atBottom := vp.AtBottom()
	vp.SetContent(strings.Join(lines, "\n"))
	if atBottom {
		vp.GotoBottom()
	}
	return vp
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

func (m model) tick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
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
