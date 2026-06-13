package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSlashNavigationDoesNotChangeInput(t *testing.T) {
	m := initialModel("test-model", true)
	m.width = 80
	m.height = 20
	m.viewport.Width = 80
	m.viewport.Height = 5
	m.textinput.SetValue("/")
	m.refreshHints()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)

	if got := m.textinput.Value(); got != "/" {
		t.Fatalf("slash navigation changed input: got %q", got)
	}
	if m.slashIdx != 1 {
		t.Fatalf("slash navigation did not move selection: got %d", m.slashIdx)
	}
}

func TestRenderHintScrollsSlashCommandsWithoutMarkers(t *testing.T) {
	m := initialModel("test-model", true)
	m.width = 80
	m.textinput.SetValue("/")
	m.refreshHints()
	m.slashIdx = len(slashCommands) - 1

	hint := m.renderHint()

	if strings.Contains(hint, "...") {
		t.Fatalf("did not expect scroll markers in hint:\n%s", hint)
	}
	if !strings.Contains(hint, "/verbose") {
		t.Fatalf("expected selected command in hint:\n%s", hint)
	}
	if strings.Contains(hint, "/connect") {
		t.Fatalf("expected top command to be clipped when scrolled:\n%s", hint)
	}
}

func TestHandleBangUnknownSkillShowsError(t *testing.T) {
	m := initialModel("test-model", true)

	handled, cmd := m.handleBang("!ghost")

	if !handled {
		t.Fatalf("expected bang to be handled")
	}
	if cmd != nil {
		t.Fatalf("expected no command for unknown skill")
	}
	if len(m.messages) == 0 || !strings.Contains(m.messages[0].text, "skill not found: ghost") {
		t.Fatalf("expected visible skill error, got %#v", m.messages)
	}
}

func TestHandleBangKnownSkillStartsBackendInvocation(t *testing.T) {
	m := initialModel("test-model", true)
	m.skills = []Skill{{Name: "demo", Description: "Demo skill"}}

	handled, cmd := m.handleBang("!demo")

	if !handled {
		t.Fatalf("expected bang to be handled")
	}
	if cmd == nil {
		t.Fatalf("expected stream command for known skill")
	}
	if len(m.messages) < 3 {
		t.Fatalf("expected user message and marker, got %#v", m.messages)
	}
	if m.messages[0].kind != messageUser || m.messages[0].text != "!demo" {
		t.Fatalf("expected compact user marker, got %#v", m.messages[0])
	}
	if !strings.Contains(m.messages[1].text, "using skill: demo") {
		t.Fatalf("expected using-skill marker, got %#v", m.messages[1])
	}
}
