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

	defaultEventInterval    = 5 * time.Second
	minEventInterval        = time.Second
	defaultMaxEventsPerTick = 3

	defaultMinAGHops = 2
	defaultMaxAGHops = 3
)

// Config captures the simulator configuration.
type Config struct {
	BaseURL         string
	RefreshInterval time.Duration
	Event           EventConfig
	Devices         []DeviceConfig
}

// EventConfig controls how the traffic engine behaves.
type EventConfig struct {
	Interval         time.Duration
	MaxEventsPerTick int
	Rotation         RotationConfig
	Actions          []ActionConfig
}

// RotationConfig defines the ordered sequence of locations a student cycles through.
type RotationConfig struct {
	Order     []RotationPhase
	MinAGHops int
	MaxAGHops int
}

// RotationPhase identifies a logical location in the rotation loop.
type RotationPhase string

const (
	RotationPhaseHeimatraum RotationPhase = "heimatraum"
	RotationPhaseAG         RotationPhase = "ag"
	RotationPhaseSchulhof   RotationPhase = "schulhof"
)

// ActionType enumerates simulator actions.
type ActionType string

const (
	ActionCheckIn          ActionType = "checkin"
	ActionCheckOut         ActionType = "checkout"
	ActionSchulhofHop      ActionType = "schulhof_hop"
	ActionAttendanceToggle ActionType = "attendance_toggle"
	ActionSupervisorSwap   ActionType = "supervisor_swap"
)

// ActionConfig defines an action the engine may perform.
type ActionConfig struct {
	Type      ActionType `yaml:"type"`
	Weight    float64    `yaml:"weight,omitempty"`
	DeviceIDs []string   `yaml:"device_ids,omitempty"`
	Disabled  bool       `yaml:"disabled,omitempty"`
}

// DeviceConfig holds credentials and metadata for a simulated device.
type DeviceConfig struct {
	DeviceID        string         `yaml:"device_id"`
	APIKey          string         `yaml:"api_key"`
	TeacherIDs      []int64        `yaml:"teacher_ids,omitempty"`
	DefaultSession  *SessionConfig `yaml:"default_session,omitempty"`
	teacherIDsParam string
}

// SessionConfig defines a default session a device should maintain.
type SessionConfig struct {
	ActivityID    int64   `yaml:"activity_id"`
	RoomID        int64   `yaml:"room_id"`
	SupervisorIDs []int64 `yaml:"supervisor_ids,omitempty"`
}

type yamlConfig struct {
	BaseURL         string          `yaml:"base_url"`
	RefreshInterval string          `yaml:"refresh_interval,omitempty"`
	Event           yamlEventConfig `yaml:"event,omitempty"`
	Devices         []DeviceConfig  `yaml:"devices"`
}

type yamlEventConfig struct {
	Interval         string             `yaml:"interval,omitempty"`
	MaxEventsPerTick *int               `yaml:"max_events_per_tick,omitempty"`
	Rotation         yamlRotationConfig `yaml:"rotation,omitempty"`
	Actions          []ActionConfig     `yaml:"actions,omitempty"`
}

