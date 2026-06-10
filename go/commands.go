package tui

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) checkBackend() tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(m.client.BaseURL + "/health")
		if err != nil || resp.StatusCode != 200 {
			return backendErrorMsg{}
		}
		return backendReadyMsg{}
	}
}

func (m model) sendMessage(text string) tea.Cmd {
	ch := m.client.SendMessage(text)
	return func() tea.Msg {
		return startStreamMsg{ch: ch}
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
		return modeMsg{auto: auto}
	}
}

func (m model) setThinking(enabled bool) tea.Cmd {
	return func() tea.Msg {
		m.client.SetThinking(enabled)
		return nil
	}
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
			model:    s.Model,
			provider: s.Provider,
			mode:     s.Mode,
			thinking: s.Thinking,
			cwd:      s.CWD,
			brief:    brief,
		}
	}
}

func (m model) fetchTools() tea.Cmd {
	return func() tea.Msg {
		tools, err := m.client.GetTools()
		if err != nil {
			return toolsResultMsg{tools: []string{"error: " + err.Error()}}
		}
		return toolsResultMsg{tools: tools}
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

func (m model) tick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m model) advance() int {
	return 1
}
