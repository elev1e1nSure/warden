package tui

import (
	"fmt"
	"strings"
)

var toolDisplayNames = map[string]string{
	"google_search":      "Web Search",
	"youtube_search":     "Web Search",
	"grep":               "Search",
	"glob":               "Find",
	"browser_read":       "Read",
	"file_read":          "Read",
	"webfetch":           "Fetch",
	"browser_open":       "Open",
	"browser_screenshot": "Screenshot",
	"browser_click":      "Click",
	"browser_fill":       "Fill",
	"http_request":       "HTTP",
	"screenshot":         "Screenshot",
	"window_list":        "Windows",
	"window_focus":       "Focus",
	"window_manage":      "Window",
	"image_locate":       "Locate",
	"ocr":                "OCR",
	"wait_for":           "Wait",
	"system_info":        "System",
	"notify":             "Notify",
	"memory":             "Memory",
	"file_write":         "Write",
	"file_delete":        "Delete",
	"file_list":          "List",
	"edit":               "Edit",
	"apply_patch":        "Patch",
	"powershell":         "Shell",
	"bash":               "Shell",
	"mouse":              "Mouse",
	"keyboard":           "Type",
	"clipboard":          "Clipboard",
	"question":           "Ask",
	"skill":              "Skill",
	"todowrite":          "Todo",
}

func toolDisplayName(name string) string {
	if d, ok := toolDisplayNames[name]; ok {
		return d
	}
	if len(name) > 0 {
		return strings.ToUpper(name[:1]) + name[1:]
	}
	return name
}

func truncateRunes(text string, limit int) string {
	if limit < 1 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-1]) + "…"
}

func toolResultIsError(result string) bool {
	lower := strings.ToLower(strings.TrimSpace(result))
	// Check for "error:" or "error " with word boundary to avoid false positives like "error123"
	return strings.HasPrefix(lower, "error:") ||
		strings.HasPrefix(lower, "error ") ||
		strings.HasPrefix(lower, "stderr")
}

func toolSummaryLine(name, args, result string) string {
	result = strings.TrimSpace(result)
	if result == "" {
		result = "(empty)"
	}
	isErr := toolResultIsError(result)
	arrow := ToolStyle().Render(contentIndent + "→ ")
	display := toolDisplayName(name)

	// Shell tools: show the command, append result only when it has content.
	if (name == "powershell" || name == "bash") && args != "" {
		cmd := truncateRunes(strings.TrimSpace(args), 80)
		var nameRender string
		if isErr {
			nameRender = ErrorStyle().Render(display)
		} else {
			nameRender = ToolStyle().Render(display)
		}
		line := arrow + nameRender + "  " + DimStyle().Render(cmd)
		if result != "(no output)" && result != "(empty)" {
			rlines := strings.Split(result, "\n")
			head := strings.TrimSpace(rlines[0])
			if len(rlines) > 1 {
				head += fmt.Sprintf("  +%d", len(rlines)-1)
			}
			head = truncateRunes(head, 60)
			if isErr {
				line += "  " + ErrorStyle().Render(head)
			} else {
				line += "  " + DimStyle().Render(head)
			}
		}
		return line
	}

	lines := strings.Split(result, "\n")
	head := strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		head += fmt.Sprintf("  +%d lines", len(lines)-1)
	}
	head = truncateRunes(head, 100)

	if isErr {
		return arrow + ErrorStyle().Render(display) + "  " + ErrorStyle().Render(head)
	}

	text, diff := renderDiffStats(head)
	nameRender := ToolStyle().Render(display)
	if diff != "" {
		return arrow + nameRender + "  " + DimStyle().Render(text) + "  " + diff
	}
	return arrow + nameRender + "  " + DimStyle().Render(head)
}

