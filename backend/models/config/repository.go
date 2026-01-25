package config

import (
	"context"
)

// SettingRepository defines operations for managing configuration settings
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
	// CRUD operations
	Create(ctx context.Context, def *SettingDefinition) error
	FindByID(ctx context.Context, id int64) (*SettingDefinition, error)
	Update(ctx context.Context, def *SettingDefinition) error
	Delete(ctx context.Context, id int64) error

	// Query operations
	FindByKey(ctx context.Context, key string) (*SettingDefinition, error)
	FindByCategory(ctx context.Context, category string) ([]*SettingDefinition, error)
	FindByScope(ctx context.Context, scopeType ScopeType) ([]*SettingDefinition, error)
	FindByGroup(ctx context.Context, groupName string) ([]*SettingDefinition, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*SettingDefinition, error)
	ListAll(ctx context.Context) ([]*SettingDefinition, error)

	// Bulk operations
	Upsert(ctx context.Context, def *SettingDefinition) error
}

// SettingValueRepository defines operations for managing scoped setting values
type SettingValueRepository interface {
	// CRUD operations
	Create(ctx context.Context, value *SettingValue) error
	FindByID(ctx context.Context, id int64) (*SettingValue, error)
	Update(ctx context.Context, value *SettingValue) error
	Delete(ctx context.Context, id int64) error

	// Query operations
	FindByDefinitionAndScope(ctx context.Context, definitionID int64, scopeType string, scopeID *int64) (*SettingValue, error)
	FindAllForScope(ctx context.Context, scopeType string, scopeID *int64) ([]*SettingValue, error)
	FindByDefinition(ctx context.Context, definitionID int64) ([]*SettingValue, error)

	// Upsert for setting values
	Upsert(ctx context.Context, value *SettingValue) error

	// Cleanup operations
	DeleteByScope(ctx context.Context, scopeType string, scopeID int64) (int, error)
}
