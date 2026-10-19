package config

import (
	"context"
	"log/slog"

	settingsRecords "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// The Resolve*OrDefault helpers accept narrow per-type interfaces instead of
// the full settingsRecords.SettingsService so that domain packages holding their own settings
// mini-interfaces (declared to avoid importing services/config in their API
// surface) can call them without widening their fields. settingsRecords.SettingsService and
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
	ResolveMany(ctx context.Context, keys []string) (*settingsRecords.SettingsSnapshot, error)
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
	snapshot, handled := overrideSnapshot(ctx, svc, key, logger)
	if handled {
		return snapshotOverrideValue(snapshot, key, fallback, logger, (*settingsRecords.SettingsSnapshot).Bool, nil)
	}
	return liveOverrideValue(ctx, svc, key, fallback, logger, svc.ResolveBool, nil)
}

// ResolveIntOrDefault mirrors ResolveBoolOrDefault for integer settings.
func ResolveIntOrDefault(ctx context.Context, svc intResolver, key string, fallback int, logger *slog.Logger) int {
	if svc == nil {
		return fallback
	}
	snapshot, handled := overrideSnapshot(ctx, svc, key, logger)
	if handled {
		return snapshotOverrideValue(snapshot, key, fallback, logger, (*settingsRecords.SettingsSnapshot).Int, nil)
	}
	return liveOverrideValue(ctx, svc, key, fallback, logger, svc.ResolveInt, nil)
}

// ResolveStringOrDefault mirrors ResolveBoolOrDefault for string settings.
// An empty tenant-override value is treated as "no override" and returns the
// fallback — matches the convention used elsewhere in the codebase.
func ResolveStringOrDefault(ctx context.Context, svc stringResolver, key string, fallback string, logger *slog.Logger) string {
	if svc == nil {
		return fallback
	}
	snapshot, handled := overrideSnapshot(ctx, svc, key, logger)
	if handled {
		return snapshotOverrideValue(snapshot, key, fallback, logger, (*settingsRecords.SettingsSnapshot).String, func(value string) bool { return value == "" })
	}
	return liveOverrideValue(ctx, svc, key, fallback, logger, svc.ResolveString, func(value string) bool { return value == "" })
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

type overrideResolver interface {
	HasTenantOverride(context.Context, string) (bool, error)
}

func overrideSnapshot(ctx context.Context, service any, key string, logger *slog.Logger) (*settingsRecords.SettingsSnapshot, bool) {
	batch, ok := service.(snapshotResolver)
	if !ok {
		return nil, false
	}
	snapshot, err := batch.ResolveMany(ctx, []string{key})
	if err != nil {
		logSettingsFallback(logger, "settings snapshot failed, using fallback", key, err)
		return nil, true
	}
	return snapshot, snapshot != nil
}

func snapshotOverrideValue[T any](snapshot *settingsRecords.SettingsSnapshot, key string, fallback T, logger *slog.Logger, read func(*settingsRecords.SettingsSnapshot, string) (T, error), empty func(T) bool) T {
	if snapshot == nil {
		return fallback
	}
	has, err := snapshot.HasOverride(key)
	if err != nil {
		logSettingsFallback(logger, "settings override check failed, using fallback", key, err)
		return fallback
	}
	if !has {
		return fallback
	}
	value, err := read(snapshot, key)
	if err != nil {
		logSettingsFallback(logger, "settings resolve failed, using fallback", key, err)
		return fallback
	}
	if empty != nil && empty(value) {
		return fallback
	}
	return value
}

func liveOverrideValue[T any](ctx context.Context, service overrideResolver, key string, fallback T, logger *slog.Logger, read func(context.Context, string) (T, error), empty func(T) bool) T {
	has, err := service.HasTenantOverride(ctx, key)
	if err != nil {
		logSettingsFallback(logger, "settings override check failed, using fallback", key, err)
		return fallback
	}
	if !has {
		return fallback
	}
	value, err := read(ctx, key)
	if err != nil {
		logSettingsFallback(logger, "settings resolve failed, using fallback", key, err)
		return fallback
	}
	if empty != nil && empty(value) {
		return fallback
	}
	return value
}
