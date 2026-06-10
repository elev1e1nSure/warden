package tui

import "github.com/charmbracelet/lipgloss"

var (
	Spruce = lipgloss.Color("#3B6B54")
	Yellow = lipgloss.Color("#ffcc00")
	Red    = lipgloss.Color("#ff4444")
	Dim    = lipgloss.Color("#666666")
	Faint  = lipgloss.Color("#444444")
	White  = lipgloss.Color("#ffffff")
	Blue   = lipgloss.Color("#4488ff")
)

func WardenStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Spruce).Bold(true)
}

func HeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(White).Bold(true)
}

func UserStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Dim)
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
	return lipgloss.NewStyle().Foreground(Spruce).Bold(true)
}

func AutoStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Red).Bold(true)
}

func SafeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Spruce).Bold(true)
}

func StatusStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Dim)
}

func ThinkingOnStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Spruce).Bold(true)
}

func ThinkingOffStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Dim)
}

func TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(White).Bold(true)
}

func ModelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(White)
}

func ConfirmYStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#4caf7d")).Bold(true)
}

func ConfirmNStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#e05555")).Bold(true)
}
