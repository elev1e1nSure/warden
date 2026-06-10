package tui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// ensureMarkdownRenderer (re)creates the glamour renderer when width changes.
func (m *model) ensureMarkdownRenderer() {
	if m.mdRenderer != nil && m.mdWidth == m.width {
		return
	}
	m.mdWidth = m.width
	var err error
	m.mdRenderer, err = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(m.width-2),
	)
	if err != nil {
		m.mdRenderer = nil
	}
}

// renderMarkdown converts markdown text to styled terminal output.
// Trims glamour's surrounding blank lines and strips its default 2-space left margin.
func (m *model) renderMarkdown(text string) string {
	if text == "" {
		return text
	}
	if m.mdRenderer == nil {
		return text
	}
	out, err := m.mdRenderer.Render(text)
	if err != nil {
		return text
	}
	out = strings.Trim(out, "\n")
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimLeft(line, " ")
	}
	return strings.Join(lines, "\n")
}
