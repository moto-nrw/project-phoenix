package config

import (
	"encoding/json"
	"errors"
	"time"
)

// SettingAuditEntry records a change to a setting value. Append-only.
// Does NOT embed base.Model because the table has no created_at/updated_at columns.
type SettingAuditEntry struct {
	ID       int64 `bun:"id,pk,autoincrement" json:"id"`
	TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`

	SettingKey string          `bun:"setting_key,notnull" json:"setting_key"`
	OldValue   json.RawMessage `bun:"old_value,type:jsonb" json:"old_value,omitempty"`
	NewValue   json.RawMessage `bun:"new_value,type:jsonb" json:"new_value,omitempty"`
	Action     string          `bun:"action,notnull" json:"action"` // set, reset, delete
	ChangedBy  *int64          `bun:"changed_by" json:"changed_by,omitempty"`
	ChangedAt  time.Time       `bun:"changed_at,notnull,default:current_timestamp" json:"changed_at"`
}

// GetID returns the entity's ID.
func (e *SettingAuditEntry) GetID() interface{} { return e.ID }

// GetTenantID returns the tenant ID (satisfies base.TenantScoped interface).
func (e *SettingAuditEntry) GetTenantID() int64 { return e.TenantID }

// SetTenantID sets the tenant ID (satisfies base.TenantScoped interface).
func (e *SettingAuditEntry) SetTenantID(id int64) { e.TenantID = id }

// GetCreatedAt returns the creation timestamp.
func (e *SettingAuditEntry) GetCreatedAt() time.Time { return e.ChangedAt }

// GetUpdatedAt returns the creation timestamp (audit entries are never updated).
func (e *SettingAuditEntry) GetUpdatedAt() time.Time { return e.ChangedAt }

// Validate checks that all required fields are set.
func (e *SettingAuditEntry) Validate() error {
	if e.SettingKey == "" {
		return errors.New("setting_key is required")
	}
	validActions := map[string]bool{"set": true, "reset": true, "delete": true}
	if !validActions[e.Action] {
		return errors.New("action must be set, reset, or delete")
	}
	return nil
}

// NewAuditEntry creates a new audit entry for a setting change.
func NewAuditEntry(
	tenantID int64,
	key string,
	action string,
	oldValue json.RawMessage,
	newValue json.RawMessage,
	changedBy *int64,
) *SettingAuditEntry {
	return &SettingAuditEntry{
		TenantID:   tenantID,
		SettingKey: key,
		Action:     action,
		OldValue:   oldValue,
		NewValue:   newValue,
		ChangedBy:  changedBy,
		ChangedAt:  time.Now(),
	}
}
