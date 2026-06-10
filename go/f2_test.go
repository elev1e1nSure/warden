package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestF2ExpansionRevealsLatestThoughtsAboveLaterContent(t *testing.T) {
	m := initialModel("qwen3:8b")
	m.width = 80
	m.height = 20
	m.viewport.Width = 80
	m.viewport.Height = 6


	for i := 0; i < 20; i++ {
		m.appendText("old line")
	}
	m.messages = append(m.messages, messageEntry{
		kind:      messageThink,
		text:      strings.Repeat("alpha beta gamma ", 20),
		startedAt: time.Now().Add(-time.Second),
		duration:  time.Second,
	})
	for i := 0; i < 20; i++ {
		m.appendText("later line")
	}

	m.syncViewport()
	m.viewport.GotoBottom()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	got := next.(model)

	if !got.thinkingExpanded {
		t.Fatal("F2 did not expand thoughts")
	}
	if !strings.Contains(got.viewport.View(), "alpha beta gamma") {
		t.Fatalf("expanded thoughts are outside the viewport:\n%s", got.viewport.View())
	}
}

func TestF2StringFallbackExpandsThoughts(t *testing.T) {
	m := initialModel("qwen3:8b")
	m.width = 80
	m.height = 20
	m.viewport.Width = 80
	m.viewport.Height = 6

	m.messages = append(m.messages, messageEntry{
		kind:      messageThink,
		text:      "alpha beta gamma",
		startedAt: time.Now().Add(-time.Second),
		duration:  time.Second,
	})
	m.syncViewport()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f2")})
	got := next.(model)

	if !got.thinkingExpanded {
		t.Fatal("f2 string fallback did not expand thoughts")
	}
	if !strings.Contains(got.viewport.View(), "alpha beta gamma") {
		t.Fatalf("expanded thoughts are not visible:\n%s", got.viewport.View())
	}
}

func TestLateThinkChunksAppendToLatestThoughtEntry(t *testing.T) {
	m := initialModel("qwen3:8b")
	m.width = 80
	m.height = 20
	m.viewport.Width = 80
	m.viewport.Height = 6


	m.appendThink()
	m.appendAssistant("answer already started")
	m.updateThink("late alpha beta gamma")
	m.finishThink()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	got := next.(model)

	if !strings.Contains(got.viewport.View(), "late alpha beta gamma") {
		t.Fatalf("late think chunk was not attached to latest thought entry:\n%s", got.viewport.View())
	}
}
