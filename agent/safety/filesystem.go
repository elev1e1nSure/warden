package safety

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var driveRe = regexp.MustCompile(`^/[a-z]:`)

func resolveWorkspace() string {
	cwd, _ := os.Getwd()
	return cwd
}

func isPathWithinWorkspace(path string, workspace string) bool {
	target, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	if workspace == "" {
		workspace = resolveWorkspace()
	}
	ws, err := filepath.Abs(workspace)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(ws, target)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func isDangerousPath(path string) bool {
	p := strings.TrimSpace(strings.ToLower(path))
	if strings.HasPrefix(p, `\\`) {
		return true
	}
	if strings.HasPrefix(p, `\\.\`) || strings.HasPrefix(p, `\\?\`) {
		return true
	}
	normalized := strings.ReplaceAll(p, `\`, "/")
	if strings.Contains(normalized, "../") || strings.Contains(normalized, "/..") {
		return true
	}
	if runtime.GOOS == "windows" && strings.HasPrefix(normalized, "/") && !driveRe.MatchString(normalized) {
		return true
	}
	return false
}
