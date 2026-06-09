package main

import (
	"net/http"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	viewport  viewport.Model
	textarea  textarea.Model
	client    *Client
	messages  []string
	streaming bool
	height    int
	streamCh  <-chan tea.Msg
}

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = "promt..."
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.SetHeight(1)
	ta.SetWidth(80)
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(80, 20)
	vp.SetContent("")

	return model{
		textarea: ta,
		viewport: vp,
		client:   NewClient("http://localhost:8765"),
		messages: []string{},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.checkBackend(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3
		m.textarea.SetWidth(msg.Width)
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			m.textarea.Reset()
		case tea.KeyEnter:
			if m.streaming {
				return m, nil
			}
			text := strings.TrimSpace(m.textarea.Value())
			if text == "" {
				return m, nil
			}
			m.messages = append(m.messages, UserStyle().Render("you")+"  "+text)
			m.textarea.Reset()
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			m.streaming = true
			return m, m.sendMessage(text)
		}

	case tokenMsg:
		if m.streaming {
			m.messages[len(m.messages)-1] += msg.text
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
		}
		return m, m.readStream()

	case toolMsg:
		m.messages = append(m.messages,
			ToolStyle().Render("▸ "+msg.tool.Name+" ")+DimStyle().Render(msg.tool.Args),
			DimStyle().Render("  "+msg.tool.Result),
		)
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
		return m, m.readStream()

	case doneMsg:
		m.streaming = false
		m.messages = append(m.messages, "")
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
		m.streamCh = nil

	case backendReadyMsg:
		m.messages = append(m.messages, WardenStyle().Render("warden")+"  готов к работе")
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case backendErrorMsg:
		m.messages = append(m.messages, ErrorStyle().Render("ошибка: бэкенд недоступен"))
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.height == 0 {
		return ""
	}
	footer := DimStyle().Render("enter — отправить  esc — очистить  ctrl+c — выход")
	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		"",
		m.textarea.View(),
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

	m.streamCh = m.client.SendMessage(text)
	return m.readStream()
}

func (m model) readStream() tea.Cmd {
	if m.streamCh == nil {
		return nil
	}
	return func() tea.Msg {
		return <-m.streamCh
	}
}
