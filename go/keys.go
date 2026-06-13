package tui

import (
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

// handleKey processes a keyboard event.
// Returns (model, cmd, true) when Update should return immediately,
// or (model, nil, false) to continue with the normal Update tail.
func (m model) handleKey(msg tea.KeyMsg) (model, tea.Cmd, bool) {
	modal := m.confirming || m.questioning || m.modelPicking

	// Native bracketed paste or multi-rune burst
	if msg.Type == tea.KeyRunes && (msg.Paste || len(msg.Runes) > 1) && !modal {
		m.insertPaste(string(msg.Runes))
		m.lastRuneAt = time.Now()
		m.syncInputHeight()
		m.refreshHints()
		return m, m.focusInput(), true
	}

	if msg.Type != tea.KeyEsc {
		m.escPending = false
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		if m.streaming {
			if m.quitPending {
				return m, tea.Quit, true
			}
			m.quitPending = true
			m.quitPendingSince = time.Now()
			return m, nil, true
		}
		return m, tea.Quit, true

	case tea.KeyUp:
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
		if !m.confirming && !m.questioning && m.textinput.Line() == 0 && len(m.history) > 0 {
			if m.historyIdx > 0 {
				m.historyIdx--
			}
			m.textinput.SetValue(m.history[m.historyIdx])
			m.textinput.CursorEnd()
			m.syncInputHeight()
			m.refreshHints()
			return m, nil, true
		}

	case tea.KeyDown:
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
		if !m.confirming && !m.questioning && m.textinput.Line() == m.textinput.LineCount()-1 && len(m.history) > 0 {
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

	case tea.KeyCtrlW:
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

	case tea.KeyTab:
		val := m.textinput.Value()
		if strings.HasPrefix(val, "!") {
			matches := matchBang(val, m.skills)
			if len(matches) == 1 {
				m.textinput.SetValue("!" + matches[0].Name)
				m.textinput.CursorEnd()
			} else if len(matches) > 1 {
				m.textinput.SetValue(bangCommonPrefix(matches))
				m.textinput.CursorEnd()
			}
		} else {
			matches := matchSlash(val)
			if len(matches) == 1 {
				m.textinput.SetValue(matches[0].name)
				m.textinput.CursorEnd()
			} else if len(matches) > 1 {
				m.textinput.SetValue(slashCommonPrefix(matches))
				m.textinput.CursorEnd()
			}
		}

	case tea.KeyShiftTab:
		if !m.streaming {
			m.autoMode = !m.autoMode
			return m, m.setMode(m.autoMode), true
		}

	case tea.KeyEsc:
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
			m.loading = true
			m.updateViewportHeight()
			m.syncViewport()
			return m, tea.Batch(m.focusInput(), m.sendQuestion(id, nil), readNext(ch), m.tick()), true
		}
		if m.confirming {
			m.confirming = false
			ch := m.confirmCh
			id := m.confirmID
			m.confirmID = ""
			m.confirmCh = nil
			m.confirmTool = ""
			m.textinput.Placeholder = ""
			m.loading = true
			m.resetInput()
			m.updateViewportHeight()
			m.syncViewport()
			return m, tea.Batch(m.focusInput(), m.sendConfirm(id, false), readNext(ch), m.tick()), true
		}
		m.resetInput()

	case tea.KeyRunes:
		if m.confirming {
			r := strings.ToLower(string(msg.Runes))
			if r == "y" || r == "н" {
				ch := m.confirmCh
				id := m.confirmID
				m.confirming = false
				m.confirmID = ""
				m.confirmCh = nil
				m.confirmTool = ""
				m.textinput.Placeholder = ""
				m.loading = true
				m.resetInput()
				return m, tea.Batch(m.focusInput(), m.sendConfirm(id, true), readNext(ch), m.tick()), true
			}
			if r == "n" || r == "т" {
				ch := m.confirmCh
				id := m.confirmID
				m.confirming = false
				m.confirmID = ""
				m.confirmCh = nil
				m.confirmTool = ""
				m.textinput.Placeholder = ""
				m.loading = true
				m.resetInput()
				return m, tea.Batch(m.focusInput(), m.sendConfirm(id, false), readNext(ch), m.tick()), true
			}
			return m, nil, true
		}
		if m.questioning {
			q := m.questionsData[m.questionIdx]
			if len(q.Options) > 0 {
				input := string(msg.Runes)
				if num, err := parseOptionNumber(input); err == nil && num >= 1 && num <= len(q.Options) {
					idx := num - 1
					m.questionAnswers = append(m.questionAnswers, []string{q.Options[idx].Label})
					m.questionIdx++
					if m.questionIdx >= len(m.questionsData) {
						ch := m.questionCh
						id := m.questionID
						answers := m.questionAnswers
						savedQuestions := m.questionsData
						m = m.clearQuestionState()
						m.appendQuizHistory(savedQuestions, answers)
						m.loading = true
						m.updateViewportHeight()
						m.syncViewport()
						return m, tea.Batch(m.focusInput(), m.sendQuestion(id, answers), readNext(ch), m.tick()), true
					}
					m.syncViewport()
					return m, m.focusInput(), true
				}
			}
		}
		m.textinput.Placeholder = ""
		m.lastRuneAt = time.Now()

	case tea.KeyEnter:
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
				m.questionAnswers = append(m.questionAnswers, []string{text})
				m.questionIdx++
				if m.questionIdx >= len(m.questionsData) {
					ch := m.questionCh
					id := m.questionID
					answers := m.questionAnswers
					savedQuestions := m.questionsData
					m = m.clearQuestionState()
					m.appendQuizHistory(savedQuestions, answers)
					m.loading = true
					m.updateViewportHeight()
					m.syncViewport()
					return m, tea.Batch(m.focusInput(), m.sendQuestion(id, answers), readNext(ch), m.tick()), true
				}
				m.syncViewport()
				return m, m.focusInput(), true
			}
			return m, nil, true
		}
		if m.confirming {
			return m, nil, true
		}
		if m.streaming {
			return m, nil, true
		}
		// Enter-guard: legacy console pasted newline arrives as KeyEnter in rune burst
		if time.Since(m.lastRuneAt) < 8*time.Millisecond {
			m.textinput.InsertString("\n")
			m.lastRuneAt = time.Now()
			m.syncInputHeight()
			return m, nil, true
		}
		val := m.textinput.Value()
		// \ at end of line + Enter = shell-style line continuation
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
		if strings.HasPrefix(val, "!") {
			matches := matchBang(val, m.skills)
			if len(matches) > 0 {
				idx := m.skillsIdx
				if idx < 0 || idx >= len(matches) {
					idx = 0
				}
				val = "!" + matches[idx].Name
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
		m.streamStart = len(m.messages)
		m.messages = append(m.messages, messageEntry{kind: messageUser, text: text})
		m.appendText("")
		m.resetInput()
		m.startChain()
		m.streaming = true
		m.loading = true
		m.spinner = 0
		m.syncViewport()
		return m, tea.Batch(m.sendMessage(text), m.tick()), true
	}

	return m, nil, false
}
