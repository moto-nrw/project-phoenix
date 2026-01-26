package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/config"
)

// Repositories needed by the scoped settings service
type ScopedSettingsRepositories struct {
	Definition config.SettingDefinitionRepository
	Value      config.SettingValueRepository
	Change     audit.SettingChangeRepository
}

// scopedSettingsService implements ScopedSettingsService
type scopedSettingsService struct {
	repos *ScopedSettingsRepositories
}

// NewScopedSettingsService creates a new ScopedSettingsService
func NewScopedSettingsService(repos *ScopedSettingsRepositories) ScopedSettingsService {
	return &scopedSettingsService{repos: repos}
}

// InitializeDefinitions syncs code-defined settings to the database
func (s *scopedSettingsService) InitializeDefinitions(ctx context.Context) error {
	for _, def := range GetDefaultDefinitions() {
		if err := s.repos.Definition.Upsert(ctx, def); err != nil {
			return fmt.Errorf("failed to upsert definition %s: %w", def.Key, err)
		}
	}
	return nil
}

// GetDefinition returns a setting definition by key
func (s *scopedSettingsService) GetDefinition(ctx context.Context, key string) (*config.SettingDefinition, error) {
	return s.repos.Definition.FindByKey(ctx, key)
}

// ListDefinitions returns definitions with optional filters
func (s *scopedSettingsService) ListDefinitions(ctx context.Context, filters map[string]interface{}) ([]*config.SettingDefinition, error) {
	return s.repos.Definition.List(ctx, filters)
}

// GetDefinitionsForScope returns definitions allowed for a scope type
func (s *scopedSettingsService) GetDefinitionsForScope(ctx context.Context, scopeType config.ScopeType) ([]*config.SettingDefinition, error) {
	return s.repos.Definition.FindByScope(ctx, scopeType)
}

// Get returns the resolved value for a setting at a given scope
func (s *scopedSettingsService) Get(ctx context.Context, key string, scope config.ScopeRef) (any, error) {
	resolved, err := s.GetWithSource(ctx, key, scope)
	if err != nil {
		return nil, err
	}
	return resolved.Value, nil
}

// GetWithSource returns the resolved value with information about where it came from
func (s *scopedSettingsService) GetWithSource(ctx context.Context, key string, scope config.ScopeRef) (*config.ResolvedSetting, error) {
	def, err := s.repos.Definition.FindByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if def == nil {
		return nil, fmt.Errorf("setting not found: %s", key)
	}

	// Build resolution chain based on scope and allowed scopes
	resolutionChain := s.buildResolutionChain(scope, def.AllowedScopes)

	// Try each scope in the chain
	var resolvedValue any
	var source *config.ScopeRef
	isDefault := true

	for _, checkScope := range resolutionChain {
		value, err := s.repos.Value.FindByDefinitionAndScope(ctx, def.ID, string(checkScope.Type), checkScope.ID)
		if err != nil {
			return nil, err
		}
		if value != nil {
			resolvedValue, err = value.GetTypedValue(def)
			if err != nil {
				return nil, fmt.Errorf("failed to parse value for %s: %w", key, err)
			}
			source = &checkScope
			isDefault = false
			break
		}
	}

	// Fall back to definition default
	if isDefault {
		resolvedValue, err = def.GetDefaultValueTyped()
		if err != nil {
			return nil, fmt.Errorf("failed to parse default value for %s: %w", key, err)
		}
	}

	// Check if setting is active (dependencies)
	isActive, err := s.IsSettingActive(ctx, key, scope)
	if err != nil {
		return nil, err
	}

	return &config.ResolvedSetting{
		Key:         def.Key,
		Value:       resolvedValue,
		Type:        def.Type,
		Category:    def.Category,
		Description: def.Description,
		GroupName:   def.GroupName,
		Source:      source,
		IsDefault:   isDefault,
		IsActive:    isActive,
		CanModify:   false, // Set by caller based on actor
		DependsOn:   def.DependsOn,
		Validation:  def.Validation,
	}, nil
}

// buildResolutionChain creates the list of scopes to check in order
func (s *scopedSettingsService) buildResolutionChain(scope config.ScopeRef, allowedScopes []string) []config.ScopeRef {
	var chain []config.ScopeRef

	// Start with the requested scope if allowed
	if slices.Contains(allowedScopes, string(scope.Type)) {
		chain = append(chain, scope)
	}

	// Add parent scopes in order of specificity
	// OG -> School -> System (most specific to least)
	parentOrder := []config.ScopeType{config.ScopeSchool, config.ScopeSystem}

	for _, parentType := range parentOrder {
		if slices.Contains(allowedScopes, string(parentType)) {
			// Skip if same as requested scope
			if parentType == scope.Type {
				continue
			}
			chain = append(chain, config.ScopeRef{Type: parentType, ID: nil})
		}
	}

	return chain
}

// GetAll returns all settings for a scope (resolved)
func (s *scopedSettingsService) GetAll(ctx context.Context, scope config.ScopeRef) ([]*config.ResolvedSetting, error) {
	defs, err := s.repos.Definition.FindByScope(ctx, scope.Type)
	if err != nil {
		return nil, err
	}

	var results []*config.ResolvedSetting
	for _, def := range defs {
		resolved, err := s.GetWithSource(ctx, def.Key, scope)
		if err != nil {
			return nil, err
		}
		results = append(results, resolved)
	}

	return results, nil
}

