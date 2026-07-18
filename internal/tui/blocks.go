package tui

import (
	"fmt"
	"github.com/elev1e1nSure/warden/internal/client"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderConfirmBlock(inner confirmMsg, width int, autoMode bool) string {
	var b strings.Builder

	b.WriteString("  " + HeaderStyle().Render("▸") + " " + HeaderStyle().Render(toolDisplayName(inner.tool)))

	if inner.preview != "" {
		b.WriteString("\n")
		limit := width - 6
		if limit < 10 {
			limit = 10
		}
		shown := 0
		lines := strings.Split(inner.preview, "\n")
		var nonOpt []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				nonOpt = append(nonOpt, line)
			}
		}
		for i, line := range nonOpt {
			b.WriteString(DimStyle().Render("    " + truncateRunes(line, limit)))
			if i < len(nonOpt)-1 && shown < 3 {
				b.WriteString("\n")
			}
			shown++
			if shown >= 4 {
				break
			}
		}
	}

	return b.String()
}

func renderQuestionBlock(q client.QuestionItem, idx, total, width int, autoMode bool) string {
	var b strings.Builder

	accent := WardenStyleAuto(autoMode)
	header := q.Header
	if total > 1 {
		header = fmt.Sprintf("%s (%d/%d)", q.Header, idx+1, total)
	}
	b.WriteString(contentIndent + accent.Render("? ") + HeaderStyle().Render(header))
	b.WriteString("\n\n")
	b.WriteString(contentIndent + DimStyle().Render(q.Question))
	b.WriteString("\n\n")

	if len(q.Options) > 0 {
		for i, opt := range q.Options {
			numStr := fmt.Sprintf("%d", i+1)
			num := accent.Render(numStr)
			label := "  " + opt.Label
			if opt.Description != "" {
				sep := DimStyle().Render("  —  ")
				used := len(numStr) + lipgloss.Width(label) + lipgloss.Width(sep) + 2
				limit := width - used - lipgloss.Width(contentIndent)
				if limit < 10 {
					limit = 10
				}
				desc := DimStyle().Render(truncateRunes(opt.Description, limit))
				b.WriteString(contentIndent + num + label + sep + desc)
			} else {
				b.WriteString(contentIndent + num + label)
			}
			b.WriteString("\n\n")
		}
		b.WriteString(contentIndent + DimStyle().Render("press 1–"+fmt.Sprintf("%d", len(q.Options))+" to select"))
	} else {
		b.WriteString(contentIndent + DimStyle().Render("type your answer and press enter"))
	}

	return b.String()
}

// modalContentWidth is the inner text width passed to renderModalFrame,
// sized so the rendered box (border + padding) reads as roughly half the
// terminal width, capped for readability on wide screens.
func (m *model) modalContentWidth() int {
	w := m.width*3/5 - 6
	if w > 66 {
		w = 66
	}
	if max := m.width - 10; w > max {
		w = max
	}
	if w < 20 {
		w = 20
	}
	return w
}

// renderModalFrame draws a centered dialog box: title, blank line, then body.
func renderModalFrame(width int, title, body string) string {
	titleStyled := HeaderStyle().Background(inputBg).Render(title)
	content := titleStyled + "\n\n" + body
	return lipgloss.NewStyle().
		Background(inputBg).
		Padding(1, 2).
		Width(width).
		Render(content)
}

func (m *model) renderModelPickerModal() string {
	dim := DimStyle()
	accent := WardenStyleAuto(m.autoMode)

	var lines []string
	lines = append(lines, m.textinput.View())
	lines = append(lines, "")

	const maxVisible = 8
	start := m.modelScrollTop
	end := start + maxVisible
	if end > len(m.modelFiltered) {
		end = len(m.modelFiltered)
	}
	if len(m.modelFiltered) == 0 {
		lines = append(lines, dim.Render("  no matches"))
	}
	for i := start; i < end; i++ {
		name := m.modelFiltered[i]
		if i == m.modelPickIdx {
			lines = append(lines, accent.Render("→ "+name))
		} else {
			lines = append(lines, dim.Render("  "+name))
		}
	}
	lines = append(lines, "")
	hint := renderKeyHintRow([2]string{"↑↓", "navigate"}, [2]string{"enter", "select"}, [2]string{"esc", "cancel"})
	lines = append(lines, hint)

	return renderModalFrame(m.modalContentWidth(), "Select a model", strings.Join(lines, "\n"))
}

