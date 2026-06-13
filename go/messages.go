package tui

import (
	"time"

	"warden/internal/client"

	tea "github.com/charmbracelet/bubbletea"
)

// ── tea.Msg types ──

type tokenMsg struct{ text string }
type thinkMsg struct{ text string }
type toolMsg struct{ tool client.ToolMsg }
type toolStartMsg struct {
	name string
	args string
}
type wardenStartMsg struct{}
type confirmMsg struct {
	id         string
	tool       string
	risk       string
	title      string
	summary    string
	details    []string
	args       string
	preview    string
	defaultVal string
}
type modeMsg struct{ auto bool }
type doneMsg struct {
	tokenCount int
	tokenLimit int
}
type compactResultMsg struct {
	tokensBefore int
	tokensAfter  int
	err          string
}
type memoryResultMsg struct {
	text string
	err  string
}
type updateResultMsg struct {
	err error
}
type backendReadyMsg struct{}
type backendErrorMsg struct{}
type tickMsg struct{}
type shellResultMsg struct{ output string }
type startStreamMsg struct{ ch <-chan client.Event }
type nextMsg struct {
	inner tea.Msg
	ch    <-chan client.Event
}

type questionMsg struct {
	id        string
	questions []client.QuestionItem
}

type statusResultMsg struct {
	model      string
	mode       string
	cwd        string
	brief      bool
	tokenCount int
	tokenLimit int
}
type clipboardDoneMsg struct{ err error }
type modelsResultMsg struct {
	models  []string
	current string
	err     string
}
type modelSetMsg struct {
	model string
	err   string
}
type connectResultMsg struct {
	ok       bool
	err      string
	model    string
	provider string
	apiURL   string
	apiKey   string
}
type skillsResultMsg struct {
	skills []client.Skill
	err    string
}
type skillLoadedMsg struct {
	name    string
	content string
	err     string
}

// ── message list ──

type messageKind int

const (
	messageText   messageKind = iota
	messageUser               // user input, rendered with background
	messageWarden             // warden label (skipped in render)
	messageThink
	messageAssistant
	messageToolActivity // persistent tool line: pending while running, summary when done
	messageToolDiff     // diff block
	messageToolFlow     // live tool activity shown as flowing lines (verbose)
	messageChainAction  // single live "what's happening now" line (skill streams / compat)
)

type messageEntry struct {
	kind       messageKind
	text       string
	startedAt  time.Time
	duration   time.Duration
	activity   string // present-tense verb for the live action line
	toolName   string // display name for pending tool (messageToolActivity while running)
	toolArgs   string // tool arguments / detail for display
	toolDone   bool   // true when the tool has finished
	expanded   bool   // user toggled expanded detail view
	toolResult string // raw tool result for expanded view (messageToolActivity)
}

func (m *model) appendText(text string) {
	m.messages = append(m.messages, messageEntry{kind: messageText, text: text})
}

func (m *model) appendToolActivity(text string) {
	m.messages = append(m.messages, messageEntry{kind: messageToolActivity, text: text, toolDone: true})
}

// startToolActivity appends a pending tool entry. The entry animates until
// finishToolActivity updates it with the completed summary.
func (m *model) startToolActivity(name, args string) {
	display := toolDisplayName(name)
	m.messages = append(m.messages, messageEntry{
		kind:     messageToolActivity,
		toolName: display,
		toolArgs: actionDetail(display, args),
	})
}

func (m *model) appendToolFlow(name, args string) {
	// collapse: if the last message is the same tool, reuse it
	if len(m.messages) > 0 {
		last := &m.messages[len(m.messages)-1]
		if last.kind == messageToolFlow && last.toolName == name {
			last.toolArgs = args
			last.toolDone = false
			return
		}
	}
	m.messages = append(m.messages, messageEntry{kind: messageToolFlow, toolName: name, toolArgs: args})
}

func (m *model) appendThink() {
	m.messages = append(m.messages, messageEntry{kind: messageThink, startedAt: time.Now(), expanded: true})
}

// resetOrAppendThink reuses the last think entry only if it is still at the
// tail of the message list. Otherwise creates a new entry so the Thinking line
// appears after completed tools.
func (m *model) resetOrAppendThink() int {
	lastThinkIdx := -1
	for i := len(m.messages) - 1; i >= m.streamStart; i-- {
		if m.messages[i].kind == messageThink {
			lastThinkIdx = i
			break
		}
	}
	if lastThinkIdx >= 0 && lastThinkIdx == len(m.messages)-1 {
		m.messages[lastThinkIdx].duration = 0
		m.messages[lastThinkIdx].activity = ""
		m.messages[lastThinkIdx].text = ""
		return lastThinkIdx
	}
	m.appendThink()
	return len(m.messages) - 1
}

func (m *model) updateThink(text string) {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].kind == messageThink {
			m.messages[i].text += text
			return
		}
	}
}

func (m *model) finishThink() {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].kind == messageThink {
			if m.messages[i].duration == 0 {
				m.messages[i].duration = time.Since(m.messages[i].startedAt)
			}
			return
		}
	}
}

func (m *model) appendToLastText(text string) {
	if len(m.messages) == 0 {
		return
	}
	last := len(m.messages) - 1
	if m.messages[last].kind != messageText {
		return
	}
	m.messages[last].text += text
}

func (m *model) appendAssistant(text string) {
	m.messages = append(m.messages, messageEntry{kind: messageAssistant, text: text})
}

func (m *model) appendToLastAssistant(text string) {
	if len(m.messages) == 0 {
		return
	}
	last := len(m.messages) - 1
	if m.messages[last].kind != messageAssistant {
		return
	}
	m.messages[last].text += text
}
