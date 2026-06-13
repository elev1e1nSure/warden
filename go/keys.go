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

	// Native bracketed paste (or multi-rune burst): collapse big/multiline
	// pastes into a [pasted #N] placeholder, insert small ones inline.
	if msg.Type == tea.KeyRunes && (msg.Paste || len(msg.Runes) > 1) && !modal {
		m.insertPaste(string(msg.Runes))
		m.lastRuneAt = time.Now()
		m.syncInputHeight()
		m.refreshHints()
		return m, m.focusInput(), true
	}
	// Clear pending confirmations if user presses a different key
	if msg.Type != tea.KeyEsc {
		m.escPending = false
	}
	// Don't auto-clear quitPending - only clear explicitly on cancel actions

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
		// history recall when cursor is on the first line
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
		// history recall when cursor is on the last line
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
			// Find word boundary using runes to handle multi-byte characters
			runes := []rune(val)
			// Search backwards from end for word boundary
			idx := len(runes)
			for idx > 0 {
				r := runes[idx-1]
				if unicode.IsSpace(r) || unicode.IsPunct(r) {
					break
				}
				idx--
			}
			// Trim trailing whitespace/punctuation before the word
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
		} else {
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
			// Second ESC — confirmed cancel
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

	case tea.KeyRunes:
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
		// Enter-guard: on legacy consoles a pasted newline arrives as a
		// KeyEnter inside a rune burst — treat it as a newline, not submit.
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

	return m, nil, false
}
