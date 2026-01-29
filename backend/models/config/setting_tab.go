package config

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableSettingTabs is the schema-qualified table name
const tableSettingTabs = "config.setting_tabs"

// SettingTab represents a UI tab for organizing settings
type SettingTab struct {
	base.Model `bun:"schema:config,table:setting_tabs"`

	// Key is the unique identifier for the tab
	Key string `bun:"key,notnull" json:"key"`

	// Name is the display name for the tab
	Name string `bun:"name,notnull" json:"name"`

	// Icon is the icon identifier (optional)
	Icon *string `bun:"icon" json:"icon,omitempty"`

	// DisplayOrder controls the tab ordering
	DisplayOrder int `bun:"display_order,notnull,default:0" json:"display_order"`

	// RequiredPermission restricts tab visibility (optional)
	RequiredPermission *string `bun:"required_permission" json:"required_permission,omitempty"`

	// DeletedAt is for soft delete support
	DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"deleted_at,omitempty"`
}

// BeforeAppendModel sets the table name
func (t *SettingTab) BeforeAppendModel(query any) error {
	switch q := query.(type) {
	case *bun.SelectQuery:
		q.ModelTableExpr(tableSettingTabs)
	case *bun.UpdateQuery:
		q.ModelTableExpr(tableSettingTabs)
	case *bun.DeleteQuery:
		q.ModelTableExpr(tableSettingTabs)
	case *bun.InsertQuery:
		q.ModelTableExpr(tableSettingTabs)
	}
	return nil
}

// TableName returns the database table name
func (t *SettingTab) TableName() string {
	return tableSettingTabs
}

// GetID returns the entity ID
func (t *SettingTab) GetID() interface{} {
	return t.ID
}

// GetCreatedAt returns the creation timestamp
func (t *SettingTab) GetCreatedAt() time.Time {
	return t.CreatedAt
}

// GetUpdatedAt returns the update timestamp
func (t *SettingTab) GetUpdatedAt() time.Time {
	return t.UpdatedAt
}

// Validate ensures the tab data is valid
func (t *SettingTab) Validate() error {
	if t.Key == "" {
		return errors.New("key is required")
	}
	if t.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

// IsDeleted returns true if the tab has been soft deleted
func (t *SettingTab) IsDeleted() bool {
	return t.DeletedAt != nil
}
