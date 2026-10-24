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
	ResolveBool(ctx context.Context, key string) (bool, error)
}

type intResolver interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}

type stringResolver interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

type snapshotResolver interface {
	ResolveMany(ctx context.Context, keys []string) (*settingsRecords.SettingsSnapshot, error)
}

// ResolveBoolOrDefault resolves the tenant override or registry default.
// Only a failed read uses fallback. Callers needing an error (in particular
// privacy and authorization decisions) must use the typed Resolve* methods.
func ResolveBoolOrDefault(ctx context.Context, svc boolResolver, key string, fallback bool, logger *slog.Logger) bool {
	if svc == nil {
		panic("settings resolver is required")
	}
	snapshot, handled := resolvedSnapshot(ctx, svc, key, logger)
	if handled {
		return snapshotResolvedValue(snapshot, key, fallback, logger, (*settingsRecords.SettingsSnapshot).Bool)
	}
	return liveResolvedValue(ctx, key, fallback, logger, svc.ResolveBool)
}

// ResolveIntOrDefault mirrors ResolveBoolOrDefault for integer settings.
func ResolveIntOrDefault(ctx context.Context, svc intResolver, key string, fallback int, logger *slog.Logger) int {
	if svc == nil {
		panic("settings resolver is required")
	}
	snapshot, handled := resolvedSnapshot(ctx, svc, key, logger)
	if handled {
		return snapshotResolvedValue(snapshot, key, fallback, logger, (*settingsRecords.SettingsSnapshot).Int)
	}
	return liveResolvedValue(ctx, key, fallback, logger, svc.ResolveInt)
}

// ResolveStringOrDefault mirrors ResolveBoolOrDefault for string settings.
// An explicit empty value remains authoritative.
func ResolveStringOrDefault(ctx context.Context, svc stringResolver, key string, fallback string, logger *slog.Logger) string {
	if svc == nil {
		panic("settings resolver is required")
	}
	snapshot, handled := resolvedSnapshot(ctx, svc, key, logger)
	if handled {
		return snapshotResolvedValue(snapshot, key, fallback, logger, (*settingsRecords.SettingsSnapshot).String)
	}
	return liveResolvedValue(ctx, key, fallback, logger, svc.ResolveString)
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

func resolvedSnapshot(ctx context.Context, service any, key string, logger *slog.Logger) (*settingsRecords.SettingsSnapshot, bool) {
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

func snapshotResolvedValue[T any](snapshot *settingsRecords.SettingsSnapshot, key string, fallback T, logger *slog.Logger, read func(*settingsRecords.SettingsSnapshot, string) (T, error)) T {
	if snapshot == nil {
		return fallback
	}
	value, err := read(snapshot, key)
	if err != nil {
		logSettingsFallback(logger, "settings resolve failed, using fallback", key, err)
		return fallback
	}
	return value
}

func liveResolvedValue[T any](ctx context.Context, key string, fallback T, logger *slog.Logger, read func(context.Context, string) (T, error)) T {
	value, err := read(ctx, key)
	if err != nil {
		logSettingsFallback(logger, "settings resolve failed, using fallback", key, err)
		return fallback
	}
	return value
}
