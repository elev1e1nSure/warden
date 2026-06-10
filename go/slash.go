package tui

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
	{"/unleash", "Unleash — dangerous commands without confirmation"},
	{"/leash", "Leash — confirmation for dangerous commands"},
	{"/reset", "Reset session"},
	{"/thinking", "Toggle model reasoning"},
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

func (m *model) clearHintState() {
	m.hintCount = 0
	m.hintVisible = false
	if m.height > 0 {
		m.viewport.Height = m.height - 10
	}
}

// handleSlash processes /commands before sending.
func (m *model) handleSlash(text string) (bool, tea.Cmd) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "/unleash":
		m.autoMode = true
		m.clearHintState()
		return true, m.setMode(true)
	case "/leash":
		m.autoMode = false
		m.clearHintState()
		return true, m.setMode(false)
	case "/reset":
		m.clearHintState()
		m.messages = []messageEntry{}
		m.syncViewport()
		m.wardenTS = time.Now().Format("15:04")
		m.appendText(m.wardenLine("Reset"))
		m.syncViewport()
		return true, func() tea.Msg {
			m.client.ResetSession()
			return noopMsg{}
		}
	case "/thinking":
		m.thinkingEnabled = !m.thinkingEnabled
		m.clearHintState()
		status := "вкл"
		if !m.thinkingEnabled {
			status = "выкл"
		}
		m.wardenTS = time.Now().Format("15:04")
		m.appendText(m.wardenLine("Размышления " + status))
		m.syncViewport()
		return true, m.setThinking(m.thinkingEnabled)
	}
	return false, nil
}
