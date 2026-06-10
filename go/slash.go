package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type slashCmd struct {
	name string
	desc string
}

var slashCommands = []slashCmd{
	{"/build", "Build mode — autonomous, no confirmations except delete"},
	{"/ask", "Ask mode — confirm before any destructive action"},
	{"/reset", "Reset session"},
	{"/thinking", "Toggle model reasoning"},
	{"/model", "Show current provider and model"},
	{"/status", "Show backend status"},
	{"/copy-last", "Copy last response to clipboard"},
	{"/clear", "Clear screen without resetting session"},
	{"/pwd", "Show current working directory"},
	{"/tools", "List available tools"},
	{"/compact", "Summarize conversation to free up context"},
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
		m.updateViewportHeight()
	}
}

// handleSlash processes /commands before sending.
func (m *model) handleSlash(text string) (bool, tea.Cmd) {
	trimmed := strings.ToLower(strings.TrimSpace(text))
	if !strings.HasPrefix(trimmed, "/") {
		return false, nil
	}
	switch trimmed {
	case "/build":
		m.autoMode = true
		m.clearHintState()
		return true, m.setMode(true)
	case "/ask":
		m.autoMode = false
		m.clearHintState()
		return true, m.setMode(false)
	case "/reset":
		m.clearHintState()
		m.messages = []messageEntry{}
		m.syncViewport()
		
		m.appendText(m.wardenLine("Reset"))
		m.syncViewport()
		return true, func() tea.Msg {
			m.client.ResetSession()
			return nil
		}
	case "/thinking":
		m.thinkingEnabled = !m.thinkingEnabled
		m.clearHintState()
		status := "on"
		if !m.thinkingEnabled {
			status = "off"
		}
		
		m.appendText(m.wardenLine("Thinking " + status))
		m.syncViewport()
		return true, m.setThinking(m.thinkingEnabled)
	case "/model":
		m.clearHintState()
		return true, m.fetchStatus(true)
	case "/status":
		m.clearHintState()
		return true, m.fetchStatus(false)
	case "/copy-last":
		m.clearHintState()
		if m.lastAssistantRaw == "" {
			
			m.appendText(m.wardenLine(DimStyle().Render("nothing to copy")))
			m.syncViewport()
			return true, nil
		}
		return true, m.copyToClipboard(m.lastAssistantRaw)
	case "/clear":
		m.clearHintState()
		m.messages = []messageEntry{}
		m.syncViewport()
		return true, nil
	case "/pwd":
		m.clearHintState()
		
		m.appendText(m.wardenLine(DimStyle().Render(m.cwd)))
		m.syncViewport()
		return true, nil
	case "/tools":
		m.clearHintState()
		return true, m.fetchTools()
	case "/compact":
		m.clearHintState()
		m.loading = true
		m.spinner = 0
		m.appendText(m.wardenLine(DimStyle().Render("compacting...")))
		m.syncViewport()
		return true, tea.Batch(m.runCompact(), m.tick())
	}
	m.appendText(m.wardenLine(ErrorStyle().Render("unknown command")))
	m.syncViewport()
	return false, nil
}
