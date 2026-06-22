package tui

import tea "github.com/charmbracelet/bubbletea"

func (m model) handleModeMsg(msg modeMsg) (model, tea.Cmd) {
	m.autoMode = msg.auto
	if msg.err != nil {
		m.appendText(ErrorStyle().Render("  failed to save auto mode: " + msg.err.Error()))
		m.appendText("")
	}
	m.syncViewport()
	return m, nil
}

func (m model) handleStatusResult(msg statusResultMsg) (model, tea.Cmd) {
	if msg.tokenLimit > 0 {
		m.tokenCount = msg.tokenCount
		m.tokenLimit = msg.tokenLimit
	}
	if msg.model != "" {
		m.modelName = msg.model
	}
	m.syncViewport()
	return m, nil
}

func (m model) handleClipboardDone(msg clipboardDoneMsg) (model, tea.Cmd) {
	m.syncViewport()
	return m, nil
}

func (m model) handleCompactResult(msg compactResultMsg) (model, tea.Cmd) {
	m.loading = false
	if msg.err == "" {
		m.tokenCount = msg.tokensAfter
	}
	m.syncViewport()
	return m, nil
}

func (m model) handleMemoryResult(msg memoryResultMsg) (model, tea.Cmd) {
	m.loading = false
	if msg.err != "" {
		m.appendText(ErrorStyle().Render("  memory error: " + msg.err))
	} else {
		m.appendText(DimStyle().Render("  " + msg.text))
	}
	m.appendText("")
	m.syncViewport()
	return m, nil
}

func (m model) handleUpdateResult(msg updateResultMsg) (model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.appendText(ErrorStyle().Render("  update failed: " + msg.err.Error()))
		m.appendText("")
		m.syncViewport()
		return m, nil
	}
	m.appendText(DimStyle().Render("  update downloaded, restarting..."))
	m.syncViewport()
	return m, tea.Quit
}

func (m model) handleBackendReady(msg backendReadyMsg) (model, tea.Cmd) {
	m.loading = false
	m.tokenCount = 0
	m.client.ResetSession()
	m.syncViewport()
	if m.autoMode {
		return m, m.setMode(true)
	}
	return m, nil
}

func (m model) handleBackendError(msg backendErrorMsg) (model, tea.Cmd) {
	m.loading = false
	m.syncViewport()
	return m, nil
}

func (m model) handleSkillsResult(msg skillsResultMsg) (model, tea.Cmd) {
	if msg.err != "" {
		m.skillsErr = msg.err
	} else {
		m.skills = msg.skills
		m.skillsErr = ""
	}
	m.syncViewport()
	return m, nil
}

func (m model) handleSkillLoaded(msg skillLoadedMsg) (model, tea.Cmd) {
	m.streaming = false
	m.loading = false
	if msg.err != "" {
		m.appendText(ErrorStyle().Render("  " + msg.err))
		m.appendText("")
		m.syncViewport()
		return m, nil
	}
	body := "Use the skill \"" + msg.name + "\". Follow these instructions:\n\n" + msg.content
	m.appendText(body)
	return m, m.beginStream(body)
}
