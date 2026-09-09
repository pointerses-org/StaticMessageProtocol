// Package cli provides the SMP CLI implementation.
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds upstream tracking and transport settings.
type Config struct {
	Server      string `json:"server"`       // smp@<ip>
	User        string `json:"user"`         // smp@<username>
	Protocol    string `json:"protocol"`     // smp|http|ssh|tcp
	ContextID   uint64 `json:"context_id"`   // last message ID
}

// LoadConfig reads the .smprc file from the home directory.
func LoadConfig() *Config {
	cfg := &Config{Protocol: "smp"}
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(filepath.Join(home, ".smprc"))
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, cfg)
	return cfg
}

// SaveConfig writes the .smprc file to the home directory.
func (c *Config) SaveConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, ".smprc"), data, 0644)
}
