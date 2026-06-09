package main

import (
	"fmt"
	"net/http"
	"strings"

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
	streamCh  <-chan tea.Msg
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "prompt..."
	ti.CharLimit = 0
	ti.Width = 80
	ti.Focus()

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
	return tea.Batch(
		textinput.Blink,
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
			return m, m.sendMessage(text)
		}

	case tokenMsg:
		fmt.Printf("[model] tokenMsg: streaming=%v, msg.text=%q\n", m.streaming, msg.text)
		if m.streaming && len(m.messages) > 0 {
			m.messages[len(m.messages)-1] += msg.text
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			fmt.Printf("[model] updated viewport, messages count=%d\n", len(m.messages))
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
	footer := DimStyle().Render("enter — send  esc — clear  ctrl+c — quit")
	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		"",
		m.textinput.View(),
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
