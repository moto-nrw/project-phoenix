package config

import (
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// SettingValue stores a single tenant-scoped setting override.
// If no override exists for a tenant, the registry default is used.
type SettingValue struct {
	base.Model `bun:"schema:config,table:setting_values"`
	base.TenantModel

	SettingKey string          `bun:"setting_key,notnull" json:"setting_key"`
	Value      json.RawMessage `bun:"value,type:jsonb,notnull" json:"value"`
	UpdatedBy  *int64          `bun:"updated_by" json:"updated_by,omitempty"`
}

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
