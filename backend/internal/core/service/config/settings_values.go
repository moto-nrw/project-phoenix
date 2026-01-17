package config

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/config"
)

// GetSettingByKey retrieves a setting by its key
func (s *service) GetSettingByKey(ctx context.Context, key string) (*config.Setting, error) {
	if key == "" {
		return nil, &ConfigError{Op: "GetSettingByKey", Err: ErrKeyEmpty}
	}

	setting, err := s.settingRepo.FindByKey(ctx, key)
	if err != nil {
		return nil, &ConfigError{Op: "GetSettingByKey", Err: err}
	}

	if setting == nil || setting.ID <= 0 {
		return nil, &ConfigError{Op: "GetSettingByKey", Err: &SettingNotFoundError{Key: key}}
	}

	return setting, nil
}

// UpdateSettingValue updates the value of a setting by its key
func (s *service) UpdateSettingValue(ctx context.Context, key string, value string) error {
	if key == "" {
		return &ConfigError{Op: "UpdateSettingValue", Err: ErrKeyEmpty}
	}

	// Check if setting exists
	setting, err := s.settingRepo.FindByKey(ctx, key)
	if err != nil {
		return &ConfigError{Op: "UpdateSettingValue", Err: err}
	}

	if setting == nil || setting.ID <= 0 {
		return &ConfigError{Op: "UpdateSettingValue", Err: &SettingNotFoundError{Key: key}}
	}

	// Update the setting value
	if err := s.settingRepo.UpdateValue(ctx, key, value); err != nil {
		return &ConfigError{Op: "UpdateSettingValue", Err: err}
	}

	return nil
}

// GetStringValue retrieves the value of a setting as a string
func (s *service) GetStringValue(ctx context.Context, key string, defaultValue string) (string, error) {
	setting, err := s.settingRepo.FindByKey(ctx, key)
	if err != nil {
		return defaultValue, &ConfigError{Op: "GetStringValue", Err: err}
	}
	if setting == nil || setting.ID <= 0 {
		return defaultValue, nil
	}

	return setting.Value, nil
}

// GetBoolValue retrieves the value of a setting as a boolean
func (s *service) GetBoolValue(ctx context.Context, key string, defaultValue bool) (bool, error) {
	setting, err := s.settingRepo.FindByKey(ctx, key)
	if err != nil {
		return defaultValue, &ConfigError{Op: "GetBoolValue", Err: err}
	}
	if setting == nil || setting.ID <= 0 {
		return defaultValue, nil
	}

	value := strings.ToLower(setting.Value)
	switch value {
	case "true", "1", "yes", "y":
		return true, nil
	case "false", "0", "no", "n":
		return false, nil
	}

	return defaultValue, &ConfigError{
		Op: "GetBoolValue",
		Err: &ValueParsingError{
			Key:   key,
			Value: setting.Value,
			Type:  "boolean",
		},
	}
}

// GetIntValue retrieves the value of a setting as an integer
func (s *service) GetIntValue(ctx context.Context, key string, defaultValue int) (int, error) {
	setting, err := s.settingRepo.FindByKey(ctx, key)
	if err != nil {
		return defaultValue, &ConfigError{Op: "GetIntValue", Err: err}
	}
	if setting == nil || setting.ID <= 0 {
		return defaultValue, nil
	}

	intVal, err := strconv.Atoi(setting.Value)
	if err != nil {
		return defaultValue, &ConfigError{
			Op: "GetIntValue",
			Err: &ValueParsingError{
				Key:   key,
				Value: setting.Value,
				Type:  "integer",
			},
		}
	}

	return intVal, nil
}

// GetFloatValue retrieves the value of a setting as a float
func (s *service) GetFloatValue(ctx context.Context, key string, defaultValue float64) (float64, error) {
	setting, err := s.settingRepo.FindByKey(ctx, key)
	if err != nil {
		return defaultValue, &ConfigError{Op: "GetFloatValue", Err: err}
	}
	if setting == nil || setting.ID <= 0 {
		return defaultValue, nil
	}

	floatVal, err := strconv.ParseFloat(setting.Value, 64)
	if err != nil {
		return defaultValue, &ConfigError{
			Op: "GetFloatValue",
			Err: &ValueParsingError{
				Key:   key,
				Value: setting.Value,
				Type:  "float",
			},
		}
	}

	return floatVal, nil
}

// GetSettingsByCategory retrieves settings by their category
func (s *service) GetSettingsByCategory(ctx context.Context, category string) ([]*config.Setting, error) {
	if category == "" {
		return nil, &ConfigError{Op: "GetSettingsByCategory", Err: ErrCategoryEmpty}
	}

	settings, err := s.settingRepo.FindByCategory(ctx, category)
	if err != nil {
		return nil, &ConfigError{Op: "GetSettingsByCategory", Err: err}
	}

	return settings, nil
}

// GetSettingByKeyAndCategory retrieves a setting by its key and category
func (s *service) GetSettingByKeyAndCategory(ctx context.Context, key string, category string) (*config.Setting, error) {
	if key == "" || category == "" {
		return nil, &ConfigError{Op: "GetSettingByKeyAndCategory", Err: ErrKeyAndCategoryEmpty}
	}

	setting, err := s.settingRepo.FindByKeyAndCategory(ctx, key, category)
	if err != nil {
		return nil, &ConfigError{Op: "GetSettingByKeyAndCategory", Err: err}
	}

	if setting == nil || setting.ID <= 0 {
		return nil, &ConfigError{Op: "GetSettingByKeyAndCategory", Err: &SettingNotFoundError{Key: fmt.Sprintf("%s.%s", category, key)}}
	}

	return setting, nil
}
