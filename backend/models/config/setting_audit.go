package config

import (
	"errors"
	"time"
)

// tableSettingAuditLog is the schema-qualified table name
const tableSettingAuditLog = "config.setting_audit_log"

// AuditAction represents the type of change made to a setting
type AuditAction string

const (
	// AuditActionCreate is logged when a new value is created
	AuditActionCreate AuditAction = "create"
	// AuditActionUpdate is logged when a value is updated
	AuditActionUpdate AuditAction = "update"
	// AuditActionDelete is logged when a value is deleted (soft delete)
	AuditActionDelete AuditAction = "delete"
	// AuditActionRestore is logged when a soft-deleted value is restored
	AuditActionRestore AuditAction = "restore"
)

// IsValid checks if the action is valid
func (a AuditAction) IsValid() bool {
	switch a {
	case AuditActionCreate, AuditActionUpdate, AuditActionDelete, AuditActionRestore:
		return true
	}
	return false
}

// SettingAuditEntry represents a change to a setting value
type SettingAuditEntry struct {
	// ID is the unique identifier
	ID int64 `bun:"id,pk,autoincrement" json:"id"`

	// DefinitionID references the setting definition
	DefinitionID int64 `bun:"definition_id,notnull" json:"definition_id"`

	// SettingKey is denormalized for easy querying (survives definition deletion)
	SettingKey string `bun:"setting_key,notnull" json:"setting_key"`

	// ScopeType indicates the scope level
	ScopeType Scope `bun:"scope_type,notnull" json:"scope_type"`

	// ScopeID identifies the specific entity (NULL for system scope)
	ScopeID *int64 `bun:"scope_id" json:"scope_id,omitempty"`

	// OldValue is the previous value (NULL for create)
	OldValue *string `bun:"old_value" json:"old_value,omitempty"`

	// NewValue is the new value (NULL for delete)
	NewValue *string `bun:"new_value" json:"new_value,omitempty"`

	// Action describes what changed (create, update, delete, restore)
	Action AuditAction `bun:"action,notnull" json:"action"`

	// ChangedByAccountID identifies who made the change
	ChangedByAccountID int64 `bun:"changed_by_account_id,notnull" json:"changed_by_account_id"`

	// ChangedByName is denormalized for display (survives account deletion)
	ChangedByName string `bun:"changed_by_name,notnull" json:"changed_by_name"`

	// ChangedAt is when the change occurred
	ChangedAt time.Time `bun:"changed_at,notnull,default:current_timestamp" json:"changed_at"`

	// IPAddress is the request origin (optional)
	IPAddress *string `bun:"ip_address" json:"ip_address,omitempty"`

	// UserAgent is the request user agent (optional)
	UserAgent *string `bun:"user_agent" json:"user_agent,omitempty"`
}

// TableName returns the database table name
func (e *SettingAuditEntry) TableName() string {
	return tableSettingAuditLog
}

// Validate ensures the audit entry data is valid
func (e *SettingAuditEntry) Validate() error {
	if e.DefinitionID == 0 {
		return errors.New("definition_id is required")
	}
	if e.SettingKey == "" {
		return errors.New("setting_key is required")
	}
	if !e.ScopeType.IsValid() {
		return errors.New("invalid scope_type")
	}
	if !e.Action.IsValid() {
		return errors.New("invalid action")
	}
	if e.ChangedByAccountID == 0 {
		return errors.New("changed_by_account_id is required")
	}
	if e.ChangedByName == "" {
		return errors.New("changed_by_name is required")
	}
	return nil
}

// AuditContext contains information about who made a change
type AuditContext struct {
	// AccountID is the ID of the account making the change
	AccountID int64
	// AccountName is the display name (e.g., "Max Mustermann")
	AccountName string
	// IPAddress is the request IP (optional)
	IPAddress string
	// UserAgent is the request user agent (optional)
	UserAgent string
}

// ToAuditEntry creates an audit entry from this context
func (ac *AuditContext) ToAuditEntry(defID int64, key string, scopeType Scope, scopeID *int64, action AuditAction, oldValue, newValue *string) *SettingAuditEntry {
	var ipAddr, userAgent *string
	if ac.IPAddress != "" {
		ipAddr = &ac.IPAddress
	}
	if ac.UserAgent != "" {
		userAgent = &ac.UserAgent
	}

	return &SettingAuditEntry{
		DefinitionID:       defID,
		SettingKey:         key,
		ScopeType:          scopeType,
		ScopeID:            scopeID,
		OldValue:           oldValue,
		NewValue:           newValue,
		Action:             action,
		ChangedByAccountID: ac.AccountID,
		ChangedByName:      ac.AccountName,
		ChangedAt:          time.Now(),
		IPAddress:          ipAddr,
		UserAgent:          userAgent,
	}
}
