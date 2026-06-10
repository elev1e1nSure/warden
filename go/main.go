package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	info("запуск frontend...")
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		error("ошибка запуска: " + err.Error())
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	success("frontend завершён")
}
