package tui

// ── non-verbose tool chain ──
// During a turn the live action state is stored directly on the
// messageChainSummary placeholder (chainSummaryIdx). This lets the summary
// entry be both the animated live indicator and the final collapsed summary.

func (m *model) startChain() {}

// setAction updates the live action verb on the summary placeholder.
// Falls back to a separate messageChainAction entry when no placeholder exists
// (verbose mode or during skill streams that bypass wardenStartMsg).
func (m *model) setAction(verb, detail string, thinking bool) {
	if m.chainSummaryIdx >= 0 && m.chainSummaryIdx < len(m.messages) {
		e := &m.messages[m.chainSummaryIdx]
		e.activity = verb
		e.toolArgs = detail
		e.thinking = thinking
		return
	}
	if n := len(m.messages); n > 0 && m.messages[n-1].kind == messageChainAction {
		e := &m.messages[n-1]
		e.activity = verb
		e.toolArgs = detail
		e.thinking = thinking
		return
	}
	m.messages = append(m.messages, messageEntry{kind: messageChainAction, activity: verb, toolArgs: detail, thinking: thinking})
}

// clearAction clears the live activity from the placeholder (or removes a
// standalone messageChainAction). Returns true if something was cleared.
func (m *model) clearAction() bool {
	if m.chainSummaryIdx >= 0 && m.chainSummaryIdx < len(m.messages) {
		e := &m.messages[m.chainSummaryIdx]
		e.activity = ""
		e.toolArgs = ""
		e.thinking = false
		return true
	}
	if n := len(m.messages); n > 0 && m.messages[n-1].kind == messageChainAction {
		m.messages = m.messages[:n-1]
		return true
	}
	return false
}

// freezeChain ends the turn: clear live activity from placeholder (entry
// itself stays to become the finalized summary).
func (m *model) freezeChain() {
	if m.chainSummaryIdx >= 0 && m.chainSummaryIdx < len(m.messages) {
		e := &m.messages[m.chainSummaryIdx]
		e.activity = ""
		e.toolArgs = ""
		e.thinking = false
		return
	}
	// fallback for standalone messageChainAction
	if n := len(m.messages); n > 0 && m.messages[n-1].kind == messageChainAction {
		m.messages = m.messages[:n-1]
	}
}
