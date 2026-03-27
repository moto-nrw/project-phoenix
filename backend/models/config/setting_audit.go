package config

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

const tableSettingAudit = "config.setting_audit"

// SettingAuditEntry records a change to a setting value. Append-only.
type SettingAuditEntry struct {
	base.Model `bun:"schema:config,table:setting_audit"`
	base.TenantModel

	SettingKey string          `bun:"setting_key,notnull" json:"setting_key"`
	OldValue   json.RawMessage `bun:"old_value,type:jsonb" json:"old_value,omitempty"`
	NewValue   json.RawMessage `bun:"new_value,type:jsonb" json:"new_value,omitempty"`
	Action     string          `bun:"action,notnull" json:"action"` // set, reset, delete
	ChangedBy  *int64          `bun:"changed_by" json:"changed_by,omitempty"`
	ChangedAt  time.Time       `bun:"changed_at,notnull,default:current_timestamp" json:"changed_at"`
}

// GetID returns the entity's ID.
func (e *SettingAuditEntry) GetID() interface{} { return e.ID }

// GetCreatedAt returns the creation timestamp.
func (e *SettingAuditEntry) GetCreatedAt() time.Time { return e.ChangedAt }

// GetUpdatedAt returns the creation timestamp (audit entries are never updated).
func (e *SettingAuditEntry) GetUpdatedAt() time.Time { return e.ChangedAt }

// TableName returns the database table name.
func (e *SettingAuditEntry) TableName() string { return tableSettingAudit }

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
	entry := &SettingAuditEntry{
		SettingKey: key,
		Action:     action,
		OldValue:   oldValue,
		NewValue:   newValue,
		ChangedBy:  changedBy,
		ChangedAt:  time.Now(),
	}
	entry.TenantID = tenantID
	return entry
}
