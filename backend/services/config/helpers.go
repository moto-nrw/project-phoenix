package config

import (
	"context"
	"log/slog"
)

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
func ResolveBoolOrDefault(ctx context.Context, svc SettingsService, key string, fallback bool, logger *slog.Logger) bool {
	if svc == nil {
		return fallback
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
func ResolveIntOrDefault(ctx context.Context, svc SettingsService, key string, fallback int, logger *slog.Logger) int {
	if svc == nil {
		return fallback
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
func ResolveStringOrDefault(ctx context.Context, svc SettingsService, key, fallback string, logger *slog.Logger) string {
	if svc == nil {
		return fallback
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
