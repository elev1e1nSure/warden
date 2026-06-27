package tui

import (
	"testing"
	"time"
)

func TestCompactThinkText(t *testing.T) {
	result := compactThinkText("  hello   world  ")
	if result != "hello world" {
		t.Errorf("expected 'hello world', got: %s", result)
	}
}

func TestFormatThinkDuration(t *testing.T) {
	cases := []struct {
		d      time.Duration
		expect string
	}{
		{5 * time.Millisecond, "10ms"},
		{500 * time.Millisecond, "500ms"},
		{1500 * time.Millisecond, "1.5s"},
		{5 * time.Second, "5.0s"},
		{90 * time.Second, "1m30s"},
		{60 * time.Second, "1m"},
	}
	for _, c := range cases {
		got := formatThinkDuration(c.d)
		if got != c.expect {
			t.Errorf("formatThinkDuration(%v) = %s, want %s", c.d, got, c.expect)
		}
	}
}

func TestWrapWords(t *testing.T) {
	lines := wrapWords("hello world foo bar", 10)
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %v", len(lines), lines)
	}
}

func TestWrapWordsEmpty(t *testing.T) {
	lines := wrapWords("", 10)
	if lines != nil {
		t.Errorf("expected nil, got %v", lines)
	}
}

func TestIndentLines(t *testing.T) {
	result := indentLines("a\nb\n", "> ")
	expect := "> a\n> b\n"
	if result != expect {
		t.Errorf("expected %q, got %q", expect, result)
	}
}

func TestIndentLinesEmpty(t *testing.T) {
	result := indentLines("", "> ")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}
