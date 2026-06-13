package tui

import "time"

// ── non-verbose tool chain ──
// During a turn the chain shows at most two live entries that update in place:
// a grouped counter ("Searched ×2 · Fetched ×6 · 12s") and a single action line
// ("Fetching <url>" / "Thinking..."). At turn end the action line is dropped and
// the counter is frozen as the summary.

func (m *model) startChain() {
	m.chainCounts = map[string]int{}
	m.chainOrder = nil
	m.chainStart = time.Now()
}

// bumpChain records a completed tool under its display name.
func (m *model) bumpChain(display string) {
	if m.chainCounts == nil {
		m.chainCounts = map[string]int{}
	}
	if _, ok := m.chainCounts[display]; !ok {
		m.chainOrder = append(m.chainOrder, display)
	}
	m.chainCounts[display]++
}

// counterIdx returns the index of this turn's counter entry, or -1.
func (m *model) counterIdx() int {
	start := m.streamStart
	if start < 0 {
		start = 0
	}
	for i := start; i < len(m.messages); i++ {
		if m.messages[i].kind == messageChainCounter {
			return i
		}
	}
	return -1
}

// ensureCounter creates the counter entry once per turn (called before setAction
// so the counter lands above the action line).
func (m *model) ensureCounter() {
	if m.counterIdx() < 0 {
		m.messages = append(m.messages, messageEntry{kind: messageChainCounter, startedAt: m.chainStart})
	}
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

// freezeChain ends the turn: drop the action line, freeze the counter time, or
// remove the counter entirely if no tools ran.
func (m *model) freezeChain() {
	m.clearAction()
	idx := m.counterIdx()
	if idx < 0 {
		return
	}
	if len(m.chainCounts) == 0 {
		m.messages = append(m.messages[:idx], m.messages[idx+1:]...)
		return
	}
	m.messages[idx].duration = time.Since(m.chainStart)
	m.messages[idx].text = m.renderChainCounter(m.messages[idx])
}
