package main

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

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
