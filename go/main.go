package tui

import tea "github.com/charmbracelet/bubbletea"

func Run(modelName string) error {
	p := tea.NewProgram(initialModel(modelName), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
