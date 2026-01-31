package settings

import (
	"fmt"
	"sync"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// TabDefinition describes a settings tab for the UI
type TabDefinition struct {
	// Key is the unique identifier (e.g., "security", "display")
	Key string

	// Name is the display name (can use i18n key)
	Name string

	// Icon is the icon identifier (e.g., "shield", "monitor")
	Icon string

	// DisplayOrder controls the position in the tab list
	DisplayOrder int

	// RequiredPermission is the permission needed to see this tab (empty = public)
	RequiredPermission string
}

// Validate ensures the tab definition is valid
func (t *TabDefinition) Validate() error {
	if t.Key == "" {
		return fmt.Errorf("tab key is required")
	}
	if t.Name == "" {
		return fmt.Errorf("tab name is required")
	}
	return nil
}

// ToSettingTab converts to the database model
func (t *TabDefinition) ToSettingTab() *config.SettingTab {
	tab := &config.SettingTab{
		Key:          t.Key,
		Name:         t.Name,
		DisplayOrder: t.DisplayOrder,
	}

	if t.Icon != "" {
		tab.Icon = &t.Icon
	}
	if t.RequiredPermission != "" {
		tab.RequiredPermission = &t.RequiredPermission
	}

	return tab
}

// tabRegistry holds all registered tab definitions
var (
	tabRegistry     = make(map[string]*TabDefinition)
	tabRegistryLock sync.RWMutex
)

// RegisterTab adds a tab definition to the registry.
// It returns an error if a tab with the same key already exists.
func RegisterTab(def TabDefinition) error {
	if err := def.Validate(); err != nil {
		return fmt.Errorf("invalid tab definition for %q: %w", def.Key, err)
	}

	tabRegistryLock.Lock()
	defer tabRegistryLock.Unlock()

	if _, exists := tabRegistry[def.Key]; exists {
		return fmt.Errorf("tab %q is already registered", def.Key)
	}

	tabRegistry[def.Key] = &def
	return nil
}

// MustRegisterTab adds a tab definition to the registry.
// It panics if registration fails.
func MustRegisterTab(def TabDefinition) {
	if err := RegisterTab(def); err != nil {
		panic(err)
	}
}

// GetTab retrieves a tab definition by key.
// Returns nil if not found.
func GetTab(key string) *TabDefinition {
	tabRegistryLock.RLock()
	defer tabRegistryLock.RUnlock()
	return tabRegistry[key]
}

// AllTabs returns all registered tab definitions.
func AllTabs() []*TabDefinition {
	tabRegistryLock.RLock()
	defer tabRegistryLock.RUnlock()

	tabs := make([]*TabDefinition, 0, len(tabRegistry))
	for _, tab := range tabRegistry {
		tabs = append(tabs, tab)
	}
	return tabs
}

// TabKeys returns all registered tab keys.
func TabKeys() []string {
	tabRegistryLock.RLock()
	defer tabRegistryLock.RUnlock()

	keys := make([]string, 0, len(tabRegistry))
	for key := range tabRegistry {
		keys = append(keys, key)
	}
	return keys
}

// TabCount returns the number of registered tabs.
func TabCount() int {
	tabRegistryLock.RLock()
	defer tabRegistryLock.RUnlock()
	return len(tabRegistry)
}

// ClearTabs removes all registered tabs (useful for testing).
func ClearTabs() {
	tabRegistryLock.Lock()
	defer tabRegistryLock.Unlock()
	tabRegistry = make(map[string]*TabDefinition)
}
