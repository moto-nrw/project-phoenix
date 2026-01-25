package audit

import (
	"encoding/json"
	"errors"
	"time"
)

// SettingChangeType represents the type of setting change
type SettingChangeType string

const (
	SettingChangeCreate SettingChangeType = "create"
	SettingChangeUpdate SettingChangeType = "update"
	SettingChangeDelete SettingChangeType = "delete"
	SettingChangeReset  SettingChangeType = "reset"
)

// SettingChange represents an audit entry for setting modifications
type SettingChange struct {
	ID         int64           `bun:"id,pk,autoincrement" json:"id"`
	AccountID  *int64          `bun:"account_id" json:"account_id,omitempty"`
	SettingKey string          `bun:"setting_key,notnull" json:"setting_key"`
	ScopeType  string          `bun:"scope_type,notnull" json:"scope_type"`
	ScopeID    *int64          `bun:"scope_id" json:"scope_id,omitempty"`
	ChangeType string          `bun:"change_type,notnull" json:"change_type"`
	OldValue   json.RawMessage `bun:"old_value,type:jsonb" json:"old_value,omitempty"`
	NewValue   json.RawMessage `bun:"new_value,type:jsonb" json:"new_value,omitempty"`
	IPAddress  string          `bun:"ip_address" json:"ip_address,omitempty"`
	UserAgent  string          `bun:"user_agent" json:"user_agent,omitempty"`
	Reason     string          `bun:"reason" json:"reason,omitempty"`
	CreatedAt  time.Time       `bun:"created_at,notnull,default:now()" json:"created_at"`
}

// TableName returns the database table name
func (sc *SettingChange) TableName() string {
	return "audit.setting_changes"
}

// GetID implements the base.Entity interface
func (sc *SettingChange) GetID() interface{} {
	return sc.ID
}

// GetCreatedAt implements the base.Entity interface
func (sc *SettingChange) GetCreatedAt() time.Time {
	return sc.CreatedAt
}

// GetUpdatedAt implements the base.Entity interface
func (sc *SettingChange) GetUpdatedAt() time.Time {
	return sc.CreatedAt
}

// Validate ensures the setting change is valid
func (sc *SettingChange) Validate() error {
	if sc.SettingKey == "" {
		return errors.New("setting_key is required")
	}

	if sc.ScopeType == "" {
		return errors.New("scope_type is required")
	}

	if sc.ChangeType == "" {
		return errors.New("change_type is required")
	}

	// Validate change type
	switch SettingChangeType(sc.ChangeType) {
	case SettingChangeCreate, SettingChangeUpdate, SettingChangeDelete, SettingChangeReset:
		// Valid types
	default:
		return errors.New("invalid change_type")
	}

	if sc.CreatedAt.IsZero() {
		sc.CreatedAt = time.Now()
	}

	return nil
}

// NewSettingChange creates a new setting change audit entry
func NewSettingChange(
	accountID *int64,
	settingKey string,
	scopeType string,
	scopeID *int64,
	changeType SettingChangeType,
	oldValue, newValue json.RawMessage,
	ipAddress, userAgent string,
) *SettingChange {
	return &SettingChange{
		AccountID:  accountID,
		SettingKey: settingKey,
		ScopeType:  scopeType,
		ScopeID:    scopeID,
		ChangeType: string(changeType),
		OldValue:   oldValue,
		NewValue:   newValue,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		CreatedAt:  time.Now(),
	}
}

// GetOldValueTyped parses the old value to a typed value
func (sc *SettingChange) GetOldValueTyped() (any, error) {
	if len(sc.OldValue) == 0 {
		return nil, nil
	}
	var wrapper struct {
		Value any `json:"value"`
	}
	if err := json.Unmarshal(sc.OldValue, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Value, nil
}

// GetNewValueTyped parses the new value to a typed value
func (sc *SettingChange) GetNewValueTyped() (any, error) {
	if len(sc.NewValue) == 0 {
		return nil, nil
	}
	var wrapper struct {
		Value any `json:"value"`
	}
	if err := json.Unmarshal(sc.NewValue, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Value, nil
}
