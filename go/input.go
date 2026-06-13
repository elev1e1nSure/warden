package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// refreshHints recomputes slash/bang hint visibility and resizes the viewport.
func (m *model) refreshHints() {
	val := m.textinput.Value()
	slashMatches := matchSlash(val)
	bangMatches := matchBang(val, m.skills)
	newCount := len(slashMatches) + len(bangMatches)
	if newCount != m.hintCount {
		m.hintCount = newCount
		m.hintVisible = newCount > 0
		if m.height > 0 {
			m.updateViewportHeight()
			m.syncViewport()
		}
	}
	if len(slashMatches) == 0 {
		m.slashIdx = -1
		m.slashTyped = ""
	}
	if len(bangMatches) == 0 {
		m.skillsIdx = -1
		m.skillsTyped = ""
	}
	if strings.HasPrefix(val, "/") && len(slashMatches) > 0 {
		if m.slashIdx < 0 || m.slashIdx >= len(slashMatches) || m.slashTyped != val {
			m.slashIdx = 0
		}
		m.slashTyped = val
	}
	if strings.HasPrefix(val, "!") && len(bangMatches) > 0 {
		if m.skillsIdx < 0 || m.skillsIdx >= len(bangMatches) || m.skillsTyped != val {
			m.skillsIdx = 0
		}
		m.skillsTyped = val
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
	accent := WardenStyleAuto(m.autoMode)
	accentCol := Green
	if m.autoMode {
		accentCol = Blue
	}
	answerStyle := lipgloss.NewStyle().Foreground(accentCol)

	labels := make([]string, len(questions))
	maxw := 0
	for i, q := range questions {
		label := q.Header
		if label == "" {
			label = q.Question
		}
		labels[i] = label
		if w := lipgloss.Width(label); w > maxw {
			maxw = w
		}
	}

	var b strings.Builder
	for i, label := range labels {
		ans := "—"
		if i < len(answers) && len(answers[i]) > 0 {
			ans = strings.Join(answers[i], ", ")
		}
		if i > 0 {
			b.WriteString("\n")
		}
		pad := strings.Repeat(" ", maxw-lipgloss.Width(label))
		b.WriteString(accent.Render("✓") + "  " + DimStyle().Render(label+pad) + "   " + answerStyle.Render(ans))
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
	// content width = centered textarea render width, no prompt
	contentW := m.inputContentWidth()
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
