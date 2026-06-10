package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

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

// setContent updates viewport and optionally pins it to the latest line.
func setContent(vp viewport.Model, lines []string, forceBottom bool) viewport.Model {
	atBottom := vp.AtBottom()
	vp.SetContent(strings.Join(lines, "\n"))
	if forceBottom || atBottom {
		vp.GotoBottom()
	}
	return vp
}

func (m model) tick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}
