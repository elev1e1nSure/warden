package main

import (
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
	loading   bool
	spinner   int
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
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3
		m.textinput.Width = msg.Width
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			m.textinput.Reset()
		case tea.KeyEnter:
			if m.streaming {
				return m, nil
			}
			text := strings.TrimSpace(m.textinput.Value())
			if text == "" {
				return m, nil
			}
			m.messages = append(m.messages, UserStyle().Render("you")+"  "+text)
			m.textinput.Reset()
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			m.streaming = true
			m.loading = true
			m.spinner = 0
			return m, tea.Batch(m.sendMessage(text), m.tick())

	case tokenMsg:
		if m.streaming && len(m.messages) > 0 {
			m.messages[len(m.messages)-1] += msg.text
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
		}

	case doneMsg:
		m.streaming = false
		m.loading = false
		m.messages = append(m.messages, "")
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case tea.TickMsg:
		if m.loading {
			m.spinner = (m.spinner + 1) % 4
			return m, m.tick()
		}

	case toolMsg:
		m.messages = append(m.messages,
			ToolStyle().Render("▸ "+msg.tool.Name+" ")+DimStyle().Render(msg.tool.Args),
			DimStyle().Render("  "+msg.tool.Result),
		)
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case backendReadyMsg:
		m.messages = append(m.messages, WardenStyle().Render("warden")+"  ready")
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
	spinner := ""
	if m.loading {
		spinners := []string{"◐", "◑", "◒", "◓"}
		spinner = KeyStyle().Render(spinners[m.spinner]) + " "
	}
	footer := DimStyle().Render("Press ") +
		KeyStyle().Render("[Enter]") +
		DimStyle().Render(" to send, ") +
		KeyStyle().Render("[Esc]") +
		DimStyle().Render(" to clear, ") +
		KeyStyle().Render("[Ctrl+C]") +
		DimStyle().Render(" to quit")
	separator := DimStyle().Render(strings.Repeat("—", m.width))
	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		"",
		separator,
		spinner+m.textinput.View(),
		footer,
	)
}

// messages
type tokenMsg struct{ text string }
type toolMsg struct{ tool ToolMsg }
type doneMsg struct{}
type backendReadyMsg struct{}
type backendErrorMsg struct{}

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
	m.messages = append(m.messages, WardenStyle().Render("warden")+"  ")
	m.viewport.SetContent(strings.Join(m.messages, "\n"))
	m.viewport.GotoBottom()
	m.streaming = true

	ch := m.client.SendMessage(text)
	var cmds []tea.Cmd
	for msg := range ch {
		cmds = append(cmds, func(msg tea.Msg) tea.Cmd {
			return func() tea.Msg { return msg }
		}(msg))
	}
	cmds = append(cmds, func() tea.Msg { return doneMsg{} })

	return tea.Sequence(cmds...)
}

func (m model) tick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tea.TickMsg(t)
	})
}
