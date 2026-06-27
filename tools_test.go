package tui

import (
	"strings"
	"testing"
)

func TestToolDisplayName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"google_search", "Web Search"},
		{"powershell", "Shell"},
		{"custom_tool", "Custom_tool"},
		{"", ""},
	}

	for _, tc := range tests {
		got := toolDisplayName(tc.input)
		if got != tc.expected {
			t.Errorf("toolDisplayName(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		text     string
		limit    int
		expected string
	}{
		{"hello", 0, "hello"},
		{"hello", -1, "hello"},
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"привет", 4, "при…"},
	}

	for _, tc := range tests {
		got := truncateRunes(tc.text, tc.limit)
		if got != tc.expected {
			t.Errorf("truncateRunes(%q, %d) = %q, expected %q", tc.text, tc.limit, got, tc.expected)
		}
	}
}

func TestToolResultIsError(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"Error: something failed", true},
		{"error: message", true},
		{"error message", true},
		{"stderr content", true},
		{"error123 no boundary", false},
		{"Success", false},
		{"", false},
	}

	for _, tc := range tests {
		got := toolResultIsError(tc.input)
		if got != tc.expected {
			t.Errorf("toolResultIsError(%q) = %v, expected %v", tc.input, got, tc.expected)
		}
	}
}

func TestToolSummaryLine(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		result   string
		contains []string
	}{
		{"powershell", "ls -la", "total 0", []string{"Shell", "ls -la", "total 0"}},
		{"powershell", "ls", "Error: failed", []string{"Shell", "ls", "Error: failed"}},
		{"file_read", "path=test.txt", "line1\nline2", []string{"Read", "line1", "+1 lines"}},
		{"edit", "path=test.txt", "edited README.md +5 -2", []string{"Edit", "README.md", "+5", "-2"}},
		{"bash", "echo", "", []string{"Shell", "echo"}},
	}

	for _, tc := range tests {
		got := toolSummaryLine(tc.name, tc.args, tc.result)
		for _, sub := range tc.contains {
			if !strings.Contains(got, sub) {
				t.Errorf("toolSummaryLine(%q, %q, %q) = %q, does not contain %q", tc.name, tc.args, tc.result, got, sub)
			}
		}
	}
}

func TestToolStartLine(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		contains []string
	}{
		{"powershell", "ls -la", []string{"Shell", "ls -la"}},
		{"google_search", "", []string{"Web Search"}},
	}

	for _, tc := range tests {
		got := toolStartLine(tc.name, tc.args)
		for _, sub := range tc.contains {
			if !strings.Contains(got, sub) {
				t.Errorf("toolStartLine(%q, %q) = %q, does not contain %q", tc.name, tc.args, got, sub)
			}
		}
	}
}

func TestToolPastTense(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Web Search", "Searched"},
		{"Read", "Read"},
		{"Write", "Wrote"},
		{"Windows", "Managed window"},
		{"Unknown", "Ran unknown"},
	}

	for _, tc := range tests {
		got := toolPastTense(tc.input)
		if got != tc.expected {
			t.Errorf("toolPastTense(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestToolPresentTense(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Web Search", "Searching"},
		{"Read", "Reading"},
		{"Shell", "Running"},
		{"Unknown", "Running unknown"},
	}

	for _, tc := range tests {
		got := toolPresentTense(tc.input)
		if got != tc.expected {
			t.Errorf("toolPresentTense(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestActionDetailAndExtract(t *testing.T) {
	tests := []struct {
		display  string
		args     string
		expected string
	}{
		{"Fetch", "url=https://example.com/foo", "https://example.com/foo"},
		{"Edit", "file_path=/path/to/my_file.go, other_arg=1", "my_file.go"},
		{"Shell", "cmd=echo 123", "echo 123"},
		{"Shell", "just_raw_text", "just_raw_text"},
		{"", "", ""},
	}

	for _, tc := range tests {
		got := actionDetail(tc.display, tc.args)
		if got != tc.expected {
			t.Errorf("actionDetail(%q, %q) = %q, expected %q", tc.display, tc.args, got, tc.expected)
		}
	}
}

func TestExtractFilenameFromResult(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"edited README.md +5 -2", "README.md"},
		{"patched main.go", "main.go"},
		{"created file.txt", "file.txt"},
		{"deleted d:/Projects/some/file.json", "file.json"},
		{"just_file.txt", "just_file.txt"},
	}

	for _, tc := range tests {
		got := extractFilenameFromResult(tc.input)
		if got != tc.expected {
			t.Errorf("extractFilenameFromResult(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestPathBase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/a/b/c", "c"},
		{"a\\b\\c", "c"},
		{"a", "a"},
		{"", ""},
		{"/", ""},
	}

	for _, tc := range tests {
		got := pathBase(tc.input)
		if got != tc.expected {
			t.Errorf("pathBase(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestRenderToolActivityEntry(t *testing.T) {
	m := newTestModel()
	m.width = 100
	m.loading = true

	// 1. Pending tool entry
	entryPending := messageEntry{
		kind:     messageToolActivity,
		toolDone: false,
		toolName: "powershell",
		toolArgs: "ls",
	}
	resPending := m.renderToolActivityEntry(entryPending, false)
	if !strings.Contains(resPending, "powershell") {
		t.Errorf("expected pending render to contain tool name, got: %q", resPending)
	}

	// 2. Completed tool entry
	entryDone := messageEntry{
		kind:       messageToolActivity,
		toolDone:   true,
		toolName:   "powershell",
		toolArgs:   "ls",
		text:       "→ Shell ls",
		toolResult: "ok\nresult",
	}
	resDone := m.renderToolActivityEntry(entryDone, false)
	if !strings.Contains(resDone, "Shell") {
		t.Errorf("expected completed render to contain tool display name, got: %q", resDone)
	}

	// 3. Completed tool entry expanded (no diff)
	entryDone.expanded = true
	resExpanded := m.renderToolActivityEntry(entryDone, false)
	if !strings.Contains(resExpanded, "ok") || !strings.Contains(resExpanded, "result") {
		t.Errorf("expected expanded render to contain result lines, got: %q", resExpanded)
	}

	// 4. Completed tool entry expanded with diff
	entryDone.toolDiff = "--- a/file\n+++ b/file\n+added line"
	resDiff := m.renderToolActivityEntry(entryDone, false)
	if !strings.Contains(resDiff, "added line") {
		t.Errorf("expected expanded diff render to contain diff content, got: %q", resDiff)
	}
}

func TestRenderChainAction(t *testing.T) {
	m := newTestModel()
	m.loading = true

	entry := messageEntry{
		activity: "running search",
		toolArgs: "query",
	}
	res := m.renderChainAction(entry, true)
	if !strings.Contains(res, "running search") || !strings.Contains(res, "query") {
		t.Errorf("expected active chain action render to contain activity and args, got: %q", res)
	}

	resInactive := m.renderChainAction(entry, false)
	if !strings.Contains(resInactive, "running search") {
		t.Errorf("expected inactive chain action render to contain activity, got: %q", resInactive)
	}
}
