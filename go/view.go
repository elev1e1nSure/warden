package tui

import "github.com/charmbracelet/lipgloss"

const wardenVersion = "v0.1.0"

// animDots returns the animated ellipsis frame for the given spinner step.
func animDots(step int) string {
	return [...]string{".", "..", "..."}[step%3]
}

func (m *model) renderMessages() []string {
	m.ensureMarkdownRenderer()
	// index of the latest think entry — only it may animate
	lastThinkIdx := -1
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].kind == messageThink {
			lastThinkIdx = i
			break
		}
	}
	out := make([]string, 0, len(m.messages))
	for i, entry := range m.messages {
		var rendered string
		switch entry.kind {
		case messageUser:
			rendered = m.renderUserMsg(entry.text)
		case messageThink:
			rendered = m.renderThinkEntry(entry, i == lastThinkIdx)
		case messageAssistant:
			rendered = indentLines(m.renderMarkdown(entry.text), "  ")
		case messageToolActivity:
			rendered = entry.text
		case messageChainCounter:
			rendered = m.renderChainCounter(entry)
		case messageChainAction:
			rendered = m.renderChainAction(entry)
		case messageToolDiff:
			rendered = renderUnifiedDiff(entry.text, m.width)
		default:
			rendered = entry.text
		}
		// always keep messageText (blank lines serve as turn separators)
		if rendered != "" || entry.kind == messageText {
			out = append(out, rendered)
		}
	}
	return out
}

func (m *model) syncViewport() {
	followTail := !m.userScrolled && (m.streaming || m.loading || m.viewport.AtBottom())
	m.viewport = setContent(m.viewport, m.renderMessages())
	if followTail {
		m.viewport.GotoBottom()
	}
}

func (m *model) layoutViewportHeight() int {
	if m.height < 1 {
		return 1
	}

	hintHeight := 0
	if m.hintVisible {
		hintHeight = lipgloss.Height(m.renderHint())
	}

	confirmHeight := 0
	if m.confirming {
		confirmHeight = lipgloss.Height(renderConfirmBlock(confirmMsg{
			title:   "Dangerous action",
			tool:    m.confirmTool,
			details: []string{},
		}, m.width, m.autoMode))
	}

	questionHeight := 0
	if m.questioning && len(m.questionsData) > 0 {
		questionHeight = lipgloss.Height(renderQuestionBlock(
			m.questionsData[m.questionIdx], m.questionIdx, len(m.questionsData), m.width, m.autoMode,
		))
	}

	modelPickerHeight := 0
	if m.modelPicking {
		modelPickerHeight = lipgloss.Height(renderModelPicker(m.modelFiltered, m.modelPickIdx, m.modelScrollTop, m.autoMode))
	}

	cwHeight := 0
	if m.cwOpen {
		cwHeight = lipgloss.Height(m.renderConnectWizard())
	}

	// input: border top + N content lines + border bottom
	// status bar: 2 lines
	inputHeight := m.inputLineCount() + 2
	reserved := hintHeight + confirmHeight + questionHeight + modelPickerHeight + cwHeight + inputHeight + 2
	height := m.height - reserved
	if height < 1 {
		height = 1
	}
	return height
}

func (m *model) updateViewportHeight() {
	m.viewport.Height = m.layoutViewportHeight()
}

func (m model) View() string {
	if m.height == 0 {
		return ""
	}

	layers := []string{m.viewport.View()}

	if m.confirming {
		layers = append(layers, renderConfirmBlock(confirmMsg{
			title:   m.confirmTitle,
			tool:    m.confirmTool,
			risk:    m.confirmRisk,
			summary: m.confirmSummary,
			details: m.confirmDetails,
			preview: m.confirmPreview,
		}, m.width, m.autoMode))
	}

	if m.questioning && len(m.questionsData) > 0 {
		layers = append(layers, renderQuestionBlock(
			m.questionsData[m.questionIdx], m.questionIdx, len(m.questionsData), m.width, m.autoMode,
		))
	}

	if m.modelPicking {
		layers = append(layers, renderModelPicker(m.modelFiltered, m.modelPickIdx, m.modelScrollTop, m.autoMode))
	}

	if m.cwOpen {
		layers = append(layers, m.renderConnectWizard())
	}

	if m.hintVisible {
		layers = append(layers, m.renderHint())
	}

	layers = append(layers, m.renderFullWave(), m.renderInput(), m.renderStatusBar())
	return lipgloss.JoinVertical(lipgloss.Left, layers...)
}
