package users

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const profileTableName = "users.profiles"

// Profile represents a user profile in the system
type Profile struct {
	base.Model `bun:"schema:users,table:profiles"`
	AccountID  int64  `bun:"account_id,notnull,unique" json:"account_id"`
	Avatar     string `bun:"avatar" json:"avatar,omitempty"`
	Bio        string `bun:"bio" json:"bio,omitempty"`
	Settings   string `bun:"settings" json:"settings,omitempty"` // JSON string

	// Relations not stored in the database
	Account *auth.Account `bun:"-" json:"account,omitempty"`

	// Parsed settings
	parsedSettings map[string]interface{} `bun:"-" json:"-"`
}

func (p *Profile) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(profileTableName)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(profileTableName)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(profileTableName)
	}
	return nil
}

// TableName returns the database table name
func (p *Profile) TableName() string {
	return profileTableName
}

// Validate ensures profile data is valid
func (p *Profile) Validate() error {
	if p.AccountID <= 0 {
		return errors.New("account ID is required")
	}

	// Validate settings JSON if provided
	if p.Settings != "" {
		if err := json.Unmarshal([]byte(p.Settings), &p.parsedSettings); err != nil {
			return fmt.Errorf("invalid settings JSON format: %w", err)
		}
	}

	return nil
}

// SetAccount links this profile to an account
func (p *Profile) SetAccount(account *auth.Account) {
	p.Account = account
	if account != nil {
		p.AccountID = account.ID
	}
}

// GetSetting retrieves a setting by key
func (p *Profile) GetSetting(key string) (interface{}, bool) {
	// Parse settings if needed
	if p.parsedSettings == nil && p.Settings != "" {
		_ = json.Unmarshal([]byte(p.Settings), &p.parsedSettings)
	}

	if p.parsedSettings == nil {
		return nil, false
	}

	value, exists := p.parsedSettings[key]
	return value, exists
}

// SetSetting sets a setting value by key
func (p *Profile) SetSetting(key string, value interface{}) error {
	// Parse settings if needed
	if p.parsedSettings == nil {
		if p.Settings != "" {
			if err := json.Unmarshal([]byte(p.Settings), &p.parsedSettings); err != nil {
				p.parsedSettings = make(map[string]interface{})
			}
		} else {
			p.parsedSettings = make(map[string]interface{})
		}
	}

	// Set the value
	p.parsedSettings[key] = value

	// Update the JSON string
	if bytes, err := json.Marshal(p.parsedSettings); err != nil {
		return err
	} else {
		p.Settings = string(bytes)
	}
	return nil
}

// DeleteSetting removes a setting by key
func (p *Profile) DeleteSetting(key string) {
	// Parse settings if needed
	if p.parsedSettings == nil && p.Settings != "" {
		_ = json.Unmarshal([]byte(p.Settings), &p.parsedSettings)
	}

	if p.parsedSettings == nil {
		return
	}

	// Delete the key
	delete(p.parsedSettings, key)

	// Update the JSON string
	settingsBytes, _ := json.Marshal(p.parsedSettings)
	p.Settings = string(settingsBytes)
}

// HasAvatar checks if the profile has an avatar
func (p *Profile) HasAvatar() bool {
	return p.Avatar != ""
}

// HasBio checks if the profile has a bio
func (p *Profile) HasBio() bool {
	return p.Bio != ""
}

// GetID returns the entity's ID
func (m *Profile) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *Profile) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *Profile) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}
