package iot

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultRefreshInterval = time.Minute
	minRefreshInterval     = 5 * time.Second
)

// Config captures the simulator configuration.
type Config struct {
	BaseURL         string
	RefreshInterval time.Duration
	Devices         []DeviceConfig
}

// DeviceConfig holds credentials and metadata for a simulated device.
type DeviceConfig struct {
	DeviceID        string  `yaml:"device_id"`
	APIKey          string  `yaml:"api_key"`
	TeacherIDs      []int64 `yaml:"teacher_ids,omitempty"`
	teacherIDsParam string
}

type yamlConfig struct {
	BaseURL         string         `yaml:"base_url"`
	RefreshInterval string         `yaml:"refresh_interval,omitempty"`
	Devices         []DeviceConfig `yaml:"devices"`
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

	var raw yamlConfig
	if err := yaml.Unmarshal([]byte(expanded), &raw); err != nil {
		return nil, fmt.Errorf("unmarshal simulator config: %w", err)
	}

	cfg := &Config{
		BaseURL: strings.TrimSpace(raw.BaseURL),
		Devices: raw.Devices,
	}

	if err := cfg.applyRefreshInterval(raw.RefreshInterval); err != nil {
		return nil, err
	}

	// Normalise device entries before validation.
	for idx := range cfg.Devices {
		cfg.Devices[idx].normalise()
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) applyRefreshInterval(raw string) error {
	if strings.TrimSpace(raw) == "" {
		c.RefreshInterval = defaultRefreshInterval
		return nil
	}

	dur, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid refresh_interval: %w", err)
	}
	if dur < minRefreshInterval {
		return fmt.Errorf("refresh_interval must be at least %s", minRefreshInterval)
	}
	c.RefreshInterval = dur
	return nil
}

// Validate checks whether the configuration is usable.
func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	if !strings.HasPrefix(c.BaseURL, "http://") && !strings.HasPrefix(c.BaseURL, "https://") {
		return fmt.Errorf("base_url must include http:// or https:// scheme")
	}

	if len(c.Devices) == 0 {
		return fmt.Errorf("at least one device must be configured")
	}

	for idx, device := range c.Devices {
		if device.DeviceID == "" {
			return fmt.Errorf("device %d is missing device_id", idx)
		}
		if device.APIKey == "" {
			return fmt.Errorf("device %d (%s) is missing api_key", idx, device.DeviceID)
		}
		for _, teacherID := range device.TeacherIDs {
			if teacherID <= 0 {
				return fmt.Errorf("device %d (%s) has invalid teacher_id %d", idx, device.DeviceID, teacherID)
			}
		}
	}

	return nil
}

func (d *DeviceConfig) normalise() {
	d.DeviceID = strings.TrimSpace(d.DeviceID)
	d.APIKey = strings.TrimSpace(d.APIKey)

	if len(d.TeacherIDs) == 0 {
		d.teacherIDsParam = ""
		return
	}

	values := make([]string, 0, len(d.TeacherIDs))
	for _, id := range d.TeacherIDs {
		if id <= 0 {
			continue
		}
		values = append(values, strconv.FormatInt(id, 10))
	}
	d.teacherIDsParam = strings.Join(values, ",")
}

// TeacherIDsParam returns the pre-joined teacher ID list for query parameters.
func (d DeviceConfig) TeacherIDsParam() string {
	return d.teacherIDsParam
}
