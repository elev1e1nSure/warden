package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(ts *httptest.Server) *Client {
	return &Client{
		BaseURL:      ts.URL,
		HTTPClient:   ts.Client(),
		StreamClient: ts.Client(),
	}
}

func requireMethodPath(t *testing.T, r *http.Request, method, path string) {
	t.Helper()
	if r.Method != method {
		t.Errorf("expected method %s, got %s", method, r.Method)
	}
	if r.URL.Path != path {
		t.Errorf("expected path %s, got %s", path, r.URL.Path)
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:8765")
	if c.BaseURL != "http://localhost:8765" {
		t.Errorf("expected BaseURL to be set")
	}
	if c.HTTPClient == nil || c.StreamClient == nil {
		t.Errorf("expected HTTP clients to be initialized")
	}
}

func TestResetSession(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		requireMethodPath(t, r, "POST", "/reset")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	if err := newTestClient(ts).ResetSession(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Errorf("expected handler to be called")
	}
}

func TestSetMode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "POST", "/mode")
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"auto":true`) {
			t.Errorf("expected auto=true in body, got %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	if err := newTestClient(ts).SetMode(true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendQuestion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "POST", "/question")
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"id":"q1"`) {
			t.Errorf("expected id=q1 in body, got %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	if err := newTestClient(ts).SendQuestion("q1", [][]string{{"a"}}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendConfirm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "POST", "/confirm")
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"id":"c1"`) || !strings.Contains(string(body), `"ok":true`) {
			t.Errorf("unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	if err := newTestClient(ts).SendConfirm("c1", true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestListModels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "GET", "/models")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models":  []string{"a", "b"},
			"current": "a",
		})
	}))
	defer ts.Close()

	models, current, err := newTestClient(ts).ListModels()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(models) != 2 || models[0] != "a" {
		t.Errorf("unexpected models: %v", models)
	}
	if current != "a" {
		t.Errorf("unexpected current: %s", current)
	}
}

func TestListModelsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []string{},
			"error":  "bad provider",
		})
	}))
	defer ts.Close()

	_, _, err := newTestClient(ts).ListModels()
	if err == nil || err.Error() != "bad provider" {
		t.Errorf("expected error, got %v", err)
	}
}

func TestConnect(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "POST", "/connect")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer ts.Close()

	if err := newTestClient(ts).Connect("openrouter", "url", "key", "model"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConnectError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "nope"})
	}))
	defer ts.Close()

	err := newTestClient(ts).Connect("openrouter", "url", "key", "model")
	if err == nil || err.Error() != "nope" {
		t.Errorf("expected nope error, got %v", err)
	}
}

func TestSetModel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "POST", "/model/set")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	if err := newTestClient(ts).SetModel("model"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "GET", "/status")
		_ = json.NewEncoder(w).Encode(StatusResult{Model: "m", TokenCount: 5, TokenLimit: 10})
	}))
	defer ts.Close()

	s, err := newTestClient(ts).GetStatus()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if s.Model != "m" || s.TokenCount != 5 || s.TokenLimit != 10 {
		t.Errorf("unexpected status: %+v", s)
	}
}

func TestCompact(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "POST", "/compact")
		_ = json.NewEncoder(w).Encode(CompactResult{Summary: "ok", TokensBefore: 100, TokensAfter: 20})
	}))
	defer ts.Close()

	r, err := newTestClient(ts).Compact()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if r.Summary != "ok" || r.TokensAfter != 20 {
		t.Errorf("unexpected compact result: %+v", r)
	}
}

func TestShutdown(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "POST", "/shutdown")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	if err := newTestClient(ts).Shutdown(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMemoryState(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			requireMethodPath(t, r, "GET", "/memory/state")
			_ = json.NewEncoder(w).Encode(MemoryState{Enabled: true, Entries: 3})
		case "POST":
			requireMethodPath(t, r, "POST", "/memory/state")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts)
	s, err := c.GetMemoryState()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !s.Enabled || s.Entries != 3 {
		t.Errorf("unexpected memory state: %+v", s)
	}
	if err := c.SetMemoryState(false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestClearMemory(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "POST", "/memory/clear")
		_ = json.NewEncoder(w).Encode(map[string]int{"cleared": 7})
	}))
	defer ts.Close()

	n, err := newTestClient(ts).ClearMemory()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 7 {
		t.Errorf("expected 7, got %d", n)
	}
}

func TestGetMemorySnapshot(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "GET", "/memory/snapshot")
		_ = json.NewEncoder(w).Encode(map[string]any{"foo": "bar"})
	}))
	defer ts.Close()

	snap, err := newTestClient(ts).GetMemorySnapshot()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if snap["foo"] != "bar" {
		t.Errorf("unexpected snapshot: %v", snap)
	}
}

func TestListSkills(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "GET", "/skills")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"skills": []Skill{{Name: "foo", Description: "d"}},
		})
	}))
	defer ts.Close()

	skills, err := newTestClient(ts).ListSkills()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(skills) != 1 || skills[0].Name != "foo" {
		t.Errorf("unexpected skills: %v", skills)
	}
}

func TestLoadSkill(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireMethodPath(t, r, "GET", "/skill/foo")
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "foo", "content": "body"})
	}))
	defer ts.Close()

	content, err := newTestClient(ts).LoadSkill("foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if content != "body" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestLoadSkillNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	_, err := newTestClient(ts).LoadSkill("missing")
	if err == nil || !strings.Contains(err.Error(), "skill not found") {
		t.Errorf("expected skill not found error, got %v", err)
	}
}

func TestPostOKNon200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	err := newTestClient(ts).ResetSession()
	if err == nil {
		t.Errorf("expected error for non-200")
	}
}

func TestPostDecodeInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "not json")
	}))
	defer ts.Close()

	_, err := newTestClient(ts).Compact()
	if err == nil {
		t.Errorf("expected json decode error")
	}
}

func TestGetJSONInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "bad")
	}))
	defer ts.Close()

	_, err := newTestClient(ts).GetStatus()
	if err == nil {
		t.Errorf("expected json decode error")
	}
}
