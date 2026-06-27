package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"github.com/elev1e1n/warden/internal/security"
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

func loadConfig() (WardenConfig, error) {
	path, err := configPath()
	if err != nil {
		return defaultConfig(), err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultConfig(), err
	}
	var cfg WardenConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig(), err
	}
	plainKey, err := security.DecryptString(cfg.APIKey)
	if err != nil {
		return defaultConfig(), err
	}
	cfg.APIKey = plainKey
	if cfg.APIURL == "" {
		cfg.APIURL = "https://openrouter.ai/api/v1"
	}
	return cfg, nil
}

func saveConfig(cfg WardenConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	plainKey, err := security.DecryptString(cfg.APIKey)
	if err != nil {
		return err
	}
	if plainKey != "" {
		encrypted, err := security.EncryptString(plainKey)
		if err != nil {
			return err
		}
		cfg.APIKey = encrypted
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
