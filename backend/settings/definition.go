package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

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

	// Validate that the default value matches the type
	if d.Default != "" {
		if err := d.validateValue(d.Default); err != nil {
			return fmt.Errorf("invalid default value for %q: %w", d.Key, err)
		}
	}

	return nil
}

// validateValue validates a value against this definition's type
func (d *Definition) validateValue(value string) error {
	if value == "" {
		return nil
	}

	switch d.Type {
	case config.ValueTypeInt:
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return errors.New("value must be an integer")
		}
		// Check validation schema constraints
		if d.Validation != nil {
			if d.Validation.Min != nil && v < *d.Validation.Min {
				return fmt.Errorf("value must be at least %d", *d.Validation.Min)
			}
			if d.Validation.Max != nil && v > *d.Validation.Max {
				return fmt.Errorf("value must be at most %d", *d.Validation.Max)
			}
		}
	case config.ValueTypeFloat:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return errors.New("value must be a number")
		}
	case config.ValueTypeBool:
		if value != "true" && value != "false" {
			return errors.New("value must be 'true' or 'false'")
		}
	case config.ValueTypeEnum:
		found := false
		for _, ev := range d.EnumValues {
			if ev == value {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("value must be one of: %v", d.EnumValues)
		}
	case config.ValueTypeTime:
		if _, err := time.Parse("15:04:05", value); err != nil {
			if _, err := time.Parse("15:04", value); err != nil {
				return errors.New("value must be a valid time (HH:MM or HH:MM:SS)")
			}
		}
	case config.ValueTypeDuration:
		if _, err := time.ParseDuration(value); err != nil {
			return errors.New("value must be a valid duration (e.g., 30m, 1h)")
		}
	case config.ValueTypeObjectRef:
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return errors.New("value must be a valid entity ID (integer)")
		}
	case config.ValueTypeJSON:
		if !json.Valid([]byte(value)) {
			return errors.New("value must be valid JSON")
		}
	case config.ValueTypeString:
		// Check validation schema constraints for strings
		if d.Validation != nil {
			if d.Validation.MinLength != nil && len(value) < *d.Validation.MinLength {
				return fmt.Errorf("value must be at least %d characters", *d.Validation.MinLength)
			}
			if d.Validation.MaxLength != nil && len(value) > *d.Validation.MaxLength {
				return fmt.Errorf("value must be at most %d characters", *d.Validation.MaxLength)
			}
		}
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
