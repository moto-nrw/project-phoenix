package config

import (
	"context"
)

// SettingValueRepository defines data access operations for tenant-scoped setting overrides.
type SettingValueRepository interface {
	// FindByTenantAndKey retrieves a single value for a tenant and key.
	// Returns (nil, nil) if not found.
	FindByTenantAndKey(ctx context.Context, tenantID int64, key string) (*SettingValue, error)

	// FindByTenantAndKeys retrieves the stored overrides for the named keys in
	// one query. Keys without an override are absent from the result.
	FindByTenantAndKeys(ctx context.Context, tenantID int64, keys []string) ([]*SettingValue, error)

	// FindByTenantsAndKeys retrieves stored overrides for several tenants and
	// keys in one query. Callers must provide an admin-scoped transaction
	// because normal tenant RLS must never expose rows from another school.
	FindByTenantsAndKeys(ctx context.Context, tenantIDs []int64, keys []string) ([]*SettingValue, error)

	// Upsert inserts or updates a setting value (INSERT ... ON CONFLICT ... DO UPDATE).
	Upsert(ctx context.Context, sv *SettingValue) error

	// Delete removes a single setting value for a tenant and key.
	Delete(ctx context.Context, tenantID int64, key string) error
}

// SettingAuditRepository defines data access operations for the setting audit trail.
type SettingAuditRepository interface {
	// Create appends a new audit entry.
	Create(ctx context.Context, entry *SettingAuditEntry) error
}
