package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

type model struct {
	viewport  viewport.Model
	textinput textinput.Model
	client    *Client
	messages  []messageEntry
	streaming bool
	height    int
	width     int
	loading   bool
	spinner   int
	thinkBuf  string
	thinkDone bool
	modelName string
	// tool execution
	toolRunning bool
	// confirmation
	confirming  bool
	confirmID   string
	confirmCh   <-chan tea.Msg
	confirmTool string
	// question
	questioning     bool
	questionID      string
	questionCh      <-chan tea.Msg
	questionsData   []QuestionItem
	questionIdx     int
	questionAnswers [][]string
	// mode
	autoMode    bool
	hintVisible bool
	hintCount   int
	// status
	thinkingEnabled  bool
	thinkingExpanded bool
	// path
	cwd          string
	providerName string
	// live tool activity line shown during streaming (replaces tool_start/pending in messages)
	liveActivity string
	// last raw assistant response (for /copy-last)
	lastAssistantRaw string
	// interrupt / rollback state
	interruptStream bool
	streamStart     int
	lastUserInput   string
	// token tracking
	tokenCount int
	tokenLimit int
	// confirm dialog data
	confirmRisk    string
	confirmTitle   string
	confirmSummary string
	confirmDetails []string
	confirmPreview string
	confirmDefault string
	// input history
	history    []string
	historyIdx int
	historySav string
	// markdown
	mdRenderer *glamour.TermRenderer
	mdWidth    int
}