type yamlRotationConfig struct {
	Order     []RotationPhase `yaml:"order,omitempty"`
	MinAGHops *int            `yaml:"min_ag_hops,omitempty"`
	MaxAGHops *int            `yaml:"max_ag_hops,omitempty"`
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

	if err := cfg.applyEventDefaults(raw.Event); err != nil {
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

func (c *Config) applyEventDefaults(raw yamlEventConfig) error {
	// Interval
	if strings.TrimSpace(raw.Interval) == "" {
		c.Event.Interval = defaultEventInterval
	} else {
		dur, err := time.ParseDuration(strings.TrimSpace(raw.Interval))
		if err != nil {
			return fmt.Errorf("invalid event.interval: %w", err)
		} else {
			c.Event.Interval = dur
		}
	}

	if raw.MaxEventsPerTick != nil {
		c.Event.MaxEventsPerTick = *raw.MaxEventsPerTick
	} else {
		c.Event.MaxEventsPerTick = defaultMaxEventsPerTick
	}

	// Rotation defaults
	order := raw.Rotation.Order
	if len(order) == 0 {
		order = []RotationPhase{
			RotationPhaseHeimatraum,
			RotationPhaseAG,
			RotationPhaseSchulhof,
			RotationPhaseHeimatraum,
		}
	}
	c.Event.Rotation.Order = order

	minAG := defaultMinAGHops
	if raw.Rotation.MinAGHops != nil {
		minAG = *raw.Rotation.MinAGHops
	}
	c.Event.Rotation.MinAGHops = minAG

	maxAG := defaultMaxAGHops
	if raw.Rotation.MaxAGHops != nil {
		maxAG = *raw.Rotation.MaxAGHops
	}
	c.Event.Rotation.MaxAGHops = maxAG

	// Actions
	actions := raw.Actions
	if len(actions) == 0 {
		actions = []ActionConfig{
			{Type: ActionCheckIn, Weight: 1},
			{Type: ActionCheckOut, Weight: 0.8},
			{Type: ActionSchulhofHop, Weight: 0.5},
			{Type: ActionAttendanceToggle, Weight: 0.6},
			{Type: ActionSupervisorSwap, Weight: 0.3},
		}
	}

	// normalise weights and ids
	normalized := make([]ActionConfig, 0, len(actions))
	for _, action := range actions {
		trimmedType := ActionType(strings.TrimSpace(string(action.Type)))
		if trimmedType == "" {
			continue
		}
		action.Type = trimmedType
		if action.Weight <= 0 {
			action.Weight = 1
		}
		for i := range action.DeviceIDs {
			action.DeviceIDs[i] = strings.TrimSpace(action.DeviceIDs[i])
		}
		normalized = append(normalized, action)
	}
	c.Event.Actions = normalized
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

	if err := c.validateEventConfig(); err != nil {
		return err
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
		if device.DefaultSession != nil {
			if device.DefaultSession.ActivityID <= 0 {
				return fmt.Errorf("device %d (%s) default_session missing activity_id", idx, device.DeviceID)
			}
			if device.DefaultSession.RoomID <= 0 {
				return fmt.Errorf("device %d (%s) default_session missing room_id", idx, device.DeviceID)
			}
			for _, supID := range device.DefaultSession.SupervisorIDs {
				if supID <= 0 {
					return fmt.Errorf("device %d (%s) default_session has invalid supervisor_id %d", idx, device.DeviceID, supID)
				}
			}
		}
	}

	return nil
}

func (c *Config) validateEventConfig() error {
	if c.Event.Interval < minEventInterval {
		return fmt.Errorf("event.interval must be at least %s", minEventInterval)
	}
	if c.Event.MaxEventsPerTick <= 0 {
		return fmt.Errorf("event.max_events_per_tick must be greater than zero")
	}

	if len(c.Event.Rotation.Order) < 2 {
		return fmt.Errorf("event.rotation.order must contain at least two phases")
	}
	// ensure all phases recognised
	for _, phase := range c.Event.Rotation.Order {
		switch phase {
		case RotationPhaseHeimatraum, RotationPhaseAG, RotationPhaseSchulhof:
			// ok
		default:
			return fmt.Errorf("event.rotation.order contains unknown phase %q", phase)
		}
	}

	if c.Event.Rotation.MinAGHops <= 0 {
		return fmt.Errorf("event.rotation.min_ag_hops must be greater than zero")
	}
	if c.Event.Rotation.MaxAGHops < c.Event.Rotation.MinAGHops {
		return fmt.Errorf("event.rotation.max_ag_hops must be >= min_ag_hops")
	}

	if len(c.Event.Actions) == 0 {
		return fmt.Errorf("event.actions must contain at least one action")
	}

	for idx, action := range c.Event.Actions {
		switch action.Type {
		case ActionCheckIn, ActionCheckOut, ActionSchulhofHop, ActionAttendanceToggle, ActionSupervisorSwap:
			// ok
		default:
			return fmt.Errorf("event.actions[%d] has unknown type %q", idx, action.Type)
		}
		if action.Weight <= 0 {
			return fmt.Errorf("event.actions[%d] weight must be positive", idx)
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
