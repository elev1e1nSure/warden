package tui

import (
	"strings"
	"testing"
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
	q := QuestionItem{
		Header:   "Test",
		Question: "What?",
		Options: []QuestionOption{
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