var toolActivityVerbs = map[string]string{
	"google_search":      "searching",
	"youtube_search":     "searching",
	"grep":               "searching",
	"glob":               "searching files",
	"file_read":          "reading",
	"browser_read":       "reading",
	"webfetch":           "fetching",
	"browser_open":       "opening",
	"browser_screenshot": "screenshotting",
	"screenshot":         "screenshotting",
	"file_write":         "writing",
	"edit":               "editing",
	"apply_patch":        "patching",
	"powershell":         "running",
	"bash":               "running",
	"mouse":              "clicking",
	"keyboard":           "typing",
	"clipboard":          "clipboard",
	"file_delete":        "deleting",
	"file_list":          "listing",
	"browser_click":      "clicking",
	"browser_fill":       "filling",
	"http_request":       "requesting",
	"window_list":        "listing windows",
	"window_focus":       "focusing",
	"window_manage":      "managing window",
	"image_locate":       "locating",
	"ocr":                "reading text",
	"wait_for":           "waiting",
	"system_info":        "reading system",
	"notify":             "notifying",
	"memory":             "remembering",
}

func toolActivityLine(name string) string {
	verb, ok := toolActivityVerbs[name]
	if !ok {
		verb = "working"
	}
	return DimStyle().Render(contentIndent + verb + "...")
}

func toolStartLine(name, args string) string {
	arrow := ToolStyle().Render(contentIndent + "→ ")
	display := ToolStyle().Render(toolDisplayName(name))
	if args == "" {
		return arrow + display
	}
	return arrow + display + "  " + DimStyle().Render(truncateRunes(args, 140))
}

func toolPastTense(name string) string {
	switch name {
	case "Web Search", "Search", "Grep":
		return "Searched"
	case "Read":
		return "Read"
	case "Write":
		return "Wrote"
	case "Glob", "Find":
		return "Found"
	case "Edit":
		return "Edited"
	case "Patch":
		return "Patched"
	case "Browser":
		return "Browsed"
	case "Fetch":
		return "Fetched"
	case "Screenshot":
		return "Screenshot"
	case "Keyboard", "Type":
		return "Typed"
	case "Todo", "List":
		return "Listed"
	case "Shell":
		return "Ran"
	case "Skill":
		return "Used"
	case "Delete":
		return "Deleted"
	case "Mouse":
		return "Clicked"
	case "Clipboard":
		return "Copied"
	case "Ask":
		return "Asked"
	case "Click", "Fill":
		return "Clicked"
	case "HTTP":
		return "Requested"
	case "Windows", "Focus", "Window":
		return "Managed window"
	case "Locate":
		return "Located"
	case "OCR":
		return "Read text"
	case "Wait":
		return "Waited"
	case "System":
		return "Read system"
	case "Notify":
		return "Notified"
	case "Memory":
		return "Remembered"
	}
	return "Ran " + strings.ToLower(name)
}

// toolPresentTenseNames maps display names to present-tense verbs for the live action line.
var toolPresentTenseNames = map[string]string{
	"Web Search": "Searching",
	"Search":     "Searching",
	"Find":       "Finding",
	"Read":       "Reading",
	"Fetch":      "Fetching",
	"Open":       "Opening",
	"Screenshot": "Capturing",
	"Write":      "Writing",
	"Delete":     "Deleting",
	"List":       "Listing",
	"Edit":       "Editing",
	"Patch":      "Patching",
	"Shell":      "Running",
	"Mouse":      "Clicking",
	"Type":       "Typing",
	"Clipboard":  "Clipboard",
	"Ask":        "Asking",
	"Skill":      "Loading",
	"Todo":       "Updating todo",
}

func toolPresentTense(display string) string {
	if v, ok := toolPresentTenseNames[display]; ok {
		return v
	}
	return "Running " + strings.ToLower(display)
}

// actionDetail extracts the tool detail for the live action line, stripping any
// trailing ellipsis/dots so URLs render clean.
func actionDetail(display, args string) string {
	return strings.TrimRight(extractToolDetail(display, args), "… ")
}

