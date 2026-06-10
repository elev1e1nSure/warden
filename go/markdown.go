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
		glamour.WithWordWrap(m.width),
	)
	if err != nil {
		m.mdRenderer = nil
	}
}

// renderMarkdown converts markdown text to styled terminal output.
func (m *model) renderMarkdown(text string) string {
	if m.mdRenderer == nil {
		return text
	}
	// Keep the first line (warden header with ANSI styles) untouched.
	lines := strings.SplitN(text, "\n", 2)
	header := lines[0]
	body := ""
	if len(lines) > 1 {
		body = lines[1]
	}
	if body == "" {
		return header
	}
	out, err := m.mdRenderer.Render(body)
	if err != nil {
		return text
	}
	return header + "\n" + strings.TrimRight(out, "\n")
}
