package config

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const tableSettingValues = "config.setting_values"

// SettingValue stores a single tenant-scoped setting override.
// If no override exists for a tenant, the registry default is used.
type SettingValue struct {
	base.Model `bun:"schema:config,table:setting_values"`
	base.TenantModel

	SettingKey string          `bun:"setting_key,notnull" json:"setting_key"`
	Value      json.RawMessage `bun:"value,type:jsonb,notnull" json:"value"`
	UpdatedBy  *int64          `bun:"updated_by" json:"updated_by,omitempty"`
}

// BeforeAppendModel sets the table expression for update/delete queries.
func (sv *SettingValue) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableSettingValues)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableSettingValues)
	}
	return nil
}

// GetID returns the entity's ID.
func (sv *SettingValue) GetID() interface{} { return sv.ID }

// GetCreatedAt returns the creation timestamp.
func (sv *SettingValue) GetCreatedAt() time.Time { return sv.CreatedAt }

// GetUpdatedAt returns the last update timestamp.
func (sv *SettingValue) GetUpdatedAt() time.Time { return sv.UpdatedAt }

// TableName returns the database table name.
func (sv *SettingValue) TableName() string { return tableSettingValues }

// Validate checks that all required fields are set.
func (sv *SettingValue) Validate() error {
	if sv.SettingKey == "" {
		return errors.New("setting_key is required")
	}
	if len(sv.Value) == 0 {
		return errors.New("value is required")
	}
	return nil
}