// GetAllByCategory returns all settings for a scope filtered by category
func (s *scopedSettingsService) GetAllByCategory(ctx context.Context, scope config.ScopeRef, category string) ([]*config.ResolvedSetting, error) {
	all, err := s.GetAll(ctx, scope)
	if err != nil {
		return nil, err
	}

	var filtered []*config.ResolvedSetting
	for _, setting := range all {
		if setting.Category == category {
			filtered = append(filtered, setting)
		}
	}

	return filtered, nil
}

// Set sets a value at a specific scope
func (s *scopedSettingsService) Set(
	ctx context.Context,
	key string,
	scope config.ScopeRef,
	value any,
	actor *Actor,
	r *http.Request,
) error {
	def, err := s.repos.Definition.FindByKey(ctx, key)
	if err != nil {
		return err
	}
	if def == nil {
		return fmt.Errorf("setting not found: %s", key)
	}

	// Validate scope is allowed
	if !def.IsScopeAllowed(scope.Type) {
		return fmt.Errorf("setting %s cannot be configured at %s level", key, scope.Type)
	}

	// Check if setting is active (dependencies)
	isActive, err := s.IsSettingActive(ctx, key, scope)
	if err != nil {
		return err
	}
	if !isActive {
		return fmt.Errorf("setting %s is inactive due to dependency", key)
	}

	// Validate the value
	if err := def.ValidateValue(value); err != nil {
		return fmt.Errorf("invalid value for %s: %w", key, err)
	}

	// Get current value for audit
	oldValue, _ := s.repos.Value.FindByDefinitionAndScope(ctx, def.ID, string(scope.Type), scope.ID)

	// Marshal the new value
	valueJSON, err := config.MarshalValue(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	// Create or update the value
	settingValue := &config.SettingValue{
		DefinitionID: def.ID,
		ScopeType:    string(scope.Type),
		ScopeID:      scope.ID,
		Value:        valueJSON,
		SetBy:        &actor.PersonID,
	}

	if err := s.repos.Value.Upsert(ctx, settingValue); err != nil {
		return fmt.Errorf("failed to save setting: %w", err)
	}

	// Log the change asynchronously
	s.logSettingChange(ctx, &audit.SettingChange{
		AccountID:  &actor.AccountID,
		SettingKey: key,
		ScopeType:  string(scope.Type),
		ScopeID:    scope.ID,
		ChangeType: s.getChangeType(oldValue),
		OldValue:   s.getOldValueJSON(oldValue),
		NewValue:   valueJSON,
		IPAddress:  getIPAddress(r),
		UserAgent:  r.UserAgent(),
	})

	return nil
}

// Reset removes a scoped value, causing it to inherit from parent scope
func (s *scopedSettingsService) Reset(
	ctx context.Context,
	key string,
	scope config.ScopeRef,
	actor *Actor,
	r *http.Request,
) error {
	if scope.Type == config.ScopeSystem {
		return fmt.Errorf("cannot reset system default, update the value instead")
	}

	def, err := s.repos.Definition.FindByKey(ctx, key)
	if err != nil {
		return err
	}
	if def == nil {
		return fmt.Errorf("setting not found: %s", key)
	}

	// Get current value for audit
	oldValue, err := s.repos.Value.FindByDefinitionAndScope(ctx, def.ID, string(scope.Type), scope.ID)
	if err != nil {
		return err
	}
	if oldValue == nil {
		return nil // Already using inherited value
	}

	// Delete the value
	if err := s.repos.Value.Delete(ctx, oldValue.ID); err != nil {
		return fmt.Errorf("failed to reset setting: %w", err)
	}

	// Log the change asynchronously
	s.logSettingChange(ctx, &audit.SettingChange{
		AccountID:  &actor.AccountID,
		SettingKey: key,
		ScopeType:  string(scope.Type),
		ScopeID:    scope.ID,
		ChangeType: string(audit.SettingChangeReset),
		OldValue:   oldValue.Value,
		IPAddress:  getIPAddress(r),
		UserAgent:  r.UserAgent(),
	})

	return nil
}

// IsSettingActive checks if a setting is active based on its dependencies
func (s *scopedSettingsService) IsSettingActive(ctx context.Context, key string, scope config.ScopeRef) (bool, error) {
	def, err := s.repos.Definition.FindByKey(ctx, key)
	if err != nil {
		return false, err
	}
	if def == nil {
		return false, fmt.Errorf("setting not found: %s", key)
	}

	// No dependency = always active
	if def.DependsOn == nil {
		return true, nil
	}

	// Get the parent setting's value
	parentValue, err := s.Get(ctx, def.DependsOn.Key, scope)
	if err != nil {
		return false, err
	}

	return s.evaluateCondition(def.DependsOn, parentValue), nil
}

// evaluateCondition checks if a dependency condition is met
func (s *scopedSettingsService) evaluateCondition(dep *config.SettingDependency, actualValue any) bool {
	switch dep.Condition {
	case "equals":
		return fmt.Sprintf("%v", actualValue) == fmt.Sprintf("%v", dep.Value)

	case "not_equals":
		return fmt.Sprintf("%v", actualValue) != fmt.Sprintf("%v", dep.Value)

	case "in":
		// Check if actualValue (slice) contains dep.Value
		if slice, ok := actualValue.([]interface{}); ok {
			for _, v := range slice {
				if fmt.Sprintf("%v", v) == fmt.Sprintf("%v", dep.Value) {
					return true
				}
			}
		}
		if slice, ok := actualValue.([]string); ok {
			for _, v := range slice {
				if v == fmt.Sprintf("%v", dep.Value) {
					return true
				}
			}
		}
		return false

	case "not_empty":
		if actualValue == nil {
			return false
		}
		switch v := actualValue.(type) {
		case string:
			return v != ""
		case int:
			return v != 0
		case float64:
			return v != 0
		case bool:
			return true
		default:
			return true
		}

	case "greater_than":
		return toFloat(actualValue) > toFloat(dep.Value)

	default:
		return true
	}
}

func toFloat(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

// CanModify checks if an actor can modify a setting at a scope
func (s *scopedSettingsService) CanModify(ctx context.Context, key string, scope config.ScopeRef, actor *Actor) (bool, error) {
	def, err := s.repos.Definition.FindByKey(ctx, key)
	if err != nil {
		return false, err
	}
	if def == nil {
		return false, nil
	}

	// Check if scope is allowed
	if !def.IsScopeAllowed(scope.Type) {
		return false, nil
	}

	// Get required permission for this scope
	requiredPerm := def.GetPermissionForScope(scope.Type)

	switch requiredPerm {
	case "none":
		return false, nil

	case "self":
		// User can only modify their own settings
		if scope.Type == config.ScopeUser && scope.ID != nil && *scope.ID == actor.PersonID {
			return true, nil
		}
		return false, nil

	case "owner":
		// Caller needs to check ownership externally
		// Return true if they have the base permission
		return slices.Contains(actor.Permissions, "settings:update"), nil

	default:
		// Standard permission check
		return slices.Contains(actor.Permissions, requiredPerm), nil
	}
}

// GetHistory returns the change history for a setting
func (s *scopedSettingsService) GetHistory(ctx context.Context, key string, scope config.ScopeRef, limit int) ([]*SettingHistoryEntry, error) {
	changes, err := s.repos.Change.FindByKeyAndScope(ctx, key, string(scope.Type), scope.ID, limit)
	if err != nil {
		return nil, err
	}

	return s.mapChangesToHistory(changes), nil
}

// GetHistoryForScope returns all changes for a scope
func (s *scopedSettingsService) GetHistoryForScope(ctx context.Context, scope config.ScopeRef, limit int) ([]*SettingHistoryEntry, error) {
	changes, err := s.repos.Change.FindByScope(ctx, string(scope.Type), scope.ID, limit)
	if err != nil {
		return nil, err
	}

	return s.mapChangesToHistory(changes), nil
}

func (s *scopedSettingsService) mapChangesToHistory(changes []*audit.SettingChange) []*SettingHistoryEntry {
	var entries []*SettingHistoryEntry
	for _, change := range changes {
		oldVal, _ := change.GetOldValueTyped()
		newVal, _ := change.GetNewValueTyped()

		entries = append(entries, &SettingHistoryEntry{
			ID:         change.ID,
			SettingKey: change.SettingKey,
			ChangeType: change.ChangeType,
			OldValue:   oldVal,
			NewValue:   newVal,
			ChangedAt:  change.CreatedAt.Format(time.RFC3339),
			Reason:     change.Reason,
		})
	}
	return entries
}

// DeleteScopeSettings removes all settings for a scope
func (s *scopedSettingsService) DeleteScopeSettings(ctx context.Context, scopeType config.ScopeType, scopeID int64) error {
	_, err := s.repos.Value.DeleteByScope(ctx, string(scopeType), scopeID)
	return err
}

// logSettingChange logs a setting change asynchronously
func (s *scopedSettingsService) logSettingChange(ctx context.Context, change *audit.SettingChange) {
	// Check if audit tracking is enabled
	trackingEnabled, _ := s.Get(ctx, "audit.track_setting_changes", config.NewSystemScope())
	if enabled, ok := trackingEnabled.(bool); ok && !enabled {
		return
	}

	go func() {
		logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()

		if err := s.repos.Change.Create(logCtx, change); err != nil {
			log.Printf("Failed to log setting change: %v", err)
		}
	}()
}

func (s *scopedSettingsService) getChangeType(existing *config.SettingValue) string {
	if existing == nil {
		return string(audit.SettingChangeCreate)
	}
	return string(audit.SettingChangeUpdate)
}

func (s *scopedSettingsService) getOldValueJSON(existing *config.SettingValue) json.RawMessage {
	if existing == nil {
		return nil
	}
	return existing.Value
}

func getIPAddress(r *http.Request) string {
	if r == nil {
		return ""
	}
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr, stripping port if present
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
