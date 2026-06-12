package tui

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	viewport  viewport.Model
	textinput textarea.Model
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
	// path
	cwd string
	// index of the in-progress tool line in messages (-1 = none)
	runningToolIdx int
	// verbose mode — shows tool lines, errors, think duration
	verboseMode bool
	// select mode — mouse capture disabled so terminal can select text
	selectMode bool
	// model picker
	modelPicking   bool
	modelList      []string
	modelFiltered  []string
	modelPickIdx   int
	modelScrollTop int
	// activity tracking (index of current think/activity entry)
	activityIdx int
	// tool chain (non-verbose collapsing): grouped tally + turn timing
	chainCounts map[string]int
	chainOrder  []string
	chainStart  time.Time
	// last raw assistant response (for /copy-last)
	lastAssistantRaw string
	// interrupt state
	interruptStream bool
	streamStart     int
	// pending double-press confirmations (during streaming)
	escPending  bool
	quitPending bool
	// viewport scroll: user manually scrolled up during streaming
	userScrolled bool
	// token tracking
	tokenCount int
	tokenLimit int
	// paste handling: stored payloads referenced by [pasted #N] placeholders
	pastes     []string
	lastRuneAt time.Time
	// input command history (recall with Up/Down at edge lines)
	history    []string
	historyIdx int
	// confirm dialog data
	confirmRisk    string
	confirmTitle   string
	confirmSummary string
	confirmDetails []string
	confirmPreview string
	confirmDefault string
	// slash command cycling
	slashIdx   int
	slashTyped string
	// skills (fetched from backend on startup)
	skills    []Skill
	skillsErr string
	// markdown
	mdRenderer *glamour.TermRenderer
	mdWidth    int
	// connection
	connected bool
	// connect wizard
	cwOpen     bool
	cwStep     int    // 0=provider 1=apikey 2=model
	cwProvider string // "openrouter" | "ollama"
	cwInput    textinput.Model
	cwModels   []string
	cwPickIdx  int
	cwScroll   int
	cwCustom   bool
	cwLoading  bool
	cwErr      string
	cwAPIKey   string
}

func filterModels(models []string, filter string) []string {
	if filter == "" {
		return models
	}
	lower := strings.ToLower(filter)
	var result []string
	for _, m := range models {
		if strings.Contains(strings.ToLower(m), lower) {
			result = append(result, m)
		}
	}
	return result
}

