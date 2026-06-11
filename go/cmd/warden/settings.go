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
	providerIdx int
	inputs      [3]textinput.Model // 0=model, 1=api_url, 2=api_key
	focusIdx    int                // sfProvider=0, sfModel=1, sfAPIURL=2, sfAPIKey=3
	done        bool
	cancelled   bool
	width       int
}

const (
	sfProvider = 0
	sfModel    = 1
	sfAPIURL   = 2
	sfAPIKey   = 3
)

var setupProviders = []string{"ollama", "openrouter"}

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

	m.inputs[0].Placeholder = "qwen3:8b"
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

	for i, p := range setupProviders {
		if p == cfg.Provider {
			m.providerIdx = i
			break
		}
	}

	m.focusIdx = sfModel
	m.inputs[0].Focus()

	return m
}

func (m settingsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m settingsModel) isOpenRouter() bool {
	return setupProviders[m.providerIdx] == "openrouter"
}

func (m settingsModel) maxField() int {
	if m.isOpenRouter() {
		return sfAPIKey
	}
	return sfModel
}

func (m *settingsModel) syncFocus() {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	switch m.focusIdx {
	case sfModel:
		m.inputs[0].Focus()
	case sfAPIURL:
		m.inputs[1].Focus()
	case sfAPIKey:
		m.inputs[2].Focus()
	}
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
			m.focusIdx++
			if m.focusIdx > m.maxField() {
				m.focusIdx = sfProvider
			}
			m.syncFocus()
			return m, nil

		case tea.KeyShiftTab:
			m.focusIdx--
			if m.focusIdx < sfProvider {
				m.focusIdx = m.maxField()
			}
			m.syncFocus()
			return m, nil

		case tea.KeyLeft:
			if m.focusIdx == sfProvider {
				m.providerIdx = (m.providerIdx - 1 + len(setupProviders)) % len(setupProviders)
				if m.focusIdx > m.maxField() {
					m.focusIdx = m.maxField()
					m.syncFocus()
				}
				return m, nil
			}

		case tea.KeyRight:
			if m.focusIdx == sfProvider {
				m.providerIdx = (m.providerIdx + 1) % len(setupProviders)
				if m.focusIdx > m.maxField() {
					m.focusIdx = m.maxField()
					m.syncFocus()
				}
				return m, nil
			}

		case tea.KeyEnter:
			if m.focusIdx == sfProvider {
				m.providerIdx = (m.providerIdx + 1) % len(setupProviders)
				return m, nil
			}
			if m.focusIdx == m.maxField() {
				m.done = true
				return m, tea.Quit
			}
			m.focusIdx++
			m.syncFocus()
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.focusIdx {
	case sfModel:
		m.inputs[0], cmd = m.inputs[0].Update(msg)
	case sfAPIURL:
		m.inputs[1], cmd = m.inputs[1].Update(msg)
	case sfAPIKey:
		m.inputs[2], cmd = m.inputs[2].Update(msg)
	}
	return m, cmd
}

func (m settingsModel) View() string {
	dimStyle := lipgloss.NewStyle().Foreground(faint)
	activeVal := lipgloss.NewStyle().Foreground(green).Bold(true)

	isActive := func(field int) bool { return m.focusIdx == field }

	valueStr := func(inputIdx, field int) string {
		if isActive(field) {
			return m.inputs[inputIdx].View()
		}
		v := m.inputs[inputIdx].Value()
		if v == "" {
			return dimStyle.Render(m.inputs[inputIdx].Placeholder)
		}
		if m.inputs[inputIdx].EchoMode == textinput.EchoPassword {
			return dimStyle.Render(strings.Repeat("•", len([]rune(v))))
		}
		return dimStyle.Render(v)
	}

	var b strings.Builder
	b.WriteString(dimStyle.Render("warden setup"))
	b.WriteString("\n\n")

	// Provider
	b.WriteString(dimStyle.Render("provider: "))
	for i, p := range setupProviders {
		if i > 0 {
			b.WriteString(dimStyle.Render(" "))
		}
		if i == m.providerIdx {
			b.WriteString(activeVal.Render(p))
		} else {
			b.WriteString(dimStyle.Render(p))
		}
	}
	b.WriteString("\n")

	// Model
	b.WriteString(dimStyle.Render("model: "))
	b.WriteString(valueStr(0, sfModel))
	b.WriteString("\n")

	// OpenRouter-only fields
	if m.isOpenRouter() {
		b.WriteString(dimStyle.Render("api url: "))
		b.WriteString(valueStr(1, sfAPIURL))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("api key: "))
		b.WriteString(valueStr(2, sfAPIKey))
		b.WriteString("\n")
	}

	return b.String()
}

func (m settingsModel) result() WardenConfig {
	model := strings.TrimSpace(m.inputs[0].Value())
	if model == "" {
		model = "qwen3:8b"
	}
	cfg := WardenConfig{
		Provider: setupProviders[m.providerIdx],
		Model:    model,
	}
	if m.isOpenRouter() {
		cfg.APIURL = strings.TrimSpace(m.inputs[1].Value())
		cfg.APIKey = strings.TrimSpace(m.inputs[2].Value())
	}
	return cfg
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
