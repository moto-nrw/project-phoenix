package config

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableSettingValues is the schema-qualified table name
const tableSettingValues = "config.setting_values"

// SettingValue represents a setting value at a specific scope
type SettingValue struct {
	base.Model `bun:"schema:config,table:setting_values"`

	// DefinitionID references the setting definition
	DefinitionID int64 `bun:"definition_id,notnull" json:"definition_id"`

	// ScopeType indicates the scope level (system, user, device)
	ScopeType Scope `bun:"scope_type,notnull" json:"scope_type"`

	// ScopeID identifies the specific entity (NULL for system scope)
	ScopeID *int64 `bun:"scope_id" json:"scope_id,omitempty"`

	// Value is the setting value (may be encrypted for sensitive settings)
	Value string `bun:"value,notnull" json:"value"`

	// DeletedAt is for soft delete support
	DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"deleted_at,omitempty"`

	// Definition is the related setting definition (populated via join)
	Definition *SettingDefinition `bun:"rel:belongs-to,join:definition_id=id" json:"definition,omitempty"`
}

// BeforeAppendModel sets the table name
func (v *SettingValue) BeforeAppendModel(query any) error {
	switch q := query.(type) {
	case *bun.SelectQuery:
		q.ModelTableExpr(tableSettingValues)
	case *bun.UpdateQuery:
		q.ModelTableExpr(tableSettingValues)
	case *bun.DeleteQuery:
		q.ModelTableExpr(tableSettingValues)
	case *bun.InsertQuery:
		q.ModelTableExpr(tableSettingValues)
	}
	return nil
}

// TableName returns the database table name
func (v *SettingValue) TableName() string {
	return tableSettingValues
}

// GetID returns the entity ID
func (v *SettingValue) GetID() interface{} {
	return v.ID
}

// GetCreatedAt returns the creation timestamp
func (v *SettingValue) GetCreatedAt() time.Time {
	return v.CreatedAt
}

// GetUpdatedAt returns the update timestamp
func (v *SettingValue) GetUpdatedAt() time.Time {
	return v.UpdatedAt
}

// Validate ensures the value data is valid
func (v *SettingValue) Validate() error {
	if v.DefinitionID == 0 {
		return errors.New("definition_id is required")
	}
	if !v.ScopeType.IsValid() {
		return errors.New("invalid scope_type")
	}
	if v.ScopeType == ScopeSystem && v.ScopeID != nil {
		return errors.New("scope_id must be NULL for system scope")
	}
	if v.ScopeType != ScopeSystem && v.ScopeID == nil {
		return errors.New("scope_id is required for non-system scope")
	}
	return nil
}

// IsDeleted returns true if the value has been soft deleted
func (v *SettingValue) IsDeleted() bool {
	return v.DeletedAt != nil
}

// Clone creates a copy of the setting value
func (v *SettingValue) Clone() *SettingValue {
	var scopeID *int64
	if v.ScopeID != nil {
		sid := *v.ScopeID
		scopeID = &sid
	}

	return &SettingValue{
		Model: base.Model{
			ID:        v.ID,
			CreatedAt: v.CreatedAt,
			UpdatedAt: v.UpdatedAt,
		},
		DefinitionID: v.DefinitionID,
		ScopeType:    v.ScopeType,
		ScopeID:      scopeID,
		Value:        v.Value,
		DeletedAt:    v.DeletedAt,
	}
}