func initialModel(modelName string) model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Prompt = "> "
	ti.CharLimit = 0
	ti.Width = 80
	ti.Focus()
	ti.BlinkSpeed = 0

	vp := viewport.New(80, 20)
	vp.SetContent("")
	vp.GotoTop()

	cwd, _ := os.Getwd()
	return model{
		textinput:       ti,
		viewport:        vp,
		client:          NewClient("http://localhost:8765"),
		messages:        []messageEntry{},
		thinkingEnabled: true,
		cwd:             cwd,
		modelName:       modelName,
		history:         []string{},
		loading:         true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.checkBackend(), m.tick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.viewport.Width = msg.Width
		m.textinput.Width = msg.Width - 6
		m.updateViewportHeight()
		m.syncViewport()

	case tea.KeyMsg:
		if msg.Type != tea.KeyF2 && msg.String() == "f2" {
			m = m.toggleThinkingExpanded()
			return m, m.focusInput()
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyF2:
			m = m.toggleThinkingExpanded()
			return m, m.focusInput()

		case tea.KeyUp:
			if !m.streaming && !m.questioning && !m.confirming && len(m.history) > 0 {
				if m.historyIdx == len(m.history) {
					m.historySav = m.textinput.Value()
				}
				if m.historyIdx > 0 {
					m.historyIdx--
					m.textinput.SetValue(m.history[m.historyIdx])
					m.textinput.CursorEnd()
				} else if m.historyIdx == 0 {
					m.textinput.Placeholder = DimStyle().Render("(start of history)")
				}
			}

		case tea.KeyDown:
			if !m.streaming && !m.questioning && !m.confirming && len(m.history) > 0 {
				if m.historyIdx < len(m.history) {
					m.historyIdx++
					m.textinput.Placeholder = ""
					if m.historyIdx < len(m.history) {
						m.textinput.SetValue(m.history[m.historyIdx])
					} else {
						m.textinput.SetValue(m.historySav)
					}
					m.textinput.CursorEnd()
				}
			}


		case tea.KeyCtrlW:
			if !m.questioning && !m.confirming {
				val := m.textinput.Value()
				// Find word boundary using runes to handle multi-byte characters
				runes := []rune(val)
				cursor := m.textinput.Cursor()
				if cursor > len(runes) {
					cursor = len(runes)
				}
				// Search backwards from cursor for word boundary
				idx := cursor
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
			}

		case tea.KeyTab:
			if !m.streaming {
				val := m.textinput.Value()
				matches := matchSlash(val)
				if len(matches) == 1 {
					m.textinput.SetValue(matches[0].name)
					m.textinput.CursorEnd()
				} else if len(matches) > 1 {
					m.textinput.SetValue(slashCommonPrefix(matches))
					m.textinput.CursorEnd()
				}
			}

		case tea.KeyEsc:
			if m.streaming && !m.questioning && !m.confirming {
				m.interruptStream = true
				m.streaming = false
				m.loading = false
				m.liveActivity = ""
				m.thinkBuf = ""
				m.thinkDone = false
				m.toolRunning = false
				if m.streamStart <= len(m.messages) {
					m.messages = m.messages[:m.streamStart]
				}
				m.textinput.SetValue(m.lastUserInput)
				m.textinput.CursorEnd()
				m.textinput.Placeholder = ""
				m.syncViewport()
				return m, m.focusInput()
			}
			if m.questioning {
				ch := m.questionCh
				id := m.questionID
				m.historySav = m.textinput.Value()
				m = m.clearQuestionState()
				return m, tea.Batch(m.focusInput(), m.sendQuestion(id, nil), readNext(ch))
			}
			if m.confirming {
				m.confirming = false
				ch := m.confirmCh
				id := m.confirmID
				m.confirmID = ""
				m.confirmCh = nil
				m.confirmTool = ""
				m.textinput.Placeholder = ""
				m.textinput.Reset()
				return m, tea.Batch(m.focusInput(), m.sendConfirm(id, false), readNext(ch))
			}
			m.textinput.Reset()

		case tea.KeyRunes:
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
							m = m.clearQuestionState()
							return m, tea.Batch(m.focusInput(), m.sendQuestion(id, answers), readNext(ch))
						}
						m.syncViewport()
						return m, m.focusInput()
					}
				}
				return m, nil
			}
			m.textinput.Placeholder = ""

		case tea.KeyEnter:
			if m.questioning {
				q := m.questionsData[m.questionIdx]
				if len(q.Options) == 0 {
					text := strings.TrimSpace(m.textinput.Value())
					m.textinput.Reset()
					m.questionAnswers = append(m.questionAnswers, []string{text})
					m.questionIdx++
					if m.questionIdx >= len(m.questionsData) {
						ch := m.questionCh
						id := m.questionID
						answers := m.questionAnswers
						m = m.clearQuestionState()
						return m, tea.Batch(m.focusInput(), m.sendQuestion(id, answers), readNext(ch))
					}
					m.syncViewport()
					return m, m.focusInput()
				}
				return m, nil
			}
			if m.confirming {
				val := strings.ToLower(strings.TrimSpace(m.textinput.Value()))
				ok := val == "y" || val == "yes"
				if val == "" {
					return m, nil
				}
				ch := m.confirmCh
				id := m.confirmID
				m.confirming = false
				m.confirmID = ""
				m.confirmCh = nil
				m.confirmTool = ""
				m.textinput.Placeholder = ""
				m.textinput.Reset()
				return m, tea.Batch(m.focusInput(), m.sendConfirm(id, ok), readNext(ch))
			}
			if m.streaming {
				return m, nil
			}
			text := strings.TrimSpace(m.textinput.Value())
			if text == "" {
				return m, nil
			}
			if handled, cmd := m.handleSlash(text); handled {
				m.textinput.Reset()
				return m, cmd
			}

			m.history = append(m.history, text)
			m.historyIdx = len(m.history)
			m.historySav = ""

			if strings.HasPrefix(text, "! ") {
				cmdText := strings.TrimPrefix(text, "! ")
				m.appendText(DimStyle().Render("  > ") + cmdText)
				m.appendText("")
				m.textinput.Reset()
				m.streaming = true
				m.loading = true
				m.spinner = 0
				m.syncViewport()
				return m, tea.Batch(m.execShell(cmdText), m.tick())
			}

			m.lastUserInput = text
			m.streamStart = len(m.messages)
			m.appendText(DimStyle().Render("  > ") + text)
			m.appendText("")
			m.textinput.Reset()
			m.streaming = true
			m.loading = true
			m.spinner = 0
			m.syncViewport()
			return m, tea.Batch(m.sendMessage(text), m.tick())
		}

	case startStreamMsg:
		cmds = append(cmds, readNext(msg.ch))

	case nextMsg:
		// drain silently if user interrupted
		if m.interruptStream {
			if _, ok := msg.inner.(doneMsg); ok {
				m.interruptStream = false
			} else {
				cmds = append(cmds, readNext(msg.ch))
			}
			break
		}
		switch inner := msg.inner.(type) {
		case wardenStartMsg:
			m.thinkBuf = ""
			m.thinkDone = false
			m.toolRunning = false
			m.liveActivity = ""
			m.lastAssistantRaw = ""
			m.appendThink()
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case thinkMsg:
			m.thinkBuf += inner.text
			m.updateThink(inner.text)
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case tokenMsg:
			if !m.thinkDone {
				m.finishThink()
				m.appendText("")
				m.appendAssistant("")
				m.thinkDone = true
			}
			m.appendToLastAssistant(inner.text)
			m.lastAssistantRaw += inner.text
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case toolStartMsg:
			m.toolRunning = true
			if !m.thinkDone && len(m.messages) > 0 {
				m.finishThink()
			}
			m.liveActivity = toolStartLine(inner.name, inner.args)
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case toolMsg:
			m.toolRunning = false
			m.liveActivity = ""
			if stickyTool(inner.tool.Name) {
				m.appendText(toolResultBlock(inner.tool.Result))
			} else {
				m.appendText(toolSummaryLine(inner.tool.Name, inner.tool.Args, inner.tool.Result))
			}
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case confirmMsg:
			m.confirming = true
			m.confirmID = inner.id
			m.confirmCh = msg.ch
			m.confirmTool = inner.tool
			m.confirmRisk = inner.risk
			m.confirmTitle = inner.title
			m.confirmSummary = inner.summary
			m.confirmDetails = inner.details
			m.confirmPreview = inner.preview
			m.confirmDefault = inner.default
			m.syncViewport()
			m.textinput.Placeholder = ""
			m.textinput.Reset()
			if inner.default != "" {
				m.textinput.SetValue(inner.default)
				m.textinput.CursorEnd()
			}

		case questionMsg:
			m.questioning = true
			m.questionID = inner.id
			m.questionCh = msg.ch
			m.questionsData = inner.questions
			m.questionIdx = 0
			m.questionAnswers = nil
			m.textinput.Placeholder = ""
			m.textinput.Reset()
			m.syncViewport()

		case doneMsg:
			m.streaming = false
			m.loading = false
			m.toolRunning = false
			m.liveActivity = ""
			m.finishThink()
			m.thinkBuf = ""
			m.thinkDone = false
			m.appendText("")
			if inner.tokenLimit > 0 {
				m.tokenCount = inner.tokenCount
				m.tokenLimit = inner.tokenLimit
			}
			m.syncViewport()
			if m.tokenLimit > 0 && m.tokenCount > int(float64(m.tokenLimit)*0.85) {
				m.loading = true
				m.appendText(m.wardenLine(DimStyle().Render("context at " + fmt.Sprintf("%d%%", m.tokenCount*100/m.tokenLimit) + ", compacting...")))
				cmds = append(cmds, m.runCompact(), m.tick())
			}
		}

	case doneMsg:
		if m.streaming {
			m.streaming = false
			m.loading = false
			m.toolRunning = false
			m.liveActivity = ""
			m.finishThink()
			m.thinkBuf = ""
			m.thinkDone = false
			m.appendText("")
			if msg.tokenLimit > 0 {
				m.tokenCount = msg.tokenCount
				m.tokenLimit = msg.tokenLimit
			}
			m.syncViewport()
			if m.tokenLimit > 0 && m.tokenCount > int(float64(m.tokenLimit)*0.85) {
				m.loading = true
				m.appendText(m.wardenLine(DimStyle().Render("context at " + fmt.Sprintf("%d%%", m.tokenCount*100/m.tokenLimit) + ", compacting...")))
				cmds = append(cmds, m.runCompact(), m.tick())
			}
		}

	case shellResultMsg:
		m.streaming = false
		m.loading = false
		m.toolRunning = false
		m.finishThink()
		m.thinkBuf = ""
		m.thinkDone = false
		m.appendText(DimStyle().Render("  ── output ──\n" + msg.output))
		m.appendText("")
		m.syncViewport()

	case tickMsg:
		if m.loading {
			m.spinner += m.advance()
			if !m.thinkDone && m.streaming && !m.confirming && !m.toolRunning && len(m.messages) > 0 {
				m.syncViewport()
			}
			return m, m.tick()
		}
		return m, nil

	case modeMsg:
		m.autoMode = msg.auto
		label := "Leashed"
		if m.autoMode {
			label = "Unleashed"
		}
		m.appendText(DimStyle().Render("  Mode: " + label))
		m.syncViewport()

	case statusResultMsg:
		if msg.tokenLimit > 0 {
			m.tokenCount = msg.tokenCount
			m.tokenLimit = msg.tokenLimit
		}
		var line string
		if msg.brief {
			line = msg.provider + " · " + msg.model
		} else {
			thinkStr := "off"
			if msg.thinking {
				thinkStr = "on"
			}
			line = "model: " + msg.model + "  provider: " + msg.provider + "  mode: " + msg.mode + "  thinking: " + thinkStr + "  cwd: " + msg.cwd
		}

		m.appendText(m.wardenLine(DimStyle().Render(line)))
		m.syncViewport()

	case toolsResultMsg:
		
		m.appendText(m.wardenLine(DimStyle().Render(strings.Join(msg.tools, "  "))))
		m.syncViewport()

	case clipboardDoneMsg:
		
		if msg.err != nil {
			m.appendText(m.wardenLine(ErrorStyle().Render("clipboard error: " + msg.err.Error())))
		} else {
			m.appendText(m.wardenLine(DimStyle().Render("copied")))
		}
		m.syncViewport()

	case compactResultMsg:
		m.loading = false
		if msg.err != "" {
			m.appendText(m.wardenLine(ErrorStyle().Render("compact: " + msg.err)))
		} else {
			before := fmt.Sprintf("%.1fK", float64(msg.tokensBefore)/1000)
			after := fmt.Sprintf("%.1fK", float64(msg.tokensAfter)/1000)
			m.tokenCount = msg.tokensAfter
			m.appendText(m.wardenLine(DimStyle().Render("compacted  " + before + " → " + after)))
		}
		m.syncViewport()

	case providerInitMsg:
		m.providerName = msg.provider

	case backendReadyMsg:
		m.loading = false
		m.tokenCount = 0
		m.client.ResetSession()
		m.syncViewport()
		cmds = append(cmds, m.initProvider())

	case backendErrorMsg:
		m.loading = false
		m.appendText(ErrorStyle().Render("Error: backend unavailable"))
		m.syncViewport()
	}

	cmds = append(cmds, m.focusInput())

	var cmd tea.Cmd
	m.textinput, cmd = m.textinput.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	// sync hint visibility and viewport height
	matches := matchSlash(m.textinput.Value())
	newCount := 0
	if !m.streaming {
		newCount = len(matches)
	}
	if newCount != m.hintCount {
		m.hintCount = newCount
		m.hintVisible = newCount > 0
		if m.height > 0 {
			m.updateViewportHeight()
			m.syncViewport()
		}
	}

	return m, tea.Batch(cmds...)
}

