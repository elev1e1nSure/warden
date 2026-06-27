package tui

import (
	"strings"
	"testing"
	"github.com/elev1e1n/warden/internal/client"
)

func TestHandleNextMsg_WardenStart(t *testing.T) {
	m := newTestModel()
	msg := nextMsg{
		inner: wardenStartMsg{},
		ch:    make(chan client.Event),
	}

	m2, _ := m.handleNextMsg(msg)
	if !m2.loading {
		t.Errorf("expected loading to be true")
	}
	if len(m2.messages) != 2 || m2.messages[0].kind != messageThink || m2.messages[1].kind != messageChainAction {
		t.Errorf("expected think entry and chain action entry to be appended, got %+v", m2.messages)
	}
}

func TestHandleNextMsg_Think(t *testing.T) {
	m := newTestModel()
	m.messages = append(m.messages, messageEntry{kind: messageThink, text: ""})
	msg := nextMsg{
		inner: thinkMsg{text: "reasoning"},
		ch:    make(chan client.Event),
	}

	m2, _ := m.handleNextMsg(msg)
	if m2.thinkBuf != "reasoning" {
		t.Errorf("expected thinkBuf to be 'reasoning', got %q", m2.thinkBuf)
	}
	if len(m2.messages) != 2 || m2.messages[0].text != "reasoning" {
		t.Errorf("expected message text to update, got %+v", m2.messages)
	}
}

func TestHandleNextMsg_Token(t *testing.T) {
	m := newTestModel()
	m.thinkDone = false
	m.messages = append(m.messages, messageEntry{kind: messageThink, text: "thought"})
	msg := nextMsg{
		inner: tokenMsg{text: "hello"},
		ch:    make(chan client.Event),
	}

	m2, _ := m.handleNextMsg(msg)
	if !m2.thinkDone {
		t.Errorf("expected thinkDone to be true")
	}
	if len(m2.messages) != 2 {
		t.Fatalf("expected 2 messages (think and assistant), got %d", len(m2.messages))
	}
	if m2.messages[1].kind != messageAssistant || m2.messages[1].text != "hello" {
		t.Errorf("expected assistant message, got %+v", m2.messages[1])
	}
}

func TestHandleNextMsg_ToolStartAndTool(t *testing.T) {
	// ToolStartMsg
	m := newTestModel()
	m.messages = append(m.messages, messageEntry{kind: messageThink, text: "thought"})
	msgStart := nextMsg{
		inner: toolStartMsg{name: "powershell", args: "ls"},
		ch:    make(chan client.Event),
	}
	m2, _ := m.handleNextMsg(msgStart)
	if !m2.toolRunning {
		t.Errorf("expected toolRunning to be true")
	}

	// ToolMsg with Diff
	msgToolWithDiff := nextMsg{
		inner: toolMsg{
			tool: client.ToolMsg{
				Name:   "edit",
				Args:   "file.txt",
				Result: "edited",
				Diff:   "--- diff",
			},
		},
		ch: make(chan client.Event),
	}
	m3, _ := m2.handleNextMsg(msgToolWithDiff)
	if m3.toolRunning {
		t.Errorf("expected toolRunning to be false")
	}
	// Check if a tool activity message with diff is appended
	foundToolActivity := false
	for _, msg := range m3.messages {
		if msg.kind == messageToolActivity {
			foundToolActivity = true
			if msg.toolDiff != "--- diff" {
				t.Errorf("expected toolDiff to be set, got %q", msg.toolDiff)
			}
		}
	}
	if !foundToolActivity {
		t.Errorf("expected tool activity message to be appended")
	}
}

