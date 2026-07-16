package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the persisted CLI state stored at ~/.config/filejump/config.json.
type Config struct {
	Server        string `json:"server"`         // e.g. https://filejump.com
	Token         string `json:"token"`          // Sanctum bearer token
	Email         string `json:"email"`          // for display
	WorkspaceID   int    `json:"workspace_id"`   // 0 = personal space
	WorkspaceName string `json:"workspace_name"` // display name of the active workspace
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "filejump", "config.json"), nil
}

// Load reads the config file. Returns an empty Config (not an error) if missing.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &c, nil
}

// Save writes the config to disk with 0600 permissions.
func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Delete removes the config file (used by logout).
func Delete() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
