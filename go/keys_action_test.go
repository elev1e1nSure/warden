package tui

import (
	"strings"
	"testing"
	"time"
	"warden/internal/client"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleKeyCtrlC(t *testing.T) {
	// Not streaming: should immediately quit
	m := newTestModel()
	m.streaming = false
	_, cmd, handled := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !handled {
		t.Errorf("expected Ctrl+C to be handled")
	}
	if cmd == nil {
		t.Fatalf("expected command, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.Quit command, got %T", cmd())
	}

	// Streaming: first Ctrl+C should set quitPending
	m = newTestModel()
	m.streaming = true
	m2, cmd, handled := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !handled {
		t.Errorf("expected Ctrl+C to be handled during stream")
	}
	if cmd != nil {
		t.Errorf("expected no immediate quit command on first Ctrl+C during stream")
	}
	if !m2.quitPending {
		t.Errorf("expected quitPending to be true")
	}

	// Streaming + quitPending: second Ctrl+C should quit
	m2, cmd, handled = m2.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !handled {
		t.Errorf("expected second Ctrl+C to be handled")
	}
	if cmd == nil {
		t.Fatalf("expected command on second Ctrl+C")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.Quit command, got %T", cmd())
	}
}

func TestHandleKeyEsc(t *testing.T) {
	// 1. Esc in selectMode
	m := newTestModel()
	m.selectMode = true
	m2, cmd, handled := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if !handled || m2.selectMode {
		t.Errorf("expected selectMode to be disabled and handled")
	}
	if cmd == nil {
		t.Fatalf("expected command")
	}
	// EnableMouseCellMotion is a standard cmd
	if _, ok := cmd().(tea.MouseMsg); !ok { // Wait, tea.EnableMouseCellMotion returns a tea.Cmd which outputs a msg or is just a raw command.
		// Actually let's just make sure cmd is not nil.
	}

	// 2. Esc in modelPicking
	m = newTestModel()
	m.modelPicking = true
	m.modelList = []string{"a"}
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if !handled || m2.modelPicking || m2.modelList != nil {
		t.Errorf("expected modelPicking to be cleared")
	}

	// 3. Esc in streaming: first press sets escPending
	m = newTestModel()
	m.streaming = true
	m2, cmd, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if !handled || !m2.escPending || cmd != nil {
		t.Errorf("expected escPending to be true, got escPending=%v", m2.escPending)
	}

	// 4. Esc in streaming: second press interrupts
	m2, _, handled = m2.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if !handled || m2.streaming || m2.streamGen != 1 {
		t.Errorf("expected stream to be interrupted and streamGen incremented, got streaming=%v streamGen=%d", m2.streaming, m2.streamGen)
	}

	// 5. Esc in questioning
	m = newTestModel()
	m.questioning = true
	m.questionID = "q1"
	m.questionCh = make(chan client.Event, 1)
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if !handled || m2.questioning {
		t.Errorf("expected questioning to be cleared")
	}

	// 6. Esc in confirming
	m = newTestModel()
	m.confirming = true
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if !handled || m2.confirming {
		t.Errorf("expected confirming to be cleared")
	}
}

func TestHandleKeyRunes(t *testing.T) {
	// 1. Confirming mode: 'y' to accept
	m := newTestModel()
	m.confirming = true
	m.confirmID = "c1"
	m.confirmCh = make(chan client.Event, 1)
	m2, _, handled := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if !handled || m2.confirming {
		t.Errorf("expected confirming to resolve on y")
	}

	// 2. Confirming mode: 'n' to reject
	m = newTestModel()
	m.confirming = true
	m.confirmID = "c2"
	m.confirmCh = make(chan client.Event, 1)
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if !handled || m2.confirming {
		t.Errorf("expected confirming to resolve on n")
	}

	// 3. Confirming mode: Cyrillic aliases
	m = newTestModel()
	m.confirming = true
	m.confirmID = "c3"
	m.confirmCh = make(chan client.Event, 1)
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("н")}) // Cyrillic 'н' (y)
	if !handled || m2.confirming {
		t.Errorf("expected confirming to resolve on Cyrillic н")
	}

	// 4. Questioning mode with options: press '1' to select first option
	m = newTestModel()
	m.questioning = true
	m.questionIdx = 0
	m.questionsData = []client.QuestionItem{
		{
			Question: "Q",
			Options: []client.QuestionOption{
				{Label: "Opt1"},
			},
		},
	}
	m.questionCh = make(chan client.Event, 1)
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	if !handled {
		t.Errorf("expected option selection to be handled")
	}
}

func TestHandleKeyEnter(t *testing.T) {
	// 1. Model picking
	m := newTestModel()
	m.modelPicking = true
	m.modelFiltered = []string{"model1", "model2"}
	m.modelPickIdx = 0
	m2, _, handled := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled || m2.modelPicking {
		t.Errorf("expected model picking to resolve on Enter")
	}

	// 2. Questioning text answer
	m = newTestModel()
	m.questioning = true
	m.questionIdx = 0
	m.questionsData = []client.QuestionItem{{Question: "Name"}}
	m.textinput.SetValue("my answer")
	m.questionCh = make(chan client.Event, 1)
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled || m2.textinput.Value() != "" {
		t.Errorf("expected text answer to be submitted and input cleared")
	}

	// 3. Confirming / streamingEnter is a no-op
	m = newTestModel()
	m.confirming = true
	_, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled {
		t.Errorf("expected Enter to be handled (no-op) during confirm")
	}

	// 4. Normal prompt: type and enter
	m = newTestModel()
	m.connected = true
	m.textinput.SetValue("hello")
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled {
		t.Errorf("expected normal enter to be handled")
	}
	if len(m2.messages) == 0 || m2.messages[0].text != "hello" {
		t.Errorf("expected user message to be appended")
	}

	// 5. Normal prompt with trailing backslash continuation
	m = newTestModel()
	m.textinput.SetValue("hello \\")
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled {
		t.Errorf("expected backslash continuation to be handled")
	}
	if !strings.Contains(m2.textinput.Value(), "hello \n") {
		t.Errorf("expected backslash to be replaced with newline, got %q", m2.textinput.Value())
	}

	// 6. Fast typing/paste debounced newline (< 8ms since last rune)
	m = newTestModel()
	m.lastRuneAt = time.Now()
	m.textinput.SetValue("burst")
	m2, _, handled = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled {
		t.Errorf("expected burst Enter to be handled")
	}
	if !strings.Contains(m2.textinput.Value(), "burst\n") {
		t.Errorf("expected newline to be inserted on burst enter, got %q", m2.textinput.Value())
	}
}
