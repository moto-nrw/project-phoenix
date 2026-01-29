package settings

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// Definition describes a configurable setting.
// Use this struct to register settings in code.
type Definition struct {
	// Key is the unique identifier (e.g., "security.password_min_length")
	Key string

	// Type defines the value type
	Type config.ValueType

	// Default is the default value (serialized as string)
	Default string

	// Category groups related settings
	Category string

	// Tab determines which UI tab this setting appears on
	Tab string

	// DisplayOrder controls position within a category
	DisplayOrder int

	// Label is the human-readable name
	Label string

	// Description explains what the setting does
	Description string

	// Scopes specifies which scope levels can have overrides
	Scopes []config.Scope

	// ViewPerm is required to see this setting
	ViewPerm string

	// EditPerm is required to modify this setting
	EditPerm string

	// Validation contains validation rules
	Validation *ValidationSchema

	// EnumValues lists allowed values for enum type
	EnumValues []string

	// ObjectRefType specifies the entity type for object_ref settings
	ObjectRefType string

	// ObjectRefFilter contains SQL-based filters for object_ref options
	ObjectRefFilter map[string]interface{}

	// RequiresRestart indicates if changing requires a restart
	RequiresRestart bool

	// IsSensitive indicates if the value should be encrypted
	IsSensitive bool
}

// ValidationSchema defines validation rules for a setting
type ValidationSchema struct {
	Min       *int64  `json:"min,omitempty"`
	Max       *int64  `json:"max,omitempty"`
	MinLength *int    `json:"min_length,omitempty"`
	MaxLength *int    `json:"max_length,omitempty"`
	Pattern   *string `json:"pattern,omitempty"`
	Required  bool    `json:"required,omitempty"`
}

// Validate ensures the definition is valid
func (d *Definition) Validate() error {
	if d.Key == "" {
		return errors.New("key is required")
	}
	if !d.Type.IsValid() {
		return errors.New("invalid type")
	}
	if d.Category == "" {
		return errors.New("category is required")
	}
	if d.Tab == "" {
		d.Tab = "general"
	}
	if len(d.Scopes) == 0 {
		d.Scopes = []config.Scope{config.ScopeSystem}
	}
	for _, scope := range d.Scopes {
		if !scope.IsValid() {
			return errors.New("invalid scope: " + string(scope))
		}
	}
	if d.Type == config.ValueTypeEnum && len(d.EnumValues) == 0 {
		return errors.New("enum_values required for enum type")
	}
	if d.Type == config.ValueTypeObjectRef && d.ObjectRefType == "" {
		return errors.New("object_ref_type required for object_ref type")
	}
	return nil
}

// WithScopes adds scopes to the definition
func (d Definition) WithScopes(scopes ...config.Scope) Definition {
	d.Scopes = scopes
	return d
}

// WithValidation adds validation to the definition
func (d Definition) WithValidation(v *ValidationSchema) Definition {
	d.Validation = v
	return d
}

// WithPermissions sets view and edit permissions
func (d Definition) WithPermissions(view, edit string) Definition {
	d.ViewPerm = view
	d.EditPerm = edit
	return d
}

// Sensitive marks the setting as sensitive
func (d Definition) Sensitive() Definition {
	d.IsSensitive = true
	return d
}

// RestartRequired marks the setting as requiring restart
func (d Definition) RestartRequired() Definition {
	d.RequiresRestart = true
	return d
}
