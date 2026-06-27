package main

import (
	"fmt"
	"os"

	tui "github.com/elev1e1nSure/warden/internal/tui"
)

func main() {
	cfg, err := loadConfig()
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "failed to load config:", err)
	}
	connected := cfg.Model != "" && cfg.APIKey != ""

	backend := NewAgentBackend(cfg.Model, cfg.APIURL, cfg.APIKey)

	err = tui.Run(backend, cfg.Model, connected)
	if err != nil {
		fmt.Fprintln(os.Stderr, "frontend error:", err)
		os.Exit(1)
	}
}
