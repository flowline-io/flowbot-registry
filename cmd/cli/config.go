package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bytedance/sonic"
	"github.com/flowline-io/flowbot-registry/pkg/json"
)

// CLIConfig represents the stored CLI configuration including auth tokens.
type CLIConfig struct {
	StoreURL     string    `json:"store_url"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// configFilePath returns the path to the CLI config file.
func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	cfgDir := filepath.Join(home, ".flowbot")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return filepath.Join(cfgDir, "config.json"), nil
}

// loadConfig reads the CLI config from disk.
func loadConfig() (*CLIConfig, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg CLIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

// saveConfig writes the CLI config to disk with restricted permissions.
func saveConfig(cfg *CLIConfig) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}

	data, err := sonic.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open config file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
