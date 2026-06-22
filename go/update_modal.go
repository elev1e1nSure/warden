package tui

import tea "github.com/charmbracelet/bubbletea"

func (m *model) handleModelsResult(msg modelsResultMsg) (*model, tea.Cmd) {
	if msg.err != "" || len(msg.models) == 0 {
		return m, nil
	}
	m.modelList = msg.models
	m.modelFiltered = msg.models
	m.modelPickIdx = 0
	for i, name := range msg.models {
		if name == msg.current {
			m.modelPickIdx = i
			break
		}
	}
	const maxVisible = 8
	m.modelScrollTop = m.modelPickIdx - maxVisible/2
	if m.modelScrollTop < 0 {
		m.modelScrollTop = 0
	}
	if m.modelScrollTop+maxVisible > len(msg.models) {
		m.modelScrollTop = len(msg.models) - maxVisible
		if m.modelScrollTop < 0 {
			m.modelScrollTop = 0
		}
	}
	m.resetInput()
	m.modelPicking = true
	m.updateViewportHeight()
	m.syncViewport()
	return m, nil
}

func (m *model) handleModelSet(msg modelSetMsg) (*model, tea.Cmd) {
	if msg.err == "" {
		m.modelName = msg.model
		m.messages = []messageEntry{}
		_ = saveWardenConfigField("model", msg.model)
	}
	return m, nil
}

func (m *model) handleConnectResult(msg connectResultMsg) (*model, tea.Cmd) {
	if msg.ok {
		m.connected = true
		m.modelName = msg.model
		m.cwOpen = false
		m.cwLoading = false
		m.cwErr = ""
		_ = saveWardenConfigField("model", msg.model)
		if msg.apiURL != "" {
			_ = saveWardenConfigField("api_url", msg.apiURL)
		}
		if msg.apiKey != "" {
			_ = saveWardenConfigField("api_key", msg.apiKey)
		}
	} else {
		m.cwErr = msg.err
		m.cwLoading = false
	}
	m.updateViewportHeight()
	m.syncViewport()
	return m, nil
}
