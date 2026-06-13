package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type slashCmd struct {
	name string
	desc string
}

var slashCommands = []slashCmd{
	{"/connect", "Set up provider and model"},
	{"/clear", "Clear chat and reset session"},
	{"/compact", "Summarize conversation to free up context"},
	{"/memory", "Toggle or show memory settings"},
	{"/models", "Switch model"},
	{"/update", "Download and install the latest release"},
	{"/select", "Toggle text selection mode"},
	{"/verbose", "Toggle verbose mode (show tool lines and errors)"},
}

func matchSlash(prefix string) []slashCmd {
	if len(prefix) == 0 || prefix[0] != '/' {
		return nil
	}
	var out []slashCmd
	lower := strings.ToLower(prefix)
	for _, cmd := range slashCommands {
		if strings.HasPrefix(cmd.name, lower) {
			out = append(out, cmd)
		}
	}
	return out
}

func (m *model) handleSlashNavigation(msg tea.KeyMsg) bool {
	val := m.textinput.Value()
	if !strings.HasPrefix(val, "/") {
		return false
	}
	matches := matchSlash(val)
	if len(matches) == 0 {
		return false
	}
	if m.slashIdx < 0 || m.slashIdx >= len(matches) {
		m.slashIdx = 0
	}
	switch msg.Type {
	case tea.KeyUp:
		if m.slashIdx == 0 {
			m.slashIdx = len(matches) - 1
		} else {
			m.slashIdx--
		}
	case tea.KeyDown:
		m.slashIdx = (m.slashIdx + 1) % len(matches)
	default:
		return false
	}
	m.updateViewportHeight()
	m.syncViewport()
	return true
}

func (m *model) handleBangNavigation(msg tea.KeyMsg) bool {
	val := m.textinput.Value()
	if !strings.HasPrefix(val, "!") {
		return false
	}
	matches := matchBang(val, m.skills)
	if len(matches) == 0 {
		return false
	}
	if m.skillsIdx < 0 || m.skillsIdx >= len(matches) {
		m.skillsIdx = 0
	}
	switch msg.Type {
	case tea.KeyUp:
		if m.skillsIdx == 0 {
			m.skillsIdx = len(matches) - 1
		} else {
			m.skillsIdx--
		}
	case tea.KeyDown:
		m.skillsIdx = (m.skillsIdx + 1) % len(matches)
	default:
		return false
	}
	m.updateViewportHeight()
	m.syncViewport()
	return true
}

