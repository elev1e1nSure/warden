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

var setupFieldDescs = map[int]string{
	sfProvider: "Where your model runs. Local server or cloud API.",
	sfModel:    "Model identifier. e.g. qwen3:8b, llama3.1, gpt-4o.",
	sfAPIURL:   "OpenAI-compatible endpoint. Default works for openrouter.",
	sfAPIKey:   "Your provider's API key. Stored in ~/.warden-config.json.",
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

	m.focusIdx = sfProvider

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
	greenStyle := lipgloss.NewStyle().Foreground(green)
	whiteBoldStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	faintStyle := lipgloss.NewStyle().Foreground(faint)
	sepStyle := lipgloss.NewStyle().Foreground(faint)

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

	providerVal := func(active bool) string {
		var parts []string
		for i, p := range setupProviders {
			if i == m.providerIdx && active {
				parts = append(parts, whiteBoldStyle.Render(p))
			} else {
				parts = append(parts, dimStyle.Render(p))
			}
		}
		return strings.Join(parts, "  ")
	}

	row := func(label string, fieldID int, value string) string {
		pad := 10
		padded := label
		if w := lipgloss.Width(padded); w < pad {
			padded += strings.Repeat(" ", pad-w)
		}
		if isActive(fieldID) {
			return greenStyle.Render("●") + "  " + whiteBoldStyle.Render(padded) + "  " + value
		}
		return faintStyle.Render("○") + "  " + dimStyle.Render(padded) + "  " + value
	}

	total := 2
	if m.isOpenRouter() {
		total = 4
	}
	stepLine := dimStyle.Render(fmt.Sprintf("step %d/%d", m.focusIdx+1, total))

	header := dimStyle.Render("warden setup")

	gap := strings.Repeat(" ", max(1, m.width-lipgloss.Width(header)-lipgloss.Width(stepLine)))
	title := header + gap + stepLine

	var rows []string
	rows = append(rows, title, "")
	rows = append(rows, row("provider", sfProvider, providerVal(isActive(sfProvider))))
	rows = append(rows, row("model", sfModel, valueStr(0, sfModel)))
	if m.isOpenRouter() {
		rows = append(rows, row("api url", sfAPIURL, valueStr(1, sfAPIURL)))
		rows = append(rows, row("api key", sfAPIKey, valueStr(2, sfAPIKey)))
	}

	sep := sepStyle.Render(strings.Repeat("─", max(20, m.width-4)))
	help := "  " + dimStyle.Render(setupFieldDescs[m.focusIdx])

	hints := "  " + faintStyle.Render("← →  switch provider    tab  next field    enter  save    esc  cancel")

	body := strings.Join(rows, "\n")
	return body + "\n\n" + sep + "\n" + help + "\n\n" + sep + "\n" + hints
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
