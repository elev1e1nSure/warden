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
	{"/models", "Switch model"},
	{"/select", "Enable text selection (disables mouse capture)"},
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
	case "/models":
		m.clearHintState()
		return true, m.fetchModels()
	}

	switch trimmed {
	case "/select":
		m.clearHintState()
		m.selectMode = true
		m.syncViewport()
		return true, tea.DisableMouse
	case "/verbose":
		m.verboseMode = !m.verboseMode
		m.clearHintState()
		m.syncViewport()
		return true, nil
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
		if name == "" || !m.hasSkill(name) {
			return true, nil
		}
		return true, m.loadSkill(name)
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
		if strings.HasPrefix(s.Name, lower) {
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
