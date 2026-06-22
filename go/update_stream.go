package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleNextMsg(msg nextMsg) (model, tea.Cmd) {
	if m.interruptStream {
		if _, ok := msg.inner.(doneMsg); ok {
			m.interruptStream = false
		} else {
			return m, readNext(msg.ch)
		}
		return m, nil
	}

	switch inner := msg.inner.(type) {
	case wardenStartMsg:
		m.thinkBuf = ""
		m.thinkDone = false
		m.toolRunning = false
		m.lastAssistantRaw = ""
		m.loading = true
		m.resetOrAppendThink()
		m.setAction("Thinking", "")
		m.syncViewport()
		return m, readNext(msg.ch)

	case thinkMsg:
		m.thinkBuf += inner.text
		m.updateThink(inner.text)
		m.setAction("Thinking", "")
		m.syncViewport()
		return m, readNext(msg.ch)

	case tokenMsg:
		if !m.thinkDone {
			m.finishThink()
			m.clearAction()
			m.appendAssistant("")
			m.thinkDone = true
		}
		m.appendToLastAssistant(inner.text)
		m.lastAssistantRaw += inner.text
		return m, readNext(msg.ch)

	case toolStartMsg:
		m.toolRunning = true
		m.finishThink()
		display := toolDisplayName(inner.name)
		m.clearAction()
		m.setAction(toolPresentTense(display), actionDetail(display, inner.args))
		m.syncViewport()
		return m, readNext(msg.ch)

	case toolMsg:
		m.toolRunning = false
		if inner.tool.Diff != "" {
			// show a compact tool line only when there's a diff to expand
			summary := toolSummaryLine(inner.tool.Name, inner.tool.Args, inner.tool.Result)
			m.clearAction()
			m.messages = append(m.messages, messageEntry{
				kind:       messageToolActivity,
				text:       summary,
				toolResult: inner.tool.Result,
				toolDiff:   inner.tool.Diff,
				toolDone:   true,
			})
		} else {
			m.clearAction()
		}
		m.syncViewport()
		return m, readNext(msg.ch)

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
		return m, nil

	case questionMsg:
		m.questioning = true
		m.loading = false
		m.finishThink()
		m.clearAction()
		m.questionID = inner.id
		m.questionCh = msg.ch
		m.questionsData = inner.questions
		m.questionIdx = 0
		m.questionAnswers = nil
		m.textinput.Placeholder = ""
		m.resetInput()
		m.updateViewportHeight()
		m.syncViewport()
		return m, nil

	case doneMsg:
		if cmd := m.finishStream(inner.tokenCount, inner.tokenLimit); cmd != nil {
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

func (m model) handleStartStreamMsg(msg startStreamMsg) (model, tea.Cmd) {
	return m, readNext(msg.ch)
}

func (m model) handleDoneMsg(msg doneMsg) (model, tea.Cmd) {
	m.interruptStream = false
	if m.streaming {
		if cmd := m.finishStream(msg.tokenCount, msg.tokenLimit); cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

func (m model) handleShellResult(msg shellResultMsg) (model, tea.Cmd) {
	m.streaming = false
	m.loading = false
	m.toolRunning = false
	m.finishThink()
	m.thinkBuf = ""
	m.thinkDone = false
	m.appendText(DimStyle().Render(strings.TrimRight(msg.output, "\n")))
	m.appendText("")
	m.syncViewport()
	return m, nil
}

func (m model) handleTick(msg tickMsg) (model, tea.Cmd) {
	if m.loading {
		m.spinner++
		if m.streaming && !m.confirming && len(m.messages) > 0 {
			m.syncViewport()
		}
		if m.quitPending && time.Since(m.quitPendingSince) > 3*time.Second {
			m.quitPending = false
		}
		return m, m.tick()
	}
	return m, nil
}