// message types
type tokenMsg struct{ text string }
type thinkMsg struct{ text string }
type toolMsg struct{ tool ToolMsg }
type toolStartMsg struct {
	name string
	args string
}
type wardenStartMsg struct{ ch <-chan tea.Msg }
type confirmMsg struct {
	id      string
	tool    string
	risk    string
	title   string
	summary string
	details []string
	args    string
	preview string
	default string
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
type backendReadyMsg struct{}
type backendErrorMsg struct{}
type tickMsg struct{}
type shellResultMsg struct{ output string }
type startStreamMsg struct{ ch <-chan tea.Msg }
type nextMsg struct {
	inner tea.Msg
	ch    <-chan tea.Msg
}
type QuestionOption struct {
	Label       string
	Description string
}

type QuestionItem struct {
	Question string
	Header   string
	Options  []QuestionOption
	Multiple bool
}

type questionMsg struct {
	id        string
	questions []QuestionItem
}

type statusResultMsg struct {
	model      string
	provider   string
	mode       string
	thinking   bool
	cwd        string
	brief      bool
	tokenCount int
	tokenLimit int
}
type toolsResultMsg struct{ tools []string }
type clipboardDoneMsg struct{ err error }
type providerInitMsg struct{ provider string }

type messageKind int

const (
	messageText messageKind = iota
	messageThink
	messageAssistant
)

type messageEntry struct {
	kind      messageKind
	text      string
	startedAt time.Time
	duration  time.Duration
}

func (m *model) appendText(text string) {
	m.messages = append(m.messages, messageEntry{kind: messageText, text: text})
}

func (m model) toggleThinkingExpanded() model {
	m.thinkingExpanded = !m.thinkingExpanded
	if m.thinkingExpanded {
		m.syncViewportToLatestThink()
	} else {
		m.syncViewport()
	}
	return m
}

func (m *model) appendThink() {
	m.messages = append(m.messages, messageEntry{kind: messageThink, startedAt: time.Now()})
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

func (m *model) focusInput() tea.Cmd {
	if m.textinput.Focused() {
		return nil
	}
	return m.textinput.Focus()
}

func (m model) clearQuestionState() model {
	m.questioning = false
	m.questionID = ""
	m.questionCh = nil
	m.questionsData = nil
	m.questionIdx = 0
	m.questionAnswers = nil
	m.textinput.Placeholder = ""
	m.textinput.Reset()
	return m
}

func parseOptionNumber(input string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(input))
}
