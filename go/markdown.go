package tui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// wardenMarkdownStyle is a custom glamour style matching the Warden palette.
// Inline code uses spruce green. Code blocks have no border (dark bg instead).
var wardenMarkdownStyle = []byte(`{
  "document": {
    "block_suffix": "\n",
    "margin_left": 2,
    "margin_right": 2,
    "color": "252"
  },
  "block_quote": {
    "indent": 1,
    "indent_token": "│ ",
    "color": "246"
  },
  "list": {
    "level_indent": 2
  },
  "list_item": {
    "block_prefix": "• "
  },
  "heading": {
    "block_suffix": "\n",
    "color": "255",
    "bold": true
  },
  "h1": {
    "block_prefix": "\n",
    "block_suffix": "\n",
    "color": "#D4A576",
    "bold": true
  },
  "h2": {
    "block_prefix": "\n",
    "block_suffix": "\n",
    "color": "#D4A576",
    "bold": true
  },
  "h3": {
    "block_prefix": "\n",
    "color": "#aaaaaa",
    "bold": true
  },
  "h4": {
    "block_prefix": "\n",
    "color": "#888888"
  },
  "h5": {
    "block_prefix": "\n",
    "color": "#666666"
  },
  "h6": {
    "block_prefix": "\n",
    "color": "#555555"
  },
  "strong": {
    "bold": true
  },
  "emph": {
    "italic": true,
    "color": "246"
  },
  "hr": {
    "format": "\n──────────────────────────────────────────\n",
    "color": "#444444"
  },
  "item": {
    "block_prefix": "• "
  },
  "enumeration": {
    "block_prefix": ". "
  },
  "task": {
    "ticked": "✓ ",
    "unticked": "✗ "
  },
  "link": {
    "underline": true,
    "color": "#52B788"
  },
  "link_text": {
    "bold": true,
    "color": "#52B788"
  },
  "image": {
    "format": "image: {{.text}}"
  },
  "code": {
    "color": "#52B788"
  },
  "code_block": {
    "color": "#666666",
    "indent": 1,
    "indent_token": "│ ",
    "margin_left": 0,
    "margin_right": 0,
    "padding_left": 0,
    "padding_right": 1,
    "padding_top": 0,
    "padding_bottom": 0,
    "margin_top": 1,
    "margin_bottom": 1
  },
  "table": {
    "center_separator": "┼",
    "column_separator": "│",
    "row_separator": "─"
  },
  "definition_description": {
    "block_prefix": "\n→ "
  }
}`)

// ensureMarkdownRenderer (re)creates the glamour renderer when width changes.
func (m *model) ensureMarkdownRenderer() {
	if m.mdRenderer != nil && m.mdWidth == m.width {
		return
	}
	m.mdWidth = m.width
	var err error
	m.mdRenderer, err = glamour.NewTermRenderer(
		glamour.WithStylesFromJSONBytes(wardenMarkdownStyle),
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
		// Only trim glamour's default 2-space margin, preserve code indentation
		if len(line) >= 2 && line[:2] == "  " {
			lines[i] = line[2:]
		} else {
			lines[i] = line
		}
	}
	return strings.Join(lines, "\n")
}
