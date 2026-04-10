package config

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// SettingsService defines operations for the schema-driven settings system.
// All settings are tenant-scoped with a registry default fallback.
type SettingsService interface {
	// GetSchema returns the full settings schema with resolved values for the
	// current tenant, filtered by the user's permissions.
	GetSchema(ctx context.Context, userPermissions []string) (*SettingsSchema, error)

	// Resolve returns the raw value for a setting key (tenant override or registry default).
	Resolve(ctx context.Context, key string) (any, error)

	// ResolveString resolves a setting as a string.
	ResolveString(ctx context.Context, key string) (string, error)

	// ResolveStringForTenant resolves a setting as a string for a specific tenant,
	// handling its own DB transaction context. Used by device auth which runs
	// before the tenant middleware sets PostgreSQL-level tenant context.
	ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error)

	// ResolveBool resolves a setting as a bool.
	ResolveBool(ctx context.Context, key string) (bool, error)

	// ResolveInt resolves a setting as an int.
	ResolveInt(ctx context.Context, key string) (int, error)

	// HasTenantOverride checks if a tenant has an explicit DB override for a setting.
	HasTenantOverride(ctx context.Context, key string) (bool, error)

	// SetValue sets a tenant override for a setting.
	// userPermissions is checked against the definition's WritePermission.
	// Pass nil to skip permission checks (for system-level callers).
	SetValue(ctx context.Context, key string, value any, changedBy *int64, userPermissions []string) error

	// ResetValue removes a tenant override, falling back to the registry default.
	// userPermissions is checked against the definition's WritePermission.
	// Pass nil to skip permission checks (for system-level callers).
	ResetValue(ctx context.Context, key string, changedBy *int64, userPermissions []string) error

	// GetLoginImageURL returns the loginImageUrl from the school's settings JSONB.
	// Returns empty string if no login image is set.
	GetLoginImageURL(ctx context.Context, tenantID int64) (string, error)

	// SetLoginImageURL sets the loginImageUrl in the school's settings JSONB.
	// Uses WithAdminTx because platform.schools requires the phoenix_admin role.
	// Returns the previous loginImageUrl (if any) so the caller can clean up the old file.
	SetLoginImageURL(ctx context.Context, tenantID int64, imageURL string) (oldURL string, err error)

	// ClearLoginImageURL removes the loginImageUrl from the school's settings JSONB.
	// Uses WithAdminTx because platform.schools requires the phoenix_admin role.
	// Returns the previous loginImageUrl (if any) so the caller can clean up the old file.
	ClearLoginImageURL(ctx context.Context, tenantID int64) (oldURL string, err error)
}

// --- Response types ---

// SettingsSchema is the top-level response for the schema endpoint.
type SettingsSchema struct {
	Tabs []*SchemaTab `json:"tabs"`
}

// SchemaTab groups categories under a named tab.
type SchemaTab struct {
	Key        string            `json:"key"`
	Label      string            `json:"label"`
	Categories []*SchemaCategory `json:"categories"`
}

// SchemaCategory groups settings under a named category.
type SchemaCategory struct {
	Key   string             `json:"key"`
	Label string             `json:"label"`
	Items []*ResolvedSetting `json:"items"`
}

// ResolvedSetting is a setting definition enriched with its resolved value.
type ResolvedSetting struct {
	Key         string           `json:"key"`
	Label       string           `json:"label"`
	Description string           `json:"description"`
	Type        config.FieldType `json:"type"`
	Default     any              `json:"default"`
	Value       any              `json:"value"`
	IsDefault   bool             `json:"is_default"`
	Writable    bool             `json:"writable"`
	Visible     bool             `json:"visible"`
	SortOrder   int              `json:"sort_order"`

	Validation *config.ValidationRules `json:"validation,omitempty"`
	DependsOn  *config.Dependency      `json:"depends_on,omitempty"`
	Options    *config.SelectOptions   `json:"options,omitempty"`
}
