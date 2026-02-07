package config

import "encoding/json"

// ResolvedSetting represents a setting with its effective value after hierarchy resolution
type ResolvedSetting struct {
	// Key is the setting identifier
	Key string `json:"key"`

	// Definition contains the setting metadata
	Definition *SettingDefinitionDTO `json:"definition"`

	// EffectiveValue is the resolved value after applying scope hierarchy
	EffectiveValue string `json:"effective_value"`

	// EffectiveScope indicates where the value came from
	EffectiveScope Scope `json:"effective_scope"`

	// EffectiveScopeID is the entity ID if not system scope
	EffectiveScopeID *int64 `json:"effective_scope_id,omitempty"`

	// IsDefault is true if using definition default (no stored values)
	IsDefault bool `json:"is_default"`

	// IsOverridden is true if there's a value at a more specific scope than system
	IsOverridden bool `json:"is_overridden"`

	// IsSensitive indicates the actual value is masked
	IsSensitive bool `json:"is_sensitive"`

	// CanEdit indicates if the current user can modify this setting
	CanEdit bool `json:"can_edit"`

	// CanView indicates if the current user can view this setting
	CanView bool `json:"can_view"`
}

// SettingDefinitionDTO is the API representation of a setting definition
type SettingDefinitionDTO struct {
	ID              int64             `json:"id"`
	Key             string            `json:"key"`
	ValueType       ValueType         `json:"value_type"`
	DefaultValue    string            `json:"default_value"`
	Category        string            `json:"category"`
	Tab             string            `json:"tab"`
	DisplayOrder    int               `json:"display_order"`
	Label           *string           `json:"label,omitempty"`
	Description     *string           `json:"description,omitempty"`
	AllowedScopes   []string          `json:"allowed_scopes"`
	EnumValues      []string          `json:"enum_values,omitempty"`
	EnumOptions     []EnumOption      `json:"enum_options,omitempty"`
	ObjectRefType   *string           `json:"object_ref_type,omitempty"`
	ObjectRefFilter json.RawMessage   `json:"object_ref_filter,omitempty"`
	RequiresRestart bool              `json:"requires_restart"`
	IsSensitive     bool              `json:"is_sensitive"`
	Validation      *ValidationSchema `json:"validation,omitempty"`
}

// ValidationSchema represents validation rules from the definition
type ValidationSchema struct {
	Min       *int64  `json:"min,omitempty"`
	Max       *int64  `json:"max,omitempty"`
	MinLength *int    `json:"min_length,omitempty"`
	MaxLength *int    `json:"max_length,omitempty"`
	Pattern   *string `json:"pattern,omitempty"`
	Required  bool    `json:"required,omitempty"`
}

// ToDTO converts a SettingDefinition to its API representation
func (d *SettingDefinition) ToDTO() *SettingDefinitionDTO {
	dto := &SettingDefinitionDTO{
		ID:              d.ID,
		Key:             d.Key,
		ValueType:       d.ValueType,
		DefaultValue:    d.DefaultValue,
		Category:        d.Category,
		Tab:             d.Tab,
		DisplayOrder:    d.DisplayOrder,
		Label:           d.Label,
		Description:     d.Description,
		AllowedScopes:   d.AllowedScopes,
		EnumValues:      d.EnumValues,
		EnumOptions:     d.EnumOptions,
		ObjectRefType:   d.ObjectRefType,
		ObjectRefFilter: d.ObjectRefFilter,
		RequiresRestart: d.RequiresRestart,
		IsSensitive:     d.IsSensitive,
	}

	// Parse validation schema if present
	if len(d.ValidationSchema) > 0 {
		var validation ValidationSchema
		if err := json.Unmarshal(d.ValidationSchema, &validation); err == nil {
			dto.Validation = &validation
		}
	}

	return dto
}

// ObjectRefOption represents an option for object reference selectors
type ObjectRefOption struct {
	// ID is the entity ID
	ID int64 `json:"id"`
	// Name is the display name
	Name string `json:"name"`
	// Description is additional context (optional)
	Description *string `json:"description,omitempty"`
	// Metadata contains additional entity-specific data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TabSettingsResponse is the API response for a settings tab
type TabSettingsResponse struct {
	// Tab is the tab metadata
	Tab *SettingTabDTO `json:"tab"`
	// Categories contains settings grouped by category
	Categories []*SettingCategory `json:"categories"`
}

// SettingTabDTO is the API representation of a setting tab
type SettingTabDTO struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	Icon         *string `json:"icon,omitempty"`
	DisplayOrder int     `json:"display_order"`
}

// ToDTO converts a SettingTab to its API representation
func (t *SettingTab) ToDTO() *SettingTabDTO {
	return &SettingTabDTO{
		Key:          t.Key,
		Name:         t.Name,
		Icon:         t.Icon,
		DisplayOrder: t.DisplayOrder,
	}
}

// SettingCategory groups settings by category within a tab
type SettingCategory struct {
	// Key is the category identifier
	Key string `json:"key"`
	// Name is the display name
	Name string `json:"name"`
	// Settings are the resolved settings in this category
	Settings []*ResolvedSetting `json:"settings"`
}

// SettingHistoryResponse contains the change history for a setting
type SettingHistoryResponse struct {
	// Key is the setting identifier
	Key string `json:"key"`
	// Entries are the audit log entries
	Entries []*SettingAuditEntryDTO `json:"entries"`
}

// SettingAuditEntryDTO is the API representation of an audit entry
type SettingAuditEntryDTO struct {
	ID            int64   `json:"id"`
	SettingKey    string  `json:"setting_key"`
	ScopeType     Scope   `json:"scope_type"`
	ScopeID       *int64  `json:"scope_id,omitempty"`
	OldValue      *string `json:"old_value,omitempty"`
	NewValue      *string `json:"new_value,omitempty"`
	Action        string  `json:"action"`
	ChangedByName string  `json:"changed_by_name"`
	ChangedAt     string  `json:"changed_at"`
}

// ToDTO converts a SettingAuditEntry to its API representation
func (e *SettingAuditEntry) ToDTO(maskSensitive bool) *SettingAuditEntryDTO {
	dto := &SettingAuditEntryDTO{
		ID:            e.ID,
		SettingKey:    e.SettingKey,
		ScopeType:     e.ScopeType,
		ScopeID:       e.ScopeID,
		Action:        string(e.Action),
		ChangedByName: e.ChangedByName,
		ChangedAt:     e.ChangedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if maskSensitive {
		if e.OldValue != nil {
			masked := "********"
			dto.OldValue = &masked
		}
		if e.NewValue != nil {
			masked := "********"
			dto.NewValue = &masked
		}
	} else {
		dto.OldValue = e.OldValue
		dto.NewValue = e.NewValue
	}

	return dto
}

// MaskedValue is the placeholder for sensitive setting values
const MaskedValue = "********"
