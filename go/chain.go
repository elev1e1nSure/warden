package tui

// setAction updates or appends a live messageChainAction entry.
// Used by skill streams and other callers that don't go through wardenStartMsg.
func (m *model) setAction(verb, detail string) {
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
	if n := len(m.messages); n > 0 && m.messages[n-1].kind == messageChainAction {
		m.messages = m.messages[:n-1]
		return true
	}
	return false
}

// freezeChain drops any trailing messageChainAction at turn end.
func (m *model) freezeChain() {
	if n := len(m.messages); n > 0 && m.messages[n-1].kind == messageChainAction {
		m.messages = m.messages[:n-1]
	}
}
