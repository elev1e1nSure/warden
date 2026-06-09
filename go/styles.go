package main

import "github.com/charmbracelet/lipgloss"

var (
	Cyan   = lipgloss.Color("#3CBE71")
	Yellow = lipgloss.Color("#ffcc00")
	Red    = lipgloss.Color("#ff4444")
	Dim    = lipgloss.Color("#666666")
	White  = lipgloss.Color("#ffffff")
	Bg     = lipgloss.Color("#0d0d0d")
	Blue   = lipgloss.Color("#4488ff")
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

func KeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Yellow).Bold(true)
}

func AutoStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Red).Background(lipgloss.Color("#330000")).Bold(true)
}

func SafeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Blue).Background(lipgloss.Color("#001133")).Bold(true)
}

func StatusStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Dim)
}
