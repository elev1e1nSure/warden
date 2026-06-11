package tui

import "github.com/charmbracelet/lipgloss"

var (
	Green      = lipgloss.Color("#52B788")
	GreenMid   = lipgloss.Color("#2D8A5A")
	GreenFaint = lipgloss.Color("#1A4D34")
	Amber      = lipgloss.Color("#D4A576")
	AmberMid   = lipgloss.Color("#A87A4A")
	AmberFaint = lipgloss.Color("#5C3D1E")
	Yellow     = Amber // alias kept for compat
	Red        = lipgloss.Color("#ff4444")
	Dim        = lipgloss.Color("#666666")
	Faint      = lipgloss.Color("#2a2a2a")
	White      = lipgloss.Color("#ffffff")
)

func AccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Green).Bold(true)
}

func WardenStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Green).Bold(true)
}

func WardenStyleAuto(autoMode bool) lipgloss.Style {
	color := Green
	if autoMode {
		color = Amber
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true)
}

func HeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(White).Bold(true)
}

func UserStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Dim).Bold(true)
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

func WardenBgStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2a"))
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
