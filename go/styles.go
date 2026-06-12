package tui

import "github.com/charmbracelet/lipgloss"

var (
	Green      = lipgloss.Color("#52B788")
	GreenMid   = lipgloss.Color("#2D8A5A")
	GreenFaint = lipgloss.Color("#1A4D34")
	Blue       = lipgloss.Color("#38BDF8")
	BlueMid    = lipgloss.Color("#0EA5E9")
	BlueFaint  = lipgloss.Color("#0C4A6E")
	Yellow     = Blue // alias kept for compat
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
		color = Blue
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

func ToolStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Blue)
}

func SlashNameStyle(active bool) lipgloss.Style {
	if active {
		return AccentStyle()
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#d0d0d0"))
}

func SlashDescStyle(active bool) lipgloss.Style {
	if active {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#585858"))
	}
	return DimStyle()
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
