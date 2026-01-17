package config

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/config"
)

// CreateSetting creates a new configuration setting
func (s *service) CreateSetting(ctx context.Context, setting *config.Setting) error {
	if setting == nil {
		return &ConfigError{Op: "CreateSetting", Err: ErrInvalidSettingData}
	}

	// Validate setting data
	if err := setting.Validate(); err != nil {
		return &ConfigError{Op: "CreateSetting", Err: err}
	}

	// Check if a setting with the same key exists
	existingSetting, err := s.settingRepo.FindByKey(ctx, setting.Key)
	if err == nil && existingSetting != nil && existingSetting.ID > 0 {
		return &ConfigError{Op: "CreateSetting", Err: &DuplicateKeyError{Key: setting.Key}}
	}

	// Create the setting
	if err := s.settingRepo.Create(ctx, setting); err != nil {
		return &ConfigError{Op: "CreateSetting", Err: err}
	}

	return nil
}

// GetSettingByID retrieves a setting by its ID
func (s *service) GetSettingByID(ctx context.Context, id int64) (*config.Setting, error) {
	if id <= 0 {
		return nil, &ConfigError{Op: "GetSettingByID", Err: ErrInvalidID}
	}

	setting, err := s.settingRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ConfigError{Op: "GetSettingByID", Err: err}
	}

	if setting == nil {
		return nil, &ConfigError{Op: "GetSettingByID", Err: ErrSettingNotFound}
	}

	return setting, nil
}

// UpdateSetting updates an existing configuration setting
func (s *service) UpdateSetting(ctx context.Context, setting *config.Setting) error {
	// Validate input
	if setting == nil || setting.ID <= 0 {
		return &ConfigError{Op: "UpdateSetting", Err: ErrInvalidSettingData}
	}

	if err := setting.Validate(); err != nil {
		return &ConfigError{Op: "UpdateSetting", Err: err}
	}

	// Check if setting exists
	existingSetting, err := s.settingRepo.FindByID(ctx, setting.ID)
	if err != nil {
		return &ConfigError{Op: "UpdateSetting", Err: err}
	}

	if existingSetting == nil {
		return &ConfigError{Op: "UpdateSetting", Err: ErrSettingNotFound}
	}

	// Validate update is allowed
	if err := s.validateSettingUpdate(ctx, existingSetting, setting); err != nil {
		return err
	}

	// Update the setting
	if err := s.settingRepo.Update(ctx, setting); err != nil {
		return &ConfigError{Op: "UpdateSetting", Err: err}
	}

	return nil
}

func (s *service) validateSettingUpdate(ctx context.Context, existing, updated *config.Setting) error {
	// If this is a system setting, verify that only the value is being changed
	if existing.IsSystemSetting() {
		if existing.Key != updated.Key || existing.Category != updated.Category {
			return &ConfigError{Op: "UpdateSetting", Err: &SystemSettingsLockedError{Key: existing.Key}}
		}
	}

	// Check for duplicate key if changed
	if existing.Key != updated.Key || existing.Category != updated.Category {
		duplicateCheck, err := s.settingRepo.FindByKeyAndCategory(ctx, updated.Key, updated.Category)
		if err == nil && duplicateCheck != nil && duplicateCheck.ID > 0 && duplicateCheck.ID != updated.ID {
			return &ConfigError{Op: "UpdateSetting", Err: &DuplicateKeyError{Key: updated.Key}}
		}
	}

	return nil
}

// DeleteSetting deletes a configuration setting by its ID
func (s *service) DeleteSetting(ctx context.Context, id int64) error {
	if id <= 0 {
		return &ConfigError{Op: "DeleteSetting", Err: ErrInvalidID}
	}

	// Check if setting exists
	existingSetting, err := s.settingRepo.FindByID(ctx, id)
	if err != nil {
		return &ConfigError{Op: "DeleteSetting", Err: err}
	}

	if existingSetting == nil {
		return &ConfigError{Op: "DeleteSetting", Err: ErrSettingNotFound}
	}

	// Prevent deletion of system settings
	if existingSetting.IsSystemSetting() {
		return &ConfigError{Op: "DeleteSetting", Err: &SystemSettingsLockedError{Key: existingSetting.Key}}
	}

	// Delete the setting
	if err := s.settingRepo.Delete(ctx, id); err != nil {
		return &ConfigError{Op: "DeleteSetting", Err: err}
	}

	return nil
}

// ListSettings retrieves settings based on filters
func (s *service) ListSettings(ctx context.Context, filters map[string]interface{}) ([]*config.Setting, error) {
	settings, err := s.settingRepo.List(ctx, filters)
	if err != nil {
		return nil, &ConfigError{Op: "ListSettings", Err: err}
	}
	return settings, nil
}
