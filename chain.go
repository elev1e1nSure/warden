package tui

import "time"

const chainActionMinDuration = 2 * time.Second

// setAction updates or appends a live messageChainAction entry.
// Used by skill streams and other callers that don't go through wardenStartMsg.
func (m *model) setAction(verb, detail string) {
	m.chainPendingClear = false
	if n := len(m.messages); n > 0 && m.messages[n-1].kind == messageChainAction {
		e := &m.messages[n-1]
		e.activity = verb
		e.toolArgs = detail
		return
	}
	m.messages = append(m.messages, messageEntry{kind: messageChainAction, activity: verb, toolArgs: detail})
}

// clearAction removes a trailing messageChainAction entry. Returns true if one was removed.
func (m *model) clearAction() bool {
	m.chainPendingClear = false
	if n := len(m.messages); n > 0 && m.messages[n-1].kind == messageChainAction {
		m.messages = m.messages[:n-1]
		return true
	}
	return false
}

// clearActionDelayed marks the chain action for removal after a minimum display duration.
// If the action was set more than chainActionMinDuration ago, it clears immediately.
func (m *model) clearActionDelayed() {
	if n := len(m.messages); n == 0 || m.messages[n-1].kind != messageChainAction {
		return
	}
	m.chainPendingClear = true
	m.chainPendingClearAt = time.Now().Add(chainActionMinDuration)
}

// tickClearAction checks whether a pending delayed clear has expired and clears if so.
func (m *model) tickClearAction() {
	if !m.chainPendingClear {
		return
	}
	if time.Now().After(m.chainPendingClearAt) {
		m.clearAction()
	}
}

// freezeChain drops any trailing messageChainAction at turn end.
func (m *model) freezeChain() {
	m.chainPendingClear = false
	if n := len(m.messages); n > 0 && m.messages[n-1].kind == messageChainAction {
		m.messages = m.messages[:n-1]
	}
}