func TestHandleNextMsg_ConfirmAndQuestion(t *testing.T) {
	// ConfirmMsg
	m := newTestModel()
	msgConfirm := nextMsg{
		inner: confirmMsg{
			id:         "c1",
			tool:       "shell",
			risk:       "high",
			title:      "Confirm",
			summary:    "Run cmd",
			defaultVal: "y",
		},
		ch: make(chan client.Event),
	}
	m2, _ := m.handleNextMsg(msgConfirm)
	if !m2.confirming {
		t.Errorf("expected confirming to be true")
	}
	if m2.confirmID != "c1" || m2.confirmTool != "shell" {
		t.Errorf("expected confirm meta to be populated, got %+v", m2)
	}
	if m2.textinput.Value() != "y" {
		t.Errorf("expected defaultVal to be pre-filled in input, got %q", m2.textinput.Value())
	}

	// QuestionMsg
	m = newTestModel()
	msgQuestion := nextMsg{
		inner: questionMsg{
			id: "q1",
			questions: []client.QuestionItem{
				{Question: "Q1"},
			},
		},
		ch: make(chan client.Event),
	}
	m3, _ := m.handleNextMsg(msgQuestion)
	if !m3.questioning {
		t.Errorf("expected questioning to be true")
	}
	if len(m3.questionsData) != 1 || m3.questionID != "q1" {
		t.Errorf("expected question data to be populated")
	}
}

func TestHandleNextMsg_StaleGenIgnored(t *testing.T) {
	m := newTestModel()
	m.streamGen = 2
	msg := nextMsg{
		inner: wardenStartMsg{},
		ch:    make(chan client.Event),
		gen:   1, // stale
	}
	m2, cmd := m.handleNextMsg(msg)
	if cmd != nil {
		t.Errorf("expected no follow-up command for stale event")
	}
	if len(m2.messages) != 0 {
		t.Errorf("expected no messages appended for stale event, got %+v", m2.messages)
	}
}

func TestHandleStartStreamMsg_StaleGenIgnored(t *testing.T) {
	m := newTestModel()
	m.streamGen = 2
	msg := startStreamMsg{ch: make(chan client.Event), gen: 1}
	_, cmd := m.handleStartStreamMsg(msg)
	if cmd != nil {
		t.Errorf("expected no command for stale startStreamMsg")
	}
}

func TestHandleDoneMsg_StaleGenIgnored(t *testing.T) {
	m := newTestModel()
	m.streaming = true
	m.loading = true
	m.streamGen = 2
	msg := doneMsg{tokenCount: 10, tokenLimit: 100, gen: 1}
	m2, _ := m.handleDoneMsg(msg)
	if !m2.streaming || !m2.loading {
		t.Errorf("expected streaming/loading to remain for stale doneMsg")
	}
}

func TestBeginStream_ResetsInterruptState(t *testing.T) {
	m := newTestModel()
	m.streaming = true
	m.escPending = true
	m.quitPending = true
	m.streamGen = 3

	cmd := m.beginStream("hello")
	if cmd == nil {
		t.Fatalf("expected command")
	}
	if m.streamGen != 4 {
		t.Errorf("expected streamGen to increment, got %d", m.streamGen)
	}
	if !m.streaming || !m.loading || m.escPending || m.quitPending {
		t.Errorf("expected streaming/loading true and pending flags reset, got streaming=%v loading=%v escPending=%v quitPending=%v", m.streaming, m.loading, m.escPending, m.quitPending)
	}
}

func TestHandleShellResult(t *testing.T) {
	m := newTestModel()
	m.streaming = true
	m.loading = true
	m.toolRunning = true
	msg := shellResultMsg{output: "output\n"}
	m2, _ := m.handleShellResult(msg)

	if m2.streaming || m2.loading || m2.toolRunning {
		t.Errorf("expected streaming, loading, and toolRunning to be false")
	}
	foundOutput := false
	for _, entry := range m2.messages {
		if strings.Contains(entry.text, "output") {
			foundOutput = true
		}
	}
	if !foundOutput {
		t.Errorf("expected output to be appended to messages")
	}
}

func TestHandleTick(t *testing.T) {
	m := newTestModel()
	m.loading = true
	m.spinner = 0
	msg := tickMsg{}

	m2, cmd := m.handleTick(msg)
	if m2.spinner != 1 {
		t.Errorf("expected spinner to increment, got %d", m2.spinner)
	}
	if cmd == nil {
		t.Errorf("expected tick command to be returned when loading")
	}

	m = newTestModel()
	m.loading = false
	_, cmd = m.handleTick(msg)
	if cmd != nil {
		t.Errorf("expected no tick command when not loading")
	}
}
