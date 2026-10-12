package config

import (
	"context"
	"log/slog"
)

// The Resolve*OrDefault helpers accept narrow per-type interfaces instead of
// the full SettingsService so that domain packages holding their own settings
// mini-interfaces (declared to avoid importing services/config in their API
// surface) can call them without widening their fields. SettingsService and
// every such mini-interface satisfy these implicitly.

type boolResolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
}

type intResolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

type stringResolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

type snapshotResolver interface {
	ResolveMany(ctx context.Context, keys []string) (*SettingsSnapshot, error)
}

// ResolveBoolOrDefault returns the tenant-override value for a boolean setting if
// one exists, otherwise the provided fallback default.
//
// This wraps the mandatory HasTenantOverride + ResolveBool pattern documented in
// .claude/rules/settings-system.md. Calling ResolveBool directly would return the
// registry default when no tenant override exists, which hides the "no override"
// vs "explicitly set to default" distinction from callers that want to fall back
// to a different value (e.g. an environment variable).
//
// On any error (DB issue, missing definition) the fallback is returned and the
// error logged, never propagated — the goal is fail-safe defaults for consumers.
func ResolveBoolOrDefault(ctx context.Context, svc boolResolver, key string, fallback bool, logger *slog.Logger) bool {
	if svc == nil {
		return fallback
	}
	if batch, ok := any(svc).(snapshotResolver); ok {
		snapshot, err := batch.ResolveMany(ctx, []string{key})
		if err != nil {
			logSettingsFallback(logger, "settings snapshot failed, using fallback", key, err)
			return fallback
		}
		if snapshot != nil {
			has, err := snapshot.HasOverride(key)
			if err != nil {
				logSettingsFallback(logger, "settings override check failed, using fallback", key, err)
				return fallback
			}
			if !has {
				return fallback
			}
			value, err := snapshot.Bool(key)
			if err != nil {
				logSettingsFallback(logger, "settings resolve failed, using fallback", key, err)
				return fallback
			}
			return value
		}
	}
	has, err := svc.HasTenantOverride(ctx, key)
	if err != nil {
		if logger != nil {
			logger.Warn("settings override check failed, using fallback",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
		return fallback
	}
	if !has {
		return fallback
	}
	val, err := svc.ResolveBool(ctx, key)
	if err != nil {
		if logger != nil {
			logger.Warn("settings resolve failed, using fallback",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
		return fallback
	}
	return val
}

// ResolveIntOrDefault mirrors ResolveBoolOrDefault for integer settings.
func ResolveIntOrDefault(ctx context.Context, svc intResolver, key string, fallback int, logger *slog.Logger) int {
	if svc == nil {
		return fallback
	}
	if batch, ok := any(svc).(snapshotResolver); ok {
		snapshot, err := batch.ResolveMany(ctx, []string{key})
		if err != nil {
			logSettingsFallback(logger, "settings snapshot failed, using fallback", key, err)
			return fallback
		}
		if snapshot != nil {
			has, err := snapshot.HasOverride(key)
			if err != nil {
				logSettingsFallback(logger, "settings override check failed, using fallback", key, err)
				return fallback
			}
			if !has {
				return fallback
			}
			value, err := snapshot.Int(key)
			if err != nil {
				logSettingsFallback(logger, "settings resolve failed, using fallback", key, err)
				return fallback
			}
			return value
		}
	}
	has, err := svc.HasTenantOverride(ctx, key)
	if err != nil {
		if logger != nil {
			logger.Warn("settings override check failed, using fallback",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
		return fallback
	}
	if !has {
		return fallback
	}
	val, err := svc.ResolveInt(ctx, key)
	if err != nil {
		if logger != nil {
			logger.Warn("settings resolve failed, using fallback",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
		return fallback
	}
	return val
}

// ResolveStringOrDefault mirrors ResolveBoolOrDefault for string settings.
// An empty tenant-override value is treated as "no override" and returns the
// fallback — matches the convention used elsewhere in the codebase.
func ResolveStringOrDefault(ctx context.Context, svc stringResolver, key, fallback string, logger *slog.Logger) string {
	if svc == nil {
		return fallback
	}
	if batch, ok := any(svc).(snapshotResolver); ok {
		snapshot, err := batch.ResolveMany(ctx, []string{key})
		if err != nil {
			logSettingsFallback(logger, "settings snapshot failed, using fallback", key, err)
			return fallback
		}
		if snapshot != nil {
			has, err := snapshot.HasOverride(key)
			if err != nil {
				logSettingsFallback(logger, "settings override check failed, using fallback", key, err)
				return fallback
			}
			if !has {
				return fallback
			}
			value, err := snapshot.String(key)
			if err != nil {
				logSettingsFallback(logger, "settings resolve failed, using fallback", key, err)
				return fallback
			}
			if value == "" {
				return fallback
			}
			return value
		}
	}
	has, err := svc.HasTenantOverride(ctx, key)
	if err != nil {
		if logger != nil {
			logger.Warn("settings override check failed, using fallback",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
		return fallback
	}
	if !has {
		return fallback
	}
	val, err := svc.ResolveString(ctx, key)
	if err != nil {
		if logger != nil {
			logger.Warn("settings resolve failed, using fallback",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
		return fallback
	}
	if val == "" {
		return fallback
	}
	return val
}

func logSettingsFallback(logger *slog.Logger, message, key string, err error) {
	if logger == nil {
		return
	}
	logger.Warn(message,
		slog.String("key", key),
		slog.String("error", err.Error()),
	)
}
