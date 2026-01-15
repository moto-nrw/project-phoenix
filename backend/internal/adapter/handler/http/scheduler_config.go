package api

import (
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/core/service/scheduler"
	"github.com/spf13/viper"
)

func buildSchedulerConfig() (scheduler.Config, error) {
	cfg := scheduler.Config{
		CleanupEnabled:                   isEnvTrue("cleanup_scheduler_enabled"),
		CleanupSchedule:                  strings.TrimSpace(viper.GetString("cleanup_scheduler_time")),
		CleanupTimeoutMinutes:            parseSchedulerInt("cleanup_scheduler_timeout_minutes", 30),
		SessionEndEnabled:                !isEnvFalse("session_end_scheduler_enabled"),
		SessionEndSchedule:               strings.TrimSpace(viper.GetString("session_end_time")),
		SessionEndTimeoutMinutes:         parseSchedulerInt("session_end_timeout_minutes", 10),
		SessionCleanupEnabled:            !isEnvFalse("session_cleanup_enabled"),
		SessionCleanupIntervalMinutes:    parseSchedulerInt("session_cleanup_interval_minutes", 15),
		SessionAbandonedThresholdMinutes: parseSchedulerInt("session_abandoned_threshold_minutes", 60),
	}

	if cfg.CleanupSchedule == "" {
		cfg.CleanupSchedule = "02:00"
	}
	if cfg.SessionEndSchedule == "" {
		cfg.SessionEndSchedule = "18:00"
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

func parseSchedulerInt(key string, defaultValue int) int {
	raw := strings.TrimSpace(viper.GetString(key))
	if raw == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return defaultValue
	}

	return parsed
}
