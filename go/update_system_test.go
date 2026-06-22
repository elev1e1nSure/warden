package tui

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"warden/internal/client"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleModeMsg(t *testing.T) {
	m := newTestModel()
	m.autoMode = false

	m2, _ := m.handleModeMsg(modeMsg{auto: true})
	if !m2.autoMode {
		t.Errorf("expected autoMode to be true")
	}
}

func TestHandleStatusResult(t *testing.T) {
	m := newTestModel()
	m2, _ := m.handleStatusResult(statusResultMsg{
		model:      "new-model",
		tokenCount: 50,
		tokenLimit: 100,
	})

	if m2.modelName != "new-model" {
		t.Errorf("expected modelName to be 'new-model', got %q", m2.modelName)
	}
	if m2.tokenCount != 50 || m2.tokenLimit != 100 {
		t.Errorf("expected token counts to update, got %d/%d", m2.tokenCount, m2.tokenLimit)
	}
}

func TestHandleClipboardDone(t *testing.T) {
	m := newTestModel()
	// Should not crash, just returns (m, nil)
	m2, cmd := m.handleClipboardDone(clipboardDoneMsg{})
	if cmd != nil {
		t.Errorf("expected nil command")
	}
	if m2.width != m.width {
		t.Errorf("expected no state changes")
	}
}

func TestHandleCompactResult(t *testing.T) {
	m := newTestModel()
	m.loading = true
	m.tokenCount = 100

	m2, _ := m.handleCompactResult(compactResultMsg{tokensAfter: 20})
	if m2.loading {
		t.Errorf("expected loading to be false")
	}
	if m2.tokenCount != 20 {
		t.Errorf("expected tokenCount to be 20, got %d", m2.tokenCount)
	}

	// Error case
	m = newTestModel()
	m.loading = true
	m.tokenCount = 100
	m2, _ = m.handleCompactResult(compactResultMsg{err: "failed", tokensAfter: 20})
	if m2.tokenCount != 100 {
		t.Errorf("expected tokenCount to remain 100 on error, got %d", m2.tokenCount)
	}
}

func TestHandleMemoryResult(t *testing.T) {
	m := newTestModel()
	m.loading = true

	// Success case
	m2, _ := m.handleMemoryResult(memoryResultMsg{text: "some memory message"})
	if m2.loading {
		t.Errorf("expected loading to be false")
	}
	foundSuccessText := false
	for _, entry := range m2.messages {
		if strings.Contains(entry.text, "some memory message") {
			foundSuccessText = true
		}
	}
	if !foundSuccessText {
		t.Errorf("expected success text in messages")
	}

	// Error case
	m = newTestModel()
	m.loading = true
	m3, _ := m.handleMemoryResult(memoryResultMsg{err: "some error"})
	foundErrorText := false
	for _, entry := range m3.messages {
		if strings.Contains(entry.text, "memory error: some error") {
			foundErrorText = true
		}
	}
	if !foundErrorText {
		t.Errorf("expected error text in messages")
	}
}

func TestHandleUpdateResult(t *testing.T) {
	// Success case returns tea.Quit
	m := newTestModel()
	m.loading = true
	m2, cmd := m.handleUpdateResult(updateResultMsg{})
	if m2.loading {
		t.Errorf("expected loading to be false")
	}
	if cmd == nil {
		t.Fatalf("expected command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.Quit command, got %T", cmd())
	}

	// Error case does not return tea.Quit
	m = newTestModel()
	m.loading = true
	m3, cmd := m.handleUpdateResult(updateResultMsg{err: errors.New("update failed")})
	if m3.loading {
		t.Errorf("expected loading to be false")
	}
	if cmd != nil {
		t.Errorf("expected nil command on update error, got %T", cmd())
	}
}

func TestHandleBackendReady(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	m := newTestModel()
	m.autoMode = false
	m.loading = true
	m.tokenCount = 100
	m.client = &client.Client{
		BaseURL:      ts.URL,
		HTTPClient:   ts.Client(),
		StreamClient: ts.Client(),
	}

	m2, cmd := m.handleBackendReady(backendReadyMsg{})
	if m2.loading {
		t.Errorf("expected loading to be false")
	}
	if m2.tokenCount != 0 {
		t.Errorf("expected tokenCount to be reset to 0")
	}
	if cmd != nil {
		t.Errorf("expected no command returned by default when not in autoMode")
	}

	// In autoMode, it should return a setMode command
	m = newTestModel()
	m.autoMode = true
	m.client = &client.Client{
		BaseURL:      ts.URL,
		HTTPClient:   ts.Client(),
		StreamClient: ts.Client(),
	}
	_, cmd = m.handleBackendReady(backendReadyMsg{})
	if cmd == nil {
		t.Errorf("expected setMode command when autoMode is true")
	}
}

func TestHandleBackendError(t *testing.T) {
	m := newTestModel()
	m.loading = true

	m2, _ := m.handleBackendError(backendErrorMsg{})
	if m2.loading {
		t.Errorf("expected loading to be false")
	}
}

func TestHandleSkillsResult(t *testing.T) {
	m := newTestModel()
	m.skillsErr = "old err"

	// Success sets skills and clears skillsErr
	m2, _ := m.handleSkillsResult(skillsResultMsg{
		skills: []client.Skill{{Name: "test-skill"}},
	})
	if len(m2.skills) != 1 || m2.skills[0].Name != "test-skill" {
		t.Errorf("expected skills to be updated")
	}
	if m2.skillsErr != "" {
		t.Errorf("expected skillsErr to be cleared")
	}

	// Error sets skillsErr
	m3, _ := m.handleSkillsResult(skillsResultMsg{err: "new error"})
	if m3.skillsErr != "new error" {
		t.Errorf("expected skillsErr to be 'new error', got %q", m3.skillsErr)
	}
}

func TestHandleSkillLoaded(t *testing.T) {
	m := newTestModel()
	m.streaming = true
	m.loading = true

	// Error case
	m2, cmd := m.handleSkillLoaded(skillLoadedMsg{err: "not found"})
	if m2.streaming || m2.loading {
		t.Errorf("expected streaming/loading to be false on error")
	}
	if cmd != nil {
		t.Errorf("expected nil command on error")
	}
	foundErr := false
	for _, msg := range m2.messages {
		if strings.Contains(msg.text, "not found") {
			foundErr = true
		}
	}
	if !foundErr {
		t.Errorf("expected error message to be appended")
	}

	// Success case starts stream
	m = newTestModel()
	m.streaming = true
	m.loading = true
	_, cmd = m.handleSkillLoaded(skillLoadedMsg{name: "my-skill", content: "do something"})
	if cmd == nil {
		t.Errorf("expected beginStream command on success")
	}
}
