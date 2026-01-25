package config

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// tableSettingValues is the schema-qualified table name
const tableSettingValues = "config.setting_values"

// SettingValue represents a scoped setting value
type SettingValue struct {
	base.Model   `bun:"schema:config,table:setting_values"`
	DefinitionID int64           `bun:"definition_id,notnull" json:"definition_id"`
	ScopeType    string          `bun:"scope_type,notnull" json:"scope_type"`
	ScopeID      *int64          `bun:"scope_id" json:"scope_id,omitempty"`
	Value        json.RawMessage `bun:"value,type:jsonb,notnull" json:"value"`
	SetBy        *int64          `bun:"set_by" json:"set_by,omitempty"`

	// Relations (optional, for eager loading)
	Definition *SettingDefinition `bun:"rel:belongs-to,join:definition_id=id" json:"definition,omitempty"`
}

// TableName returns the database table name
func (v *SettingValue) TableName() string {
	return tableSettingValues
}

// GetID returns the entity's ID
func (v *SettingValue) GetID() interface{} {
	return v.ID
}

// GetCreatedAt returns the creation timestamp
func (v *SettingValue) GetCreatedAt() time.Time {
	return v.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (v *SettingValue) GetUpdatedAt() time.Time {
	return v.UpdatedAt
}

// Validate ensures the setting value is valid
func (v *SettingValue) Validate() error {
	if v.DefinitionID <= 0 {
		return errors.New("definition_id is required")
	}

	if v.ScopeType == "" {
		return errors.New("scope_type is required")
	}

	if !IsValidScopeType(v.ScopeType) {
		return errors.New("invalid scope_type")
	}

	// System scope should have nil scope_id
	if v.ScopeType == string(ScopeSystem) && v.ScopeID != nil {
		return errors.New("system scope must not have scope_id")
	}

	// Non-system scopes require scope_id
	if v.ScopeType != string(ScopeSystem) && (v.ScopeID == nil || *v.ScopeID <= 0) {
		return errors.New("non-system scope requires scope_id")
	}

	if len(v.Value) == 0 {
		return errors.New("value is required")
	}

	return nil
}

// GetTypedValue returns the value parsed to the appropriate Go type
func (v *SettingValue) GetTypedValue(def *SettingDefinition) (any, error) {
	return def.parseValue(v.Value)
}

// ScopeRef represents a reference to a specific scope
type ScopeRef struct {
	Type ScopeType
	ID   *int64
}

// NewSystemScope creates a scope reference for system-level settings
func NewSystemScope() ScopeRef {
	return ScopeRef{Type: ScopeSystem, ID: nil}
}

// NewOGScope creates a scope reference for OG-level settings
func NewOGScope(ogID int64) ScopeRef {
	return ScopeRef{Type: ScopeOG, ID: &ogID}
}

// NewUserScope creates a scope reference for user-level settings
func NewUserScope(personID int64) ScopeRef {
	return ScopeRef{Type: ScopeUser, ID: &personID}
}

// String returns a string representation of the scope
func (s ScopeRef) String() string {
	if s.ID == nil {
		return string(s.Type)
	}
	return string(s.Type) + ":" + string(rune(*s.ID))
}

// ResolvedSetting represents a setting value with resolution metadata
type ResolvedSetting struct {
	Key         string             `json:"key"`
	Value       any                `json:"value"`
	Type        SettingType        `json:"type"`
	Category    string             `json:"category"`
	Description string             `json:"description,omitempty"`
	GroupName   string             `json:"group_name,omitempty"`
	Source      *ScopeRef          `json:"source,omitempty"`
	IsDefault   bool               `json:"is_default"`
	IsActive    bool               `json:"is_active"`
	CanModify   bool               `json:"can_modify"`
	DependsOn   *SettingDependency `json:"depends_on,omitempty"`
	Validation  *Validation        `json:"validation,omitempty"`
	LastChanged *SettingChangeInfo `json:"last_changed,omitempty"`
}

// SettingChangeInfo contains metadata about the last change
type SettingChangeInfo struct {
	At        time.Time `json:"at"`
	By        string    `json:"by,omitempty"`
	ByID      *int64    `json:"by_id,omitempty"`
	FromValue any       `json:"from,omitempty"`
	ToValue   any       `json:"to,omitempty"`
}
