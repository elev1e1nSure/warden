package tui

import (
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleKeyUp(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	if m.handleSlashNavigation(msg) {
		return m, nil, true
	}
	if m.handleBangNavigation(msg) {
		return m, nil, true
	}
	if m.modelPicking {
		if m.modelPickIdx > 0 {
			m.modelPickIdx--
			if m.modelPickIdx < m.modelScrollTop {
				m.modelScrollTop = m.modelPickIdx
			}
			m.updateViewportHeight()
			m.syncViewport()
		}
		return m, nil, true
	}
	if !m.selectMode && !m.confirming && !m.questioning && m.textinput.Line() == 0 && len(m.history) > 0 {
		if m.historyIdx > 0 {
			m.historyIdx--
		}
		m.textinput.SetValue(m.history[m.historyIdx])
		m.textinput.CursorEnd()
		m.syncInputHeight()
		m.refreshHints()
		return m, nil, true
	}
	return m, nil, false
}

func (m model) handleKeyDown(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	if m.handleSlashNavigation(msg) {
		return m, nil, true
	}
	if m.handleBangNavigation(msg) {
		return m, nil, true
	}
	if m.modelPicking {
		if m.modelPickIdx < len(m.modelFiltered)-1 {
			m.modelPickIdx++
			const maxVisible = 8
			if m.modelPickIdx >= m.modelScrollTop+maxVisible {
				m.modelScrollTop = m.modelPickIdx - maxVisible + 1
			}
			m.updateViewportHeight()
			m.syncViewport()
		}
		return m, nil, true
	}
	if !m.selectMode && !m.confirming && !m.questioning && m.textinput.Line() == m.textinput.LineCount()-1 && len(m.history) > 0 {
		if m.historyIdx < len(m.history)-1 {
			m.historyIdx++
			m.textinput.SetValue(m.history[m.historyIdx])
			m.textinput.CursorEnd()
		} else {
			m.historyIdx = len(m.history)
			m.resetInput()
		}
		m.syncInputHeight()
		m.refreshHints()
		return m, nil, true
	}
	return m, nil, false
}

func (m model) handleKeyTab(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	val := m.textinput.Value()
	if len(val) > 0 && val[0] == '!' {
		matches := matchBang(val, m.skills)
		if len(matches) > 0 {
			idx := m.skillsIdx
			if idx < 0 || idx >= len(matches) {
				idx = 0
			}
			m.textinput.SetValue("!" + matches[idx].Name)
			m.textinput.CursorEnd()
		}
		m.syncInputHeight()
		m.refreshHints()
		m.syncViewport()
		return m, nil, true
	}
	matches := matchSlash(val)
	if len(matches) > 0 {
		idx := m.slashIdx
		if idx < 0 || idx >= len(matches) {
			idx = 0
		}
		m.textinput.SetValue(matches[idx].name)
		m.textinput.CursorEnd()
	}
	m.syncInputHeight()
	m.refreshHints()
	m.syncViewport()
	return m, nil, true
}

func (m model) handleKeyShiftTab(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	if !m.streaming {
		m.autoMode = !m.autoMode
		return m, m.setMode(m.autoMode), true
	}
	return m, nil, false
}

func (m model) handleKeyCtrlW(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	if !m.questioning && !m.confirming {
		val := m.textinput.Value()
		runes := []rune(val)
		idx := len(runes)
		for idx > 0 {
			r := runes[idx-1]
			if unicode.IsSpace(r) || unicode.IsPunct(r) {
				break
			}
			idx--
		}
		for idx > 0 {
			r := runes[idx-1]
			if !unicode.IsSpace(r) && !unicode.IsPunct(r) {
				break
			}
			idx--
		}
		m.textinput.SetValue(string(runes[:idx]))
		m.textinput.CursorEnd()
		m.syncInputHeight()
	}
	return m, nil, true
}
