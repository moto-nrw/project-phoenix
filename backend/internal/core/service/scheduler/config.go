package scheduler

import (
	"fmt"
	"strings"
)

// Config defines scheduler behavior and timing.
// Values should be provided by adapters (e.g., env parsing in HTTP server).
type Config struct {
	CleanupEnabled                   bool
	CleanupSchedule                  string
	CleanupTimeoutMinutes            int
	SessionEndEnabled                bool
	SessionEndSchedule               string
	SessionEndTimeoutMinutes         int
	SessionCleanupEnabled            bool
	SessionCleanupIntervalMinutes    int
	SessionAbandonedThresholdMinutes int
}

// Validate ensures configuration is internally consistent.
func (c Config) Validate() error {
	if c.CleanupEnabled {
		if strings.TrimSpace(c.CleanupSchedule) == "" {
			return fmt.Errorf("cleanup schedule is required when cleanup is enabled")
		}
		if _, _, err := parseScheduledTime(c.CleanupSchedule); err != nil {
			return fmt.Errorf("cleanup schedule invalid: %w", err)
		}
		if c.CleanupTimeoutMinutes <= 0 {
			return fmt.Errorf("cleanup timeout must be greater than zero")
		}
	}

	if c.SessionEndEnabled {
		if strings.TrimSpace(c.SessionEndSchedule) == "" {
			return fmt.Errorf("session end schedule is required when session end is enabled")
		}
		if _, _, err := parseScheduledTime(c.SessionEndSchedule); err != nil {
			return fmt.Errorf("session end schedule invalid: %w", err)
		}
		if c.SessionEndTimeoutMinutes <= 0 {
			return fmt.Errorf("session end timeout must be greater than zero")
		}
	}

	if c.SessionCleanupEnabled {
		if c.SessionCleanupIntervalMinutes <= 0 {
			return fmt.Errorf("session cleanup interval must be greater than zero")
		}
		if c.SessionAbandonedThresholdMinutes <= 0 {
			return fmt.Errorf("session abandoned threshold must be greater than zero")
		}
	}

	return nil
}
