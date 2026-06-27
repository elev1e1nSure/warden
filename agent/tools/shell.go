package tools

import (
	"os/exec"
	"strings"
)

type PowerShellTool struct{}

func (t *PowerShellTool) Name() string { return "powershell" }

func (t *PowerShellTool) Execute(args map[string]any) Result {
	cmd, _ := args["command"].(string)
	shell := shellExe()
	wrapped := "[Console]::OutputEncoding=[System.Text.Encoding]::UTF8; " + cmd
	proc := exec.Command(shell, "-NonInteractive", "-NoProfile", "-Command", wrapped)
	out, err := proc.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			errStr := clean(string(ee.Stderr))
			if errStr != "" {
				return R("stderr: " + trunc(errStr, 500))
			}
			return R("error: " + err.Error())
		}
	}
	output := clean(string(out))
	output = strings.TrimSpace(output)
	if output == "" {
		return R("(no output)")
	}
	return R(trunc(output, 1000))
}

type BashTool struct{}

func (t *BashTool) Name() string { return "bash" }
func (t *BashTool) Execute(args map[string]any) Result {
	p := PowerShellTool{}
	return p.Execute(args)
}

func shellExe() string {
	if _, err := exec.LookPath("pwsh"); err == nil {
		return "pwsh"
	}
	return "powershell"
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