func (m *model) renderHint() string {
	val := m.textinput.Value()
	if strings.HasPrefix(val, "/") {
		matches := matchSlash(val)
		if len(matches) == 0 {
			return ""
		}
		selected := m.slashIdx
		if selected < 0 || selected >= len(matches) {
			selected = 0
		}
		const maxVisible = 5
		start := 0
		if len(matches) > maxVisible {
			start = selected - maxVisible/2
			if start < 0 {
				start = 0
			}
			if start > len(matches)-maxVisible {
				start = len(matches) - maxVisible
			}
		}
		end := start + maxVisible
		if end > len(matches) {
			end = len(matches)
		}
		lines := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			cmd := matches[i]
			active := i == selected
			name := fmt.Sprintf("/%-13s", cmd.name[1:])
			nameStyle := SlashNameStyle(active, m.autoMode)
			descStyle := SlashDescStyle(active)
			descLimit := m.width - lipgloss.Width(name) - 6
			if descLimit < 0 {
				descLimit = 0
			}
			lines = append(lines,
				"    "+nameStyle.Render(name)+"  "+descStyle.Render(truncateRunes(cmd.desc, descLimit)),
			)
		}
		return strings.Join(lines, "\n")
	}
	if strings.HasPrefix(val, "!") {
		matches := matchBang(val, m.skills)
		if len(matches) == 0 {
			return ""
		}
		selected := m.skillsIdx
		if selected < 0 || selected >= len(matches) {
			selected = 0
		}
		const maxVisible = 5
		start := 0
		if len(matches) > maxVisible {
			start = selected - maxVisible/2
			if start < 0 {
				start = 0
			}
			if start > len(matches)-maxVisible {
				start = len(matches) - maxVisible
			}
		}
		end := start + maxVisible
		if end > len(matches) {
			end = len(matches)
		}
		lines := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			s := matches[i]
			active := i == selected
			name := fmt.Sprintf("!%-13s", s.Name)
			nameStyle := SlashNameStyle(active, m.autoMode)
			descStyle := SlashDescStyle(active)
			descLimit := m.width - lipgloss.Width(name) - 6
			if descLimit < 0 {
				descLimit = 0
			}
			lines = append(lines,
				"    "+nameStyle.Render(name)+"  "+descStyle.Render(truncateRunes(s.Description, descLimit)),
			)
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

func (m *model) renderConnectWizard() string {
	acc := WardenStyleAuto(m.autoMode)
	dim := DimStyle()
	errStyle := ErrorStyle()
	cw := m.modalContentWidth()

	if m.cwErr != "" {
		body := errStyle.Render("•  "+m.cwErr) + "\n\n" + renderKeyHint("esc", "dismiss")
		return renderModalFrame(cw, "Connect a provider", body)
	}

	if m.cwLoading {
		return renderModalFrame(cw, "Connect a provider", dim.Render("connecting..."))
	}

	var lines []string

	switch m.cwStep {
	case 0:
		providers := []string{"openrouter", "ollama"}
		for i, p := range providers {
			if i == m.cwPickIdx {
				lines = append(lines, acc.Render("→ "+p))
			} else {
				lines = append(lines, dim.Render("  "+p))
			}
		}
		lines = append(lines, "")
		hint := renderKeyHintRow([2]string{"↑↓", "navigate"}, [2]string{"enter", "select"}, [2]string{"esc", "cancel"})
		lines = append(lines, hint)
		return renderModalFrame(cw, "Connect a provider", strings.Join(lines, "\n"))

	case 1:
		lines = append(lines, m.cwInput.View())
		lines = append(lines, dim.Render("get one at openrouter.ai/keys"))
		lines = append(lines, "")
		hint := renderKeyHintRow([2]string{"enter", "confirm"}, [2]string{"esc", "back"})
		lines = append(lines, hint)
		return renderModalFrame(cw, "API key", strings.Join(lines, "\n"))

	case 2:
		if m.cwCustom {
			lines = append(lines, m.cwInput.View())
			lines = append(lines, "")
			hint := renderKeyHintRow([2]string{"enter", "confirm"}, [2]string{"esc", "back"})
			lines = append(lines, hint)
			return renderModalFrame(cw, "Custom model", strings.Join(lines, "\n"))
		}

		lines = append(lines, m.cwSearch.View())
		lines = append(lines, "")
		const maxVis = 8
		start := m.cwScroll
		end := start + maxVis
		if end > len(m.cwModels) {
			end = len(m.cwModels)
		}
		if len(m.cwModels) == 0 {
			lines = append(lines, dim.Render("  no matches"))
		}
		for i := start; i < end; i++ {
			name := m.cwModels[i]
			if i == m.cwPickIdx {
				lines = append(lines, acc.Render("→ "+name))
			} else {
				lines = append(lines, dim.Render("  "+name))
			}
		}
		lines = append(lines, "")
		hint := renderKeyHintRow([2]string{"↑↓", "navigate"}, [2]string{"enter", "select"}, [2]string{"esc", "back"})
		lines = append(lines, hint)
		return renderModalFrame(cw, "Select a model", strings.Join(lines, "\n"))
	}

	return ""
}
