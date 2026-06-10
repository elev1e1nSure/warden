package tui

import "github.com/charmbracelet/lipgloss"

var (
	Green      = lipgloss.Color("#00D47A")
	GreenMid   = lipgloss.Color("#00904F")
	GreenFaint = lipgloss.Color("#004D2A")
	Amber      = lipgloss.Color("#D4A576")
	Yellow     = Amber // alias kept for compat
	Red        = lipgloss.Color("#ff4444")
	Dim        = lipgloss.Color("#666666")
	Faint      = lipgloss.Color("#444444")
	White      = lipgloss.Color("#ffffff")
)

func AccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Green).Bold(true)
}

func WardenStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Green).Bold(true)
}

func HeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(White).Bold(true)
}

func UserStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Yellow).Bold(true)
}

func DimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Dim)
}

func FaintStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Faint)
}

func ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Red)
}

func ToolStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Yellow)
}

func KeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Green).Bold(true)
}

func ConfirmYStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#4caf7d")).Bold(true)
}

func ConfirmNStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#e05555")).Bold(true)
}
