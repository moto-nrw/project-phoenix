package config

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
)

// SettingCRUD handles basic setting CRUD operations
type SettingCRUD interface {
	CreateSetting(ctx context.Context, setting *config.Setting) error
	GetSettingByID(ctx context.Context, id int64) (*config.Setting, error)
	UpdateSetting(ctx context.Context, setting *config.Setting) error
	DeleteSetting(ctx context.Context, id int64) error
	ListSettings(ctx context.Context, filters map[string]interface{}) ([]*config.Setting, error)
}

// SettingValueReader handles typed value retrieval operations
type SettingValueReader interface {
	GetSettingByKey(ctx context.Context, key string) (*config.Setting, error)
	UpdateSettingValue(ctx context.Context, key string, value string) error
	GetStringValue(ctx context.Context, key string, defaultValue string) (string, error)
	GetBoolValue(ctx context.Context, key string, defaultValue bool) (bool, error)
	GetIntValue(ctx context.Context, key string, defaultValue int) (int, error)
	GetFloatValue(ctx context.Context, key string, defaultValue float64) (float64, error)
}

// SettingCategoryOperations handles category-based operations
type SettingCategoryOperations interface {
	GetSettingsByCategory(ctx context.Context, category string) ([]*config.Setting, error)
	GetSettingByKeyAndCategory(ctx context.Context, key string, category string) (*config.Setting, error)
}

// SettingBulkOperations handles bulk import and initialization
type SettingBulkOperations interface {
	ImportSettings(ctx context.Context, settings []*config.Setting) ([]error, error)
	InitializeDefaultSettings(ctx context.Context) error
}

// SystemStateOperations handles system state checks
type SystemStateOperations interface {
	RequiresRestart(ctx context.Context) (bool, error)
	RequiresDatabaseReset(ctx context.Context) (bool, error)
}

// TimeoutOperations handles timeout configuration
type TimeoutOperations interface {
	GetTimeoutSettings(ctx context.Context) (*config.TimeoutSettings, error)
	UpdateTimeoutSettings(ctx context.Context, settings *config.TimeoutSettings) error
	GetDeviceTimeoutSettings(ctx context.Context, deviceID int64) (*config.TimeoutSettings, error)
}

// Service composes all configuration-related operations.
// Existing callers can continue using this full interface.
// New code can depend on smaller sub-interfaces for better decoupling.
type Service interface {
	base.TransactionalService
	SettingCRUD
	SettingValueReader
	SettingCategoryOperations
	SettingBulkOperations
	SystemStateOperations
	TimeoutOperations
}
