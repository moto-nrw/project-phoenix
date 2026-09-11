package config

import (
	"context"
	"errors"
)

var ErrRuntimeUnavailable = errors.New("settings runtime is not configured")

// Runtime supplies infrastructure operations without coupling settings
// application code to the transaction, ORM, or authorization adapters.
type Runtime interface {
	TenantID(context.Context) int64
	HasTransaction(context.Context) bool
	WithinTenant(context.Context, int64, func(context.Context) error) error
	WithinAdmin(context.Context, func(context.Context) error) error
	AcquireLock(context.Context, string, bool) error
}

// SchoolSettingsStore owns the platform.schools JSONB translation needed by
// the legacy login-image settings. The settings service only sees the value.
type SchoolSettingsStore interface {
	FindSettings(context.Context, int64) (string, error)
	UpdateSettings(context.Context, int64, func(string) (string, error)) error
}
