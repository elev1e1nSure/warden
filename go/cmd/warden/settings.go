package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type WardenConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIURL   string `json:"api_url"`
	APIKey   string `json:"api_key"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".warden-config.json"), nil
}

func defaultConfig() WardenConfig {
	return WardenConfig{Provider: "ollama", Model: "qwen3:8b"}
}

func loadConfig() (WardenConfig, bool) {
	path, err := configPath()
	if err != nil {
		return defaultConfig(), false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultConfig(), false
	}
	var cfg WardenConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig(), false
	}
	if cfg.Provider == "" {
		cfg.Provider = "ollama"
	}
	if cfg.Model == "" {
		cfg.Model = "qwen3:8b"
	}
	return cfg, true
}

func saveConfig(cfg WardenConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// settingsModel is the setup screen shown on first run or --setup.
type settingsModel struct {
	inputs    [3]textinput.Model // 0=model, 1=api_url, 2=api_key
	focusIdx  int
	done      bool
	cancelled bool
	width     int
}

const (
	sfModel  = 0
	sfAPIURL = 1
	sfAPIKey = 2
)

var setupFieldDescs = [3]string{
	"Model identifier. e.g. gpt-4o, deepseek/deepseek-r1, llama-3.3-70b.",
	"OpenAI-compatible endpoint URL.",
	"Your API key. Stored in ~/.warden-config.json.",
}

func newSettingsModel(cfg WardenConfig) settingsModel {
	m := settingsModel{width: 80}

	for i := range m.inputs {
		ti := textinput.New()
		ti.Prompt = ""
		ti.CharLimit = 256
		ti.BlinkSpeed = 0
		ti.Width = 50
		m.inputs[i] = ti
	}

	m.inputs[0].Placeholder = "deepseek/deepseek-r1"
	m.inputs[0].SetValue(cfg.Model)

	m.inputs[1].Placeholder = "https://openrouter.ai/api/v1"
	apiURL := cfg.APIURL
	if apiURL == "" {
		apiURL = "https://openrouter.ai/api/v1"
	}
	m.inputs[1].SetValue(apiURL)

	m.inputs[2].Placeholder = "sk-or-v1-..."
	m.inputs[2].EchoMode = textinput.EchoPassword
	m.inputs[2].EchoCharacter = '•'
	m.inputs[2].SetValue(cfg.APIKey)

	m.focusIdx = sfModel
	m.inputs[0].Focus()

	return m
}

func (m settingsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *settingsModel) syncFocus() {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	m.inputs[m.focusIdx].Focus()
}

func (m settingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		w := msg.Width - 20
		if w < 20 {
			w = 20
		}
		for i := range m.inputs {
			m.inputs[i].Width = w
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit

		case tea.KeyTab:
			m.focusIdx = (m.focusIdx + 1) % 3
			m.syncFocus()
			return m, nil

		case tea.KeyShiftTab:
			m.focusIdx = (m.focusIdx + 2) % 3
			m.syncFocus()
			return m, nil

		case tea.KeyEnter:
			if m.focusIdx == sfAPIKey {
				m.done = true
				return m, tea.Quit
			}
			m.focusIdx++
			m.syncFocus()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.inputs[m.focusIdx], cmd = m.inputs[m.focusIdx].Update(msg)
	return m, cmd
}

func (m settingsModel) View() string {
	dimStyle := lipgloss.NewStyle().Foreground(faint)
	greenStyle := lipgloss.NewStyle().Foreground(green)
	whiteBoldStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	faintStyle := lipgloss.NewStyle().Foreground(faint)
	sepStyle := lipgloss.NewStyle().Foreground(faint)

	isActive := func(field int) bool { return m.focusIdx == field }

	valueStr := func(field int) string {
		if isActive(field) {
			return m.inputs[field].View()
		}
		v := m.inputs[field].Value()
		if v == "" {
			return dimStyle.Render(m.inputs[field].Placeholder)
		}
		if m.inputs[field].EchoMode == textinput.EchoPassword {
			return dimStyle.Render(strings.Repeat("•", len([]rune(v))))
		}
		return dimStyle.Render(v)
	}

	row := func(label string, fieldID int) string {
		pad := 10
		padded := label
		if w := lipgloss.Width(padded); w < pad {
			padded += strings.Repeat(" ", pad-w)
		}
		if isActive(fieldID) {
			return greenStyle.Render("●") + "  " + whiteBoldStyle.Render(padded) + "  " + valueStr(fieldID)
		}
		return faintStyle.Render("○") + "  " + dimStyle.Render(padded) + "  " + valueStr(fieldID)
	}

	stepLine := dimStyle.Render(fmt.Sprintf("step %d/3", m.focusIdx+1))
	header := dimStyle.Render("warden setup")
	gap := strings.Repeat(" ", max(1, m.width-lipgloss.Width(header)-lipgloss.Width(stepLine)))
	title := header + gap + stepLine

	var rows []string
	rows = append(rows, title, "")
	rows = append(rows, row("model", sfModel))
	rows = append(rows, row("api url", sfAPIURL))
	rows = append(rows, row("api key", sfAPIKey))

	sep := sepStyle.Render(strings.Repeat("─", max(20, m.width-4)))
	help := "  " + dimStyle.Render(setupFieldDescs[m.focusIdx])
	hints := "  " + faintStyle.Render("tab  next field    enter  save    esc  cancel")

	return strings.Join(rows, "\n") + "\n\n" + sep + "\n" + help + "\n\n" + sep + "\n" + hints
}

func (m settingsModel) result() WardenConfig {
	model := strings.TrimSpace(m.inputs[sfModel].Value())
	if model == "" {
		model = "deepseek/deepseek-r1"
	}
	return WardenConfig{
		Provider: "openrouter",
		Model:    model,
		APIURL:   strings.TrimSpace(m.inputs[sfAPIURL].Value()),
		APIKey:   strings.TrimSpace(m.inputs[sfAPIKey].Value()),
	}
}

func runSetup(cfg WardenConfig) WardenConfig {
	sm := newSettingsModel(cfg)
	p := tea.NewProgram(sm)
	result, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup error:", err)
		os.Exit(1)
	}
	fin := result.(settingsModel)
	if fin.cancelled {
		os.Exit(0)
	}
	return fin.result()
}
