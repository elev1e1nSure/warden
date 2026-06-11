package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type WardenConfig struct {
	Model  string `json:"model"`
	APIURL string `json:"api_url"`
	APIKey string `json:"api_key"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".warden-config.json"), nil
}

func defaultConfig() WardenConfig {
	return WardenConfig{
		APIURL: "https://openrouter.ai/api/v1",
	}
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
	if cfg.APIURL == "" {
		cfg.APIURL = "https://openrouter.ai/api/v1"
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
