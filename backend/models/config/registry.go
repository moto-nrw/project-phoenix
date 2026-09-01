package config

import (
	"fmt"
	"regexp"
	"sync"
)

// FieldType defines the type of input control for a setting.
type FieldType string

const (
	FieldBoolean  FieldType = "boolean"
	FieldNumber   FieldType = "number"
	FieldTime     FieldType = "time"
	FieldDate     FieldType = "date"
	FieldText     FieldType = "text"
	FieldTextarea FieldType = "textarea"
	FieldPassword FieldType = "password"
	FieldSelect   FieldType = "select"
)

// validFieldTypes contains all known field types for validation.
var validFieldTypes = map[FieldType]bool{
	FieldBoolean: true, FieldNumber: true, FieldTime: true, FieldDate: true,
	FieldText: true, FieldTextarea: true, FieldPassword: true, FieldSelect: true,
}

// AccessPolicy classifies which audience can read and write a setting.
// Storage remains per-tenant (config.setting_values.tenant_id) in every case;
// this is purely about who sees the setting in the schema and who may modify it.
type AccessPolicy string

const (
	// AccessShared: both tenant admins and platform operators can read and
	// write the setting. The default when the field is left unset.
	AccessShared AccessPolicy = "shared"
	// AccessAdminOnly: only tenant admins can read or write. Operators do
	// not see the setting in their schema and cannot set/reset/reveal it.
	AccessAdminOnly AccessPolicy = "admin_only"
	// AccessOperatorOnly: only platform operators can read or write. Tenant
	// admins do not see the setting in their schema and cannot write it.
	AccessOperatorOnly AccessPolicy = "operator_only"
)

// validAccessPolicies lists the accepted non-empty values for Definition.AccessPolicy.
var validAccessPolicies = map[AccessPolicy]bool{
	AccessShared:       true,
	AccessAdminOnly:    true,
	AccessOperatorOnly: true,
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

	// AccessPolicy classifies the audience. Empty defaults to AccessShared
	// in Register() — preserves existing behaviour for all previously
	// registered settings and future settings that don't declare a policy.
	AccessPolicy AccessPolicy `json:"access_policy,omitempty"`
}

// ValidationRules defines constraints on a setting value.
type ValidationRules struct {
	Required        bool           `json:"required,omitempty"`
	AllowEmpty      bool           `json:"allow_empty,omitempty"` // Permit an empty string without applying Pattern
	Min             *float64       `json:"min,omitempty"`
	Max             *float64       `json:"max,omitempty"`
	Pattern         *string        `json:"pattern,omitempty"` // Regex pattern for string/password fields
	CompiledPattern *regexp.Regexp `json:"-"`                 // Pre-compiled pattern, set during Validate()
}

// Dependency makes a setting conditionally visible based on another setting's value.
type Dependency struct {
	Key       string `json:"key"`
	Condition string `json:"condition"` // eq, neq, not_empty
	Value     any    `json:"value"`
}

// DependsOnEq builds a Dependency that shows the setting when key equals value.
func DependsOnEq(key string, value any) *Dependency {
	return &Dependency{Key: key, Condition: "eq", Value: value}
}

// DependsOnNeq builds a Dependency that shows the setting when key does not equal value.
func DependsOnNeq(key string, value any) *Dependency {
	return &Dependency{Key: key, Condition: "neq", Value: value}
}

// Range builds ValidationRules constraining a numeric setting to [minVal, maxVal].
func Range(minVal, maxVal float64) *ValidationRules {
	return &ValidationRules{Min: &minVal, Max: &maxVal}
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
	"reminders",
	"devices",
	"enrollment",
	"gdpr",
	"system",
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
	if d.AccessPolicy != "" && !validAccessPolicies[d.AccessPolicy] {
		return fmt.Errorf("invalid AccessPolicy %q for setting %s", d.AccessPolicy, d.Key)
	}
	if d.Validation != nil && d.Validation.Pattern != nil {
		compiled, err := regexp.Compile(*d.Validation.Pattern)
		if err != nil {
			return fmt.Errorf("invalid validation pattern for %s: %w", d.Key, err)
		}
		d.Validation.CompiledPattern = compiled
	}
	// Validate default against min/max rules
	if d.Validation != nil && d.Default != nil && d.Type == FieldNumber {
		if num, ok := toFloat(d.Default); ok {
			if d.Validation.Min != nil && num < *d.Validation.Min {
				return fmt.Errorf("default value %v for %s is below minimum %v", num, d.Key, *d.Validation.Min)
			}
			if d.Validation.Max != nil && num > *d.Validation.Max {
				return fmt.Errorf("default value %v for %s exceeds maximum %v", num, d.Key, *d.Validation.Max)
			}
		}
	}
	return nil
}

// toFloat converts a numeric value to float64 for validation.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// Registry owns a set of setting definitions.
type Registry struct {
	mu          sync.RWMutex
	definitions map[string]*Definition
}

func NewRegistry() *Registry {
	return &Registry{definitions: make(map[string]*Definition)}
}

var defaultRegistry = NewRegistry()

// Register adds a setting definition to the global registry.
// It panics on duplicate keys or invalid definitions (catches errors at startup).
func Register(def Definition) {
	defaultRegistry.Register(def)
}

func (r *Registry) Register(def Definition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.definitions[def.Key]; exists {
		panic(fmt.Sprintf("config: duplicate setting key: %s", def.Key))
	}
	if def.AccessPolicy == "" {
		def.AccessPolicy = AccessShared
	}
	if err := def.Validate(); err != nil {
		panic(fmt.Sprintf("config: invalid setting definition: %v", err))
	}
	defCopy := def
	r.definitions[def.Key] = &defCopy
}

// GetDefinition returns a definition by key, or nil if not registered.
func GetDefinition(key string) *Definition {
	return defaultRegistry.GetDefinition(key)
}

func (r *Registry) GetDefinition(key string) *Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.definitions[key]
}

func (r *Registry) AllDefinitions() map[string]*Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*Definition, len(r.definitions))
	for k, v := range r.definitions {
		defCopy := *v
		result[k] = &defCopy
	}
	return result
}

// DefaultRegistry returns the process registry populated by defaults packages.
// Callers must not mutate it after application startup.
func DefaultRegistry() *Registry { return defaultRegistry }
