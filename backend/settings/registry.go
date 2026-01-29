package settings

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// registry holds all registered setting definitions
var (
	registry     = make(map[string]*Definition)
	registryLock sync.RWMutex
)

// Register adds a setting definition to the registry.
// It returns an error if a setting with the same key already exists.
func Register(def Definition) error {
	if err := def.Validate(); err != nil {
		return fmt.Errorf("invalid definition for %q: %w", def.Key, err)
	}

	registryLock.Lock()
	defer registryLock.Unlock()

	if _, exists := registry[def.Key]; exists {
		return fmt.Errorf("setting %q is already registered", def.Key)
	}

	registry[def.Key] = &def
	return nil
}

// MustRegister adds a setting definition to the registry.
// It panics if registration fails.
func MustRegister(def Definition) {
	if err := Register(def); err != nil {
		panic(err)
	}
}

// Get retrieves a setting definition by key.
// Returns nil if not found.
func Get(key string) *Definition {
	registryLock.RLock()
	defer registryLock.RUnlock()
	return registry[key]
}

// All returns all registered setting definitions.
func All() []*Definition {
	registryLock.RLock()
	defer registryLock.RUnlock()

	defs := make([]*Definition, 0, len(registry))
	for _, def := range registry {
		defs = append(defs, def)
	}
	return defs
}

// AllByTab returns all definitions for a specific tab.
func AllByTab(tab string) []*Definition {
	registryLock.RLock()
	defer registryLock.RUnlock()

	defs := make([]*Definition, 0)
	for _, def := range registry {
		if def.Tab == tab {
			defs = append(defs, def)
		}
	}
	return defs
}

// AllByCategory returns all definitions for a specific category.
func AllByCategory(category string) []*Definition {
	registryLock.RLock()
	defer registryLock.RUnlock()

	defs := make([]*Definition, 0)
	for _, def := range registry {
		if def.Category == category {
			defs = append(defs, def)
		}
	}
	return defs
}

// Keys returns all registered setting keys.
func Keys() []string {
	registryLock.RLock()
	defer registryLock.RUnlock()

	keys := make([]string, 0, len(registry))
	for key := range registry {
		keys = append(keys, key)
	}
	return keys
}

// Count returns the number of registered settings.
func Count() int {
	registryLock.RLock()
	defer registryLock.RUnlock()
	return len(registry)
}

// Clear removes all registered settings (useful for testing).
func Clear() {
	registryLock.Lock()
	defer registryLock.Unlock()
	registry = make(map[string]*Definition)
}

// ToSettingDefinition converts a registry Definition to a database model.
func (d *Definition) ToSettingDefinition() *config.SettingDefinition {
	scopes := make([]string, len(d.Scopes))
	for i, s := range d.Scopes {
		scopes[i] = string(s)
	}

	def := &config.SettingDefinition{
		Key:             d.Key,
		ValueType:       d.Type,
		DefaultValue:    d.Default,
		Category:        d.Category,
		Tab:             d.Tab,
		DisplayOrder:    d.DisplayOrder,
		AllowedScopes:   scopes,
		EnumValues:      d.EnumValues,
		RequiresRestart: d.RequiresRestart,
		IsSensitive:     d.IsSensitive,
	}

	if d.Label != "" {
		def.Label = &d.Label
	}
	if d.Description != "" {
		def.Description = &d.Description
	}
	if d.ViewPerm != "" {
		def.ViewPermission = &d.ViewPerm
	}
	if d.EditPerm != "" {
		def.EditPermission = &d.EditPerm
	}
	if d.ObjectRefType != "" {
		def.ObjectRefType = &d.ObjectRefType
	}
	if d.ObjectRefFilter != nil {
		filterJSON, _ := json.Marshal(d.ObjectRefFilter)
		def.ObjectRefFilter = filterJSON
	}
	if d.Validation != nil {
		validationJSON, _ := json.Marshal(d.Validation)
		def.ValidationSchema = validationJSON
	}

	return def
}
