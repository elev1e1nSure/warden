package tui

import tea "github.com/charmbracelet/bubbletea"

func Run() error {
	info("starting frontend...")
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		logError("startup error: " + err.Error())
		return err
	}
	success("frontend stopped")
	return nil
}
