package tui

import (
	"testing"
)

func TestSetAction(t *testing.T) {
	m := newTestModel()
	m.setAction("running", "ls")
	if len(m.messages) != 1 || m.messages[0].kind != messageChainAction {
		t.Errorf("expected 1 action message")
	}
	m.setAction("fetching", "url")
	if len(m.messages) != 1 || m.messages[0].activity != "fetching" {
		t.Errorf("expected action updated in place")
	}
}

func TestClearAction(t *testing.T) {
	m := newTestModel()
	m.setAction("running", "x")
	if !m.clearAction() {
		t.Errorf("expected clearAction to return true")
	}
	if len(m.messages) != 0 {
		t.Errorf("expected 0 messages after clear")
	}
	if m.clearAction() {
		t.Errorf("expected clearAction to return false when empty")
	}
}

func TestFreezeChain(t *testing.T) {
	m := newTestModel()
	m.setAction("Thinking", "")
	m.freezeChain()
	if len(m.messages) != 0 {
		t.Errorf("expected action line removed after freezeChain")
	}
}
