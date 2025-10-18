package iot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config captures the simulator bootstrap configuration.
type Config struct {
	BaseURL  string         `yaml:"base_url"`
	Devices  []DeviceConfig `yaml:"devices"`
	Metadata map[string]any `yaml:"metadata,omitempty"` // reserved for future use
}

// DeviceConfig holds credentials for a single simulated device.
type DeviceConfig struct {
	DeviceID string `yaml:"device_id"`
	APIKey   string `yaml:"api_key"`
}

// LoadConfig loads the simulator configuration from disk and validates it.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("config path is required")
	}

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read simulator config: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal simulator config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks whether the configuration is usable.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("base_url is required")
	}

	if len(c.Devices) == 0 {
		return fmt.Errorf("at least one device must be configured")
	}

	for idx, device := range c.Devices {
		if strings.TrimSpace(device.DeviceID) == "" {
			return fmt.Errorf("device %d is missing device_id", idx)
		}
		if strings.TrimSpace(device.APIKey) == "" {
			return fmt.Errorf("device %d (%s) is missing api_key", idx, device.DeviceID)
		}
	}

	return nil
}
