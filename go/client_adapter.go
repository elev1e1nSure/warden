package tui

import (
	"github.com/elev1e1n/warden/internal/client"

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

func readNext(ch <-chan client.Event, gen int) tea.Cmd {
	return func() tea.Msg {
		inner, ok := <-ch
		if !ok {
			return doneMsg{gen: gen}
		}
		return nextMsg{inner: toTeaMsg(inner), ch: ch, gen: gen}
	}
}

func (m *model) sendMessage(text string, gen int) tea.Cmd {
	return func() tea.Msg {
		ch := m.backend.StreamChat(map[string]string{"type": "message", "text": text})
		return startStreamMsg{ch: ch, gen: gen}
	}
}

func (m *model) sendSkill(name, args string, gen int) tea.Cmd {
	return func() tea.Msg {
		payload := map[string]string{"type": "message", "text": "Use skill: " + name, "skill": name}
		if args != "" {
			payload["args"] = args
		}
		ch := m.backend.StreamChat(payload)
		return startStreamMsg{ch: ch, gen: gen}
	}
}

func (m *model) sendInterrupt() tea.Cmd {
	return func() tea.Msg {
		if err := m.backend.Interrupt(); err != nil {
			return statusResultMsg{brief: true}
		}
		return statusResultMsg{brief: true}
	}
}
