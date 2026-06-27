package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	Green      = lipgloss.Color("#ffffff")
	GreenMid   = lipgloss.Color("#b0b0b0")
	GreenFaint = lipgloss.Color("#3a3a3a")
	Blue       = lipgloss.Color("#5C9CF5")
	BlueMid    = lipgloss.Color("#2F80ED")
	BlueFaint  = lipgloss.Color("#0F3562")
	Red        = lipgloss.Color("#ff4444")
	Dim        = lipgloss.Color("#666666")
	DimHover   = lipgloss.Color("#999999")
	Faint      = lipgloss.Color("#2a2a2a")
	White      = lipgloss.Color("#ffffff")
)

// RGB triples for smooth color interpolation in animations.
var (
	greenRGB      = [3]int{0xFF, 0xFF, 0xFF}
	greenFaintRGB = [3]int{0x3A, 0x3A, 0x3A}
	blueRGB       = [3]int{0x5C, 0x9C, 0xF5}
	blueFaintRGB  = [3]int{0x0F, 0x35, 0x62}
	dimRGB        = [3]int{0x66, 0x66, 0x66}
	whiteRGB      = [3]int{0xFF, 0xFF, 0xFF}
)

// lerpHex linearly interpolates between two RGB triples and returns a
// lipgloss color. t is clamped to [0, 1].
func lerpHex(a, b [3]int, t float64) lipgloss.Color {
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	r := int(float64(a[0]) + float64(b[0]-a[0])*t)
	g := int(float64(a[1]) + float64(b[1]-a[1])*t)
	bl := int(float64(a[2]) + float64(b[2]-a[2])*t)
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", r, g, bl))
}

func AccentStyle() lipgloss.Style {
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

func DimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Dim)
}

func HoverStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(DimHover)
}

func FaintStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Faint)
}

func ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Red)
}

func ToolStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Dim)
}

func SlashNameStyle(active bool, autoMode bool) lipgloss.Style {
	if active {
		return WardenStyleAuto(autoMode)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#d0d0d0"))
}

func SlashDescStyle(active bool) lipgloss.Style {
	if active {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#585858"))
	}
	return DimStyle()
}

func ConfirmYStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#4caf7d")).Bold(true)
}

func ConfirmNStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#e05555")).Bold(true)
}