func bangCommonPrefix(matches []Skill) string {
	if len(matches) == 0 {
		return ""
	}
	prefix := "!" + matches[0].Name
	for _, s := range matches[1:] {
		full := "!" + s.Name
		for !strings.HasPrefix(full, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

func slashCommonPrefix(matches []slashCmd) string {
	if len(matches) == 0 {
		return ""
	}
	prefix := matches[0].name
	for _, m := range matches[1:] {
		for !strings.HasPrefix(m.name, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

func (m *model) clearHintState() {
	m.hintCount = 0
	m.hintVisible = false
	if m.height > 0 {
		m.updateViewportHeight()
	}
}

// handleSlash processes /commands before sending.
func (m *model) handleSlash(text string) (bool, tea.Cmd) {
	trimmed := strings.ToLower(strings.TrimSpace(text))
	if !strings.HasPrefix(trimmed, "/") {
		return false, nil
	}
	switch trimmed {
	case "/connect":
		m.clearHintState()
		m.cwOpen = true
		m.cwStep = 0
		m.cwPickIdx = 0
		m.cwProvider = ""
		m.cwErr = ""
		m.cwCustom = false
		m.cwLoading = false
		m.cwAPIKey = ""
		m.cwModels = nil
		m.updateViewportHeight()
		m.syncViewport()
		return true, nil
	case "/clear":
		m.clearHintState()
		m.messages = []messageEntry{}
		m.lastAssistantRaw = ""
		m.syncViewport()
		return true, func() tea.Msg {
			m.client.ResetSession()
			return nil
		}
	case "/compact":
		m.clearHintState()
		m.loading = true
		m.spinner = 0
		m.syncViewport()
		return true, tea.Batch(m.runCompact(), m.tick())
	case "/memory":
		m.clearHintState()
		m.loading = true
		m.spinner = 0
		m.syncViewport()
		return true, tea.Batch(m.runMemoryStatus(), m.tick())
	case "/models":
		m.clearHintState()
		return true, m.fetchModels()
	case "/update":
		m.clearHintState()
		m.loading = true
		m.spinner = 0
		m.appendText(DimStyle().Render("  downloading update..."))
		m.syncViewport()
		return true, tea.Batch(m.runUpdate(), m.tick())
	}

	switch trimmed {
	case "/select":
		m.clearHintState()
		m.selectMode = !m.selectMode
		m.syncViewport()
		if m.selectMode {
			return true, tea.DisableMouse
		}
		return true, tea.EnableMouseCellMotion
	case "/verbose":
		m.verboseMode = !m.verboseMode
		m.clearHintState()
		m.syncViewport()
		return true, nil
	}

	// /memory subcommands
	if strings.HasPrefix(trimmed, "/memory ") {
		sub := strings.TrimSpace(strings.TrimPrefix(trimmed, "/memory"))
		m.clearHintState()
		m.loading = true
		m.spinner = 0
		m.syncViewport()
		switch sub {
		case "on":
			return true, tea.Batch(m.runMemoryOn(), m.tick())
		case "off":
			return true, tea.Batch(m.runMemoryOff(), m.tick())
		case "clear":
			return true, tea.Batch(m.runMemoryClear(), m.tick())
		case "status":
			return true, tea.Batch(m.runMemoryStatus(), m.tick())
		default:
			m.loading = false
			m.appendText(DimStyle().Render("  usage: /memory [on|off|clear|status]"))
			m.syncViewport()
			return true, nil
		}
	}
	return true, nil
}

// handleBang processes !<name> skill invocations and `! <cmd>` shell shortcuts.
func (m *model) handleBang(text string) (bool, tea.Cmd) {
	// `! <cmd>` (with leading space) = shell shortcut, preserved from before skills
	if strings.HasPrefix(text, "! ") {
		cmdText := strings.TrimPrefix(text, "! ")
		m.appendText("  " + cmdText)
		m.appendText("")
		m.streaming = true
		m.loading = true
		m.spinner = 0
		m.syncViewport()
		return true, tea.Batch(m.execShell(cmdText), m.tick())
	}

	// `!<name>` (no space) = skill invocation
	if strings.HasPrefix(text, "!") {
		name := strings.TrimSpace(strings.TrimPrefix(text, "!"))
		if name == "" {
			return true, nil
		}
		if !m.hasSkill(name) {
			m.appendText(ErrorStyle().Render("  skill not found: " + name))
			m.appendText("")
			m.syncViewport()
			return true, nil
		}
		m.recordHistory("!" + name)
		m.messages = append(m.messages, messageEntry{kind: messageUser, text: "!" + name})
		m.appendText("")
		m.appendText(DimStyle().Render("  Using skill: " + name))
		m.syncViewport()
		return true, m.beginSkillStream(name)
	}

	return false, nil
}

func (m *model) hasSkill(name string) bool {
	for _, s := range m.skills {
		if s.Name == name {
			return true
		}
	}
	return false
}

func matchBang(prefix string, skills []Skill) []Skill {
	if len(prefix) == 0 || prefix[0] != '!' {
		return nil
	}
	lower := strings.ToLower(strings.TrimPrefix(prefix, "!"))
	if lower == "" {
		// show all when just "!"
		out := make([]Skill, len(skills))
		copy(out, skills)
		return out
	}
	var out []Skill
	for _, s := range skills {
		if strings.HasPrefix(strings.ToLower(s.Name), lower) {
			out = append(out, s)
		}
	}
	return out
}

func wardenConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".warden-config.json"), nil
}

func saveWardenConfigField(key string, value any) error {
	path, err := wardenConfigPath()
	if err != nil {
		return err
	}
	var cfg map[string]any
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	cfg[key] = value
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0600)
}
