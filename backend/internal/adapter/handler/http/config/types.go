package config

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/config"
)

// SettingResponse represents a setting API response
type SettingResponse struct {
	ID              int64     `json:"id"`
	Key             string    `json:"key"`
	Value           string    `json:"value"`
	Category        string    `json:"category"`
	Description     string    `json:"description,omitempty"`
	RequiresRestart bool      `json:"requires_restart"`
	RequiresDBReset bool      `json:"requires_db_reset"`
	IsSystemSetting bool      `json:"is_system_setting"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// SettingRequest represents a setting creation/update request
type SettingRequest struct {
	Key             string `json:"key"`
	Value           string `json:"value"`
	Category        string `json:"category"`
	Description     string `json:"description,omitempty"`
	RequiresRestart bool   `json:"requires_restart"`
	RequiresDBReset bool   `json:"requires_db_reset"`
}

// Bind validates the setting request
func (req *SettingRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Key, validation.Required),
		validation.Field(&req.Value, validation.Required),
		validation.Field(&req.Category, validation.Required),
	)
}

// SettingValueRequest represents a setting value update request
type SettingValueRequest struct {
	Value string `json:"value"`
}

// Bind validates the setting value request
func (req *SettingValueRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Value, validation.Required),
	)
}

// ImportSettingsRequest represents a settings import request
type ImportSettingsRequest struct {
	Settings []SettingRequest `json:"settings"`
}

// Bind validates the import settings request
func (req *ImportSettingsRequest) Bind(r *http.Request) error {
	if len(req.Settings) == 0 {
		return errors.New("at least one setting is required")
	}

	// Validate each setting
	for i, setting := range req.Settings {
		if err := (&setting).Bind(r); err != nil {
			return errors.New("invalid setting at index " + strconv.Itoa(i) + ": " + err.Error())
		}
	}

	return nil
}

// SystemStatusResponse represents the system status response
type SystemStatusResponse struct {
	RequiresRestart bool `json:"requires_restart"`
	RequiresDBReset bool `json:"requires_db_reset"`
}

// newSettingResponse converts a setting model to a response object
func newSettingResponse(setting *config.Setting) SettingResponse {
	return SettingResponse{
		ID:              setting.ID,
		Key:             setting.Key,
		Value:           setting.Value,
		Category:        setting.Category,
		Description:     setting.Description,
		RequiresRestart: setting.RequiresRestart,
		RequiresDBReset: setting.RequiresDBReset,
		IsSystemSetting: setting.IsSystemSetting(),
		CreatedAt:       setting.CreatedAt,
		UpdatedAt:       setting.UpdatedAt,
	}
}