func initialModel(modelName string, connected bool) model {
	ti := textarea.New()
	ti.Placeholder = ""
	ti.Prompt = "> "
	ti.ShowLineNumbers = false
	ti.CharLimit = 0
	ti.EndOfBufferCharacter = 0

	// strip textarea default styles: no backgrounds, no borders
	plain := lipgloss.NewStyle()
	dimPrompt := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	for _, s := range []*textarea.Style{&ti.FocusedStyle, &ti.BlurredStyle} {
		s.Base = plain
		s.CursorLine = plain
		s.CursorLineNumber = plain
		s.EndOfBuffer = plain
		s.LineNumber = plain
		s.Prompt = dimPrompt
		s.Text = plain
	}

	ti.SetWidth(80)
	ti.SetHeight(1)
	ti.Focus()

	vp := viewport.New(80, 20)
	vp.SetContent("")
	vp.GotoTop()
	vp.MouseWheelEnabled = true

	cwd, _ := os.Getwd()
	m := model{
		textinput:      ti,
		viewport:       vp,
		client:         NewClient("http://localhost:8765"),
		messages:       []messageEntry{},
		autoMode:       loadAutoMode(),
		cwd:            cwd,
		modelName:      modelName,
		connected:      connected,
		loading:        true,
		runningToolIdx: -1,
		slashIdx:       -1,
		activityIdx:    -1,
	}
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.checkBackend(), m.tick(), m.fetchSkills())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// route key events to wizard when open
	if key, ok := msg.(tea.KeyMsg); ok && m.cwOpen {
		if handled, cmd := m.handleConnectWizardKey(key); handled {
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.viewport.Width = msg.Width
		m.textinput.SetWidth(msg.Width - 6)
		m.updateViewportHeight()
		m.syncViewport()

	case tea.KeyMsg:
		modal := m.confirming || m.questioning || m.modelPicking
		// Native bracketed paste (or multi-rune burst): collapse big/multiline
		// pastes into a [pasted #N] placeholder, insert small ones inline.
		if msg.Type == tea.KeyRunes && (msg.Paste || len(msg.Runes) > 1) && !modal {
			m.insertPaste(string(msg.Runes))
			m.lastRuneAt = time.Now()
			m.syncInputHeight()
			m.refreshHints()
			return m, m.focusInput()
		}
		// Clear pending confirmations if user presses a different key
		if msg.Type != tea.KeyEsc {
			m.escPending = false
		}
		// Only clear quitPending on non-control keys (Ctrl alone shouldn't reset)
		if !strings.HasPrefix(string(msg.Type), "ctrl") && msg.Type != tea.KeyCtrlC {
			m.quitPending = false
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.streaming {
				if m.quitPending {
					return m, tea.Quit
				}
				m.quitPending = true
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyUp:
			if m.modelPicking {
				if m.modelPickIdx > 0 {
					m.modelPickIdx--
					if m.modelPickIdx < m.modelScrollTop {
						m.modelScrollTop = m.modelPickIdx
					}
					m.updateViewportHeight()
					m.syncViewport()
				}
				return m, nil
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
				return m, nil
			}

		case tea.KeyDown:
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
				return m, nil
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
				return m, nil
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
			return m, nil

		case tea.KeyTab:
			val := m.textinput.Value()
			matches := matchSlash(val)
			if len(matches) == 1 {
				m.textinput.SetValue(matches[0].name)
				m.textinput.CursorEnd()
			} else if len(matches) > 1 {
				m.textinput.SetValue(slashCommonPrefix(matches))
				m.textinput.CursorEnd()
			}

		case tea.KeyShiftTab:
			if !m.streaming {
				m.autoMode = !m.autoMode
				return m, m.setMode(m.autoMode)
			}

		case tea.KeyEsc:
			if m.selectMode {
				m.selectMode = false
				return m, tea.EnableMouseCellMotion
			}

			if m.modelPicking {
				m.modelPicking = false
				m.modelList = nil
				m.modelFiltered = nil
				m.resetInput()
				m.updateViewportHeight()
				m.syncViewport()
				return m, m.focusInput()
			}
			if m.streaming && !m.questioning && !m.confirming {
				if !m.escPending {
					m.escPending = true
					return m, nil
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
				return m, m.focusInput()
			}
			if m.questioning {
				ch := m.questionCh
				id := m.questionID
				m = m.clearQuestionState()
				m.updateViewportHeight()
				m.syncViewport()
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
				m.resetInput()
				m.updateViewportHeight()
				m.syncViewport()
				return m, tea.Batch(m.focusInput(), m.sendConfirm(id, false), readNext(ch))
			}
			m.resetInput()

		case tea.KeyRunes:
			if m.confirming {
				r := strings.ToLower(string(msg.Runes))
				if r == "y" || r == "н" {
					ok := true
					ch := m.confirmCh
					id := m.confirmID
					m.confirming = false
					m.confirmID = ""
					m.confirmCh = nil
					m.confirmTool = ""
					m.textinput.Placeholder = ""
					m.resetInput()
					return m, tea.Batch(m.focusInput(), m.sendConfirm(id, ok), readNext(ch))
				}
				if r == "n" || r == "т" {
					ok := false
					ch := m.confirmCh
					id := m.confirmID
					m.confirming = false
					m.confirmID = ""
					m.confirmCh = nil
					m.confirmTool = ""
					m.textinput.Placeholder = ""
					m.resetInput()
					return m, tea.Batch(m.focusInput(), m.sendConfirm(id, ok), readNext(ch))
				}
				return m, nil
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
							m.updateViewportHeight()
							m.syncViewport()
							return m, tea.Batch(m.focusInput(), m.sendQuestion(id, answers), readNext(ch))
						}
						m.syncViewport()
						return m, m.focusInput()
					}
				}
			}
			m.textinput.Placeholder = ""
			m.lastRuneAt = time.Now()

		case tea.KeyEnter:
			if m.modelPicking {
				if m.modelPickIdx < len(m.modelFiltered) {
					chosen := m.modelFiltered[m.modelPickIdx]
					m.modelPicking = false
					m.modelList = nil
					m.modelFiltered = nil
					m.resetInput()
					m.updateViewportHeight()
					return m, tea.Batch(m.focusInput(), m.applyModel(chosen))
				}
				return m, nil
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
						m.updateViewportHeight()
						m.syncViewport()
						return m, tea.Batch(m.focusInput(), m.sendQuestion(id, answers), readNext(ch))
					}
					m.syncViewport()
					return m, m.focusInput()
				}
				return m, nil
			}
			if m.confirming {
				return m, nil
			}
			if m.streaming {
				return m, nil
			}
			// Enter-guard: on legacy consoles a pasted newline arrives as a
			// KeyEnter inside a rune burst — treat it as a newline, not submit.
			if time.Since(m.lastRuneAt) < 8*time.Millisecond {
				m.textinput.InsertString("\n")
				m.lastRuneAt = time.Now()
				m.syncInputHeight()
				return m, nil
			}
			val := m.textinput.Value()
			// \ at end of line + Enter = shell-style line continuation
			if strings.HasSuffix(val, "\\") {
				m.textinput.SetValue(val[:len(val)-1] + "\n")
				m.textinput.CursorEnd()
				m.syncInputHeight()
				return m, nil
			}
			text := strings.TrimSpace(m.expandPastes(val))
			if text == "" {
				return m, nil
			}
			if handled, cmd := m.handleSlash(text); handled {
				m.resetInput()
				return m, cmd
			}

			if strings.HasPrefix(text, "!") {
				if handled, cmd := m.handleBang(text); handled {
					m.resetInput()
					return m, cmd
				}
				return m, nil
			}

			if !m.connected {
				m.resetInput()
				return m, nil
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
			m.lastAssistantRaw = ""
			if m.verboseMode {
				m.activityIdx = m.resetOrAppendThink()
			} else {
				m.setAction("Thinking", "", true)
			}
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case thinkMsg:
			m.thinkBuf += inner.text
			if m.verboseMode {
				m.updateThink(inner.text)
			} else {
				m.setAction("Thinking", "", true)
			}
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case tokenMsg:
			if !m.thinkDone {
				if m.verboseMode {
					m.finishThink()
				} else {
					m.clearAction()
				}
				m.appendAssistant("")
				m.thinkDone = true
			}
			m.appendToLastAssistant(inner.text)
			m.lastAssistantRaw += inner.text
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case toolStartMsg:
			m.toolRunning = true
			if m.verboseMode {
				m.finishThink()
				m.appendToolActivity(toolStartLine(inner.name, inner.args))
				m.runningToolIdx = len(m.messages) - 1
			} else {
				display := toolDisplayName(inner.name)
				m.clearAction()
				m.ensureCounter()
				m.setAction(toolPresentTense(display), actionDetail(display, inner.args), false)
			}
			m.syncViewport()
			cmds = append(cmds, readNext(msg.ch))

		case toolMsg:
			m.toolRunning = false
			if m.verboseMode {
				summary := toolSummaryLine(inner.tool.Name, inner.tool.Args, inner.tool.Result)
				if m.runningToolIdx >= 0 && m.runningToolIdx < len(m.messages) {
					m.messages[m.runningToolIdx].text = summary
				} else {
					m.appendToolActivity(summary)
				}
			} else {
				m.bumpChain(toolDisplayName(inner.tool.Name))
			}
			if m.verboseMode && inner.tool.Diff != "" {
				m.messages = append(m.messages, messageEntry{kind: messageToolDiff, text: inner.tool.Diff})
			}
			m.runningToolIdx = -1
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
			m.confirmDefault = inner.defaultVal
			m.updateViewportHeight()
			m.syncViewport()
			m.textinput.Placeholder = ""
			m.resetInput()
			if inner.defaultVal != "" && inner.defaultVal != "cancel" {
				m.textinput.SetValue(inner.defaultVal)
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
			m.resetInput()
			m.updateViewportHeight()
			m.syncViewport()

		case doneMsg:
			m.streaming = false
			m.loading = false
			m.toolRunning = false
			m.escPending = false
			m.quitPending = false
			m.userScrolled = false
			if m.verboseMode {
				m.finishThink()
			} else {
				m.freezeChain()
			}
			m.thinkBuf = ""
			m.thinkDone = false
			m.activityIdx = -1
			m.appendText("")
			if inner.tokenLimit > 0 {
				m.tokenCount = inner.tokenCount
				m.tokenLimit = inner.tokenLimit
			}
			m.syncViewport()
			if m.tokenLimit > 0 && m.tokenCount > int(float64(m.tokenLimit)*0.85) {
				m.loading = true
				cmds = append(cmds, m.runCompact(), m.tick())
			}
		}

	case doneMsg:
		if m.streaming {
			m.streaming = false
			m.loading = false
			m.toolRunning = false
			if m.verboseMode {
				m.finishThink()
			} else {
				m.freezeChain()
			}
			m.thinkBuf = ""
			m.thinkDone = false
			m.activityIdx = -1
			m.appendText("")
			if msg.tokenLimit > 0 {
				m.tokenCount = msg.tokenCount
				m.tokenLimit = msg.tokenLimit
			}
			m.syncViewport()
			if m.tokenLimit > 0 && m.tokenCount > int(float64(m.tokenLimit)*0.85) {
				m.loading = true
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
		m.appendText(DimStyle().Render(strings.TrimRight(msg.output, "\n")))
		m.appendText("")
		m.syncViewport()

	case tickMsg:
		if m.loading {
			m.spinner += m.advance()
			if m.streaming && !m.confirming && len(m.messages) > 0 {
				m.syncViewport()
			}
			return m, m.tick()
		}
		return m, nil

	case modeMsg:
		m.autoMode = msg.auto
		m.syncViewport()

	case statusResultMsg:
		if msg.tokenLimit > 0 {
			m.tokenCount = msg.tokenCount
			m.tokenLimit = msg.tokenLimit
		}
		if msg.model != "" {
			m.modelName = msg.model
		}
		m.syncViewport()

	case clipboardDoneMsg:
		m.syncViewport()

	case compactResultMsg:
		m.loading = false
		if msg.err == "" {
			m.tokenCount = msg.tokensAfter
		}
		m.syncViewport()

	case modelsResultMsg:
		if msg.err != "" || len(msg.models) == 0 {
			break
		} else {
			m.modelList = msg.models
			m.modelFiltered = msg.models
			m.modelPickIdx = 0
			for i, name := range msg.models {
				if name == msg.current {
					m.modelPickIdx = i
					break
				}
			}
			const maxVisible = 8
			m.modelScrollTop = m.modelPickIdx - maxVisible/2
			if m.modelScrollTop < 0 {
				m.modelScrollTop = 0
			}
			if m.modelScrollTop+maxVisible > len(msg.models) {
				m.modelScrollTop = len(msg.models) - maxVisible
				if m.modelScrollTop < 0 {
					m.modelScrollTop = 0
				}
			}
			m.resetInput()
			m.modelPicking = true
			m.updateViewportHeight()
			m.syncViewport()
		}

	case modelSetMsg:
		if msg.err == "" {
			m.modelName = msg.model
			m.messages = []messageEntry{}
			_ = saveWardenConfigField("model", msg.model)
		}

	case connectResultMsg:
		if msg.ok {
			m.connected = true
			m.modelName = msg.model
			m.cwOpen = false
			m.cwLoading = false
			m.cwErr = ""
			_ = saveWardenConfigField("model", msg.model)
			if msg.apiURL != "" {
				_ = saveWardenConfigField("api_url", msg.apiURL)
			}
			if msg.apiKey != "" {
				_ = saveWardenConfigField("api_key", msg.apiKey)
			}
			m.updateViewportHeight()
			m.syncViewport()
		} else {
			m.cwErr = msg.err
			m.cwLoading = false
			m.updateViewportHeight()
			m.syncViewport()
		}

	case backendReadyMsg:
		m.loading = false
		m.tokenCount = 0
		m.client.ResetSession()
		m.syncViewport()
		if m.autoMode {
			cmds = append(cmds, m.setMode(true))
		}

	case skillsResultMsg:
		if msg.err != "" {
			m.skillsErr = msg.err
			break
		}
		m.skills = msg.skills
		m.skillsErr = ""

	case skillLoadedMsg:
		m.streaming = false
		m.loading = false
		if msg.err != "" {
			break
		}
		body := "Use the skill \"" + msg.name + "\". Follow these instructions:\n\n" + msg.content
		m.streamStart = len(m.messages)
		m.resetInput()
		m.startChain()
		m.streaming = true
		m.loading = true
		m.spinner = 0
		m.syncViewport()
		cmds = append(cmds, tea.Batch(m.sendMessage(body), m.tick()))

	case backendErrorMsg:
		m.loading = false
		m.syncViewport()
	}

	cmds = append(cmds, m.focusInput())

	var cmd tea.Cmd
	oldVal := m.textinput.Value()
	m.textinput, cmd = m.textinput.Update(msg)
	cmds = append(cmds, cmd)
	m.syncInputHeight()
	if m.slashIdx >= 0 && m.textinput.Value() != oldVal {
		m.slashIdx = -1
	}
	if m.modelPicking && m.textinput.Value() != oldVal {
		m.modelFiltered = filterModels(m.modelList, m.textinput.Value())
		m.modelPickIdx = 0
		m.modelScrollTop = 0
		m.updateViewportHeight()
		m.syncViewport()
	}
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	m.refreshHints()

	return m, tea.Batch(cmds...)
}

// refreshHints recomputes slash/bang hint visibility and resizes the viewport.
func (m *model) refreshHints() {
	slashMatches := matchSlash(m.textinput.Value())
	bangMatches := matchBang(m.textinput.Value(), m.skills)
	newCount := len(slashMatches) + len(bangMatches)
	if newCount != m.hintCount {
		m.hintCount = newCount
		m.hintVisible = newCount > 0
		if m.height > 0 {
			m.updateViewportHeight()
			m.syncViewport()
		}
	}
}

// insertPaste inserts pasted text at the cursor. Multiline or long pastes become
// a [pasted #N, M lines] placeholder expanded on submit; small ones go inline.
func (m *model) insertPaste(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Count(text, "\n") + 1
	if lines > 1 || len([]rune(text)) > 120 {
		m.pastes = append(m.pastes, text)
		m.textinput.InsertString(fmt.Sprintf("[pasted #%d, %d lines]", len(m.pastes), lines))
		return
	}
	m.textinput.InsertString(text)
}

var pastePlaceholderRe = regexp.MustCompile(`\[pasted #(\d+), \d+ lines\]`)

// expandPastes swaps [pasted #N] placeholders back to their stored payloads.
func (m *model) expandPastes(s string) string {
	if len(m.pastes) == 0 {
		return s
	}
	return pastePlaceholderRe.ReplaceAllStringFunc(s, func(match string) string {
		sub := pastePlaceholderRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		idx, err := strconv.Atoi(sub[1])
		if err != nil || idx < 1 || idx > len(m.pastes) {
			return match
		}
		return m.pastes[idx-1]
	})
}

// recordHistory appends a submitted line to the recall history.
func (m *model) recordHistory(text string) {
	if n := len(m.history); n > 0 && m.history[n-1] == text {
		m.historyIdx = len(m.history)
		return
	}
	m.history = append(m.history, text)
	m.historyIdx = len(m.history)
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
	skills []Skill
	err    string
}
type skillLoadedMsg struct {
	name    string
	content string
	err     string
}

type messageKind int

const (
	messageText   messageKind = iota
	messageUser               // user input, rendered with background
	messageWarden             // warden label, first line of response block
	messageThink
	messageAssistant
	messageToolActivity // tool line, filtered out at turn end in normal mode
	messageToolDiff     // diff block, persists in history even in non-verbose mode
	messageToolFlow     // live tool activity shown as flowing lines (verbose)
	messageChainCounter // non-verbose: running grouped tool tally, frozen at turn end
	messageChainAction  // non-verbose: single live "what's happening now" line
)

type messageEntry struct {
	kind      messageKind
	text      string
	startedAt time.Time
	duration  time.Duration
	activity  string // present-tense verb for the live action line
	toolName  string // display name for messageToolFlow
	toolArgs  string // tool arguments / detail (query, url, file) for display
	toolDone  bool   // true when the tool has finished
	thinking  bool   // chain action line: model is reasoning (animated dots)
}

func (m *model) appendText(text string) {
	m.messages = append(m.messages, messageEntry{kind: messageText, text: text})
}

func (m *model) appendToolActivity(text string) {
	m.messages = append(m.messages, messageEntry{kind: messageToolActivity, text: text})
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
	m.messages = append(m.messages, messageEntry{kind: messageThink, startedAt: time.Now()})
}

// resetOrAppendThink reuses the last think entry only if it is still at the
// tail of the message list (no tools were added after it). Otherwise creates a
// new entry so the new Thinking line appears after completed tools.
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
	m.resetInput()
	return m
}

