package tui

import "github.com/charmbracelet/lipgloss"

var (
	Cyan   = lipgloss.Color("#3CBE71")
	Yellow = lipgloss.Color("#ffcc00")
	Red    = lipgloss.Color("#ff4444")
	Dim    = lipgloss.Color("#666666")
	White  = lipgloss.Color("#ffffff")
	Blue   = lipgloss.Color("#4488ff")
)

func WardenStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(White).Bold(true)
}

func UserStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(White).Bold(true)
}

func DimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Dim)
}

func ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Red)
}

func ToolStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Yellow)
}

func KeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Yellow).Bold(true)
}

func AutoStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Red).Bold(true)
}

func SafeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Blue).Bold(true)
}

func StatusStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Dim)
}

func ThinkingOnStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#ff9966")).Bold(true)
}

func ThinkingOffStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Dim)
}

func TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(White).Bold(true)
}

func ConfirmYStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#4caf7d")).Bold(true)
}

func ConfirmNStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#e05555")).Bold(true)
}
