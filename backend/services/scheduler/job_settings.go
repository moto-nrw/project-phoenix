package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// resolveStringSetting resolves a setting via the settings service with env var fallback.
func (s *Scheduler) resolveStringSetting(ctx context.Context, key string, envVar string, defaultVal string) string {
	fallback := defaultVal
	if val := s.env(envVar); val != "" {
		fallback = val
	}
	return resolveStringOrDefault(ctx, s.settings, key, fallback, s.getLogger())
}

// resolveBoolSetting resolves a boolean setting via the settings service with env var fallback.
func (s *Scheduler) resolveBoolSetting(ctx context.Context, key string, envVar string, defaultVal bool) bool {
	fallback := defaultVal
	if val := s.env(envVar); val != "" {
		fallback = val == "true"
	}
	return resolveBoolOrDefault(ctx, s.settings, key, fallback, s.getLogger())
}

// resolveIntSetting resolves an integer setting via the settings service with env var fallback.
func (s *Scheduler) resolveIntSetting(ctx context.Context, key string, envVar string, defaultVal int) int {
	fallback := defaultVal
	if val := s.env(envVar); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			fallback = parsed
		}
	}
	if val := resolveIntOrDefault(ctx, s.settings, key, fallback, s.getLogger()); val > 0 {
		return val
	}
	return fallback
}

func (s *Scheduler) resolveRequiredPositiveIntSetting(ctx context.Context, key string, envVar string) (int, error) {
	if s.settings == nil {
		return 0, fmt.Errorf("settings resolver not configured")
	}
	hasOverride, err := s.settings.HasTenantOverride(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("check override for %s: %w", key, err)
	}
	if !hasOverride {
		if val := s.env(envVar); val != "" {
			parsed, err := strconv.Atoi(val)
			if err != nil || parsed <= 0 {
				return 0, fmt.Errorf("environment variable %s must be positive integer, got %q", envVar, val)
			}
			return parsed, nil
		}
	}
	val, err := s.settings.ResolveInt(ctx, key)
	if err != nil {
		return 0, err
	}
	if val <= 0 {
		return 0, fmt.Errorf("setting %s must be positive, got %d", key, val)
	}
	return val, nil
}

// resolveNonNegativeIntSetting is resolveIntSetting for settings where zero is
// a meaningful value (e.g. tracking.auto_checkout_grace_minutes = 0 means
// checkout exactly at the planned shift end).
func (s *Scheduler) resolveNonNegativeIntSetting(ctx context.Context, key string, envVar string, defaultVal int) int {
	fallback := defaultVal
	if val := s.env(envVar); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed >= 0 {
			fallback = parsed
		}
	}
	if val := resolveIntOrDefault(ctx, s.settings, key, fallback, s.getLogger()); val >= 0 {
		return val
	}
	return fallback
}

// timeMatchesNow checks if an HH:MM time string matches the current minute.
func timeMatchesNow(timeStr string) bool {
	return timeMatchesAt(timeStr, time.Now())
}

// timeMatchesAt checks if an HH:MM time string matches the minute of now.
func timeMatchesAt(timeStr string, now time.Time) bool {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return false
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return false
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return false
	}
	return now.Hour() == hour && now.Minute() == minute
}

// wasRunToday checks if a per-tenant job already ran today.
func wasRunToday(lastRunMap *sync.Map, tenantID int64) bool {
	return wasRunAt(lastRunMap, tenantID, time.Now())
}

func wasRunAt(lastRunMap *sync.Map, tenantID int64, now time.Time) bool {
	val, ok := lastRunMap.Load(tenantID)
	if !ok {
		return false
	}
	lastRun, ok := val.(time.Time)
	if !ok {
		return false
	}
	return lastRun.Year() == now.Year() && lastRun.YearDay() == now.YearDay()
}

func (s *Scheduler) markRunTodayAfterCommit(ctx context.Context, lastRunMap *sync.Map, tenantID int64) {
	s.markRunAtAfterCommit(ctx, lastRunMap, tenantID, time.Now())
}

func markRunAt(lastRunMap *sync.Map, tenantID int64, now time.Time) {
	lastRunMap.Store(tenantID, now)
}

func (s *Scheduler) markRunAtAfterCommit(ctx context.Context, lastRunMap *sync.Map, tenantID int64, now time.Time) {
	s.afterCommit(ctx, func() {
		markRunAt(lastRunMap, tenantID, now)
	})
}

// schedulerTime supplies the optional injected clock with the production default.
func schedulerTime(now func() time.Time) time.Time {
	if now == nil {
		return time.Now()
	}
	return now()
}
