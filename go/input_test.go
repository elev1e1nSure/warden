package tui

import (
	"testing"
)

func TestExpandPastes(t *testing.T) {
	m := newTestModel()
	m.pastes = []string{"paste1", "paste2"}
	result := m.expandPastes("see [pasted #1, 1 lines] and [pasted #2, 2 lines]")
	if result != "see paste1 and paste2" {
		t.Errorf("expected expanded pastes, got: %s", result)
	}
}

func TestExpandPastesInvalidIndex(t *testing.T) {
	m := newTestModel()
	m.pastes = []string{"paste1"}
	result := m.expandPastes("see [pasted #99, 1 lines]")
	if result != "see [pasted #99, 1 lines]" {
		t.Errorf("expected unchanged placeholder, got: %s", result)
	}
}

func TestRecordHistory(t *testing.T) {
	m := newTestModel()
	m.recordHistory("hello")
	m.recordHistory("hello")
	m.recordHistory("world")
	if len(m.history) != 2 {
		t.Errorf("expected 2 entries (deduped consecutive), got %d", len(m.history))
	}
	if m.historyIdx != 2 {
		t.Errorf("expected historyIdx 2, got %d", m.historyIdx)
	}
}

func TestClearQuestionState(t *testing.T) {
	m := newTestModel()
	m.questioning = true
	m.questionID = "q1"
	m.questionIdx = 1
	m = m.clearQuestionState()
	if m.questioning || m.questionID != "" || m.questionIdx != 0 {
		t.Errorf("expected question state cleared")
	}
}
