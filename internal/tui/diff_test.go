package tui

import (
	"strings"
	"testing"
)

func TestRenderUnifiedDiff(t *testing.T) {
	diff := `--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2 modified
 line3`
	result := renderUnifiedDiff(diff, 80)
	if !strings.Contains(result, "line2 modified") {
		t.Errorf("expected modified line, got: %s", result)
	}
	// each body line must be separated by a real newline
	lines := strings.Split(result, "\n")
	if len(lines) < 3 {
		t.Errorf("expected at least 3 body lines, got %d: %q", len(lines), result)
	}
}

func TestRenderUnifiedDiffEmpty(t *testing.T) {
	result := renderUnifiedDiff("", 80)
	// empty diff yields an empty body — just don't panic
	_ = result
}

func TestRenderUnifiedDiffLineNumbers(t *testing.T) {
	diff := `--- a/f.go
+++ b/f.go
@@ -5,1 +5,1 @@
-old
+new`
	result := renderUnifiedDiff(diff, 80)
	if !strings.Contains(result, "5") {
		t.Errorf("expected line number 5, got: %s", result)
	}
}

func TestRenderDiffStats(t *testing.T) {
	prefix, stats := renderDiffStats("edited file.txt  +3 -2")
	if prefix != "edited file.txt" {
		t.Errorf("expected prefix 'edited file.txt', got: %s", prefix)
	}
	if stats == "" {
		t.Errorf("expected non-empty stats, got empty")
	}
}

func TestRenderDiffStatsNoMatch(t *testing.T) {
	prefix, stats := renderDiffStats("no stats here")
	if prefix != "no stats here" {
		t.Errorf("expected unchanged string, got: %s", prefix)
	}
	if stats != "" {
		t.Errorf("expected empty stats, got: %s", stats)
	}
}
