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
