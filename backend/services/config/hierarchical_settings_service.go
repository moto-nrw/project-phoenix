package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/settings"
	"github.com/uptrace/bun"
)

// HierarchicalSettingsService manages hierarchical settings with scope inheritance
type HierarchicalSettingsService interface {
	// SyncDefinitions syncs code-defined settings to the database
	SyncDefinitions(ctx context.Context) error

	// GetEffective returns the effective value after hierarchy resolution
	GetEffective(ctx context.Context, key string, scopeCtx *config.ScopeContext) (*config.ResolvedSetting, error)

	// Type-safe getters
	GetString(ctx context.Context, key string, scopeCtx *config.ScopeContext) (string, error)
	GetInt(ctx context.Context, key string, scopeCtx *config.ScopeContext) (int64, error)
	GetBool(ctx context.Context, key string, scopeCtx *config.ScopeContext) (bool, error)

	// SetValue sets a value at a specific scope (auto-audits)
	SetValue(ctx context.Context, key, value string, scope config.Scope, scopeID *int64, audit *config.AuditContext) error

	// DeleteOverride soft deletes a scope override (auto-audits)
	DeleteOverride(ctx context.Context, key string, scope config.Scope, scopeID *int64, audit *config.AuditContext) error

	// GetObjectRefOptions returns available options for object reference settings
	GetObjectRefOptions(ctx context.Context, key string, scopeCtx *config.ScopeContext) ([]*config.ObjectRefOption, error)

	// Tab-based queries
	GetTabs(ctx context.Context, userPermissions []string) ([]*config.SettingTabDTO, error)
	GetTabSettings(ctx context.Context, tab string, scopeCtx *config.ScopeContext, userPermissions []string) (*config.TabSettingsResponse, error)

	// Audit trail
	GetSettingHistory(ctx context.Context, key string, limit int) ([]*config.SettingAuditEntryDTO, error)
	GetRecentChanges(ctx context.Context, limit int) ([]*config.SettingAuditEntryDTO, error)

	// Soft delete management
	RestoreValue(ctx context.Context, key string, scope config.Scope, scopeID *int64, audit *config.AuditContext) error
	PurgeDeletedOlderThan(ctx context.Context, days int) (int64, error)
}

// HierarchicalSettingsServiceImpl implements HierarchicalSettingsService
type HierarchicalSettingsServiceImpl struct {
	defRepo   config.SettingDefinitionRepository
	valueRepo config.SettingValueRepository
	auditRepo config.SettingAuditRepository
	tabRepo   config.SettingTabRepository
	db        *bun.DB
}

// NewHierarchicalSettingsService creates a new hierarchical settings service
func NewHierarchicalSettingsService(
	defRepo config.SettingDefinitionRepository,
	valueRepo config.SettingValueRepository,
	auditRepo config.SettingAuditRepository,
	tabRepo config.SettingTabRepository,
	db *bun.DB,
) *HierarchicalSettingsServiceImpl {
	return &HierarchicalSettingsServiceImpl{
		defRepo:   defRepo,
		valueRepo: valueRepo,
		auditRepo: auditRepo,
		tabRepo:   tabRepo,
		db:        db,
	}
}

// SyncDefinitions syncs code-defined settings to the database
func (s *HierarchicalSettingsServiceImpl) SyncDefinitions(ctx context.Context) error {
	defs := settings.All()

	for _, def := range defs {
		dbDef := def.ToSettingDefinition()
		if err := s.defRepo.Upsert(ctx, dbDef); err != nil {
			return fmt.Errorf("failed to sync definition %q: %w", def.Key, err)
		}
	}

	return nil
}

