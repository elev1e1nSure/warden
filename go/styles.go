package main

import "github.com/charmbracelet/lipgloss"

var (
	Cyan   = lipgloss.Color("#00ffff")
	Yellow = lipgloss.Color("#ffcc00")
	Red    = lipgloss.Color("#ff4444")
	Dim    = lipgloss.Color("#666666")
	White  = lipgloss.Color("#ffffff")
	Bg     = lipgloss.Color("#0d0d0d")
)

func WardenStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Cyan).Bold(true)
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
