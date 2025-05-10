package users

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Profile represents a user profile in the system
type Profile struct {
	base.Model
	AccountID int64  `bun:"account_id,notnull,unique" json:"account_id"`
	Avatar    string `bun:"avatar" json:"avatar,omitempty"`
	Bio       string `bun:"bio" json:"bio,omitempty"`
	Settings  string `bun:"settings" json:"settings,omitempty"` // JSON string

	// Relations not stored in the database
	Account *auth.Account `bun:"-" json:"account,omitempty"`

	// Parsed settings
	parsedSettings map[string]interface{} `bun:"-" json:"-"`
}

// TableName returns the database table name
func (p *Profile) TableName() string {
	return "users.profiles"
}

// Validate ensures profile data is valid
func (p *Profile) Validate() error {
	if p.AccountID <= 0 {
		return errors.New("account ID is required")
	}

	// Validate settings JSON if provided
	if p.Settings != "" {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(p.Settings), &settings); err != nil {
			return errors.New("invalid settings JSON format")
		}
		p.parsedSettings = settings
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
	settingsBytes, err := json.Marshal(p.parsedSettings)
	if err != nil {
		return err
	}

	p.Settings = string(settingsBytes)
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
