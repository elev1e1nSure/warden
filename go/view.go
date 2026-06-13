package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
	// index of the latest chain-action entry — only it may animate
	lastActionIdx := -1
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].kind == messageChainAction {
			lastActionIdx = i
			break
		}
	}
	// gutter is the shared left margin for all content, matching the input box's
	// side margin so messages line up with the bar below. The user block centers
	// itself to the same width, so it's exempt.
	gutter := strings.Repeat(" ", m.sideMargin())

	// lineMap maps each viewport line to the messages index that produced it.
	lineMap := make([]int, 0, len(m.messages)*3)
	addLines := func(rendered string, msgIdx int) {
		n := strings.Count(rendered, "\n") + 1
		for j := 0; j < n; j++ {
			lineMap = append(lineMap, msgIdx)
		}
	}

	out := make([]string, 0, len(m.messages)+1)
	out = append(out, "") // top padding
	lineMap = append(lineMap, -1)

	prevRenderedEmpty := true // top padding counts as empty
	for i, entry := range m.messages {
		var rendered string
		hovered := i == m.hoveredMsgIdx
		switch entry.kind {
		case messageUser:
			rendered = m.renderUserMsg(entry.text)
		case messageThink:
			rendered = indentLines(m.renderThinkEntry(entry, i == lastThinkIdx, hovered), gutter)
		case messageAssistant:
			rendered = indentLines(m.renderMarkdown(entry.text), gutter+contentIndent)
		case messageToolActivity:
			rendered = indentLines(m.renderToolActivityEntry(entry, hovered), gutter)
		case messageChainAction:
			rendered = indentLines(m.renderChainAction(entry, i == lastActionIdx), gutter)
		case messageToolDiff:
			rendered = indentLines(renderUnifiedDiff(entry.text, m.barWidth()), gutter)
		case messageChainSummary:
			rendered = indentLines(m.renderChainSummary(entry, hovered), gutter)
		default:
			rendered = indentLines(entry.text, gutter)
		}

		// vertical spacing: blank line above action group start and above assistant text
		if rendered != "" && !prevRenderedEmpty {
			switch entry.kind {
			case messageThink, messageChainSummary:
				rendered = "\n" + rendered
			case messageAssistant:
				rendered = "\n" + rendered
			}
		}

		// always keep messageText (blank lines serve as turn separators)
		if rendered != "" || entry.kind == messageText {
			out = append(out, rendered)
			addLines(rendered, i)
			prevRenderedEmpty = rendered == ""
		}
	}

	m.lineMap = lineMap
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

	// input box: top-pad + N content lines + blank spacer + status + bottom-pad
	// blank + wave: 2 lines, bottom "": 1 line
	inputHeight := m.inputLineCount() + 4
	reserved := hintHeight + confirmHeight + questionHeight + modelPickerHeight + cwHeight + inputHeight + 3
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
	gutter := strings.Repeat(" ", m.sideMargin())

	if m.confirming {
		block := renderConfirmBlock(confirmMsg{
			title:   m.confirmTitle,
			tool:    m.confirmTool,
			risk:    m.confirmRisk,
			summary: m.confirmSummary,
			details: m.confirmDetails,
			preview: m.confirmPreview,
		}, m.barWidth(), m.autoMode)
		layers = append(layers, indentLines(block, gutter))
	}

	if m.questioning && len(m.questionsData) > 0 {
		block := renderQuestionBlock(
			m.questionsData[m.questionIdx], m.questionIdx, len(m.questionsData), m.barWidth(), m.autoMode,
		)
		layers = append(layers, indentLines(block, gutter))
	}

	if m.modelPicking {
		layers = append(layers, indentLines(renderModelPicker(m.modelFiltered, m.modelPickIdx, m.modelScrollTop, m.autoMode), gutter))
	}

	if m.cwOpen {
		layers = append(layers, indentLines(m.renderConnectWizard(), gutter))
	}

	if m.hintVisible {
		layers = append(layers, m.renderHint())
	}

	layers = append(layers, "", m.renderFullWave(), m.renderInput(), "")
	return lipgloss.JoinVertical(lipgloss.Left, layers...)
}
