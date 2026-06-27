package tui

import (
	"testing"
	"github.com/elev1e1n/warden/internal/client"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleKeyUpDown(t *testing.T) {
	// 1. Model Picker navigation
	m := newTestModel()
	m.modelPicking = true
	m.modelFiltered = []string{"a", "b", "c"}
	m.modelPickIdx = 1

	// Up key
	m2, _, handled := m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	if !handled || m2.modelPickIdx != 0 {
		t.Errorf("expected Up arrow to navigate model list up, got idx=%d", m2.modelPickIdx)
	}

	// Down key
	m.modelPickIdx = 1
	m3, _, handled := m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if !handled || m3.modelPickIdx != 2 {
		t.Errorf("expected Down arrow to navigate model list down, got idx=%d", m3.modelPickIdx)
	}

	// 2. History Navigation
	m = newTestModel()
	m.history = []string{"cmd1", "cmd2"}
	m.historyIdx = 2
	m.textinput.Focus()

	// Up recalls history
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	if !handled || m2.textinput.Value() != "cmd2" || m2.historyIdx != 1 {
		t.Errorf("expected Up to recall last history cmd, got val=%q, idx=%d", m2.textinput.Value(), m2.historyIdx)
	}

	m2, _, handled = m2.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	if !handled || m2.textinput.Value() != "cmd1" || m2.historyIdx != 0 {
		t.Errorf("expected Up to recall older history cmd, got val=%q, idx=%d", m2.textinput.Value(), m2.historyIdx)
	}

	// Down recalls newer history
	m3, _, handled = m2.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if !handled || m3.textinput.Value() != "cmd2" || m3.historyIdx != 1 {
		t.Errorf("expected Down to recall newer history, got val=%q, idx=%d", m3.textinput.Value(), m3.historyIdx)
	}

	m3, _, handled = m3.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if !handled || m3.textinput.Value() != "" || m3.historyIdx != 2 {
		t.Errorf("expected Down to clear input when at end of history, got val=%q, idx=%d", m3.textinput.Value(), m3.historyIdx)
	}
}

func TestHandleKeyTab(t *testing.T) {
	// Slash autocomplete
	m := newTestModel()
	m.textinput.SetValue("/con")
	m2, _, handled := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if !handled || m2.textinput.Value() != "/connect" {
		t.Errorf("expected Tab to autocomplete /connect, got %q", m2.textinput.Value())
	}

	// Bang autocomplete
	m = newTestModel()
	m.skills = []client.Skill{{Name: "creator", Description: "d"}}
	m.textinput.SetValue("!cre")
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	if !handled || m2.textinput.Value() != "!creator" {
		t.Errorf("expected Tab to autocomplete !creator, got %q", m2.textinput.Value())
	}
}

func TestHandleKeyShiftTab(t *testing.T) {
	// Not streaming: shifts mode
	m := newTestModel()
	m.autoMode = false
	m2, _, handled := m.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	if !handled || !m2.autoMode {
		t.Errorf("expected Shift+Tab to toggle autoMode when not streaming")
	}

	// Streaming: Shift+Tab should not shift mode (no-op or handled=false depending on code)
	m = newTestModel()
	m.streaming = true
	m.autoMode = false
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m2.autoMode {
		t.Errorf("expected Shift+Tab during streaming to not toggle autoMode")
	}
}

func TestHandleKeyCtrlW(t *testing.T) {
	m := newTestModel()
	m.textinput.SetValue("hello world test")
	m.textinput.CursorEnd()

	m2, _, handled := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	if !handled {
		t.Errorf("expected Ctrl+W to be handled")
	}
	if m2.textinput.Value() != "hello world" {
		t.Errorf("expected Ctrl+W to delete last word, got %q", m2.textinput.Value())
	}

	m3, _, _ := m2.handleKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	if m3.textinput.Value() != "hello" {
		t.Errorf("expected Ctrl+W to delete last word again, got %q", m3.textinput.Value())
	}
}
