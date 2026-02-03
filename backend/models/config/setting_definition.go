package config

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// tableSettingDefinitions is the schema-qualified table name
const tableSettingDefinitions = "config.setting_definitions"

// EnumOption represents a single option for enum-type settings
// with both a value (stored in DB) and a display label (shown in UI)
type EnumOption struct {
	// Value is the actual value stored in the database
	Value string `json:"value"`
	// Label is the human-readable display name
	Label string `json:"label"`
}

// SettingDefinition describes a configurable setting
type SettingDefinition struct {
	base.Model `bun:"schema:config,table:setting_definitions"`

	// Key is the unique identifier for the setting (e.g., "security.password_min_length")
	Key string `bun:"key,notnull" json:"key"`

	// ValueType defines the data type of the setting value
	ValueType ValueType `bun:"value_type,notnull" json:"value_type"`

	// DefaultValue is the default value (serialized as string)
	DefaultValue string `bun:"default_value,notnull" json:"default_value"`

	// Category groups related settings (e.g., "security", "email")
	Category string `bun:"category,notnull" json:"category"`

	// Tab determines which UI tab this setting appears on
	Tab string `bun:"tab,notnull,default:'general'" json:"tab"`

	// DisplayOrder controls the position within a category
	DisplayOrder int `bun:"display_order,notnull,default:0" json:"display_order"`

	// Label is the human-readable name shown in UI
	Label *string `bun:"label" json:"label,omitempty"`

	// Description explains what the setting does
	Description *string `bun:"description" json:"description,omitempty"`

	// AllowedScopes specifies which scope levels can have overrides
	AllowedScopes []string `bun:"allowed_scopes,array,notnull" json:"allowed_scopes"`

	// ViewPermission is required to see this setting (empty means public)
	ViewPermission *string `bun:"view_permission" json:"view_permission,omitempty"`

	// EditPermission is required to modify this setting
	EditPermission *string `bun:"edit_permission" json:"edit_permission,omitempty"`

	// ValidationSchema contains JSON Schema for value validation
	ValidationSchema json.RawMessage `bun:"validation_schema,type:jsonb" json:"validation_schema,omitempty"`

	// EnumValues lists allowed values for enum type settings (simple mode, no labels)
	EnumValues []string `bun:"enum_values,array" json:"enum_values,omitempty"`

	// EnumOptions lists allowed values with display labels for enum type settings
	// Stored as JSONB. If set, takes precedence over EnumValues for display
	EnumOptions []EnumOption `bun:"enum_options,type:jsonb" json:"enum_options,omitempty"`

	// ObjectRefType specifies the entity type for object_ref settings (room, group, staff, etc.)
	ObjectRefType *string `bun:"object_ref_type" json:"object_ref_type,omitempty"`

	// ObjectRefFilter contains SQL-based filters as JSON for object_ref options
	ObjectRefFilter json.RawMessage `bun:"object_ref_filter,type:jsonb" json:"object_ref_filter,omitempty"`

	// RequiresRestart indicates if changing this setting requires a service restart
	RequiresRestart bool `bun:"requires_restart,notnull,default:false" json:"requires_restart"`

	// IsSensitive indicates if the value should be encrypted and write-only
	IsSensitive bool `bun:"is_sensitive,notnull,default:false" json:"is_sensitive"`

	// DeletedAt is for soft delete support
	DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"deleted_at,omitempty"`
}

// BeforeAppendModel sets the table name
func (d *SettingDefinition) BeforeAppendModel(query any) error {
	switch q := query.(type) {
	case *bun.SelectQuery:
		q.ModelTableExpr(tableSettingDefinitions)
	case *bun.UpdateQuery:
		q.ModelTableExpr(tableSettingDefinitions)
	case *bun.DeleteQuery:
		q.ModelTableExpr(tableSettingDefinitions)
	case *bun.InsertQuery:
		q.ModelTableExpr(tableSettingDefinitions)
	}
	return nil
}

// TableName returns the database table name
func (d *SettingDefinition) TableName() string {
	return tableSettingDefinitions
}

// GetID returns the entity ID
func (d *SettingDefinition) GetID() interface{} {
	return d.ID
}

// GetCreatedAt returns the creation timestamp
func (d *SettingDefinition) GetCreatedAt() time.Time {
	return d.CreatedAt
}

// GetUpdatedAt returns the update timestamp
func (d *SettingDefinition) GetUpdatedAt() time.Time {
	return d.UpdatedAt
}

// Validate ensures the definition data is valid
func (d *SettingDefinition) Validate() error {
	if d.Key == "" {
		return errors.New("key is required")
	}
	if !d.ValueType.IsValid() {
		return errors.New("invalid value type")
	}
	if d.Category == "" {
		return errors.New("category is required")
	}
	if d.Tab == "" {
		d.Tab = "general"
	}
	if len(d.AllowedScopes) == 0 {
		d.AllowedScopes = []string{string(ScopeSystem)}
	}
	for _, scope := range d.AllowedScopes {
		if !Scope(scope).IsValid() {
			return errors.New("invalid scope: " + scope)
		}
	}
	if d.ValueType == ValueTypeEnum && len(d.EnumValues) == 0 && len(d.EnumOptions) == 0 {
		return errors.New("enum_values or enum_options required for enum type")
	}
	if d.ValueType == ValueTypeObjectRef && (d.ObjectRefType == nil || *d.ObjectRefType == "") {
		return errors.New("object_ref_type required for object_ref type")
	}
	return nil
}

// IsScopeAllowed checks if the given scope can have overrides
func (d *SettingDefinition) IsScopeAllowed(scope Scope) bool {
	for _, s := range d.AllowedScopes {
		if s == string(scope) {
			return true
		}
	}
	return false
}

// ValidateValue validates a value against this definition
func (d *SettingDefinition) ValidateValue(value string) error {
	if value == "" {
		return nil // Empty is allowed (will use default)
	}

	switch d.ValueType {
	case ValueTypeInt:
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return errors.New("value must be an integer")
		}
	case ValueTypeFloat:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return errors.New("value must be a number")
		}
	case ValueTypeBool:
		if value != "true" && value != "false" {
			return errors.New("value must be 'true' or 'false'")
		}
	case ValueTypeEnum:
		found := false
		// Check EnumOptions first (takes precedence)
		if len(d.EnumOptions) > 0 {
			for _, opt := range d.EnumOptions {
				if opt.Value == value {
					found = true
					break
				}
			}
		} else {
			// Fall back to EnumValues
			for _, ev := range d.EnumValues {
				if ev == value {
					found = true
					break
				}
			}
		}
		if !found {
			return errors.New("value must be one of the allowed options")
		}
	case ValueTypeTime:
		if _, err := time.Parse("15:04:05", value); err != nil {
			if _, err := time.Parse("15:04", value); err != nil {
				return errors.New("value must be a valid time (HH:MM or HH:MM:SS)")
			}
		}
	case ValueTypeDuration:
		if _, err := time.ParseDuration(value); err != nil {
			return errors.New("value must be a valid duration (e.g., 30m, 1h)")
		}
	case ValueTypeObjectRef:
		// Validate that it's a valid int64 (entity ID)
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return errors.New("value must be a valid entity ID (integer)")
		}
	case ValueTypeJSON:
		if !json.Valid([]byte(value)) {
			return errors.New("value must be valid JSON")
		}
	}

	return nil
}

// GetLabelOrKey returns the label if set, otherwise the key
func (d *SettingDefinition) GetLabelOrKey() string {
	if d.Label != nil && *d.Label != "" {
		return *d.Label
	}
	return d.Key
}
