package tui

import (
	"fmt"
	"strings"
	"testing"
)

func TestSyncViewportFollowsTail(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)
	m.width = 80
	m.height = 20
	m.viewport.Width = 80
	m.viewport.Height = 5

	for i := 0; i < 40; i++ {
		m.appendText(fmt.Sprintf("line %02d", i))
		m.syncViewport()
	}

	view := m.viewport.View()
	if !strings.Contains(view, "line 39") {
		t.Fatalf("viewport did not follow latest content:\n%s", view)
	}
}

func TestSyncViewportPreservesManualScrollWhenIdle(t *testing.T) {
	m := initialModel(&mockBackend{},"test-model", true)
	m.width = 80
	m.height = 20
	m.viewport.Width = 80
	m.viewport.Height = 5
	m.loading = false // simulate idle state after backend ready

	for i := 0; i < 40; i++ {
		m.appendText(fmt.Sprintf("line %02d", i))
	}
	m.syncViewport()
	m.viewport.SetYOffset(10)

	m.appendText("line 40")
	m.syncViewport()

	view := m.viewport.View()
	if strings.Contains(view, "line 40") {
		t.Fatalf("idle viewport jumped to tail after manual scroll:\n%s", view)
	}
}
