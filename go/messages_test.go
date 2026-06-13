package tui

import (
	"testing"
	"time"
)

func newTestModel() model {
	m := initialModel("test", false)
	m.messages = make([]messageEntry, 0)
	m.streamStart = 0
	return m
}

func TestAppendText(t *testing.T) {
	m := newTestModel()
	m.appendText("hello")
	if len(m.messages) != 1 || m.messages[0].text != "hello" {
		t.Errorf("expected 1 text message, got %v", m.messages)
	}
}

func TestAppendToolActivity(t *testing.T) {
	m := newTestModel()
	m.appendToolActivity("running ls")
	if len(m.messages) != 1 || m.messages[0].kind != messageToolActivity {
		t.Errorf("expected 1 tool activity, got %v", m.messages)
	}
}

func TestAppendToolFlowCollapse(t *testing.T) {
	m := newTestModel()
	m.appendToolFlow("grep", "pattern")
	m.appendToolFlow("grep", "pattern2")
	if len(m.messages) != 1 {
		t.Errorf("expected collapsed to 1 message, got %d", len(m.messages))
	}
	if m.messages[0].toolArgs != "pattern2" {
		t.Errorf("expected updated args, got %s", m.messages[0].toolArgs)
	}
}

func TestAppendThink(t *testing.T) {
	m := newTestModel()
	m.appendThink()
	if len(m.messages) != 1 || m.messages[0].kind != messageThink {
		t.Errorf("expected 1 think message, got %v", m.messages)
	}
}

func TestResetOrAppendThink(t *testing.T) {
	m := newTestModel()
	idx := m.resetOrAppendThink()
	if idx != 0 {
		t.Errorf("expected idx 0, got %d", idx)
	}
	idx2 := m.resetOrAppendThink()
	if idx2 != 0 {
		t.Errorf("expected reuse idx 0, got %d", idx2)
	}
}

func TestUpdateThink(t *testing.T) {
	m := newTestModel()
	m.appendThink()
	m.updateThink(" reasoning")
	if m.messages[0].text != " reasoning" {
		t.Errorf("expected updated think text, got %s", m.messages[0].text)
	}
}

func TestFinishThink(t *testing.T) {
	m := newTestModel()
	m.appendThink()
	time.Sleep(10 * time.Millisecond)
	m.finishThink()
	if m.messages[0].duration == 0 {
		t.Errorf("expected non-zero duration")
	}
}

func TestAppendToLastText(t *testing.T) {
	m := newTestModel()
	m.appendText("hello")
	m.appendToLastText(" world")
	if m.messages[0].text != "hello world" {
		t.Errorf("expected appended text, got %s", m.messages[0].text)
	}
}

func TestAppendAssistant(t *testing.T) {
	m := newTestModel()
	m.appendAssistant("hi")
	if len(m.messages) != 1 || m.messages[0].kind != messageAssistant {
		t.Errorf("expected 1 assistant message, got %v", m.messages)
	}
}

func TestAppendToLastAssistant(t *testing.T) {
	m := newTestModel()
	m.appendAssistant("hi")
	m.appendToLastAssistant(" there")
	if m.messages[0].text != "hi there" {
		t.Errorf("expected appended assistant text, got %s", m.messages[0].text)
	}
}
