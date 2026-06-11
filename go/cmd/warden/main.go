package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"warden"
)

const (
	port              = 8765
	startupTimeout    = 60 * time.Second
	healthCheckPeriod = 500 * time.Millisecond
)

var (
	green  = lipgloss.Color("#00D47A")
	amber  = lipgloss.Color("#D4A576")
	faint  = lipgloss.Color("#555555")
	subtle = lipgloss.Color("#888888")
	danger = lipgloss.Color("#cc5555")
)

type state int

const (
	stateBoot state = iota
	stateWaiting
	stateReady
	stateFailed
)

type launchModel struct {
	state     state
	spinner   int
	backend   *exec.Cmd
	deadline  time.Time
	ready     bool
	errMsg    string
	modelName string
}

type tickMsg struct{}
type readyMsg struct{}

type backendExitMsg struct{ err error }

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func tickCmd() tea.Cmd {
	return tea.Tick(healthCheckPeriod, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func checkHealthCmd() tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return readyMsg{}
			}
		}
		return tickMsg{}
	}
}

func (m launchModel) Init() tea.Cmd {
	return tickCmd()
}

func (m launchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

	case tickMsg:
		if time.Now().After(m.deadline) {
			m.state = stateFailed
			m.errMsg = fmt.Sprintf("backend did not become healthy in %v", startupTimeout)
			return m, tea.Quit
		}
		m.spinner = (m.spinner + 1) % len(spinnerFrames)
		if m.backend != nil && m.backend.Process != nil {
			// Check if process already exited (ProcessState set after Wait)
			// We avoid Wait here; health timeout is sufficient.
		}
		return m, checkHealthCmd()

	case readyMsg:
		m.state = stateReady
		m.ready = true
		return m, tea.Quit

	case backendExitMsg:
		m.state = stateFailed
		m.errMsg = fmt.Sprintf("backend exited: %v", msg.err)
		return m, tea.Quit
	}

	return m, nil
}

func (m launchModel) View() string {
	title := lipgloss.NewStyle().Foreground(amber).Bold(true).Render("warden")
	var body string
	switch m.state {
	case stateBoot, stateWaiting:
		frame := spinnerFrames[m.spinner%len(spinnerFrames)]
		body = lipgloss.NewStyle().Foreground(faint).Render("  " + frame + "  starting...")
	case stateReady:
		label := "  ready"
		if m.modelName != "" {
			label += "  " + m.modelName
		}
		body = lipgloss.NewStyle().Foreground(subtle).Render(label)
	case stateFailed:
		body = lipgloss.NewStyle().Foreground(danger).Render("  error: " + m.errMsg)
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, "", body, "")
}

func findProjectRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("unable to get caller")
	}
	dir := filepath.Dir(filename)
	for i := 0; i < 3; i++ {
		dir = filepath.Dir(dir)
	}
	return dir, nil
}

func preCheck() (alreadyRunning bool, err error) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return true, nil
		}
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 2*time.Second)
	if err == nil {
		conn.Close()
		return false, fmt.Errorf("port %d is busy, but /health is not healthy", port)
	}
	return false, nil
}

var setupFlag = flag.Bool("setup", false, "Show setup screen to change provider/model/API key.")

func startBackend(root string, cfg WardenConfig) (*exec.Cmd, error) {
	runtimeDir := filepath.Join(root, ".warden")
	os.MkdirAll(runtimeDir, 0755)

	outLog := filepath.Join(runtimeDir, "backend.out.log")
	errLog := filepath.Join(runtimeDir, "backend.err.log")

	outFile, err := os.Create(outLog)
	if err != nil {
		return nil, err
	}
	errFile, err := os.Create(errLog)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("python", "-m", "agent.server")
	cmd.Dir = root

	env := os.Environ()
	env = append(env,
		"PYTHONPATH="+root,
		"PYTHONUTF8=1",
		"PYTHONIOENCODING=utf-8",
		"WARDEN_MODEL="+cfg.Model,
	)
	if cfg.APIURL != "" {
		env = append(env, "WARDEN_API_URL="+cfg.APIURL)
	}
	if cfg.APIKey != "" {
		env = append(env, "OPENROUTER_API_KEY="+cfg.APIKey)
	}

	cmd.Env = env
	cmd.Stdout = outFile
	cmd.Stderr = errFile

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func stopBackend(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	cmd.Process.Kill()
	if runtime.GOOS == "windows" {
		exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	}
	cmd.Wait()
}

func killBackendByPort() {
	if runtime.GOOS == "windows" {
		exec.Command("for", "/f", "\"tokens=5\"", "%a", "in", "('netstat -ano ^| findstr :8765')", "do", "taskkill /F /PID %a").Run()
	} else {
		exec.Command("pkill", "-f", "agent.server").Run()
	}
}

func runLauncher(alreadyRunning bool, modelName string) (ready bool, backend *exec.Cmd) {
	m := launchModel{
		state:     stateBoot,
		deadline:  time.Now().Add(startupTimeout),
		ready:     alreadyRunning,
		modelName: modelName,
	}
	if alreadyRunning {
		m.state = stateReady
		m.ready = true
	}

	p := tea.NewProgram(m)
	model, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "launcher error:", err)
		return false, nil
	}
	lm := model.(launchModel)
	return lm.ready, lm.backend
}

func main() {
	flag.Parse()

	root, err := findProjectRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "find root failed:", err)
		os.Exit(1)
	}

	cfg, exists := loadConfig()
	if !exists || *setupFlag {
		cfg = runSetup(cfg)
		if err := saveConfig(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "save config failed:", err)
		}
	}

	alreadyRunning, err := preCheck()
	if err != nil {
		fmt.Fprintln(os.Stderr, "precheck failed:", err)
		os.Exit(1)
	}

	var backend *exec.Cmd
	if !alreadyRunning {
		backend, err = startBackend(root, cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "start backend failed:", err)
			os.Exit(1)
		}
	}

	ready, _ := runLauncher(alreadyRunning, cfg.Model)
	if !ready {
		if backend != nil {
			stopBackend(backend)
		}
		os.Exit(1)
	}

	err = tui.Run(cfg.Model)
	if err != nil {
		fmt.Fprintln(os.Stderr, "frontend error:", err)
	}

	client := tui.NewClient(fmt.Sprintf("http://localhost:%d", port))
	client.Shutdown()

	if backend != nil {
		stopBackend(backend)
	} else if alreadyRunning {
		killBackendByPort()
	}
}