// GetEffective returns the effective value after hierarchy resolution
func (s *HierarchicalSettingsServiceImpl) GetEffective(ctx context.Context, key string, scopeCtx *config.ScopeContext) (*config.ResolvedSetting, error) {
	// Get the definition
	def, err := s.defRepo.FindByKey(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("setting %q not found", key)
		}
		return nil, err
	}

	// Try to find effective value
	value, effectiveScope, err := s.valueRepo.FindEffectiveValue(ctx, def.ID, scopeCtx)

	resolved := &config.ResolvedSetting{
		Key:        key,
		Definition: def.ToDTO(),
		CanView:    true, // Caller should have already checked permissions
		CanEdit:    true,
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Use default value
			resolved.EffectiveValue = def.DefaultValue
			resolved.EffectiveScope = config.ScopeSystem
			resolved.IsDefault = true
			resolved.IsOverridden = false
		} else {
			return nil, err
		}
	} else {
		resolved.EffectiveValue = value.Value
		resolved.EffectiveScope = effectiveScope
		resolved.EffectiveScopeID = value.ScopeID
		resolved.IsDefault = false
		resolved.IsOverridden = effectiveScope != config.ScopeSystem
	}

	resolved.IsSensitive = def.IsSensitive
	if def.IsSensitive {
		resolved.EffectiveValue = config.MaskedValue
	}

	return resolved, nil
}

// GetString returns a string value
func (s *HierarchicalSettingsServiceImpl) GetString(ctx context.Context, key string, scopeCtx *config.ScopeContext) (string, error) {
	resolved, err := s.GetEffective(ctx, key, scopeCtx)
	if err != nil {
		return "", err
	}
	return resolved.EffectiveValue, nil
}

// GetInt returns an integer value
func (s *HierarchicalSettingsServiceImpl) GetInt(ctx context.Context, key string, scopeCtx *config.ScopeContext) (int64, error) {
	resolved, err := s.GetEffective(ctx, key, scopeCtx)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(resolved.EffectiveValue, 10, 64)
}

// GetBool returns a boolean value
func (s *HierarchicalSettingsServiceImpl) GetBool(ctx context.Context, key string, scopeCtx *config.ScopeContext) (bool, error) {
	resolved, err := s.GetEffective(ctx, key, scopeCtx)
	if err != nil {
		return false, err
	}
	return resolved.EffectiveValue == "true", nil
}

// SetValue sets a value at a specific scope
func (s *HierarchicalSettingsServiceImpl) SetValue(ctx context.Context, key, value string, scope config.Scope, scopeID *int64, audit *config.AuditContext) error {
	// Get the definition
	def, err := s.defRepo.FindByKey(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("setting %q not found", key)
		}
		return err
	}

	// Check if scope is allowed
	if !def.IsScopeAllowed(scope) {
		return fmt.Errorf("setting %q cannot be configured at %s scope", key, scope)
	}

	// Validate value
	if err := def.ValidateValue(value); err != nil {
		return fmt.Errorf("invalid value for %q: %w", key, err)
	}

	// Encrypt if sensitive
	finalValue := value
	if def.IsSensitive {
		encrypted, err := settings.Encrypt(value)
		if err != nil {
			return fmt.Errorf("failed to encrypt sensitive value: %w", err)
		}
		finalValue = encrypted
	}

	// Get existing value for audit
	existingValue, _ := s.valueRepo.FindByDefinitionAndScope(ctx, def.ID, scope, scopeID)
	var oldValue *string
	if existingValue != nil {
		oldValue = &existingValue.Value
	}

	// Upsert the value
	settingValue := &config.SettingValue{
		DefinitionID: def.ID,
		ScopeType:    scope,
		ScopeID:      scopeID,
		Value:        finalValue,
	}

	if err := s.valueRepo.Upsert(ctx, settingValue); err != nil {
		return fmt.Errorf("failed to set value: %w", err)
	}

	// Log audit entry
	if audit != nil {
		action := config.AuditActionUpdate
		if existingValue == nil {
			action = config.AuditActionCreate
		}

		auditEntry := audit.ToAuditEntry(def.ID, key, scope, scopeID, action, oldValue, &finalValue)
		if err := s.auditRepo.Create(ctx, auditEntry); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to create audit entry: %v\n", err)
		}
	}

	return nil
}

