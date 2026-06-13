package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleKeyCtrlC(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	if m.streaming {
		if m.quitPending {
			return m, tea.Quit, true
		}
		m.quitPending = true
		m.quitPendingSince = time.Now()
		return m, nil, true
	}
	return m, tea.Quit, true
}

func (m model) handleKeyEsc(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	m.quitPending = false
	if m.selectMode {
		m.selectMode = false
		return m, tea.EnableMouseCellMotion, true
	}
	if m.modelPicking {
		m.modelPicking = false
		m.modelList = nil
		m.modelFiltered = nil
		m.resetInput()
		m.updateViewportHeight()
		m.syncViewport()
		return m, m.focusInput(), true
	}
	if m.streaming && !m.questioning && !m.confirming {
		if !m.escPending {
			m.escPending = true
			return m, nil, true
		}
		m.escPending = false
		m.interruptStream = true
		m.streaming = false
		m.loading = false
		m.runningToolIdx = -1
		m.thinkBuf = ""
		m.thinkDone = false
		m.toolRunning = false
		m.userScrolled = false
		m.finishThink()
		m.textinput.Placeholder = ""
		m.syncViewport()
		return m, m.focusInput(), true
	}
	if m.questioning {
		ch := m.questionCh
		id := m.questionID
		m = m.clearQuestionState()
		m.updateViewportHeight()
		m.syncViewport()
		return m, tea.Batch(m.focusInput(), m.sendQuestion(id, nil), readNext(ch)), true
	}
	if m.confirming {
		newM, cmd := m.resolveConfirm(false)
		return newM, cmd, true
	}
	m.resetInput()
	return m, nil, true
}

func (m model) handleKeyRunes(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	if m.confirming {
		switch strings.ToLower(string(msg.Runes)) {
		case "y", "н":
			newM, cmd := m.resolveConfirm(true)
			return newM, cmd, true
		case "n", "т":
			newM, cmd := m.resolveConfirm(false)
			return newM, cmd, true
		}
		return m, nil, true
	}
	if m.questioning {
		q := m.questionsData[m.questionIdx]
		if len(q.Options) > 0 {
			if num, err := parseOptionNumber(string(msg.Runes)); err == nil && num >= 1 && num <= len(q.Options) {
				newM, cmd := m.answerQuestion(q.Options[num-1].Label)
				return newM, cmd, true
			}
		}
	}
	m.textinput.Placeholder = ""
	m.lastRuneAt = time.Now()
	return m, nil, false
}

func (m model) handleKeyEnter(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	m.quitPending = false
	if m.modelPicking {
		if m.modelPickIdx < len(m.modelFiltered) {
			chosen := m.modelFiltered[m.modelPickIdx]
			m.modelPicking = false
			m.modelList = nil
			m.modelFiltered = nil
			m.resetInput()
			m.updateViewportHeight()
			return m, tea.Batch(m.focusInput(), m.applyModel(chosen)), true
		}
		return m, nil, true
	}
	if m.questioning {
		q := m.questionsData[m.questionIdx]
		if len(q.Options) == 0 {
			text := strings.TrimSpace(m.textinput.Value())
			m.resetInput()
			newM, cmd := m.answerQuestion(text)
			return newM, cmd, true
		}
		return m, nil, true
	}
	if m.confirming {
		return m, nil, true
	}
	if m.streaming {
		return m, nil, true
	}
	if time.Since(m.lastRuneAt) < 8*time.Millisecond {
		m.textinput.InsertString("\n")
		m.lastRuneAt = time.Now()
		m.syncInputHeight()
		return m, nil, true
	}
	val := m.textinput.Value()
	if strings.HasSuffix(val, "\\") {
		m.textinput.SetValue(val[:len(val)-1] + "\n")
		m.textinput.CursorEnd()
		m.syncInputHeight()
		return m, nil, true
	}
	if strings.HasPrefix(val, "/") {
		matches := matchSlash(val)
		if len(matches) > 0 {
			idx := m.slashIdx
			if idx < 0 || idx >= len(matches) {
				idx = 0
			}
			val = matches[idx].name
			m.textinput.SetValue(val)
			m.textinput.CursorEnd()
		}
	}
	text := strings.TrimSpace(m.expandPastes(val))
	if text == "" {
		return m, nil, true
	}
	if handled, cmd := m.handleSlash(text); handled {
		m.resetInput()
		return m, cmd, true
	}
	if strings.HasPrefix(text, "!") {
		if handled, cmd := m.handleBang(text); handled {
			m.resetInput()
			return m, cmd, true
		}
		return m, nil, true
	}
	if !m.connected {
		m.resetInput()
		return m, nil, true
	}
	m.recordHistory(text)
	m.messages = append(m.messages, messageEntry{kind: messageUser, text: text})
	m.appendText("")
	return m, m.beginStream(text), true
}
