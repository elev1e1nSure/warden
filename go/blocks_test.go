package tui

import (
	"strings"
	"testing"
	"time"
	"github.com/elev1e1n/warden/internal/client"
)

func TestRenderConfirmBlock(t *testing.T) {
	msg := confirmMsg{
		tool:    "bash",
		preview: "ls -la",
		summary: "run shell command",
		details: []string{"detail1"},
	}
	result := renderConfirmBlock(msg, 80, false)
	if !strings.Contains(result, "Shell") {
		t.Errorf("expected tool display name in output")
	}
	if !strings.Contains(result, "run") {
		t.Errorf("expected run action in output")
	}
}

func TestRenderQuestionBlock(t *testing.T) {
	q := client.QuestionItem{
		Header:   "Test",
		Question: "What?",
		Options: []client.QuestionOption{
			{Label: "Yes", Description: "Confirm"},
		},
	}
	result := renderQuestionBlock(q, 0, 1, 80, false)
	if !strings.Contains(result, "Test") {
		t.Errorf("expected header in output")
	}
	if !strings.Contains(result, "Yes") {
		t.Errorf("expected option label in output")
	}
}

func TestRenderModelPicker(t *testing.T) {
	models := []string{"gpt-4", "gpt-3.5", "claude"}
	result := renderModelPicker(models, 1, 0, false)
	if !strings.Contains(result, "gpt-4") {
		t.Errorf("expected model names in output")
	}
}

func TestRenderThinkEntryHasLeadingIndent(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)
	got := m.renderThinkEntry(messageEntry{duration: 2 * time.Second}, false, false)

	// think summary is indented to align with assistant text (column 2)
	if !strings.Contains(got, "  + Thought:") && !strings.Contains(got, "  - Thought:") {
		t.Fatalf("expected leading indent in think line, got %q", got)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func TestRenderChainActionHasNoLeadingIndent(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)
	m.loading = true
	got := m.renderChainAction(messageEntry{activity: "Thinking"}, true)

	stripped := stripANSI(got)
	// live line is "<orb> Thinking": orb fills the indent slot, text at column 2
	if !strings.Contains(stripped, "Thinking") {
		t.Fatalf("expected chain action text, got %q", got)
	}
	if strings.Contains(stripped, "  Thinking") {
		t.Fatalf("expected orb in indent slot, not double space, got %q", got)
	}
}
