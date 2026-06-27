//go:build release

package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed warden-backend*
var embeddedBackend []byte

func extractBackend(destDir string) (string, error) {
	name := "warden-backend"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dest := filepath.Join(destDir, name)
	if existing, err := os.ReadFile(dest); err == nil && len(existing) == len(embeddedBackend) {
		return dest, nil
	}
	if err := os.WriteFile(dest, embeddedBackend, 0755); err != nil {
		return "", fmt.Errorf("extract backend: %w", err)
	}
	return dest, nil
}
