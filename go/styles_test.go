package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestAccentStyle(t *testing.T) {
	s := AccentStyle()
	if s.GetForeground() != lipgloss.Color("#8AB89A") {
		t.Errorf("unexpected foreground color")
	}
}

func TestWardenStyleAuto(t *testing.T) {
	s := WardenStyleAuto(true)
	if s.GetForeground() != Blue {
		t.Errorf("expected Blue for auto mode")
	}
	s = WardenStyleAuto(false)
	if s.GetForeground() != Green {
		t.Errorf("expected Green for non-auto mode")
	}
}

func TestHeaderStyle(t *testing.T) {
	s := HeaderStyle()
	if s.GetForeground() != White {
		t.Errorf("expected White")
	}
}

func TestDimStyle(t *testing.T) {
	s := DimStyle()
	if s.GetForeground() != Dim {
		t.Errorf("expected Dim")
	}
}

func TestFaintStyle(t *testing.T) {
	s := FaintStyle()
	if s.GetForeground() != Faint {
		t.Errorf("expected Faint")
	}
}

func TestErrorStyle(t *testing.T) {
	s := ErrorStyle()
	if s.GetForeground() != Red {
		t.Errorf("expected Red")
	}
}

func TestToolStyle(t *testing.T) {
	s := ToolStyle()
	if s.GetForeground() != Dim {
		t.Errorf("expected Dim")
	}
}

func TestSlashNameStyle(t *testing.T) {
	s := SlashNameStyle(true, false)
	if s.GetForeground() != Green {
		t.Errorf("expected Green for active")
	}
	s = SlashNameStyle(false, false)
	if s.GetForeground() != lipgloss.Color("#d0d0d0") {
		t.Errorf("expected inactive color")
	}
}

func TestSlashDescStyle(t *testing.T) {
	s := SlashDescStyle(true)
	if s.GetForeground() != lipgloss.Color("#585858") {
		t.Errorf("unexpected active desc color")
	}
	s = SlashDescStyle(false)
	if s.GetForeground() != Dim {
		t.Errorf("expected Dim for inactive")
	}
}

func TestConfirmStyles(t *testing.T) {
	sy := ConfirmYStyle()
	if sy.GetForeground() != lipgloss.Color("#4caf7d") {
		t.Errorf("unexpected confirm-y color")
	}
	sn := ConfirmNStyle()
	if sn.GetForeground() != lipgloss.Color("#e05555") {
		t.Errorf("unexpected confirm-n color")
	}
}
