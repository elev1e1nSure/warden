package tui

import (
	"testing"
)

func TestHandleModelsResult(t *testing.T) {
	m := newTestModel()
	m.modelPicking = false

	// Success case
	m2, _ := m.handleModelsResult(modelsResultMsg{
		models:  []string{"model1", "model2"},
		current: "model2",
	})
	if !m2.modelPicking {
		t.Errorf("expected modelPicking to be true")
	}
	if m2.modelPickIdx != 1 {
		t.Errorf("expected modelPickIdx to match current model, got %d", m2.modelPickIdx)
	}

	// Error or empty case should be no-op
	m = newTestModel()
	m3, _ := m.handleModelsResult(modelsResultMsg{err: "failed"})
	if m3.modelPicking {
		t.Errorf("expected no-op on error")
	}
}

func TestHandleModelSet(t *testing.T) {
	// Redirect config path to a temp dir
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	m := newTestModel()
	m.modelName = "old-model"

	m2, _ := m.handleModelSet(modelSetMsg{
		model: "new-model",
	})
	if m2.modelName != "new-model" {
		t.Errorf("expected modelName to be updated to 'new-model', got %q", m2.modelName)
	}

	// On error, should not update modelName
	m = newTestModel()
	m.modelName = "old-model"
	m3, _ := m.handleModelSet(modelSetMsg{
		model: "new-model",
		err:   "failed",
	})
	if m3.modelName != "old-model" {
		t.Errorf("expected modelName to remain 'old-model' on error")
	}
}

func TestHandleConnectResult(t *testing.T) {
	// Redirect config path to a temp dir
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	m := newTestModel()
	m.connected = false
	m.cwOpen = true
	m.cwLoading = true
	m.cwErr = "some err"

	// Success case
	m2, _ := m.handleConnectResult(connectResultMsg{
		ok:     true,
		model:  "my-model",
		apiURL: "http://api",
		apiKey: "key",
	})
	if !m2.connected {
		t.Errorf("expected connected to be true")
	}
	if m2.modelName != "my-model" {
		t.Errorf("expected modelName to be 'my-model'")
	}
	if m2.cwOpen || m2.cwLoading || m2.cwErr != "" {
		t.Errorf("expected wizard flags and errors to be cleared")
	}

	// Failure case
	m = newTestModel()
	m.connected = false
	m.cwLoading = true
	m3, _ := m.handleConnectResult(connectResultMsg{
		ok:  false,
		err: "failed connection",
	})
	if m3.connected {
		t.Errorf("expected connected to remain false")
	}
	if m3.cwErr != "failed connection" {
		t.Errorf("expected cwErr to contain error message")
	}
	if m3.cwLoading {
		t.Errorf("expected cwLoading to be false")
	}
}
