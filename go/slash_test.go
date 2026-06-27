package tui

import (
	"strings"
	"testing"
	"github.com/elev1e1n/warden/internal/client"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSlashNavigationDoesNotChangeInput(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)
	m.width = 80
	m.height = 20
	m.viewport.Width = 80
	m.viewport.Height = 5
	m.textinput.SetValue("/")
	m.refreshHints()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(*model)

	if got := m.textinput.Value(); got != "/" {
		t.Fatalf("slash navigation changed input: got %q", got)
	}
	if m.slashIdx != 1 {
		t.Fatalf("slash navigation did not move selection: got %d", m.slashIdx)
	}
}

func TestBangNavigationDoesNotChangeInput(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)
	m.width = 80
	m.height = 20
	m.viewport.Width = 80
	m.viewport.Height = 5
	m.skills = []client.Skill{
		{Name: "alpha", Description: "Alpha skill"},
		{Name: "beta", Description: "Beta skill"},
		{Name: "gamma", Description: "Gamma skill"},
	}
	m.textinput.SetValue("!")
	m.refreshHints()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(*model)

	if got := m.textinput.Value(); got != "!" {
		t.Fatalf("bang navigation changed input: got %q", got)
	}
	if m.skillsIdx != 1 {
		t.Fatalf("bang navigation did not move selection: got %d", m.skillsIdx)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(*model)
	if m.skillsIdx != 0 {
		t.Fatalf("bang navigation did not move back: got %d", m.skillsIdx)
	}
}

func TestBangTabSelectsCurrentHintOnPartialPrefix(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)
	m.skills = []client.Skill{
		{Name: "build-web", Description: "Build web"},
		{Name: "build-worker", Description: "Build worker"},
	}
	m.textinput.SetValue("!bu")
	m.refreshHints()
	m.skillsIdx = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(*model)

	if got := m.textinput.Value(); got != "!build-worker" {
		t.Fatalf("expected selected skill, got %q", got)
	}
}

func TestBangTabCompletesSingleMatch(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)
	m.skills = []client.Skill{
		{Name: "cat-text", Description: "Cat text"},
		{Name: "skill-creator", Description: "Skill creator"},
	}
	m.textinput.SetValue("!cat")
	m.refreshHints()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(*model)

	if got := m.textinput.Value(); got != "!cat-text" {
		t.Fatalf("expected full skill name, got %q", got)
	}
}

func TestSlashTabSelectsCurrentHintOnPartialPrefix(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)
	m.textinput.SetValue("/c")
	m.refreshHints()
	m.slashIdx = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(*model)

	if got := m.textinput.Value(); got != "/clear" {
		t.Fatalf("expected selected slash command, got %q", got)
	}
}

func TestBangTabSelectsCurrentHintOnBarePrefix(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)
	m.skills = []client.Skill{
		{Name: "alpha", Description: "Alpha skill"},
		{Name: "beta", Description: "Beta skill"},
	}
	m.textinput.SetValue("!")
	m.refreshHints()
	m.skillsIdx = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(*model)

	if got := m.textinput.Value(); got != "!beta" {
		t.Fatalf("expected selected skill, got %q", got)
	}
}

func TestRenderHintScrollsSlashCommandsWithoutMarkers(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)
	m.width = 80
	m.textinput.SetValue("/")
	m.refreshHints()
	m.slashIdx = len(slashCommands) - 1

	hint := m.renderHint()

	if strings.Contains(hint, "...") {
		t.Fatalf("did not expect scroll markers in hint:\n%s", hint)
	}
	if !strings.Contains(hint, "/select") {
		t.Fatalf("expected selected command in hint:\n%s", hint)
	}
	if strings.Contains(hint, "/connect") {
		t.Fatalf("expected top command to be clipped when scrolled:\n%s", hint)
	}
}

func TestHandleBangUnknownSkillShowsError(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)

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
	m := initialModel(&mockBackend{},"test-model", true)
	m.skills = []client.Skill{{Name: "demo", Description: "Demo skill"}}

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
	foundMarker := false
	for _, msg := range m.messages {
		if strings.Contains(msg.text, "Using skill: demo") {
			foundMarker = true
			break
		}
	}
	if !foundMarker {
		t.Fatalf("expected using-skill marker, got %#v", m.messages)
	}
}

func TestHandleBangKnownSkillWithArgsStartsBackendInvocation(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)
	m.skills = []client.Skill{{Name: "demo", Description: "Demo skill"}}

	handled, cmd := m.handleBang("!demo arg1 arg2")

	if !handled {
		t.Fatalf("expected bang to be handled")
	}
	if cmd == nil {
		t.Fatalf("expected stream command for known skill")
	}
	if m.messages[0].kind != messageUser || m.messages[0].text != "!demo arg1 arg2" {
		t.Fatalf("expected full user text with args, got %#v", m.messages[0])
	}
	foundMarker := false
	for _, msg := range m.messages {
		if strings.Contains(msg.text, "Using skill: demo") {
			foundMarker = true
			break
		}
	}
	if !foundMarker {
		t.Fatalf("expected using-skill marker, got %#v", m.messages)
	}
}