func extractToolDetail(name, args string) string {
	if args == "" {
		return ""
	}
	// Fetch: extract only the URL
	if name == "Fetch" {
		for _, part := range strings.Split(args, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "url=") {
				v := strings.TrimSpace(part[4:])
				v = strings.Trim(v, `"'`)
				return truncateRunes(v, 60)
			}
		}
		return ""
	}
	// Edit/Patch: show only filename, not old_string/new_string
	if name == "Edit" || name == "Patch" {
		for _, part := range strings.Split(args, ", ") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "file_path=") {
				v := strings.TrimSpace(part[10:])
				v = strings.Trim(v, `"'`)
				return truncateRunes(pathBase(v), 50)
			}
		}
	}
	// default: take the first key=value, strip the key
	parts := strings.SplitN(args, "=", 2)
	if len(parts) == 2 {
		v := strings.TrimSpace(parts[1])
		if comma := strings.Index(v, ", "); comma >= 0 {
			v = v[:comma]
		}
		v = strings.Trim(v, `"'`)
		if v != "" {
			return truncateRunes(v, 60)
		}
	}
	return truncateRunes(args, 60)
}

// pathBase returns the last component of a file path (handles both / and \).
func pathBase(p string) string {
	p = strings.TrimRight(p, "/\\")
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func (m model) renderToolFlowEntry(idx int, entry messageEntry) string {
	prefix := contentIndent
	detail := extractToolDetail(entry.toolName, entry.toolArgs)
	if entry.toolDone {
		past := toolPastTense(entry.toolName)
		if detail != "" {
			past += " " + detail
		}
		return DimStyle().Render(prefix + past)
	}
	if detail != "" {
		detail = " -> " + detail
	}
	// Only the currently running tool gets the live breathing orb and shimmer
	if idx == m.runningToolIdx {
		return prefix + m.pulse() + m.shimmer(entry.toolName+detail)
	}
	return DimStyle().Render(prefix + entry.toolName + detail)
}

// renderToolActivityEntry renders a tool line.
// While pending (toolDone=false): animated pulse+shimmer.
// When done: static summary with optional +/- expand toggle.
func (m model) renderToolActivityEntry(entry messageEntry, hovered bool) string {
	if !entry.toolDone {
		// pending: animate only while loading
		line := entry.toolName
		if entry.toolArgs != "" {
			line += "  " + entry.toolArgs
		}
		if m.loading {
			return contentIndent + m.pulse() + m.shimmer(line)
		}
		return DimStyle().Render(contentIndent + "~ " + line)
	}

	// completed: no expandable content — just return the summary line
	if entry.toolResult == "" {
		return entry.text
	}

	// Replace leading "→" with "+/-" indicator when result is expandable.
	arrow := ToolStyle().Render(contentIndent + "→ ")
	toggle := "+"
	if entry.expanded {
		toggle = "-"
	}
	indicatorStyle := ToolStyle()
	if hovered {
		indicatorStyle = HoverStyle()
	}
	indicator := indicatorStyle.Render(contentIndent + toggle + " ")
	summaryLine := strings.Replace(entry.text, arrow, indicator, 1)

	if !entry.expanded {
		return summaryLine
	}
	result := strings.TrimSpace(entry.toolResult)
	resultLines := strings.Split(result, "\n")
	maxWidth := m.barWidth() - len(bodyIndent)
	if maxWidth < 10 {
		maxWidth = 10
	}
	out := make([]string, 0, len(resultLines)+1)
	out = append(out, summaryLine)
	for _, l := range resultLines {
		out = append(out, DimStyle().Render(bodyIndent+truncateRunes(l, maxWidth)))
	}
	return strings.Join(out, "\n")
}

// renderChainAction renders the single live "what's happening now" line.
func (m model) renderChainAction(entry messageEntry, active bool) string {
	if !m.loading {
		return ""
	}
	line := entry.activity
	if entry.toolArgs != "" {
		line += " " + entry.toolArgs
	}
	if !active {
		return DimStyle().Render(contentIndent + line)
	}
	return contentIndent + m.pulse() + m.shimmer(line)
}
