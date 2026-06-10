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
	cyan  = lipgloss.Color("#3CBE71")
	dim   = lipgloss.Color("#666666")
	red   = lipgloss.Color("#ff4444")
	green = lipgloss.Color("#32CD32")
)

func titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(cyan).Bold(true)
}

func dimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(dim)
}

func okStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(green).Bold(true)
}

func errStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(red).Bold(true)
}

type state int

const (
	stateBoot state = iota
	stateWaiting
	stateReady
	stateFailed
)

type launchModel struct {
	state    state
	spinner  int
	backend  *exec.Cmd
	deadline time.Time
	ready    bool
	errMsg   string
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
	var body string
	switch m.state {
	case stateBoot:
		body = "  starting backend... " + spinnerFrames[m.spinner]
	case stateWaiting:
		body = "  waiting for backend... " + spinnerFrames[m.spinner]
	case stateReady:
		body = okStyle().Render("  backend ready")
	case stateFailed:
		body = errStyle().Render("  error: " + m.errMsg)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle().Render("warden"),
		dimStyle().Render("────────────────────────"),
		body,
		"",
	)
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

var (
	providerFlag = flag.String("provider", "ollama", "LLM provider: ollama | openrouter")
	apiURLFlag   = flag.String("api", "", "Override API base URL. If empty, provider picks the default.")
	modelFlag    = flag.String("model", "qwen3:8b", "Model name (e.g. qwen/qwen3-coder:free, poolside/laguna-m.1:free)")
)

func startBackend(root string, apiURL string, model string) (*exec.Cmd, error) {
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
	cmd.Env = append(os.Environ(),
		"PYTHONPATH="+root,
		"PYTHONUTF8=1",
		"PYTHONIOENCODING=utf-8",
		"WARDEN_MODEL="+model,
	)
	if apiURL != "" {
		cmd.Env = append(cmd.Env, "WARDEN_API_URL="+apiURL)
	}
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

func runLauncher(alreadyRunning bool) (ready bool, backend *exec.Cmd) {
	m := launchModel{
		state:    stateBoot,
		deadline: time.Now().Add(startupTimeout),
		ready:    alreadyRunning,
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

	apiURL := *apiURLFlag
	if apiURL == "" && *providerFlag == "openrouter" {
		apiURL = "https://openrouter.ai/api/v1"
	}

	alreadyRunning, err := preCheck()
	if err != nil {
		fmt.Fprintln(os.Stderr, "precheck failed:", err)
		os.Exit(1)
	}

	var backend *exec.Cmd
	if !alreadyRunning {
		root, err := findProjectRoot()
		if err != nil {
			fmt.Fprintln(os.Stderr, "find root failed:", err)
			os.Exit(1)
		}
		backend, err = startBackend(root, apiURL, *modelFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "start backend failed:", err)
			os.Exit(1)
		}
	}

	ready, _ := runLauncher(alreadyRunning)
	if !ready {
		if backend != nil {
			stopBackend(backend)
		}
		os.Exit(1)
	}

	err = tui.Run(*modelFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "frontend error:", err)
	}

	if backend != nil {
		stopBackend(backend)
	} else if alreadyRunning {
		killBackendByPort()
	}
}
