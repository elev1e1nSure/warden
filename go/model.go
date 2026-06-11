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
	escPending  bool
	quitPending bool
	// viewport scroll: user manually scrolled up during streaming
	userScrolled bool
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
	vp.MouseWheelEnabled = false

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
	if !connected {
		m.appendText(m.wardenLine("not set up — type /connect to get started"))
	}
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.checkBackend(), m.tick(), m.fetchSkills())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if mouse, ok := msg.(tea.MouseMsg); ok {
		switch mouse.Button {
		case tea.MouseButtonWheelUp:
			m.userScrolled = true
			m.viewport.LineUp(5)
		case tea.MouseButtonWheelDown:
			m.viewport.LineDown(5)
			if m.viewport.AtBottom() {
				m.userScrolled = false
			}
		}
		return m, nil
	}
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
		m.textinput.Width = msg.Width - 6
		m.updateViewportHeight()
		m.syncViewport()

	case tea.KeyMsg:
		// Clear pending confirmations if user presses a different key
		if msg.Type != tea.KeyEsc {
			m.escPending = false
		}
		if msg.Type != tea.KeyCtrlC {
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
			if m.streaming {
				m.userScrolled = true
				m.viewport.LineUp(3)
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
			if m.streaming {
				m.viewport.LineDown(3)
				if m.viewport.AtBottom() {
					m.userScrolled = false
				}
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
			}

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
				m.textinput.Reset()
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
				m.textinput.Reset()
				m.updateViewportHeight()
				m.syncViewport()
				return m, tea.Batch(m.focusInput(), m.sendConfirm(id, false), readNext(ch))
			}
			m.textinput.Reset()

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
					m.textinput.Reset()
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
					m.textinput.Reset()
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

		case tea.KeyEnter:
			if m.modelPicking {
				if m.modelPickIdx < len(m.modelFiltered) {
					chosen := m.modelFiltered[m.modelPickIdx]
					m.modelPicking = false
					m.modelList = nil
					m.modelFiltered = nil
					m.textinput.Reset()
					m.updateViewportHeight()
					return m, tea.Batch(m.focusInput(), m.applyModel(chosen))
				}
				return m, nil
			}
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
			text := strings.TrimSpace(m.textinput.Value())
			if text == "" {
				return m, nil
			}
			if handled, cmd := m.handleSlash(text); handled {
				m.textinput.Reset()
				return m, cmd
			}

			if strings.HasPrefix(text, "!") {
				if handled, cmd := m.handleBang(text); handled {
					m.textinput.Reset()
					return m, cmd
				}
				return m, nil
			}

			if !m.connected {
				m.appendText(UserStyle().Render("  > ") + text)
				m.appendText(m.wardenLine(DimStyle().Render("not connected — run /connect to get started")))
				m.textinput.Reset()
				m.syncViewport()
				return m, nil
			}

			m.streamStart = len(m.messages)
			m.appendText(UserStyle().Render("> ") + text)
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
			m.lastAssistantRaw = ""
			if m.activityIdx == -1 {
				m.appendText(WardenStyleAuto(m.autoMode).Render("  warden"))
			}
			m.activityIdx = m.resetOrAppendThink()
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
				if !m.thinkDone && len(m.messages) > 0 {
					m.finishThink()
				}
				m.appendToolActivity(toolStartLine(inner.name, inner.args))
				m.runningToolIdx = len(m.messages) - 1
			} else {
				m.appendToolFlow(toolDisplayName(inner.name), inner.args)
				m.runningToolIdx = len(m.messages) - 1
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
				if m.runningToolIdx >= 0 && m.runningToolIdx < len(m.messages) {
					if m.messages[m.runningToolIdx].kind == messageToolFlow {
						m.messages[m.runningToolIdx].toolDone = true
					}
				}
			}
			if inner.tool.Diff != "" {
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
			m.textinput.Reset()
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
			m.textinput.Reset()
			m.updateViewportHeight()
			m.syncViewport()

		case doneMsg:
			m.streaming = false
			m.loading = false
			m.toolRunning = false
			m.escPending = false
			m.quitPending = false
			m.userScrolled = false
			m.finishThink()
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
				m.appendText(m.wardenLine(DimStyle().Render("context at " + fmt.Sprintf("%d%%", m.tokenCount*100/m.tokenLimit) + ", compacting...")))
				cmds = append(cmds, m.runCompact(), m.tick())
			}
		}

	case doneMsg:
		if m.streaming {
			m.streaming = false
			m.loading = false
			m.toolRunning = false
			m.finishThink()
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
		var line string
		if msg.brief {
			line = msg.model
		} else {
			line = "model: " + msg.model + "  mode: " + msg.mode + "  cwd: " + msg.cwd
		}

		m.appendText(m.wardenLine(DimStyle().Render(line)))
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

	case modelsResultMsg:
		if msg.err != "" {
			m.appendText(m.wardenLine(ErrorStyle().Render("models: " + msg.err)))
			m.syncViewport()
		} else if len(msg.models) == 0 {
			m.appendText(m.wardenLine(DimStyle().Render("no models found")))
			m.syncViewport()
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
			m.textinput.Reset()
			m.modelPicking = true
			m.updateViewportHeight()
			m.syncViewport()
		}

	case modelSetMsg:
		if msg.err != "" {
			m.appendText(m.wardenLine(ErrorStyle().Render("model: " + msg.err)))
			m.syncViewport()
		} else {
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
			m.appendText(m.wardenLine(DimStyle().Render("connected  " + msg.model)))
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
			m.appendText(ErrorStyle().Render("error: " + msg.err))
			m.syncViewport()
			break
		}
		// update the "! name (loading)" text line
		for i := len(m.messages) - 1; i >= 0; i-- {
			if m.messages[i].kind == messageText && strings.HasPrefix(m.messages[i].text, "! ") {
				m.messages[i].text = "! " + msg.name
				break
			}
		}
		// send skill body as user message
		body := "Use the skill \"" + msg.name + "\". Follow these instructions:\n\n" + msg.content
		m.streamStart = len(m.messages)
		m.appendText(UserStyle().Render("  > ") + body[:min(len(body), 200)])
		m.textinput.Reset()
		m.streaming = true
		m.loading = true
		m.spinner = 0
		m.syncViewport()
		cmds = append(cmds, tea.Batch(m.sendMessage(body), m.tick()))

	case backendErrorMsg:
		m.loading = false
		m.appendText(ErrorStyle().Render("Error: backend unavailable"))
		m.syncViewport()
	}

	cmds = append(cmds, m.focusInput())

	var cmd tea.Cmd
	oldVal := m.textinput.Value()
	m.textinput, cmd = m.textinput.Update(msg)
	cmds = append(cmds, cmd)
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

	// sync hint visibility and viewport height
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
	messageText messageKind = iota
	messageThink
	messageAssistant
	messageToolActivity // tool line, filtered out at turn end in normal mode
	messageToolDiff     // diff block, persists in history even in non-verbose mode
	messageToolFlow     // live tool activity shown as flowing lines
)

