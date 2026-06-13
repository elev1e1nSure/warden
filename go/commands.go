package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) checkBackend() tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(m.client.BaseURL + "/health")
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			return backendErrorMsg{}
		}
		resp.Body.Close()
		return backendReadyMsg{}
	}
}

func (m model) sendQuestion(id string, answers [][]string) tea.Cmd {
	return func() tea.Msg {
		m.client.SendQuestion(id, answers)
		return nil
	}
}

func (m model) sendConfirm(id string, ok bool) tea.Cmd {
	return func() tea.Msg {
		m.client.SendConfirm(id, ok)
		return nil
	}
}

func (m model) setMode(auto bool) tea.Cmd {
	return func() tea.Msg {
		m.client.SetMode(auto)
		saveAutoMode(auto)
		return modeMsg{auto: auto}
	}
}

func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".warden-settings.json"), nil
}

func loadAutoMode() bool {
	path, err := settingsPath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var s struct {
		AutoMode bool `json:"auto_mode"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return false
	}
	return s.AutoMode
}

func saveAutoMode(auto bool) {
	path, err := settingsPath()
	if err != nil {
		return
	}
	data, _ := json.Marshal(map[string]bool{"auto_mode": auto})
	os.WriteFile(path, data, 0644)
}

func (m model) fetchStatus(brief bool) tea.Cmd {
	return func() tea.Msg {
		s, err := m.client.GetStatus()
		if err != nil {
			return statusResultMsg{model: "error: " + err.Error(), brief: brief}
		}
		return statusResultMsg{
			model:      s.Model,
			mode:       s.Mode,
			cwd:        s.CWD,
			brief:      brief,
			tokenCount: s.TokenCount,
			tokenLimit: s.TokenLimit,
		}
	}
}

func (m model) runCompact() tea.Cmd {
	return func() tea.Msg {
		result, err := m.client.Compact()
		if err != nil {
			return compactResultMsg{err: err.Error()}
		}
		return compactResultMsg{
			tokensBefore: result.TokensBefore,
			tokensAfter:  result.TokensAfter,
		}
	}
}

func (m model) runMemoryOn() tea.Cmd {
	return func() tea.Msg {
		if err := m.client.SetMemoryState(true); err != nil {
			return memoryResultMsg{err: err.Error()}
		}
		return memoryResultMsg{text: "memory enabled"}
	}
}

func (m model) runMemoryOff() tea.Cmd {
	return func() tea.Msg {
		if err := m.client.SetMemoryState(false); err != nil {
			return memoryResultMsg{err: err.Error()}
		}
		return memoryResultMsg{text: "memory disabled"}
	}
}

func (m model) runMemoryClear() tea.Cmd {
	return func() tea.Msg {
		count, err := m.client.ClearMemory()
		if err != nil {
			return memoryResultMsg{err: err.Error()}
		}
		return memoryResultMsg{text: fmt.Sprintf("memory cleared: %d entries", count)}
	}
}

func (m model) runMemoryStatus() tea.Cmd {
	return func() tea.Msg {
		state, err := m.client.GetMemoryState()
		if err != nil {
			return memoryResultMsg{err: err.Error()}
		}
		status := "off"
		if state.Enabled {
			status = "on"
		}
		return memoryResultMsg{text: fmt.Sprintf(
			"memory: %s | entries: %d | snapshots: %d | db: %d KB",
			status, state.Entries, state.Snapshots, state.DBSize/1024,
		)}
	}
}

func (m model) copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("powershell", "-NonInteractive", "-NoProfile", "-Command", "$env:WARDEN_CLIP | Set-Clipboard")
		cmd.Env = append(os.Environ(), "WARDEN_CLIP="+text)
		if err := cmd.Run(); err != nil {
			return clipboardDoneMsg{err: fmt.Errorf("Set-Clipboard: %w", err)}
		}
		return clipboardDoneMsg{}
	}
}

// setContent updates viewport content without forcing scroll.
func setContent(vp viewport.Model, lines []string) viewport.Model {
	vp.SetContent(strings.Join(lines, "\n"))
	return vp
}

func (m model) execShell(cmdText string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("powershell", "-NonInteractive", "-NoProfile", "-Command", cmdText)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return tokenMsg{text: "\n" + string(out) + "\n" + err.Error()}
		}
		return shellResultMsg{output: string(out)}
	}
}

func (m model) tick() tea.Cmd {
	return tea.Tick(70*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m model) fetchModels() tea.Cmd {
	return func() tea.Msg {
		models, current, err := m.client.ListModels()
		if err != nil {
			return modelsResultMsg{err: err.Error()}
		}
		return modelsResultMsg{models: models, current: current}
	}
}

func (m model) applyModel(name string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.SetModel(name); err != nil {
			return modelSetMsg{err: err.Error()}
		}
		return modelSetMsg{model: name}
	}
}

func (m model) fetchSkills() tea.Cmd {
	return func() tea.Msg {
		skills, err := m.client.ListSkills()
		if err != nil {
			return skillsResultMsg{err: err.Error()}
		}
		return skillsResultMsg{skills: skills}
	}
}

func (m model) loadSkill(name string) tea.Cmd {
	return func() tea.Msg {
		content, err := m.client.LoadSkill(name)
		if err != nil {
			return skillLoadedMsg{name: name, err: err.Error()}
		}
		return skillLoadedMsg{name: name, content: content}
	}
}

func (m *model) handleConnectWizardKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return false, nil
	}
	if m.cwErr != "" {
		if msg.Type == tea.KeyEsc {
			m.cwErr = ""
			m.updateViewportHeight()
			m.syncViewport()
		}
		return true, nil
	}
	if m.cwLoading {
		return true, nil
	}
	switch m.cwStep {
	case 0:
		switch msg.Type {
		case tea.KeyUp:
			if m.cwPickIdx > 0 {
				m.cwPickIdx--
			}
		case tea.KeyDown:
			if m.cwPickIdx < 1 {
				m.cwPickIdx++
			}
		case tea.KeyEnter:
			providers := []string{"openrouter", "ollama"}
			m.cwProvider = providers[m.cwPickIdx]
			// no hardcoded model lists — models are free-form
			m.cwModels = []string{"enter custom..."}
			if m.cwProvider == "openrouter" {
				ti := textinput.New()
				ti.Prompt = ""
				ti.EchoMode = textinput.EchoPassword
				ti.EchoCharacter = '•'
				ti.CharLimit = 256
				ti.Width = 50
				ti.Focus()
				m.cwInput = ti
				m.cwStep = 1
			} else {
				m.cwPickIdx = 0
				m.cwCustom = false
				m.cwStep = 2
			}
		case tea.KeyEsc:
			m.cwOpen = false
		}
		m.updateViewportHeight()
		m.syncViewport()
		return true, nil

	case 1:
		switch msg.Type {
		case tea.KeyEnter:
			val := strings.TrimSpace(m.cwInput.Value())
			if val != "" {
				m.cwAPIKey = val
				m.cwPickIdx = 0
				m.cwCustom = false
				m.cwStep = 2
				m.updateViewportHeight()
				m.syncViewport()
			}
		case tea.KeyEsc:
			m.cwStep = 0
			m.cwPickIdx = 0
			m.cwInput.Reset()
			m.updateViewportHeight()
			m.syncViewport()
		default:
			var cmd tea.Cmd
			m.cwInput, cmd = m.cwInput.Update(msg)
			m.syncViewport()
			return true, cmd
		}
		return true, nil

	case 2:
		if m.cwCustom {
			switch msg.Type {
			case tea.KeyEnter:
				val := strings.TrimSpace(m.cwInput.Value())
				if val != "" {
					return true, m.submitConnect()
				}
			case tea.KeyEsc:
				m.cwCustom = false
				m.cwInput.Reset()
				m.updateViewportHeight()
				m.syncViewport()
			default:
				var cmd tea.Cmd
				m.cwInput, cmd = m.cwInput.Update(msg)
				m.syncViewport()
				return true, cmd
			}
			return true, nil
		}
		switch msg.Type {
		case tea.KeyUp:
			if m.cwPickIdx > 0 {
				m.cwPickIdx--
				if m.cwPickIdx < m.cwScroll {
					m.cwScroll = m.cwPickIdx
				}
			}
			m.updateViewportHeight()
			m.syncViewport()
		case tea.KeyDown:
			if m.cwPickIdx < len(m.cwModels)-1 {
				m.cwPickIdx++
				const maxVis = 7
				if m.cwPickIdx >= m.cwScroll+maxVis {
					m.cwScroll = m.cwPickIdx - maxVis + 1
				}
			}
			m.updateViewportHeight()
			m.syncViewport()
		case tea.KeyEnter:
			if m.cwPickIdx == len(m.cwModels)-1 {
				ti := textinput.New()
				ti.Prompt = ""
				ti.CharLimit = 256
				ti.Width = 50
				ti.Focus()
				m.cwInput = ti
				m.cwCustom = true
				m.updateViewportHeight()
				m.syncViewport()
			} else {
				return true, m.submitConnect()
			}
		case tea.KeyEsc:
			if m.cwProvider == "openrouter" {
				m.cwStep = 1
			} else {
				m.cwStep = 0
				m.cwPickIdx = 0
			}
			m.cwCustom = false
			m.updateViewportHeight()
			m.syncViewport()
		}
		return true, nil
	}
	return true, nil
}

func (m *model) submitConnect() tea.Cmd {
	var modelName string
	if m.cwCustom {
		modelName = strings.TrimSpace(m.cwInput.Value())
	} else {
		modelName = m.cwModels[m.cwPickIdx]
	}
	provider := m.cwProvider
	var apiKey, apiURL string
	if provider == "openrouter" {
		apiURL = "https://openrouter.ai/api/v1"
		apiKey = m.cwAPIKey
	}
	m.cwLoading = true
	m.cwErr = ""
	m.updateViewportHeight()
	m.syncViewport()
	return func() tea.Msg {
		err := m.client.Connect(provider, apiURL, apiKey, modelName)
		if err != nil {
			return connectResultMsg{ok: false, err: err.Error()}
		}
		return connectResultMsg{
			ok:       true,
			model:    modelName,
			provider: provider,
			apiURL:   apiURL,
			apiKey:   apiKey,
		}
	}
}
