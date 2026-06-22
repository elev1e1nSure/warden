package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStreamChat_Success(t *testing.T) {
	events := []map[string]any{
		{"type": "warden_start"},
		{"type": "think", "text": "let me think"},
		{"type": "token", "text": "hello"},
		{"type": "tool_start", "name": "shell", "args": "ls"},
		{"type": "tool", "name": "shell", "args": "ls", "result": "ok", "diff": "none"},
		{
			"type":    "confirm",
			"id":      "c123",
			"tool":    "shell",
			"risk":    "high",
			"title":   "Run command",
			"summary": "Run ls",
			"details": []string{"detail1"},
			"args":    "ls",
			"preview": "ls preview",
			"default": "y",
		},
		{
			"type": "question",
			"id":   "q123",
			"questions": []map[string]any{
				{
					"question": "what is your name?",
					"header":   "Name",
					"multiple": false,
					"options": []map[string]string{
						{"label": "a", "description": "desc a"},
					},
				},
			},
		},
		{"type": "done", "token_count": 10, "token_limit": 100},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json-seq")
		w.WriteHeader(http.StatusOK)
		for _, ev := range events {
			b, _ := json.Marshal(ev)
			_, _ = w.Write(b)
			_, _ = w.Write([]byte("\n"))
		}
	}))
	defer ts.Close()

	c := newTestClient(ts)
	ch := c.StreamChat(map[string]string{"prompt": "hello"})

	var received []Event
	for ev := range ch {
		received = append(received, ev)
	}

	if len(received) != 8 {
		t.Fatalf("expected 8 events, got %d", len(received))
	}

	if _, ok := received[0].(EventWardenStart); !ok {
		t.Errorf("expected EventWardenStart, got %T", received[0])
	}
	if ev, ok := received[1].(EventThink); !ok || ev.Text != "let me think" {
		t.Errorf("expected EventThink with text, got %+v", received[1])
	}
	if ev, ok := received[2].(EventToken); !ok || ev.Text != "hello" {
		t.Errorf("expected EventToken with text, got %+v", received[2])
	}
	if ev, ok := received[3].(EventToolStart); !ok || ev.Name != "shell" || ev.Args != "ls" {
		t.Errorf("expected EventToolStart, got %+v", received[3])
	}
	if ev, ok := received[4].(EventTool); !ok || ev.Tool.Name != "shell" || ev.Tool.Result != "ok" {
		t.Errorf("expected EventTool, got %+v", received[4])
	}
	if ev, ok := received[5].(EventConfirm); !ok || ev.ID != "c123" || ev.DefaultVal != "y" {
		t.Errorf("expected EventConfirm, got %+v", received[5])
	}
	if ev, ok := received[6].(EventQuestion); !ok || ev.ID != "q123" || len(ev.Questions) != 1 || ev.Questions[0].Question != "what is your name?" {
		t.Errorf("expected EventQuestion, got %+v", received[6])
	}
	if ev, ok := received[7].(EventDone); !ok || ev.TokenCount != 10 || ev.TokenLimit != 100 {
		t.Errorf("expected EventDone, got %+v", received[7])
	}
}

func TestStreamChat_ErrorLineAndMalformed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json-seq")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"type": "error", "text": "something went wrong"}` + "\n"))
		_, _ = w.Write([]byte(`invalid json` + "\n"))
		_, _ = w.Write([]byte(`{"type": "unknown_type"}` + "\n"))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	ch := c.StreamChat(map[string]string{"prompt": "hello"})

	var received []Event
	for ev := range ch {
		received = append(received, ev)
	}

	// Should receive EventError, then EventDone (from error event handling which returns EventDone),
	// and skipped invalid json/unknown types.
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
	if ev, ok := received[0].(EventError); !ok || ev.Text != "something went wrong" {
		t.Errorf("expected EventError, got %+v", received[0])
	}
	if _, ok := received[1].(EventDone); !ok {
		t.Errorf("expected EventDone, got %+v", received[1])
	}
}

func TestStreamChat_NetworkError(t *testing.T) {
	// Point to an invalid server or closed server
	c := NewClient("http://localhost:9999") // no server running here

	ch := c.StreamChat(map[string]string{"prompt": "hello"})
	var received []Event
	for ev := range ch {
		received = append(received, ev)
	}

	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
	if _, ok := received[0].(EventError); !ok {
		t.Errorf("expected EventError, got %T", received[0])
	}
	if _, ok := received[1].(EventDone); !ok {
		t.Errorf("expected EventDone, got %T", received[1])
	}
}

func TestStreamChat_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	ch := c.StreamChat(map[string]string{"prompt": "hello"})

	var received []Event
	for ev := range ch {
		received = append(received, ev)
	}

	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
	if _, ok := received[0].(EventError); !ok {
		t.Errorf("expected EventError, got %T", received[0])
	}
	if _, ok := received[1].(EventDone); !ok {
		t.Errorf("expected EventDone, got %T", received[1])
	}
}
