package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// SettingType represents the data type of a setting
type SettingType string

const (
	SettingTypeBool   SettingType = "bool"
	SettingTypeInt    SettingType = "int"
	SettingTypeFloat  SettingType = "float"
	SettingTypeString SettingType = "string"
	SettingTypeEnum   SettingType = "enum"
	SettingTypeTime   SettingType = "time"
	SettingTypeJSON   SettingType = "json"
)

// ScopeType represents the scope level for a setting
type ScopeType string

const (
	ScopeSystem ScopeType = "system"
	ScopeSchool ScopeType = "school"
	ScopeOG     ScopeType = "og"
	ScopeUser   ScopeType = "user"
	ScopeDevice ScopeType = "device"
)

// AllScopeTypes returns all valid scope types
func AllScopeTypes() []ScopeType {
	return []ScopeType{ScopeSystem, ScopeSchool, ScopeOG, ScopeUser, ScopeDevice}
}

// IsValidScopeType checks if a scope type is valid
func IsValidScopeType(s string) bool {
	for _, st := range AllScopeTypes() {
		if string(st) == s {
			return true
		}
	}
	return false
}

// Validation represents validation rules for a setting
type Validation struct {
	Min     *float64 `json:"min,omitempty"`
	Max     *float64 `json:"max,omitempty"`
	Options []string `json:"options,omitempty"`
	Pattern string   `json:"pattern,omitempty"`
}

// SettingDependency represents a dependency on another setting
type SettingDependency struct {
	Key       string `json:"key"`
	Condition string `json:"condition"` // equals, not_equals, in, not_empty, greater_than
	Value     any    `json:"value"`
}

// tableSettingDefinitions is the schema-qualified table name
const tableSettingDefinitions = "config.setting_definitions"

// SettingDefinition represents the schema for a configurable setting
type SettingDefinition struct {
	base.Model       `bun:"schema:config,table:setting_definitions"`
	Key              string             `bun:"key,notnull,unique" json:"key"`
	Type             SettingType        `bun:"type,notnull" json:"type"`
	DefaultValue     json.RawMessage    `bun:"default_value,type:jsonb,notnull" json:"default_value"`
	Category         string             `bun:"category,notnull" json:"category"`
	Description      string             `bun:"description" json:"description,omitempty"`
	Validation       *Validation        `bun:"validation,type:jsonb" json:"validation,omitempty"`
	AllowedScopes    []string           `bun:"allowed_scopes,array" json:"allowed_scopes"`
	ScopePermissions map[string]string  `bun:"scope_permissions,type:jsonb" json:"scope_permissions"`
	DependsOn        *SettingDependency `bun:"depends_on,type:jsonb" json:"depends_on,omitempty"`
	GroupName        string             `bun:"group_name" json:"group_name,omitempty"`
	SortOrder        int                `bun:"sort_order,notnull,default:0" json:"sort_order"`
}

// TableName returns the database table name
func (d *SettingDefinition) TableName() string {
	return tableSettingDefinitions
}

// GetID returns the entity's ID
func (d *SettingDefinition) GetID() interface{} {
	return d.ID
}

