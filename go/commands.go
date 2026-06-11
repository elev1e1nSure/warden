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

func (m model) sendMessage(text string) tea.Cmd {
	ch := m.client.SendMessage(text)
	return func() tea.Msg {
		return startStreamMsg{ch: ch}
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

func readNext(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		inner, ok := <-ch
		if !ok {
			return doneMsg{}
		}
		return nextMsg{inner: inner, ch: ch}
	}
}

func (m model) initProvider() tea.Cmd {
	return func() tea.Msg {
		s, err := m.client.GetStatus()
		if err != nil {
			return providerInitMsg{provider: "ollama"}
		}
		return providerInitMsg{provider: s.Provider}
	}
}

func (m model) fetchStatus(brief bool) tea.Cmd {
	return func() tea.Msg {
		s, err := m.client.GetStatus()
		if err != nil {
			return statusResultMsg{model: "error: " + err.Error(), brief: brief}
		}
		return statusResultMsg{
			model:      s.Model,
			provider:   s.Provider,
			mode:       s.Mode,
			thinking:   s.Thinking,
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

func (m model) advance() int {
	return 1
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

func (m model) fetchProviders() tea.Cmd {
	return func() tea.Msg {
		providers, current, err := m.client.ListProviders()
		if err != nil {
			return providersResultMsg{err: err.Error()}
		}
		return providersResultMsg{providers: providers, current: current}
	}
}

func (m model) switchProvider(name string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.SetProvider(name); err != nil {
			return providerSetMsg{err: err.Error()}
		}
		// reload models after switch
		models, current, err := m.client.ListModels()
		if err != nil {
			return providerSetMsg{provider: name, err: err.Error()}
		}
		return providerSetMsg{provider: name, models: models, current: current}
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
