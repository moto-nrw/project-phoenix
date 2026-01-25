package config

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// Actor represents the user performing an action
type Actor struct {
	AccountID   int64
	PersonID    int64
	Permissions []string
}

// ScopedSettingsService defines operations for scoped settings
type ScopedSettingsService interface {
	// === Definition Management ===

	// InitializeDefinitions syncs code-defined settings to the database
	InitializeDefinitions(ctx context.Context) error

	// GetDefinition returns a setting definition by key
	GetDefinition(ctx context.Context, key string) (*config.SettingDefinition, error)

	// ListDefinitions returns definitions with optional filters
	ListDefinitions(ctx context.Context, filters map[string]interface{}) ([]*config.SettingDefinition, error)

	// GetDefinitionsForScope returns definitions allowed for a scope type
	GetDefinitionsForScope(ctx context.Context, scopeType config.ScopeType) ([]*config.SettingDefinition, error)

	// === Value Resolution ===

	// Get returns the resolved value for a setting at a given scope
	// Resolution follows: scope -> parent scopes -> system -> default
	Get(ctx context.Context, key string, scope config.ScopeRef) (any, error)

	// GetWithSource returns the resolved value with information about where it came from
	GetWithSource(ctx context.Context, key string, scope config.ScopeRef) (*config.ResolvedSetting, error)

	// GetAll returns all settings for a scope (resolved)
	GetAll(ctx context.Context, scope config.ScopeRef) ([]*config.ResolvedSetting, error)

	// GetAllByCategory returns all settings for a scope filtered by category
	GetAllByCategory(ctx context.Context, scope config.ScopeRef, category string) ([]*config.ResolvedSetting, error)

	// === Value Modification ===

	// Set sets a value at a specific scope
	// Validates permissions, type, and dependencies
	Set(ctx context.Context, key string, scope config.ScopeRef, value any, actor *Actor, r *http.Request) error

	// Reset removes a scoped value, causing it to inherit from parent scope
	Reset(ctx context.Context, key string, scope config.ScopeRef, actor *Actor, r *http.Request) error

	// === Dependency Management ===

	// IsSettingActive checks if a setting is active based on its dependencies
	IsSettingActive(ctx context.Context, key string, scope config.ScopeRef) (bool, error)

	// === Permission Checking ===

	// CanModify checks if an actor can modify a setting at a scope
	CanModify(ctx context.Context, key string, scope config.ScopeRef, actor *Actor) (bool, error)

	// === Audit ===

	// GetHistory returns the change history for a setting
	GetHistory(ctx context.Context, key string, scope config.ScopeRef, limit int) ([]*SettingHistoryEntry, error)

	// GetHistoryForScope returns all changes for a scope
	GetHistoryForScope(ctx context.Context, scope config.ScopeRef, limit int) ([]*SettingHistoryEntry, error)

	// === Cleanup ===

	// DeleteScopeSettings removes all settings for a scope (when entity is deleted)
	DeleteScopeSettings(ctx context.Context, scopeType config.ScopeType, scopeID int64) error
}

// SettingHistoryEntry represents a change in the audit log
type SettingHistoryEntry struct {
	ID         int64  `json:"id"`
	SettingKey string `json:"setting_key"`
	ChangeType string `json:"change_type"`
	OldValue   any    `json:"old_value,omitempty"`
	NewValue   any    `json:"new_value,omitempty"`
	ChangedBy  string `json:"changed_by,omitempty"`
	ChangedAt  string `json:"changed_at"`
	Reason     string `json:"reason,omitempty"`
}