// GetCreatedAt returns the creation timestamp
func (d *SettingDefinition) GetCreatedAt() time.Time {
	return d.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (d *SettingDefinition) GetUpdatedAt() time.Time {
	return d.UpdatedAt
}

// Validate ensures the definition is valid
func (d *SettingDefinition) Validate() error {
	if d.Key == "" {
		return errors.New("key is required")
	}

	// Normalize key to lowercase
	d.Key = strings.ToLower(d.Key)

	if d.Type == "" {
		return errors.New("type is required")
	}

	// Validate type
	switch d.Type {
	case SettingTypeBool, SettingTypeInt, SettingTypeFloat,
		SettingTypeString, SettingTypeEnum, SettingTypeTime, SettingTypeJSON:
		// Valid types
	default:
		return fmt.Errorf("invalid setting type: %s", d.Type)
	}

	if len(d.DefaultValue) == 0 {
		return errors.New("default_value is required")
	}

	if d.Category == "" {
		return errors.New("category is required")
	}

	// Normalize category
	d.Category = strings.ToLower(d.Category)

	if len(d.AllowedScopes) == 0 {
		return errors.New("allowed_scopes is required")
	}

	// Validate allowed scopes
	for _, scope := range d.AllowedScopes {
		if !IsValidScopeType(scope) {
			return fmt.Errorf("invalid scope type: %s", scope)
		}
	}

	if len(d.ScopePermissions) == 0 {
		return errors.New("scope_permissions is required")
	}

	// Validate scope permissions match allowed scopes
	for _, scope := range d.AllowedScopes {
		if _, ok := d.ScopePermissions[scope]; !ok {
			return fmt.Errorf("missing permission for scope: %s", scope)
		}
	}

	// Validate enum type has options
	if d.Type == SettingTypeEnum {
		if d.Validation == nil || len(d.Validation.Options) == 0 {
			return errors.New("enum type requires validation.options")
		}
	}

	return nil
}

// IsScopeAllowed checks if a scope type is allowed for this setting
func (d *SettingDefinition) IsScopeAllowed(scopeType ScopeType) bool {
	return slices.Contains(d.AllowedScopes, string(scopeType))
}

// GetPermissionForScope returns the required permission for a scope
func (d *SettingDefinition) GetPermissionForScope(scopeType ScopeType) string {
	return d.ScopePermissions[string(scopeType)]
}

// GetDefaultValueTyped returns the default value parsed to the appropriate Go type
func (d *SettingDefinition) GetDefaultValueTyped() (any, error) {
	return d.parseValue(d.DefaultValue)
}

// parseValue parses a JSONB value to the appropriate Go type
func (d *SettingDefinition) parseValue(data json.RawMessage) (any, error) {
	var wrapper struct {
		Value any `json:"value"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse value: %w", err)
	}

	switch d.Type {
	case SettingTypeBool:
		if v, ok := wrapper.Value.(bool); ok {
			return v, nil
		}
		return false, fmt.Errorf("expected bool, got %T", wrapper.Value)

	case SettingTypeInt:
		// JSON numbers are float64
		if v, ok := wrapper.Value.(float64); ok {
			return int(v), nil
		}
		return 0, fmt.Errorf("expected int, got %T", wrapper.Value)

	case SettingTypeFloat:
		if v, ok := wrapper.Value.(float64); ok {
			return v, nil
		}
		return 0.0, fmt.Errorf("expected float, got %T", wrapper.Value)

	case SettingTypeString, SettingTypeEnum, SettingTypeTime:
		if v, ok := wrapper.Value.(string); ok {
			return v, nil
		}
		return "", fmt.Errorf("expected string, got %T", wrapper.Value)

	case SettingTypeJSON:
		return wrapper.Value, nil

	default:
		return wrapper.Value, nil
	}
}

// ValidateValue validates a value against this definition
func (d *SettingDefinition) ValidateValue(value any) error {
	// Type validation
	switch d.Type {
	case SettingTypeBool:
		if _, ok := value.(bool); !ok {
			return errors.New("value must be a boolean")
		}

	case SettingTypeInt:
		var intVal int
		switch v := value.(type) {
		case int:
			intVal = v
		case int64:
			intVal = int(v)
		case float64:
			intVal = int(v)
		default:
			return errors.New("value must be an integer")
		}
		if d.Validation != nil {
			if d.Validation.Min != nil && float64(intVal) < *d.Validation.Min {
				return fmt.Errorf("value must be at least %.0f", *d.Validation.Min)
			}
			if d.Validation.Max != nil && float64(intVal) > *d.Validation.Max {
				return fmt.Errorf("value must be at most %.0f", *d.Validation.Max)
			}
		}

	case SettingTypeFloat:
		var floatVal float64
		switch v := value.(type) {
		case float64:
			floatVal = v
		case float32:
			floatVal = float64(v)
		case int:
			floatVal = float64(v)
		default:
			return errors.New("value must be a number")
		}
		if d.Validation != nil {
			if d.Validation.Min != nil && floatVal < *d.Validation.Min {
				return fmt.Errorf("value must be at least %f", *d.Validation.Min)
			}
			if d.Validation.Max != nil && floatVal > *d.Validation.Max {
				return fmt.Errorf("value must be at most %f", *d.Validation.Max)
			}
		}

	case SettingTypeString:
		if _, ok := value.(string); !ok {
			return errors.New("value must be a string")
		}

	case SettingTypeEnum:
		strVal, ok := value.(string)
		if !ok {
			return errors.New("value must be a string")
		}
		if d.Validation != nil && len(d.Validation.Options) > 0 {
			if !slices.Contains(d.Validation.Options, strVal) {
				return fmt.Errorf("value must be one of: %s", strings.Join(d.Validation.Options, ", "))
			}
		}

	case SettingTypeTime:
		strVal, ok := value.(string)
		if !ok {
			return errors.New("value must be a time string (HH:MM)")
		}
		// Basic time format validation
		if len(strVal) != 5 || strVal[2] != ':' {
			return errors.New("value must be in HH:MM format")
		}
	}

	return nil
}

// MarshalValue wraps a value in the JSONB format
func MarshalValue(value any) (json.RawMessage, error) {
	wrapper := map[string]any{"value": value}
	return json.Marshal(wrapper)
}