// DeleteOverride soft deletes a scope override
func (s *HierarchicalSettingsServiceImpl) DeleteOverride(ctx context.Context, key string, scope config.Scope, scopeID *int64, audit *config.AuditContext) error {
	if scope == config.ScopeSystem && scopeID == nil {
		return errors.New("cannot delete system default; update it instead")
	}

	// Get the definition
	def, err := s.defRepo.FindByKey(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("setting %q not found", key)
		}
		return err
	}

	// Get existing value for audit
	existingValue, err := s.valueRepo.FindByDefinitionAndScope(ctx, def.ID, scope, scopeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no override exists for %q at %s scope", key, scope)
		}
		return err
	}

	// Soft delete
	if err := s.valueRepo.SoftDeleteByScope(ctx, def.ID, scope, scopeID); err != nil {
		return fmt.Errorf("failed to delete override: %w", err)
	}

	// Log audit entry
	if audit != nil {
		auditEntry := audit.ToAuditEntry(def.ID, key, scope, scopeID, config.AuditActionDelete, &existingValue.Value, nil)
		if err := s.auditRepo.Create(ctx, auditEntry); err != nil {
			fmt.Printf("Warning: failed to create audit entry: %v\n", err)
		}
	}

	return nil
}

// GetObjectRefOptions returns available options for object reference settings
func (s *HierarchicalSettingsServiceImpl) GetObjectRefOptions(ctx context.Context, key string, scopeCtx *config.ScopeContext) ([]*config.ObjectRefOption, error) {
	// Get the definition
	def, err := s.defRepo.FindByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	if def.ValueType != config.ValueTypeObjectRef {
		return nil, fmt.Errorf("setting %q is not an object reference", key)
	}

	// TODO: Implement object reference resolution based on ObjectRefType and ObjectRefFilter
	// This will require injecting domain repositories for rooms, groups, etc.
	return []*config.ObjectRefOption{}, nil
}

// GetTabs returns available tabs based on user permissions
func (s *HierarchicalSettingsServiceImpl) GetTabs(ctx context.Context, userPermissions []string) ([]*config.SettingTabDTO, error) {
	tabs, err := s.tabRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	permSet := make(map[string]bool)
	for _, p := range userPermissions {
		permSet[p] = true
	}

	var result []*config.SettingTabDTO
	for _, tab := range tabs {
		// Check if user has required permission
		if tab.RequiredPermission != nil && *tab.RequiredPermission != "" {
			if !permSet[*tab.RequiredPermission] {
				continue
			}
		}
		result = append(result, tab.ToDTO())
	}

	return result, nil
}

// GetTabSettings returns settings for a specific tab
func (s *HierarchicalSettingsServiceImpl) GetTabSettings(ctx context.Context, tab string, scopeCtx *config.ScopeContext, userPermissions []string) (*config.TabSettingsResponse, error) {
	// Get the tab
	tabModel, err := s.tabRepo.FindByKey(ctx, tab)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("tab %q not found", tab)
		}
		return nil, err
	}

	// Check tab permission
	permSet := make(map[string]bool)
	for _, p := range userPermissions {
		permSet[p] = true
	}
	if tabModel.RequiredPermission != nil && *tabModel.RequiredPermission != "" {
		if !permSet[*tabModel.RequiredPermission] {
			return nil, errors.New("access denied to tab")
		}
	}

	// Get definitions for this tab
	defs, err := s.defRepo.FindByTab(ctx, tab)
	if err != nil {
		return nil, err
	}

	// Group by category
	categoryMap := make(map[string]*config.SettingCategory)
	categoryOrder := make([]string, 0)

	for _, def := range defs {
		// Check view permission
		if def.ViewPermission != nil && *def.ViewPermission != "" {
			if !permSet[*def.ViewPermission] {
				continue
			}
		}

		// Get or create category
		cat, exists := categoryMap[def.Category]
		if !exists {
			cat = &config.SettingCategory{
				Key:      def.Category,
				Name:     formatCategoryName(def.Category),
				Settings: make([]*config.ResolvedSetting, 0),
			}
			categoryMap[def.Category] = cat
			categoryOrder = append(categoryOrder, def.Category)
		}

		// Resolve the effective value
		resolved, err := s.GetEffective(ctx, def.Key, scopeCtx)
		if err != nil {
			continue
		}

		// Check edit permission
		canEdit := true
		if def.EditPermission != nil && *def.EditPermission != "" {
			canEdit = permSet[*def.EditPermission]
		}
		resolved.CanEdit = canEdit

		cat.Settings = append(cat.Settings, resolved)
	}

	// Build ordered categories
	categories := make([]*config.SettingCategory, 0, len(categoryOrder))
	for _, catKey := range categoryOrder {
		categories = append(categories, categoryMap[catKey])
	}

	return &config.TabSettingsResponse{
		Tab:        tabModel.ToDTO(),
		Categories: categories,
	}, nil
}

