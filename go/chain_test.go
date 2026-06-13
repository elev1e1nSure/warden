package tui

import (
	"testing"
)

func TestStartChain(t *testing.T) {
	m := newTestModel()
	m.startChain()
	// startChain is now a no-op after counter removal; just ensure no panic
}

func TestSetAction(t *testing.T) {
	m := newTestModel()
	m.setAction("running", "ls", false)
	if len(m.messages) != 1 || m.messages[0].kind != messageChainAction {
		t.Errorf("expected 1 action message")
	}
	m.setAction("fetching", "url", true)
	if len(m.messages) != 1 || m.messages[0].activity != "fetching" {
		t.Errorf("expected action updated in place")
	}
}

func TestClearAction(t *testing.T) {
	m := newTestModel()
	m.setAction("running", "x", false)
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
	m.startChain()
	m.setAction("Thinking", "", true)
	m.freezeChain()
	if len(m.messages) != 0 {
		t.Errorf("expected action line removed after freezeChain")
	}
}
