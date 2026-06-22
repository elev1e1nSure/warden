package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
	"warden/internal/client"

	tui "warden"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	port              = 8765
	startupTimeout    = 60 * time.Second
	healthCheckPeriod = 1500 * time.Millisecond
	spinnerPeriod     = 16 * time.Millisecond
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

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
	loadingMs float64
	deadline  time.Time
	startedAt time.Time
	ready     bool
	errMsg    string
	attaching bool
}

type tickMsg struct{}
type readyMsg struct{}

type backendExitMsg struct{ err error }

func tickCmd() tea.Cmd {
	return tea.Tick(spinnerPeriod, func(time.Time) tea.Msg {
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
		time.Sleep(spinnerPeriod)
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
		m.loadingMs += 0.1
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
	frame := spinnerFrames[m.spinner%len(spinnerFrames)]

	switch m.state {
	case stateReady:
		return "ready"
	case stateFailed:
		s := "startup failed"
		if m.errMsg != "" {
			s += ": " + m.errMsg
		}
		return s
	default:
		return fmt.Sprintf("%s Warden loading (%.1fms.)...", frame, m.loadingMs)
	}
}

func findProjectRoot() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("unable to get executable path: %w", err)
	}
	return filepath.Dir(filepath.Clean(exe)), nil
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

func ensurePythonDeps(root string, logFile *os.File) error {
	req := filepath.Join(root, "requirements.txt")
	if _, err := os.Stat(req); err != nil {
		return nil // no requirements.txt, skip
	}
	cmd := exec.Command("pip", "install", "-r", req, "-q", "--disable-pip-version-check")
	cmd.Dir = root
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pip install failed — run manually: pip install -r requirements.txt\n  %w", err)
	}
	return nil
}

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

	var cmd *exec.Cmd
	backendExe := filepath.Join(root, "warden-backend.exe")
	if _, statErr := os.Stat(backendExe); statErr == nil {
		cmd = exec.Command(backendExe)
	} else {
		if err := ensurePythonDeps(root, outFile); err != nil {
			return nil, err
		}
		cmd = exec.Command("python", "-m", "agent.server")
	}
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
	setupCmd(cmd)

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

func runLauncher(root string, cfg WardenConfig, alreadyRunning bool) (ready bool) {
	m := launchModel{
		state:     stateBoot,
		deadline:  time.Now().Add(startupTimeout),
		startedAt: time.Now(),
		ready:     alreadyRunning,
		attaching: alreadyRunning,
	}
	if alreadyRunning {
		m.state = stateReady
		m.ready = true
	}

	p := tea.NewProgram(m)
	model, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "launcher error:", err)
		return false
	}
	lm := model.(launchModel)
	return lm.ready
}

func main() {
	root, err := findProjectRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "find root failed:", err)
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "failed to load config:", err)
	}
	connected := cfg.Model != "" && cfg.APIKey != ""

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

	ready := runLauncher(root, cfg, alreadyRunning)
	if !ready {
		if backend != nil {
			stopBackend(backend)
		}
		os.Exit(1)
	}

	cli := client.NewClient(fmt.Sprintf("http://localhost:%d", port))

	err = tui.Run(cli, cfg.Model, connected)
	if err != nil {
		fmt.Fprintln(os.Stderr, "frontend error:", err)
	}

	cli.Shutdown()

	if backend != nil {
		stopBackend(backend)
	} else if alreadyRunning {
		killBackendByPort()
	}
}
