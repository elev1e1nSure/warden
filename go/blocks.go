package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderConfirmBlock(inner confirmMsg, width int, autoMode bool) string {
	var b strings.Builder

	accent := WardenStyleAuto(autoMode)
	b.WriteString("  " + accent.Render("▸") + "  " + ToolStyle().Bold(true).Render(toolDisplayName(inner.tool)))
	b.WriteString("\n")

	if inner.preview != "" {
		limit := width - 6
		if limit < 10 {
			limit = 10
		}
		shown := 0
		for _, line := range strings.Split(inner.preview, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			b.WriteString(DimStyle().Render("    " + truncateRunes(line, limit)))
			b.WriteString("\n")
			shown++
			if shown >= 4 {
				break
			}
		}
	}

	details := inner.details
	if len(details) == 0 && inner.summary != "" {
		details = []string{inner.summary}
	}
	if len(details) > 0 {
		b.WriteString("\n")
		for _, d := range details {
			b.WriteString(DimStyle().Render("  " + d))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(ConfirmYStyle().Render("  Y  run  ") + ConfirmNStyle().Render("  N  cancel  "))

	return b.String()
}

func renderQuestionBlock(q QuestionItem, idx, total, width int, autoMode bool) string {
	var b strings.Builder

	accent := WardenStyleAuto(autoMode)
	header := q.Header
	if total > 1 {
		header = fmt.Sprintf("%s (%d/%d)", q.Header, idx+1, total)
	}
	b.WriteString(accent.Render("? ") + HeaderStyle().Render(header))
	b.WriteString("\n")
	b.WriteString("  " + q.Question)
	b.WriteString("\n")

	if len(q.Options) > 0 {
		b.WriteString("\n")
		for i, opt := range q.Options {
			num := accent.Render(fmt.Sprintf("  %d", i+1))
			label := "  " + opt.Label
			if opt.Description != "" {
				sep := DimStyle().Render("  —  ")
				desc := DimStyle().Render(truncateRunes(opt.Description, width-lipgloss.Width(num)-lipgloss.Width(label)-lipgloss.Width(sep)))
				b.WriteString(num + label + sep + desc)
			} else {
				b.WriteString(num + label)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(DimStyle().Render("  press 1–" + fmt.Sprintf("%d", len(q.Options)) + " to select"))
	} else {
		b.WriteString("\n")
		b.WriteString(DimStyle().Render("  type your answer and press enter"))
	}

	return b.String()
}

func renderModelPicker(filtered []string, idx, scrollTop int, autoMode bool) string {
	const maxVisible = 8
	start := scrollTop
	end := start + maxVisible
	if end > len(filtered) {
		end = len(filtered)
	}
	lines := make([]string, 0, maxVisible+4)

	accent := WardenStyleAuto(autoMode)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(Green)
	if autoMode {
		keyStyle = lipgloss.NewStyle().Bold(true).Foreground(Blue)
	}
	key := func(s string) string { return keyStyle.Render(s) }
	hint := key("←→") + DimStyle().Render(" navigate   ") +
		key("Enter") + DimStyle().Render(" select   ") +
		key("Esc") + DimStyle().Render(" cancel")
	lines = append(lines, "  "+hint)
	lines = append(lines, "")

	for i := start; i < end; i++ {
		name := filtered[i]
		if i == idx {
			lines = append(lines, accent.Render("  → "+name))
		} else {
			lines = append(lines, DimStyle().Render("    "+name))
		}
	}

	return strings.Join(lines, "\n")
}

func (m model) renderHint() string {
	val := m.textinput.Value()
	accent := WardenStyleAuto(m.autoMode)
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
			if active {
				descLimit = m.width - lipgloss.Width(name) - 4
			}
			if descLimit < 0 {
				descLimit = 0
			}
			if active {
				lines = append(lines,
					"  "+accent.Render(">")+" "+nameStyle.Render(name)+"  "+descStyle.Render(truncateRunes(cmd.desc, descLimit)),
				)
				continue
			}
			lines = append(lines,
				"    "+nameStyle.Render(name)+"  "+descStyle.Render(truncateRunes(cmd.desc, descLimit)),
			)
		}
		return strings.Join(lines, "\n")
	}
	if strings.HasPrefix(val, "!") {
		skills := matchBang(val, m.skills)
		lines := make([]string, 0, len(skills))
		for _, s := range skills {
			name := fmt.Sprintf("!%-13s", s.Name)
			lines = append(lines,
				accent.Render(name)+"  "+DimStyle().Render(s.Description),
			)
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

func (m model) renderConnectWizard() string {
	acc := WardenStyleAuto(m.autoMode)
	dim := DimStyle()
	errStyle := ErrorStyle()
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(Green)
	if m.autoMode {
		keyStyle = lipgloss.NewStyle().Bold(true).Foreground(Blue)
	}
	key := func(s string) string { return keyStyle.Render(s) }

	var lines []string

	if m.cwErr != "" {
		lines = append(lines, "  "+errStyle.Render("•  "+m.cwErr))
		lines = append(lines, "  "+dim.Render(key("esc")+" dismiss"))
		return strings.Join(lines, "\n")
	}

	if m.cwLoading {
		lines = append(lines, "  "+dim.Render("connecting..."))
		return strings.Join(lines, "\n")
	}

	switch m.cwStep {
	case 0:
		lines = append(lines, "  "+acc.Render("connect"), "")
		providers := []string{"openrouter", "ollama"}
		for i, p := range providers {
			if i == m.cwPickIdx {
				lines = append(lines, "  "+acc.Render("→ "+p))
			} else {
				lines = append(lines, "  "+dim.Render("  "+p))
			}
		}
		lines = append(lines, "")
		hint := key("←→") + dim.Render(" navigate   ") + key("Enter") + dim.Render(" select   ") + key("Esc") + dim.Render(" cancel")
		lines = append(lines, "  "+hint)

	case 1:
		lines = append(lines, "  "+acc.Render("api key"), "")
		lines = append(lines, "  "+m.cwInput.View())
		lines = append(lines, "  "+dim.Render("get one at openrouter.ai/keys"))
		lines = append(lines, "")
		hint := key("Enter") + dim.Render(" confirm   ") + key("Esc") + dim.Render(" back")
		lines = append(lines, "  "+hint)

	case 2:
		lines = append(lines, "  "+acc.Render("model"), "")
		if m.cwCustom {
			lines = append(lines, "  "+m.cwInput.View())
			lines = append(lines, "")
			hint := key("Enter") + dim.Render(" confirm   ") + key("Esc") + dim.Render(" back")
			lines = append(lines, "  "+hint)
		} else {
			const maxVis = 7
			start := m.cwScroll
			end := start + maxVis
			if end > len(m.cwModels) {
				end = len(m.cwModels)
			}
			for i := start; i < end; i++ {
				name := m.cwModels[i]
				if i == m.cwPickIdx {
					lines = append(lines, "  "+acc.Render("→ "+name))
				} else {
					lines = append(lines, "  "+dim.Render("  "+name))
				}
			}
			lines = append(lines, "")
			hint := key("←→") + dim.Render(" navigate   ") + key("Enter") + dim.Render(" select   ") + key("Esc") + dim.Render(" back")
			lines = append(lines, "  "+hint)
		}
	}

	return strings.Join(lines, "\n")
}