type messageEntry struct {
	kind      messageKind
	text      string
	startedAt time.Time
	duration  time.Duration
	activity  string // current verb shown in single-line activity mode
	toolName  string // display name for messageToolFlow
	toolArgs  string // tool arguments (e.g. search query) for display
	toolDone  bool   // true when the tool has finished
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

// resetOrAppendThink reuses the existing think entry for the current turn (keeps
// original startedAt so total duration is correct), or creates a new one if none
// exists yet. Old text is cleared since this is a fresh thinking phase.
func (m *model) resetOrAppendThink() int {
	for i := len(m.messages) - 1; i >= m.streamStart; i-- {
		if m.messages[i].kind == messageThink {
			m.messages[i].duration = 0
			m.messages[i].activity = ""
			m.messages[i].text = ""
			return i
		}
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

// compactToolFlow removes live think/tool flow entries and leaves one summary line.
func (m *model) compactToolFlow() {
	if m.verboseMode {
		return
	}
	var tools []string
	seen := make(map[string]bool)
	var filtered []messageEntry
	for _, entry := range m.messages {
		switch entry.kind {
		case messageToolFlow:
			if entry.toolName != "" && !seen[entry.toolName] {
				seen[entry.toolName] = true
				tools = append(tools, entry.toolName)
			}
		case messageThink:
			// drop
		default:
			filtered = append(filtered, entry)
		}
	}
	m.messages = filtered
	if len(tools) > 0 {
		summary := strings.Join(tools, " · ")
		m.appendText(DimStyle().Render("  " + summary))
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
