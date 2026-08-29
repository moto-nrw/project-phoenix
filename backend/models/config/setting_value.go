package config

import (
	"encoding/json"
	"errors"
	"time"
)

// SettingValue stores a single tenant-scoped setting override.
// If no override exists for a tenant, the registry default is used.
type SettingValue struct {
	ID         int64           `bun:"id,pk,autoincrement" json:"id"`
	TenantID   int64           `bun:"tenant_id,notnull" json:"tenant_id"`
	SettingKey string          `bun:"setting_key,notnull" json:"setting_key"`
	Value      json.RawMessage `bun:"value,type:jsonb,notnull" json:"value"`
	UpdatedBy  *int64          `bun:"updated_by" json:"updated_by,omitempty"`
	CreatedAt  time.Time       `bun:"created_at,notnull,default:now()" json:"created_at"`
	UpdatedAt  time.Time       `bun:"updated_at,notnull,default:now()" json:"updated_at"`
}

func (sv *SettingValue) GetID() interface{}      { return sv.ID }
func (sv *SettingValue) GetTenantID() int64      { return sv.TenantID }
func (sv *SettingValue) SetTenantID(id int64)    { sv.TenantID = id }
func (sv *SettingValue) GetCreatedAt() time.Time { return sv.CreatedAt }
func (sv *SettingValue) GetUpdatedAt() time.Time { return sv.UpdatedAt }

// Validate checks that all required fields are set.
func (sv *SettingValue) Validate() error {
	if sv.SettingKey == "" {
		return errors.New("setting_key is required")
	}
	if len(sv.Value) == 0 {
		return errors.New("value is required")
	}
	return nil
}
