package tui

import tea "github.com/charmbracelet/bubbletea"

func (m model) handleStreamEvent(msg nextMsg) (model, []tea.Cmd) {
	var cmds []tea.Cmd

	if m.interruptStream {
		if _, ok := msg.inner.(doneMsg); ok {
			m.interruptStream = false
		} else {
			cmds = append(cmds, readNext(msg.ch))
		}
		return m, cmds
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
		if m.diffMode && inner.tool.Diff != "" {
			entry := messageEntry{kind: messageToolDiff, text: inner.tool.Diff}
			// Keep the live chain action line (if any) at the tail so in-place
			// clearAction/setAction keep working — slot the diff above it.
			if n := len(m.messages); n > 0 && m.messages[n-1].kind == messageChainAction {
				m.messages = append(m.messages, messageEntry{})
				copy(m.messages[n:], m.messages[n-1:])
				m.messages[n-1] = entry
			} else {
				m.messages = append(m.messages, entry)
			}
		}
		m.runningToolIdx = -1
		m.syncViewport()
		cmds = append(cmds, readNext(msg.ch))

	case confirmMsg:
		// pause the live indicators while we wait on the user
		m.loading = false
		if m.verboseMode {
			m.finishThink()
		} else {
			m.clearAction()
		}
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
		// pause the live indicators while we wait on the user
		m.loading = false
		if m.verboseMode {
			m.finishThink()
		} else {
			m.clearAction()
		}
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

	return m, cmds
}
