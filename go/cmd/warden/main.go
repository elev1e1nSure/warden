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
	"github.com/charmbracelet/lipgloss"
)

const (
	port              = 8765
	startupTimeout    = 60 * time.Second
	healthCheckPeriod = 1500 * time.Millisecond
	spinnerPeriod     = 16 * time.Millisecond
)

var (
	green  = lipgloss.Color("#00D47A")
	blue   = lipgloss.Color("#38BDF8")
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
	deadline  time.Time
	startedAt time.Time
	ready     bool
	errMsg    string
	attaching bool
}

type tickMsg struct{}
type readyMsg struct{}

type backendExitMsg struct{ err error }

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

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
	title := lipgloss.NewStyle().Foreground(blue).Bold(true).Render("warden")
	elapsed := time.Since(m.startedAt).Round(time.Millisecond)
	if elapsed < 0 {
		elapsed = 0
	}

	keyStyle := lipgloss.NewStyle().Foreground(blue).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(faint)

	var status string
	switch m.state {
	case stateBoot, stateWaiting:
		frame := spinnerFrames[m.spinner%len(spinnerFrames)]
		if m.attaching {
			status = "attaching to running warden"
		} else {
			status = "starting warden"
		}
		status = frame + "  " + status
	case stateReady:
		status = lipgloss.NewStyle().Foreground(green).Render("ready")
	case stateFailed:
		status = lipgloss.NewStyle().Foreground(danger).Render("startup failed")
	}

	lines := []string{
		title,
		"",
		keyStyle.Render(status),
		dimStyle.Render("  " + elapsed.String()),
	}
	if m.state == stateFailed && m.errMsg != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(danger).Render(m.errMsg))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
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

	cfg, _ := loadConfig()
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

	err = tui.Run(cfg.Model, connected)
	if err != nil {
		fmt.Fprintln(os.Stderr, "frontend error:", err)
	}

	cli := client.NewClient(fmt.Sprintf("http://localhost:%d", port))
	cli.Shutdown()

	if backend != nil {
		stopBackend(backend)
	} else if alreadyRunning {
		killBackendByPort()
	}
}
