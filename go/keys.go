package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handleKey processes a keyboard event.
// Returns (model, cmd, true) when Update should return immediately,
// or (model, nil, false) to continue with the normal Update tail.
func (m *model) handleKey(msg tea.KeyMsg) (*model, tea.Cmd, bool) {
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
		return m.handleKeyCtrlC(msg)
	case tea.KeyUp:
		return m.handleKeyUp(msg)
	case tea.KeyDown:
		return m.handleKeyDown(msg)
	case tea.KeyCtrlW:
		return m.handleKeyCtrlW(msg)
	case tea.KeyTab:
		return m.handleKeyTab(msg)
	case tea.KeyShiftTab:
		return m.handleKeyShiftTab(msg)
	case tea.KeyEsc:
		return m.handleKeyEsc(msg)
	case tea.KeyRunes:
		return m.handleKeyRunes(msg)
	case tea.KeyEnter:
		return m.handleKeyEnter(msg)
	}

	return m, nil, false
}
