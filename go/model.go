package tui

import (
	"os"
	"strings"
	"time"

	"warden/internal/client"

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
	client    *client.Client
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
	confirmCh   <-chan client.Event
	confirmTool string
	// question
	questioning     bool
	questionID      string
	questionCh      <-chan client.Event
	questionsData   []client.QuestionItem
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
	// viewport line → messages slice index (-1 = no entry)
	lineMap []int
	// index of message currently under the mouse cursor (-1 = none)
	hoveredMsgIdx int
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
	skills    []client.Skill
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
		client:         client.NewClient("http://localhost:8765"),
		messages:       []messageEntry{},
		autoMode:       loadAutoMode(),
		cwd:            cwd,
		modelName:      modelName,
		connected:      connected,
		loading:        true,
		runningToolIdx: -1,
		slashIdx:       -1,
		hoveredMsgIdx:  -1,
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
	var cmd tea.Cmd

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
		m, cmd = m.handleStartStreamMsg(msg)
		cmds = append(cmds, cmd)

	case nextMsg:
		m, cmd = m.handleNextMsg(msg)
		cmds = append(cmds, cmd)

	case doneMsg:
		m, cmd = m.handleDoneMsg(msg)
		cmds = append(cmds, cmd)

	case shellResultMsg:
		m, cmd = m.handleShellResult(msg)
		cmds = append(cmds, cmd)

	case tickMsg:
		m, cmd = m.handleTick(msg)
		cmds = append(cmds, cmd)

	case modeMsg:
		m, cmd = m.handleModeMsg(msg)
		cmds = append(cmds, cmd)

	case statusResultMsg:
		m, cmd = m.handleStatusResult(msg)
		cmds = append(cmds, cmd)

	case clipboardDoneMsg:
		m, cmd = m.handleClipboardDone(msg)
		cmds = append(cmds, cmd)

	case compactResultMsg:
		m, cmd = m.handleCompactResult(msg)
		cmds = append(cmds, cmd)

	case memoryResultMsg:
		m, cmd = m.handleMemoryResult(msg)
		cmds = append(cmds, cmd)

	case updateResultMsg:
		m, cmd = m.handleUpdateResult(msg)
		cmds = append(cmds, cmd)

	case modelsResultMsg:
		m, cmd = m.handleModelsResult(msg)
		cmds = append(cmds, cmd)

	case modelSetMsg:
		m, cmd = m.handleModelSet(msg)
		cmds = append(cmds, cmd)

	case connectResultMsg:
		m, cmd = m.handleConnectResult(msg)
		cmds = append(cmds, cmd)

	case backendReadyMsg:
		m, cmd = m.handleBackendReady(msg)
		cmds = append(cmds, cmd)

	case skillsResultMsg:
		m, cmd = m.handleSkillsResult(msg)
		cmds = append(cmds, cmd)

	case skillLoadedMsg:
		m, cmd = m.handleSkillLoaded(msg)
		cmds = append(cmds, cmd)

	case backendErrorMsg:
		m, cmd = m.handleBackendError(msg)
		cmds = append(cmds, cmd)

	case tea.MouseMsg:
		inViewport := msg.Y < m.layoutViewportHeight()
		if inViewport {
			line := msg.Y + m.viewport.YOffset
			newHover := -1
			if line >= 0 && line < len(m.lineMap) {
				idx := m.lineMap[line]
				if idx >= 0 && idx < len(m.messages) && m.isClickable(idx) {
					newHover = idx
				}
			}
			if newHover != m.hoveredMsgIdx {
				m.hoveredMsgIdx = newHover
				m.syncViewport()
			}
		} else if m.hoveredMsgIdx >= 0 {
			m.hoveredMsgIdx = -1
			m.syncViewport()
		}

		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && !m.selectMode && inViewport {
			line := msg.Y + m.viewport.YOffset
			if line >= 0 && line < len(m.lineMap) {
				idx := m.lineMap[line]
				if idx >= 0 && idx < len(m.messages) && m.isClickable(idx) {
					m.messages[idx].expanded = !m.messages[idx].expanded
					m.syncViewport()
					return m, tea.Batch(cmds...)
				}
			}
		}
	}

	cmds = append(cmds, m.focusInput())

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
	oldYOffset := m.viewport.YOffset
	if mouseMsg, ok := msg.(tea.MouseMsg); ok && (mouseMsg.Type == tea.MouseWheelUp || mouseMsg.Type == tea.MouseWheelDown) {
		if mouseMsg.Y < m.layoutViewportHeight() {
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.viewport.YOffset > oldYOffset {
		m.userScrolled = true
	}

	m.refreshHints()

	return m, tea.Batch(cmds...)
}

// isClickable reports whether the message at idx can be expanded/collapsed.
func (m model) isClickable(idx int) bool {
	if idx < 0 || idx >= len(m.messages) {
		return false
	}
	e := m.messages[idx]
	return (e.kind == messageThink && e.text != "") ||
		(e.kind == messageToolActivity && e.toolDone && (e.toolResult != "" || e.toolDiff != ""))
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
	return m, tea.Batch(m.focusInput(), m.sendQuestion(id, answers), readNext(ch), m.tick())
}

// beginStream marks the start of a streaming turn and sends text to the backend.
func (m *model) beginStream(text string) tea.Cmd {
	m.streamStart = len(m.messages)
	m.resetInput()
	m.streaming = true
	m.loading = true
	m.spinner = 0
	m.userScrolled = false
	m.syncViewport()
	return tea.Batch(m.sendMessage(text), m.tick())
}

func (m *model) beginSkillStream(name, args string) tea.Cmd {
	m.streamStart = len(m.messages)
	m.resetInput()
	m.streaming = true
	m.loading = true
	m.spinner = 0
	m.userScrolled = false
	m.syncViewport()
	return tea.Batch(m.sendSkill(name, args), m.tick())
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
	m.finishThink()
	m.freezeChain() // remove any live chain action line (non-verbose tool/think indicators)
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
