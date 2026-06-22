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
		m.activityIdx = m.resetOrAppendThink() // think entry created, invisible until finalized in non-verbose
		if m.verboseMode {
			// verbose: think entry animates itself
		} else {
			m.setAction("Thinking", "") // live chain action indicator
		}
		m.syncViewport()
		return m, readNext(msg.ch)

	case thinkMsg:
		m.thinkBuf += inner.text
		m.updateThink(inner.text)
		if !m.verboseMode {
			m.setAction("Thinking", "")
		}
		m.syncViewport()
		return m, readNext(msg.ch)

	case tokenMsg:
		if !m.thinkDone {
			m.finishThink()
			if !m.verboseMode {
				m.clearAction()
			}
			m.appendAssistant("")
			m.thinkDone = true
		}
		m.appendToLastAssistant(inner.text)
		m.lastAssistantRaw += inner.text
		return m, readNext(msg.ch)

	case toolStartMsg:
		m.toolRunning = true
		if m.verboseMode {
			m.finishThink()
			m.startToolActivity(inner.name, inner.args)
			m.runningToolIdx = len(m.messages) - 1
		} else {
			display := toolDisplayName(inner.name)
			m.clearAction()
			m.setAction(toolPresentTense(display), actionDetail(display, inner.args))
		}
		m.syncViewport()
		return m, readNext(msg.ch)

	case toolMsg:
		m.toolRunning = false
		summary := toolSummaryLine(inner.tool.Name, inner.tool.Args, inner.tool.Result)
		if m.verboseMode {
			if m.runningToolIdx >= 0 && m.runningToolIdx < len(m.messages) {
				m.messages[m.runningToolIdx].text = summary
				m.messages[m.runningToolIdx].toolResult = inner.tool.Result
				m.messages[m.runningToolIdx].toolDiff = inner.tool.Diff
				m.messages[m.runningToolIdx].toolDone = true
			} else {
				m.messages = append(m.messages, messageEntry{
					kind:       messageToolActivity,
					text:       summary,
					toolResult: inner.tool.Result,
					toolDiff:   inner.tool.Diff,
					toolDone:   true,
				})
			}
		} else if inner.tool.Diff != "" {
			// Non-verbose: still show a compact tool line when there's a diff to expand.
			m.clearAction()
			m.messages = append(m.messages, messageEntry{
				kind:       messageToolActivity,
				text:       summary,
				toolResult: inner.tool.Result,
				toolDiff:   inner.tool.Diff,
				toolDone:   true,
			})
		}
		m.runningToolIdx = -1
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
		if !m.verboseMode {
			m.clearAction()
		}
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
