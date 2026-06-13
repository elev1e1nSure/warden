package tui

import (
	"os"
	"strings"
	"time"

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
	escPending       bool
	quitPending      bool
	quitPendingSince time.Time
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
	// skills hint cycling
	skillsIdx   int
	skillsTyped string
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
	ti.Prompt = ""
	ti.ShowLineNumbers = false
	ti.CharLimit = 0
	ti.EndOfBufferCharacter = 0

	// strip textarea default styles: no backgrounds, no borders
	plain := lipgloss.NewStyle()
	for _, s := range []*textarea.Style{&ti.FocusedStyle, &ti.BlurredStyle} {
		s.Base = plain
		s.CursorLine = plain
		s.CursorLineNumber = plain
		s.EndOfBuffer = plain
		s.LineNumber = plain
		s.Prompt = plain
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
		skillsIdx:      -1,
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
		m.textinput.SetWidth(m.inputContentWidth())
		m.updateViewportHeight()
		m.syncViewport()

	case tea.KeyMsg:
		newM, kcmd, handled := m.handleKey(msg)
		m = newM
		if handled {
			return m, kcmd
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
			m.loading = true
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
			m.loading = false
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
			m.loading = false
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
			if cmd := m.finishStream(inner.tokenCount, inner.tokenLimit); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case doneMsg:
		m.interruptStream = false
		if m.streaming {
			if cmd := m.finishStream(msg.tokenCount, msg.tokenLimit); cmd != nil {
				cmds = append(cmds, cmd)
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
			m.spinner++
			if m.streaming && !m.confirming && len(m.messages) > 0 {
				m.syncViewport()
			}
			// Clear quitPending after 3 seconds
			if m.quitPending && time.Since(m.quitPendingSince) > 3*time.Second {
				m.quitPending = false
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

	case memoryResultMsg:
		m.loading = false
		if msg.err != "" {
			m.appendText(ErrorStyle().Render("  memory error: " + msg.err))
		} else {
			m.appendText(DimStyle().Render("  " + msg.text))
		}
		m.appendText("")
		m.syncViewport()

	case updateResultMsg:
		m.loading = false
		if msg.err != nil {
			m.appendText(ErrorStyle().Render("  update failed: " + msg.err.Error()))
			m.appendText("")
			m.syncViewport()
			return m, nil
		}
		m.appendText(DimStyle().Render("  update downloaded, restarting..."))
		m.syncViewport()
		return m, tea.Quit

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
			m.appendText(ErrorStyle().Render("  " + msg.err))
			m.appendText("")
			m.syncViewport()
			break
		}
		body := "Use the skill \"" + msg.name + "\". Follow these instructions:\n\n" + msg.content
		m.appendText(body)
		cmds = append(cmds, m.beginStream(body))

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
	if m.skillsIdx >= 0 && m.textinput.Value() != oldVal {
		m.skillsIdx = -1
	}
	if m.modelPicking && m.textinput.Value() != oldVal {
		m.modelFiltered = filterModels(m.modelList, m.textinput.Value())
		m.modelPickIdx = 0
		m.modelScrollTop = 0
		m.updateViewportHeight()
		m.syncViewport()
	}
	// Don't scroll message history when the mouse wheel is used over the
	// prompt bar or overlays (everything below the viewport).
	if mouseMsg, ok := msg.(tea.MouseMsg); ok && (mouseMsg.Type == tea.MouseWheelUp || mouseMsg.Type == tea.MouseWheelDown) {
		if mouseMsg.Y < m.layoutViewportHeight() {
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.refreshHints()

	return m, tea.Batch(cmds...)
}

// resolveConfirm closes the confirm dialog and sends the verdict to the backend.
func (m model) resolveConfirm(ok bool) (model, tea.Cmd) {
	ch := m.confirmCh
	id := m.confirmID
	m.confirming = false
	m.confirmID = ""
	m.confirmCh = nil
	m.confirmTool = ""
	m.textinput.Placeholder = ""
	m.resetInput()
	m.updateViewportHeight()
	m.syncViewport()
	return m, tea.Batch(m.focusInput(), m.sendConfirm(id, ok), readNext(ch))
}

// answerQuestion records the answer for the current question and advances;
// after the last question it sends all answers to the backend.
func (m model) answerQuestion(answer string) (model, tea.Cmd) {
	m.questionAnswers = append(m.questionAnswers, []string{answer})
	m.questionIdx++
	if m.questionIdx < len(m.questionsData) {
		m.syncViewport()
		return m, m.focusInput()
	}
	ch := m.questionCh
	id := m.questionID
	answers := m.questionAnswers
	saved := m.questionsData
	m = m.clearQuestionState()
	m.appendQuizHistory(saved, answers)
	m.updateViewportHeight()
	m.syncViewport()
	return m, tea.Batch(m.focusInput(), m.sendQuestion(id, answers), readNext(ch))
}

// beginStream marks the start of a streaming turn and sends text to the backend.
func (m *model) beginStream(text string) tea.Cmd {
	m.streamStart = len(m.messages)
	m.resetInput()
	m.startChain()
	m.streaming = true
	m.loading = true
	m.spinner = 0
	m.syncViewport()
	return tea.Batch(m.sendMessage(text), m.tick())
}

func (m *model) beginSkillStream(name string) tea.Cmd {
	m.streamStart = len(m.messages)
	m.resetInput()
	m.startChain()
	m.streaming = true
	m.loading = true
	m.spinner = 0
	m.syncViewport()
	return tea.Batch(m.sendSkill(name), m.tick())
}

// finishStream resets streaming state at turn end; returns a compact command
// when the context is close to the token limit.
func (m *model) finishStream(tokenCount, tokenLimit int) tea.Cmd {
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
	if tokenLimit > 0 {
		m.tokenCount = tokenCount
		m.tokenLimit = tokenLimit
	}
	m.syncViewport()
	if m.tokenLimit > 0 && m.tokenCount > int(float64(m.tokenLimit)*0.85) {
		m.loading = true
		return tea.Batch(m.runCompact(), m.tick())
	}
	return nil
}
