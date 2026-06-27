package tools

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type ProcessListTool struct{}

func (t *ProcessListTool) Name() string { return "process_list" }

func (t *ProcessListTool) Execute(args map[string]any) Result {
	filter := strings.ToLower(strings.TrimSpace(getStr(args, "filter")))
	var rows []string
	var err error
	if runtime.GOOS == "windows" {
		rows, err = listWindows(filter)
	} else {
		rows, err = listUnix(filter)
	}
	if err != nil {
		return R("error: " + err.Error())
	}
	if len(rows) == 0 {
		if filter != "" {
			return R("no processes matching '" + filter + "'")
		}
		return R("no processes")
	}
	var b strings.Builder
	for i, r := range rows {
		if i >= 200 {
			b.WriteString(fmt.Sprintf("\n... and %d more", len(rows)-200))
			break
		}
		b.WriteString(r)
		b.WriteString("\n")
	}
	return R(strings.TrimRight(b.String(), "\n"))
}

func listWindows(filter string) ([]string, error) {
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var rows []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// CSV: "name","pid",...
		parts := strings.Split(line, "\",\"")
		if len(parts) < 2 {
			continue
		}
		name := strings.Trim(parts[0], "\" ")
		pidStr := strings.Trim(parts[1], "\" ")
		if _, err := strconv.Atoi(pidStr); err != nil {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(name), filter) {
			continue
		}
		rows = append(rows, fmt.Sprintf("%7s  %s", pidStr, name))
	}
	return rows, nil
}

var psRe = regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)

func listUnix(filter string) ([]string, error) {
	cmd := exec.Command("ps", "-eo", "pid=,comm=")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var rows []string
	for _, line := range strings.Split(string(out), "\n") {
		m := psRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pid, name := m[1], strings.TrimSpace(m[2])
		if filter != "" && !strings.Contains(strings.ToLower(name), filter) {
			continue
		}
		rows = append(rows, fmt.Sprintf("%7s  %s", pid, name))
	}
	return rows, nil
}

type ProcessKillTool struct{}

func (t *ProcessKillTool) Name() string { return "process_kill" }

func (t *ProcessKillTool) Execute(args map[string]any) Result {
	if _, ok := args["pid"]; !ok {
		return R("error: pid is required")
	}
	pid := int(getFloat(args, "pid"))
	if pid <= 1 {
		return R("error: refusing to kill PID 0 or 1 (init/system)")
	}
	if pid == os.Getpid() {
		return R("error: refusing to kill self")
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
	} else {
		cmd = exec.Command("kill", "-9", strconv.Itoa(pid))
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		errStr := strings.TrimSpace(string(out))
		if errStr == "" {
			errStr = err.Error()
		}
		return R("error: failed to kill PID " + strconv.Itoa(pid) + ": " + trunc(errStr, 200))
	}
	return R("killed PID " + strconv.Itoa(pid))
}
