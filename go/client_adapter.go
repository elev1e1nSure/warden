package tui

import (
	"warden/internal/client"

	tea "github.com/charmbracelet/bubbletea"
)

// toTeaMsg converts a neutral client.Event into the tea.Msg types used by the TUI.
func toTeaMsg(ev client.Event) tea.Msg {
	switch e := ev.(type) {
	case client.EventWardenStart:
		return wardenStartMsg{}
	case client.EventToken:
		return tokenMsg{text: e.Text}
	case client.EventThink:
		return thinkMsg{text: e.Text}
	case client.EventToolStart:
		return toolStartMsg{name: e.Name, args: e.Args}
	case client.EventTool:
		return toolMsg{tool: e.Tool}
	case client.EventConfirm:
		return confirmMsg{
			id:         e.ID,
			tool:       e.Tool,
			risk:       e.Risk,
			title:      e.Title,
			summary:    e.Summary,
			details:    e.Details,
			args:       e.Args,
			preview:    e.Preview,
			defaultVal: e.DefaultVal,
		}
	case client.EventQuestion:
		return questionMsg{
			id:        e.ID,
			questions: e.Questions,
		}
	case client.EventDone:
		return doneMsg{tokenCount: e.TokenCount, tokenLimit: e.TokenLimit}
	case client.EventError:
		return tokenMsg{text: e.Text}
	default:
		return doneMsg{}
	}
}

func readNext(ch <-chan client.Event) tea.Cmd {
	return func() tea.Msg {
		inner, ok := <-ch
		if !ok {
			return doneMsg{}
		}
		return nextMsg{inner: toTeaMsg(inner), ch: ch}
	}
}

func (m model) sendMessage(text string) tea.Cmd {
	ch := m.client.StreamChat(map[string]string{"type": "message", "text": text})
	return func() tea.Msg {
		return startStreamMsg{ch: ch}
	}
}

func (m model) sendSkill(name, args string) tea.Cmd {
	payload := map[string]string{"type": "message", "text": "Use skill: " + name, "skill": name}
	if args != "" {
		payload["args"] = args
	}
	ch := m.client.StreamChat(payload)
	return func() tea.Msg {
		return startStreamMsg{ch: ch}
	}
}
