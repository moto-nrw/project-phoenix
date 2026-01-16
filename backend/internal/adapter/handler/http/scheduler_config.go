package api

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/core/service/scheduler"
	"github.com/spf13/viper"
)

func buildSchedulerConfig() (scheduler.Config, error) {
	cfg := scheduler.Config{
		CleanupEnabled:        isEnvTrue("cleanup_scheduler_enabled"),
		SessionEndEnabled:     !isEnvFalse("session_end_scheduler_enabled"),
		SessionCleanupEnabled: !isEnvFalse("session_cleanup_enabled"),
	}

	var err error
	if cfg.CleanupEnabled {
		cfg.CleanupSchedule, err = requireViperString("cleanup_scheduler_time")
		if err != nil {
			return cfg, err
		}
		cfg.CleanupTimeoutMinutes, err = requirePositiveInt("cleanup_scheduler_timeout_minutes")
		if err != nil {
			return cfg, err
		}
	}

	if cfg.SessionEndEnabled {
		cfg.SessionEndSchedule, err = requireViperString("session_end_time")
		if err != nil {
			return cfg, err
		}
		cfg.SessionEndTimeoutMinutes, err = requirePositiveInt("session_end_timeout_minutes")
		if err != nil {
			return cfg, err
		}
	}

	if cfg.SessionCleanupEnabled {
		cfg.SessionCleanupIntervalMinutes, err = requirePositiveInt("session_cleanup_interval_minutes")
		if err != nil {
			return cfg, err
		}
		cfg.SessionAbandonedThresholdMinutes, err = requirePositiveInt("session_abandoned_threshold_minutes")
		if err != nil {
			return cfg, err
		}
	}

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func isEnvTrue(key string) bool {
	return strings.EqualFold(strings.TrimSpace(viper.GetString(key)), "true")
}

func isEnvFalse(key string) bool {
	return strings.EqualFold(strings.TrimSpace(viper.GetString(key)), "false")
}

func requireViperString(key string) (string, error) {
	value := strings.TrimSpace(viper.GetString(key))
	if value == "" {
		return "", fmt.Errorf("%s environment variable is required", strings.ToUpper(key))
	}
	return value, nil
}

func requirePositiveInt(key string) (int, error) {
	raw := strings.TrimSpace(viper.GetString(key))
	if raw == "" {
		return 0, fmt.Errorf("%s environment variable is required", strings.ToUpper(key))
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", strings.ToUpper(key))
	}

	return parsed, nil
}
