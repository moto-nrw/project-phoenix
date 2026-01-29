package config

import (
	"context"
)

// SettingRepository defines operations for managing configuration settings (legacy flat model)
type SettingRepository interface {
	Create(ctx context.Context, setting *Setting) error
	FindByID(ctx context.Context, id interface{}) (*Setting, error)
	Update(ctx context.Context, setting *Setting) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*Setting, error)
	FindByKey(ctx context.Context, key string) (*Setting, error)
	FindByCategory(ctx context.Context, category string) ([]*Setting, error)
	FindByKeyAndCategory(ctx context.Context, key string, category string) (*Setting, error)
	UpdateValue(ctx context.Context, key string, value string) error
	GetValue(ctx context.Context, key string) (string, error)
	GetBoolValue(ctx context.Context, key string) (bool, error)
	GetFullKey(ctx context.Context, category string, key string) (string, error)
}

// SettingDefinitionRepository defines operations for managing setting definitions
type SettingDefinitionRepository interface {
	// Create inserts a new setting definition
	Create(ctx context.Context, def *SettingDefinition) error

	// Update modifies an existing setting definition
	Update(ctx context.Context, def *SettingDefinition) error

	// FindByID retrieves a definition by ID (excludes soft-deleted)
	FindByID(ctx context.Context, id int64) (*SettingDefinition, error)

	// FindByKey retrieves a definition by key (excludes soft-deleted)
	FindByKey(ctx context.Context, key string) (*SettingDefinition, error)

	// FindByKeys retrieves definitions by keys (excludes soft-deleted)
	FindByKeys(ctx context.Context, keys []string) ([]*SettingDefinition, error)

	// FindByTab retrieves definitions for a specific tab
	FindByTab(ctx context.Context, tab string) ([]*SettingDefinition, error)

	// FindByCategory retrieves definitions for a specific category
	FindByCategory(ctx context.Context, category string) ([]*SettingDefinition, error)

	// FindAll retrieves all active definitions
	FindAll(ctx context.Context) ([]*SettingDefinition, error)

	// SoftDelete marks a definition as deleted
	SoftDelete(ctx context.Context, id int64) error

	// Restore unmarks a soft-deleted definition
	Restore(ctx context.Context, id int64) error

	// PurgeDeletedOlderThan permanently removes definitions deleted before the given days
	PurgeDeletedOlderThan(ctx context.Context, days int) (int64, error)

	// Upsert creates or updates a definition by key
	Upsert(ctx context.Context, def *SettingDefinition) error
}

// SettingValueRepository defines operations for managing scoped setting values
type SettingValueRepository interface {
	// Create inserts a new setting value
	Create(ctx context.Context, value *SettingValue) error

	// Update modifies an existing setting value
	Update(ctx context.Context, value *SettingValue) error

	// FindByID retrieves a value by ID (excludes soft-deleted)
	FindByID(ctx context.Context, id int64) (*SettingValue, error)

	// FindByDefinitionAndScope retrieves a value for a specific definition and scope
	FindByDefinitionAndScope(ctx context.Context, defID int64, scopeType Scope, scopeID *int64) (*SettingValue, error)

	// FindByDefinitionID retrieves all values for a definition (excludes soft-deleted)
	FindByDefinitionID(ctx context.Context, defID int64) ([]*SettingValue, error)

	// FindEffectiveValue returns the value at the highest-priority scope
	// Returns the value, the scope it came from, and any error
	FindEffectiveValue(ctx context.Context, defID int64, scopeCtx *ScopeContext) (*SettingValue, Scope, error)

	// FindByScopeType retrieves all values for a scope type
	FindByScopeType(ctx context.Context, scopeType Scope) ([]*SettingValue, error)

	// FindByScopeEntity retrieves all values for a specific scope entity
	FindByScopeEntity(ctx context.Context, scopeType Scope, scopeID int64) ([]*SettingValue, error)

	// SoftDelete marks a value as deleted
	SoftDelete(ctx context.Context, id int64) error

	// SoftDeleteByScope soft deletes a value by definition and scope
	SoftDeleteByScope(ctx context.Context, defID int64, scopeType Scope, scopeID *int64) error

	// Restore unmarks a soft-deleted value
	Restore(ctx context.Context, id int64) error

	// PurgeDeletedOlderThan permanently removes values deleted before the given days
	PurgeDeletedOlderThan(ctx context.Context, days int) (int64, error)

	// Upsert creates or updates a value by definition and scope
	Upsert(ctx context.Context, value *SettingValue) error

	// DeleteByScopeEntity deletes all values for a scope entity (used when entity is deleted)
	DeleteByScopeEntity(ctx context.Context, scopeType Scope, scopeID int64) error
}

// SettingAuditRepository defines operations for managing setting audit logs
type SettingAuditRepository interface {
	// Create inserts a new audit entry
	Create(ctx context.Context, entry *SettingAuditEntry) error

	// FindByDefinitionID retrieves audit entries for a definition
	FindByDefinitionID(ctx context.Context, defID int64, limit int) ([]*SettingAuditEntry, error)

	// FindBySettingKey retrieves audit entries for a setting key
	FindBySettingKey(ctx context.Context, key string, limit int) ([]*SettingAuditEntry, error)

	// FindByScope retrieves audit entries for a specific scope
	FindByScope(ctx context.Context, scopeType Scope, scopeID *int64, limit int) ([]*SettingAuditEntry, error)

	// FindByAccountID retrieves audit entries by who made the change
	FindByAccountID(ctx context.Context, accountID int64, limit int) ([]*SettingAuditEntry, error)

	// FindRecent retrieves the most recent audit entries
	FindRecent(ctx context.Context, limit int) ([]*SettingAuditEntry, error)

	// CountByDefinitionID returns the count of audit entries for a definition
	CountByDefinitionID(ctx context.Context, defID int64) (int64, error)
}

// SettingTabRepository defines operations for managing setting tabs
type SettingTabRepository interface {
	// Create inserts a new tab
	Create(ctx context.Context, tab *SettingTab) error

	// Update modifies an existing tab
	Update(ctx context.Context, tab *SettingTab) error

	// FindByID retrieves a tab by ID (excludes soft-deleted)
	FindByID(ctx context.Context, id int64) (*SettingTab, error)

	// FindByKey retrieves a tab by key (excludes soft-deleted)
	FindByKey(ctx context.Context, key string) (*SettingTab, error)

	// FindAll retrieves all active tabs ordered by display_order
	FindAll(ctx context.Context) ([]*SettingTab, error)

	// SoftDelete marks a tab as deleted
	SoftDelete(ctx context.Context, id int64) error

	// Restore unmarks a soft-deleted tab
	Restore(ctx context.Context, id int64) error

	// Upsert creates or updates a tab by key
	Upsert(ctx context.Context, tab *SettingTab) error
}
