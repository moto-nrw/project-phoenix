package config

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
)

// Service defines the configuration service operations
type Service interface {
	base.TransactionalService
	// Core operations
	CreateSetting(ctx context.Context, setting *config.Setting) error
	GetSettingByID(ctx context.Context, id int64) (*config.Setting, error)
	UpdateSetting(ctx context.Context, setting *config.Setting) error
	DeleteSetting(ctx context.Context, id int64) error
	ListSettings(ctx context.Context, filters map[string]interface{}) ([]*config.Setting, error)

	// Key-based operations
	GetSettingByKey(ctx context.Context, key string) (*config.Setting, error)
	UpdateSettingValue(ctx context.Context, key string, value string) error
	GetStringValue(ctx context.Context, key string, defaultValue string) (string, error)
	GetBoolValue(ctx context.Context, key string, defaultValue bool) (bool, error)
	GetIntValue(ctx context.Context, key string, defaultValue int) (int, error)
	GetFloatValue(ctx context.Context, key string, defaultValue float64) (float64, error)

	// Category operations
	GetSettingsByCategory(ctx context.Context, category string) ([]*config.Setting, error)
	GetSettingByKeyAndCategory(ctx context.Context, key string, category string) (*config.Setting, error)

	// Bulk operations
	ImportSettings(ctx context.Context, settings []*config.Setting) ([]error, error)
	InitializeDefaultSettings(ctx context.Context) error

	// System operations
	RequiresRestart(ctx context.Context) (bool, error)
	RequiresDatabaseReset(ctx context.Context) (bool, error)

	// Transaction support
	// WithTx is already defined in base.TransactionalService
}
