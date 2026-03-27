package config

import (
	"encoding/json"
	"fmt"
	"sync"
)

// FieldType defines the type of input control for a setting.
type FieldType string

const (
	FieldBoolean  FieldType = "boolean"
	FieldNumber   FieldType = "number"
	FieldTime     FieldType = "time"
	FieldText     FieldType = "text"
	FieldPassword FieldType = "password"
	FieldSelect   FieldType = "select"
)

// validFieldTypes contains all known field types for validation.
var validFieldTypes = map[FieldType]bool{
	FieldBoolean: true, FieldNumber: true, FieldTime: true,
	FieldText: true, FieldPassword: true, FieldSelect: true,
}

// Definition describes a single setting: its type, default, permissions,
// categorization, dependencies, and select options.
//
// In v1, all settings are tenant-scoped with a system default from the registry.
// Adding more scopes later is a ~10 line change (no migration needed).
type Definition struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Description string    `json:"description"`
	Type        FieldType `json:"type"`
	Default     any       `json:"default"`

	Validation *ValidationRules `json:"validation,omitempty"`

	ReadPermission  string `json:"read_permission,omitempty"`
	WritePermission string `json:"write_permission,omitempty"`

	Tab       string `json:"tab"`
	Category  string `json:"category"`
	SortOrder int    `json:"sort_order"`

	DependsOn *Dependency    `json:"depends_on,omitempty"`
	Options   *SelectOptions `json:"options,omitempty"`
}

// ValidationRules defines constraints on a setting value.
type ValidationRules struct {
	Required bool     `json:"required,omitempty"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
}

// Dependency makes a setting conditionally visible based on another setting's value.
type Dependency struct {
	Key       string `json:"key"`
	Condition string `json:"condition"` // eq, neq, not_empty
	Value     any    `json:"value"`
}

// SelectOptions provides static choices for a FieldSelect setting.
type SelectOptions struct {
	Static []SelectOption `json:"static,omitempty"`
}

// SelectOption is a single static option for a FieldSelect.
type SelectOption struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// TabOrder defines the display ordering of setting tabs.
var TabOrder = []string{
	"operations",
	"gdpr",
	"security",
	"general",
}

// Validate checks that a Definition has all required fields and consistent configuration.
func (d *Definition) Validate() error {
	if d.Key == "" {
		return fmt.Errorf("setting key is required")
	}
	if !validFieldTypes[d.Type] {
		return fmt.Errorf("unknown field type %q for setting %s", d.Type, d.Key)
	}
	if d.Tab == "" {
		return fmt.Errorf("tab is required for setting %s", d.Key)
	}
	if d.Category == "" {
		return fmt.Errorf("category is required for setting %s", d.Key)
	}
	if d.Options != nil && d.Type != FieldSelect {
		return fmt.Errorf("options can only be set for select fields, setting %s has type %s", d.Key, d.Type)
	}
	if d.Type == FieldSelect && d.Options == nil {
		return fmt.Errorf("select field %s must have options", d.Key)
	}
	return nil
}

// MarshalDefault returns the default value as JSON bytes.
func (d *Definition) MarshalDefault() (json.RawMessage, error) {
	if d.Default == nil {
		return json.RawMessage("null"), nil
	}
	return json.Marshal(d.Default)
}

// --- Registry singleton ---

var (
	registry   = make(map[string]*Definition)
	registryMu sync.RWMutex
)

// Register adds a setting definition to the global registry.
// It panics on duplicate keys or invalid definitions (catches errors at startup).
func Register(def Definition) {
	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := registry[def.Key]; exists {
		panic(fmt.Sprintf("config: duplicate setting key: %s", def.Key))
	}
	if err := def.Validate(); err != nil {
		panic(fmt.Sprintf("config: invalid setting definition: %v", err))
	}
	defCopy := def
	registry[def.Key] = &defCopy
}

// GetDefinition returns a definition by key, or nil if not registered.
func GetDefinition(key string) *Definition {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[key]
}

// AllDefinitions returns a copy of all registered definitions.
func AllDefinitions() map[string]*Definition {
	registryMu.RLock()
	defer registryMu.RUnlock()

	result := make(map[string]*Definition, len(registry))
	for k, v := range registry {
		result[k] = v
	}
	return result
}

// ResetRegistry clears all registered definitions. For testing only.
func ResetRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[string]*Definition)
}