func (m *model) appendQuizHistory(questions []QuestionItem, answers [][]string) {
	var b strings.Builder
	for i, q := range questions {
		if i > 0 {
			b.WriteString("\n")
		}
		ans := "—"
		if i < len(answers) && len(answers[i]) > 0 {
			ans = strings.Join(answers[i], ", ")
		}
		label := q.Header
		if label == "" {
			label = q.Question
		}
		b.WriteString(AccentStyle().Render("  ? ") + DimStyle().Render(label) + "   " + ans)
	}
	m.appendText(b.String())
}

func parseOptionNumber(input string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(input))
}

// inputLineCount returns the number of visual lines the textarea content
// occupies, accounting for soft wrapping. Capped at 8.
func (m *model) inputLineCount() int {
	val := m.textinput.Value()
	if val == "" {
		return 1
	}
	// content width = textarea render width (m.width-6) minus prompt "> " (2)
	contentW := m.width - 8
	if contentW < 1 {
		contentW = 1
	}
	lines := strings.Split(val, "\n")
	total := 0
	for _, line := range lines {
		runes := []rune(line)
		if len(runes) == 0 {
			total++
			continue
		}
		wrapped := (len(runes) + contentW - 1) / contentW
		total += wrapped
	}
	if total < 1 {
		total = 1
	}
	if total > 8 {
		total = 8
	}
	return total
}

func (m *model) syncInputHeight() {
	n := m.inputLineCount()
	m.textinput.SetHeight(n)
}

func (m *model) resetInput() {
	m.textinput.Reset()
	m.textinput.SetHeight(1)
	m.pastes = nil
}
