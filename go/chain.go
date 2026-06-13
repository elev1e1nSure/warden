package tui

import "time"

// ── non-verbose tool chain ──
// During a turn there is one live action line that updates in place
// ("Fetching <url>" / "Thinking..."). At turn end the action line is dropped.

func (m *model) startChain() {
	m.chainCounts = map[string]int{}
	m.chainOrder = nil
	m.chainStart = time.Now()
}

// setAction updates the live action line in place, or appends it at the tail.
func (m *model) setAction(verb, detail string, thinking bool) {
	if n := len(m.messages); n > 0 && m.messages[n-1].kind == messageChainAction {
		e := &m.messages[n-1]
		e.activity = verb
		e.toolArgs = detail
		e.thinking = thinking
		return
	}
	m.messages = append(m.messages, messageEntry{kind: messageChainAction, activity: verb, toolArgs: detail, thinking: thinking})
}

// clearAction removes the live action line if it is at the tail.
func (m *model) clearAction() bool {
	if n := len(m.messages); n > 0 && m.messages[n-1].kind == messageChainAction {
		m.messages = m.messages[:n-1]
		return true
	}
	return false
}

// freezeChain ends the turn: simply drop the live action line.
func (m *model) freezeChain() {
	m.clearAction()
}
