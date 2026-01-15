package config

import (
	"context"

	domain "github.com/moto-nrw/project-phoenix/internal/core/domain/config"
)

type Setting = domain.Setting

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
