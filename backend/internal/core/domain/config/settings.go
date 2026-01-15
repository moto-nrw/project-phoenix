package config

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/uptrace/bun"
)

// tableConfigSettings is the schema-qualified table name for settings
const tableConfigSettings = "config.settings"

// Setting represents a system configuration setting
type Setting struct {
	base.Model      `bun:"schema:config,table:settings"`
	Key             string `bun:"key,notnull,unique" json:"key"`
	Value           string `bun:"value,notnull" json:"value"`
	Category        string `bun:"category,notnull" json:"category"`
	Description     string `bun:"description" json:"description,omitempty"`
	RequiresRestart bool   `bun:"requires_restart,notnull,default:false" json:"requires_restart"`
	RequiresDBReset bool   `bun:"requires_db_reset,notnull,default:false" json:"requires_db_reset"`
}

func (s *Setting) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableConfigSettings)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableConfigSettings)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableConfigSettings)
	}
	return nil
}

// GetID returns the entity's ID
func (s *Setting) GetID() interface{} {
	return s.ID
}

// GetCreatedAt returns the creation timestamp
func (s *Setting) GetCreatedAt() time.Time {
	return s.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (s *Setting) GetUpdatedAt() time.Time {
	return s.UpdatedAt
}

// TableName returns the database table name
func (s *Setting) TableName() string {
	return tableConfigSettings
}

// Validate ensures setting data is valid
func (s *Setting) Validate() error {
	if s.Key == "" {
		return errors.New("key is required")
	}

	if s.Value == "" {
		return errors.New("value is required")
	}

	if s.Category == "" {
		return errors.New("category is required")
	}

	// Normalize key to lowercase and replace spaces with underscores
	s.Key = strings.ToLower(strings.ReplaceAll(s.Key, " ", "_"))

	// Normalize category to lowercase
	s.Category = strings.ToLower(s.Category)

	return nil
}

// IsSystemSetting checks if this is a system-level setting
func (s *Setting) IsSystemSetting() bool {
	return s.Category == "system"
}

// GetBoolValue returns the setting value as a boolean
func (s *Setting) GetBoolValue() bool {
	return strings.ToLower(s.Value) == "true"
}

// GetFullKey returns a combined category and key
func (s *Setting) GetFullKey() string {
	return s.Category + "." + s.Key
}

// Clone creates a copy of the setting
func (s *Setting) Clone() *Setting {
	return &Setting{
		Model: base.Model{
			ID:        s.ID,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
		},
		Key:             s.Key,
		Value:           s.Value,
		Category:        s.Category,
		Description:     s.Description,
		RequiresRestart: s.RequiresRestart,
		RequiresDBReset: s.RequiresDBReset,
	}
}
