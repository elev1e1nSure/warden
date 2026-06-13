package tui

import (
	"testing"
)

func TestStartChain(t *testing.T) {
	m := newTestModel()
	m.startChain()
	if m.chainCounts == nil {
		t.Errorf("expected chainCounts initialized")
	}
}

func TestBumpChain(t *testing.T) {
	m := newTestModel()
	m.bumpChain("search")
	m.bumpChain("search")
	m.bumpChain("fetch")
	if m.chainCounts["search"] != 2 {
		t.Errorf("expected search count 2, got %d", m.chainCounts["search"])
	}
	if m.chainCounts["fetch"] != 1 {
		t.Errorf("expected fetch count 1, got %d", m.chainCounts["fetch"])
	}
}

func TestCounterIdx(t *testing.T) {
	m := newTestModel()
	if m.counterIdx() != -1 {
		t.Errorf("expected -1 for empty messages")
	}
	m.messages = append(m.messages, messageEntry{kind: messageChainCounter})
	if m.counterIdx() != 0 {
		t.Errorf("expected 0, got %d", m.counterIdx())
	}
}

func TestEnsureCounter(t *testing.T) {
	m := newTestModel()
	m.ensureCounter()
	if m.counterIdx() != 0 {
		t.Errorf("expected counter at 0")
	}
	m.ensureCounter()
	if len(m.messages) != 1 {
		t.Errorf("expected only 1 counter message")
	}
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
	m.ensureCounter()
	m.freezeChain()
	if m.counterIdx() >= 0 {
		t.Errorf("expected counter removed when no tools ran")
	}
}
