package tui

import tea "github.com/charmbracelet/bubbletea"

func Run(modelName string, connected bool) error {
	p := tea.NewProgram(initialModel(modelName, connected), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