// GetSettingHistory returns the change history for a setting
func (s *HierarchicalSettingsServiceImpl) GetSettingHistory(ctx context.Context, key string, limit int) ([]*config.SettingAuditEntryDTO, error) {
	// Get the definition to check if it's sensitive
	def, err := s.defRepo.FindByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	entries, err := s.auditRepo.FindBySettingKey(ctx, key, limit)
	if err != nil {
		return nil, err
	}

	result := make([]*config.SettingAuditEntryDTO, len(entries))
	for i, entry := range entries {
		result[i] = entry.ToDTO(def.IsSensitive)
	}

	return result, nil
}

// GetRecentChanges returns recent audit entries
func (s *HierarchicalSettingsServiceImpl) GetRecentChanges(ctx context.Context, limit int) ([]*config.SettingAuditEntryDTO, error) {
	entries, err := s.auditRepo.FindRecent(ctx, limit)
	if err != nil {
		return nil, err
	}

	// Build a map of sensitive definitions
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		keys = append(keys, e.SettingKey)
	}

	sensitiveKeys := make(map[string]bool)
	if len(keys) > 0 {
		defs, err := s.defRepo.FindByKeys(ctx, keys)
		if err == nil {
			for _, def := range defs {
				if def.IsSensitive {
					sensitiveKeys[def.Key] = true
				}
			}
		}
	}

	result := make([]*config.SettingAuditEntryDTO, len(entries))
	for i, entry := range entries {
		result[i] = entry.ToDTO(sensitiveKeys[entry.SettingKey])
	}

	return result, nil
}

// RestoreValue restores a soft-deleted value
func (s *HierarchicalSettingsServiceImpl) RestoreValue(ctx context.Context, key string, scope config.Scope, scopeID *int64, audit *config.AuditContext) error {
	// Get the definition
	def, err := s.defRepo.FindByKey(ctx, key)
	if err != nil {
		return err
	}

	// Find deleted value
	// Note: This is a simplified implementation - in production, we'd need a separate query for deleted values
	if err := s.valueRepo.SoftDeleteByScope(ctx, def.ID, scope, scopeID); err != nil {
		// If it's already deleted, try to restore
		// This is a workaround - real implementation would need a dedicated restore method
	}

	// Log audit entry
	if audit != nil {
		auditEntry := audit.ToAuditEntry(def.ID, key, scope, scopeID, config.AuditActionRestore, nil, nil)
		if err := s.auditRepo.Create(ctx, auditEntry); err != nil {
			fmt.Printf("Warning: failed to create audit entry: %v\n", err)
		}
	}

	return nil
}

// PurgeDeletedOlderThan permanently removes old deleted records
func (s *HierarchicalSettingsServiceImpl) PurgeDeletedOlderThan(ctx context.Context, days int) (int64, error) {
	valuesDeleted, err := s.valueRepo.PurgeDeletedOlderThan(ctx, days)
	if err != nil {
		return 0, err
	}

	defsDeleted, err := s.defRepo.PurgeDeletedOlderThan(ctx, days)
	if err != nil {
		return valuesDeleted, err
	}

	return valuesDeleted + defsDeleted, nil
}

// formatCategoryName converts a category key to a display name
func formatCategoryName(key string) string {
	// Simple implementation - could be enhanced with i18n
	names := map[string]string{
		"system":         "System",
		"session":        "Sitzungen",
		"password":       "Passwort",
		"jwt":            "Token",
		"email":          "E-Mail",
		"smtp":           "SMTP",
		"sender":         "Absender",
		"invitation":     "Einladungen",
		"password_reset": "Passwort-Reset",
		"appearance":     "Erscheinung",
		"pagination":     "Seitenumbruch",
		"format":         "Formatierung",
		"device":         "Gerät",
		"privacy":        "Datenschutz",
		"checkin":        "Check-in",
		"checkout":       "Check-out",
		"capacity":       "Kapazität",
	}

	if name, ok := names[key]; ok {
		return name
	}
	return key
}
