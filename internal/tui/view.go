package tui

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/cellbuf"
)

// overlayDimFg is the forced foreground for the dimmed backdrop. Faint alone
// is too subtle — many terminals render it as a barely-noticeable dim, so we
// also flatten the foreground color to a fixed dark gray for a stronger,
// more consistent darkening across terminals.
var overlayDimFg = color.RGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xff}

var wardenVersion = "dev"

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
		default:
			rendered = indentLines(entry.text, gutter)
		}

		// blank line above action blocks and above assistant text when something preceded them
		if rendered != "" && !prevRenderedEmpty {
			switch entry.kind {
			case messageThink, messageChainAction, messageToolActivity:
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
		hintHeight = lipgloss.Height(m.renderHint()) + 1
	}

	confirmHeight := 0
	if m.confirming {
		confirmHeight = lipgloss.Height(renderConfirmBlock(confirmMsg{
			title:   "Dangerous action",
			tool:    m.confirmTool,
			details: []string{},
		}, m.width, m.autoMode)) + 1
	}

	questionHeight := 0
	if m.questioning && len(m.questionsData) > 0 {
		questionHeight = lipgloss.Height(renderQuestionBlock(
			m.questionsData[m.questionIdx], m.questionIdx, len(m.questionsData), m.width, m.autoMode,
		)) + 1
	}

	// input box: top-pad + N content lines + blank spacer + status + bottom-pad
	// blank + wave: 2 lines, bottom "": 1 line
	inputHeight := m.inputLineCount() + 4
	reserved := hintHeight + confirmHeight + questionHeight + inputHeight + 3
	height := m.height - reserved
	if height < 1 {
		height = 1
	}
	return height
}

func (m *model) updateViewportHeight() {
	m.viewport.Height = m.layoutViewportHeight()
}

// renderOverlay dims the normal layout and draws a dialog over it. Terminal
// cells have no alpha channel, so the faint attribute is the closest portable
// equivalent to the GUI's translucent black backdrop.
func (m *model) renderOverlay(base, box string) string {
	canvas := cellbuf.NewBuffer(m.width, m.height)
	cellbuf.SetContent(canvas, base)
	for y := range m.height {
		for x := range m.width {
			cell := canvas.Cell(x, y)
			if cell == nil || cell.Width == 0 {
				continue
			}
			dimmed := cell.Clone()
			dimmed.Style.Faint(true)
			dimmed.Style.Foreground(overlayDimFg)
			dimmed.Style.Background(color.Black)
			canvas.SetCell(x, y, dimmed)
		}
	}

	boxWidth := lipgloss.Width(box)
	boxHeight := lipgloss.Height(box)
	modal := cellbuf.NewBuffer(boxWidth, boxHeight)
	cellbuf.SetContent(modal, box)

	left := max(0, (m.width-boxWidth)/2)
	top := max(0, (m.height-boxHeight)/2)
	for y := range boxHeight {
		for x := range boxWidth {
			if cell := modal.Cell(x, y); cell != nil {
				canvas.SetCell(left+x, top+y, cell)
			}
		}
	}

	return cellbuf.Render(canvas)
}

func (m *model) View() string {
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
		layers = append(layers, "", indentLines(block, gutter))
	}

	if m.questioning && len(m.questionsData) > 0 {
		block := renderQuestionBlock(
			m.questionsData[m.questionIdx], m.questionIdx, len(m.questionsData), m.barWidth(), m.autoMode,
		)
		layers = append(layers, "", indentLines(block, gutter))
	}

	if m.hintVisible {
		layers = append(layers, "", indentLines(m.renderHint(), gutter))
	}

	layers = append(layers, "", m.renderFullWave(), m.renderInput(), "")
	base := lipgloss.JoinVertical(lipgloss.Left, layers...)

	if m.cwOpen {
		return m.renderOverlay(base, m.renderConnectWizard())
	}
	if m.modelPicking {
		return m.renderOverlay(base, m.renderModelPickerModal())
	}
	return base
}
