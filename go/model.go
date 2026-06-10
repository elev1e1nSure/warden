package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	viewport  viewport.Model
	textinput textinput.Model
	client    *Client
	messages  []messageEntry
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
	thinkingEnabled  bool
	thinkingExpanded bool
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
		messages:        []messageEntry{},
		thinkingEnabled: true,
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
		m.viewport.Height = msg.Height - 4 - m.hintCount
		m.textinput.Width = msg.Width
		m.syncViewport()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyF2:
			m.thinkingExpanded = !m.thinkingExpanded
			m.syncViewport()
			return m, nil

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
				val := strings.ToLower(strings.TrimSpace(m.textinput.Value()))
				ok := val == "y" || val == "yes"
				if val == "" || val == "n" || val == "no" {
					ok = false
				}
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
			m.appendText(ts + "  " + UserStyle().Render("you:") + "  " + text)
			m.textinput.Reset()
			m.streaming = true
			m.loading = true
			m.spinner = 0
			m.syncViewport()
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
			m.appendThink()
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case thinkMsg:
			m.thinkBuf += inner.text
			m.updateThink(inner.text)
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case tokenMsg:
			if !m.thinkDone {
				m.finishThink()
				m.appendText(m.wardenLine(""))
				m.thinkDone = true
			}
			m.appendToLastText(inner.text)
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case toolStartMsg:
			m.toolRunning = true
			if !m.thinkDone && len(m.messages) > 0 {
				m.finishThink()
			}
			m.appendText(
				toolStartLine(inner.name, inner.args),
			)
			m.appendText(toolPendingLine())
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case toolMsg:
			m.toolRunning = false
			sticky := stickyTool(inner.tool.Name)
			if len(m.messages) > 0 && m.messages[len(m.messages)-1].text == toolPendingLine() {
				if sticky {
					m.messages[len(m.messages)-1].text = toolResultBlock(inner.tool.Result)
				} else {
					m.messages = m.messages[:len(m.messages)-1]
					if len(m.messages) > 0 {
						m.messages[len(m.messages)-1].text = toolSummaryLine(inner.tool.Name, inner.tool.Result)
					} else {
						m.appendText(toolSummaryLine(inner.tool.Name, inner.tool.Result))
					}
				}
			} else if sticky {
				m.appendText(toolResultBlock(inner.tool.Result))
			} else {
				m.appendText(toolSummaryLine(inner.tool.Name, inner.tool.Result))
			}
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case confirmMsg:
			m.confirming = true
			m.confirmID = inner.id
			m.confirmCh = msg.ch
			m.appendText(renderConfirmBlock(inner, m.width))
			m.syncViewport()
			m.textinput.Placeholder = ""
			m.textinput.Reset()

		case doneMsg:
			m.streaming = false
			m.loading = false
			m.toolRunning = false
			m.finishThink()
			m.thinkBuf = ""
			m.thinkDone = false
			m.appendText("")
			m.syncViewport()
		}

	case doneMsg:
		if m.streaming {
			m.streaming = false
			m.loading = false
			m.toolRunning = false
			m.finishThink()
			m.thinkBuf = ""
			m.thinkDone = false
			m.appendText("")
			m.syncViewport()
		}

	case tickMsg:
		if m.loading {
			m.spinner = (m.spinner + 1) % 24
			if !m.thinkDone && m.streaming && !m.confirming && !m.toolRunning && len(m.messages) > 0 {
				m.syncViewport()
			}
			return m, m.tick()
		}
		return m, nil

	case modeMsg:
		m.autoMode = msg.auto
		label := "Leashed"
		if m.autoMode {
			label = "Unleashed"
		}
		m.appendText(DimStyle().Render("  Mode: " + label))
		m.syncViewport()

	case backendReadyMsg:
		m.client.ResetSession()
		m.wardenTS = time.Now().Format("15:04")
		m.appendText(m.wardenLine(randomWardenPresence()))
		m.syncViewport()

	case backendErrorMsg:
		m.appendText(ErrorStyle().Render("Error: backend unavailable"))
		m.syncViewport()
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
	id            string
	tool          string
	risk          string
	title         string
	summary       string
	details       []string
	args          string
	preview       string
	defaultAction string
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

type messageKind int

const (
	messageText messageKind = iota
	messageThink
)

type messageEntry struct {
	kind      messageKind
	text      string
	startedAt time.Time
	duration  time.Duration
}

func (m *model) appendText(text string) {
	m.messages = append(m.messages, messageEntry{kind: messageText, text: text})
}

func (m *model) appendThink() {
	m.messages = append(m.messages, messageEntry{kind: messageThink, startedAt: time.Now()})
}

func (m *model) updateThink(text string) {
	if len(m.messages) == 0 {
		return
	}
	last := len(m.messages) - 1
	if m.messages[last].kind != messageThink {
		return
	}
	m.messages[last].text += text
}

func (m *model) finishThink() {
	if len(m.messages) == 0 {
		return
	}
	last := len(m.messages) - 1
	if m.messages[last].kind != messageThink {
		return
	}
	if m.messages[last].duration == 0 {
		m.messages[last].duration = time.Since(m.messages[last].startedAt)
	}
}

func (m *model) appendToLastText(text string) {
	if len(m.messages) == 0 {
		return
	}
	last := len(m.messages) - 1
	if m.messages[last].kind != messageText {
		return
	}
	m.messages[last].text += text
}
