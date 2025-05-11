package config

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// service implements the Service interface
type service struct {
	settingRepo config.SettingRepository
	db          *bun.DB
	txHandler   *base.TxHandler
}

// NewService creates a new config service
func NewService(settingRepo config.SettingRepository, db *bun.DB) Service {
	return &service{
		settingRepo: settingRepo,
		db:          db,
		txHandler:   base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *service) WithTx(tx bun.Tx) interface{} {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var settingRepo config.SettingRepository = s.settingRepo

	// Try to cast repository to TransactionalRepository and apply the transaction
	if txRepo, ok := s.settingRepo.(base.TransactionalRepository); ok {
		settingRepo = txRepo.WithTx(tx).(config.SettingRepository)
	}

	// Return a new service with the transaction
	return &service{
		settingRepo: settingRepo,
		db:          s.db,
		txHandler:   s.txHandler.WithTx(tx),
	}
}

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
		return nil, &ConfigError{Op: "GetSettingByID", Err: errors.New("invalid ID")}
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
	if setting == nil || setting.ID <= 0 {
		return &ConfigError{Op: "UpdateSetting", Err: ErrInvalidSettingData}
	}

	// Validate setting data
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

	// If this is a system setting, verify that only the value is being changed
	if existingSetting.IsSystemSetting() {
		if existingSetting.Key != setting.Key || existingSetting.Category != setting.Category {
			return &ConfigError{Op: "UpdateSetting", Err: &SystemSettingsLockedError{Key: existingSetting.Key}}
		}
	}

	// Check for duplicate key if changed
	if existingSetting.Key != setting.Key || existingSetting.Category != setting.Category {
		duplicateCheck, err := s.settingRepo.FindByKeyAndCategory(ctx, setting.Key, setting.Category)
		if err == nil && duplicateCheck != nil && duplicateCheck.ID > 0 && duplicateCheck.ID != setting.ID {
			return &ConfigError{Op: "UpdateSetting", Err: &DuplicateKeyError{Key: setting.Key}}
		}
	}

	// Update the setting
	if err := s.settingRepo.Update(ctx, setting); err != nil {
		return &ConfigError{Op: "UpdateSetting", Err: err}
	}

	return nil
}

// DeleteSetting deletes a configuration setting by its ID
func (s *service) DeleteSetting(ctx context.Context, id int64) error {
	if id <= 0 {
		return &ConfigError{Op: "DeleteSetting", Err: errors.New("invalid ID")}
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

// GetSettingByKey retrieves a setting by its key
func (s *service) GetSettingByKey(ctx context.Context, key string) (*config.Setting, error) {
	if key == "" {
		return nil, &ConfigError{Op: "GetSettingByKey", Err: errors.New("key cannot be empty")}
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
		return &ConfigError{Op: "UpdateSettingValue", Err: errors.New("key cannot be empty")}
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
	if err != nil || setting == nil || setting.ID <= 0 {
		return defaultValue, nil
	}

	return setting.Value, nil
}

// GetBoolValue retrieves the value of a setting as a boolean
func (s *service) GetBoolValue(ctx context.Context, key string, defaultValue bool) (bool, error) {
	setting, err := s.settingRepo.FindByKey(ctx, key)
	if err != nil || setting == nil || setting.ID <= 0 {
		return defaultValue, nil
	}

	value := strings.ToLower(setting.Value)
	if value == "true" || value == "1" || value == "yes" || value == "y" {
		return true, nil
	} else if value == "false" || value == "0" || value == "no" || value == "n" {
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
	if err != nil || setting == nil || setting.ID <= 0 {
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
	if err != nil || setting == nil || setting.ID <= 0 {
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
		return nil, &ConfigError{Op: "GetSettingsByCategory", Err: errors.New("category cannot be empty")}
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
		return nil, &ConfigError{Op: "GetSettingByKeyAndCategory", Err: errors.New("key and category cannot be empty")}
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

// ImportSettings imports multiple settings in a batch operation
func (s *service) ImportSettings(ctx context.Context, settings []*config.Setting) ([]error, error) {
	if len(settings) == 0 {
		return nil, nil
	}

	var errors []error

	// Execute in transaction using txHandler
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(Service)

		// Process all settings
		for _, setting := range settings {
			// Check if setting exists
			existingSetting, err := txService.GetSettingByKeyAndCategory(ctx, setting.Key, setting.Category)
			if err == nil && existingSetting != nil && existingSetting.ID > 0 {
				// Update existing setting
				existingSetting.Value = setting.Value
				existingSetting.Description = setting.Description
				existingSetting.RequiresRestart = setting.RequiresRestart
				existingSetting.RequiresDBReset = setting.RequiresDBReset

				if err := txService.UpdateSetting(ctx, existingSetting); err != nil {
					errors = append(errors, &ConfigError{Op: "ImportSettings", Err: err})
				}
			} else {
				// Create new setting
				if err := txService.CreateSetting(ctx, setting); err != nil {
					errors = append(errors, &ConfigError{Op: "ImportSettings", Err: err})
				}
			}
		}

		// If any errors occurred, rollback the transaction
		if len(errors) > 0 {
			return fmt.Errorf("import failed with %d errors", len(errors))
		}

		return nil
	})

	if err != nil {
		// Include transaction error in the list
		errors = append(errors, &ConfigError{Op: "ImportSettings", Err: err})
	}

	if len(errors) > 0 {
		return errors, &BatchOperationError{Errors: errors}
	}

	return nil, nil
}

// InitializeDefaultSettings initializes default settings if they don't exist
func (s *service) InitializeDefaultSettings(ctx context.Context) error {
	// Define default settings
	defaultSettings := []*config.Setting{
		// System settings
		{
			Key:             "app_name",
			Value:           "Project Phoenix",
			Category:        "system",
			Description:     "The name of the application",
			RequiresRestart: false,
			RequiresDBReset: false,
		},
		{
			Key:             "version",
			Value:           "1.0.0",
			Category:        "system",
			Description:     "The version of the application",
			RequiresRestart: false,
			RequiresDBReset: false,
		},
		{
			Key:             "debug_mode",
			Value:           "false",
			Category:        "system",
			Description:     "Enable debug mode",
			RequiresRestart: true,
			RequiresDBReset: false,
		},
		// Email settings
		{
			Key:             "smtp_host",
			Value:           "smtp.example.com",
			Category:        "email",
			Description:     "SMTP server hostname",
			RequiresRestart: true,
			RequiresDBReset: false,
		},
		{
			Key:             "smtp_port",
			Value:           "587",
			Category:        "email",
			Description:     "SMTP server port",
			RequiresRestart: true,
			RequiresDBReset: false,
		},
		{
			Key:             "smtp_username",
			Value:           "",
			Category:        "email",
			Description:     "SMTP server username",
			RequiresRestart: true,
			RequiresDBReset: false,
		},
		{
			Key:             "smtp_password",
			Value:           "",
			Category:        "email",
			Description:     "SMTP server password",
			RequiresRestart: true,
			RequiresDBReset: false,
		},
		{
			Key:             "email_from",
			Value:           "no-reply@example.com",
			Category:        "email",
			Description:     "Email from address",
			RequiresRestart: false,
			RequiresDBReset: false,
		},
		// Security settings
		{
			Key:             "password_min_length",
			Value:           "8",
			Category:        "security",
			Description:     "Minimum password length",
			RequiresRestart: false,
			RequiresDBReset: false,
		},
		{
			Key:             "password_require_uppercase",
			Value:           "true",
			Category:        "security",
			Description:     "Require uppercase letters in passwords",
			RequiresRestart: false,
			RequiresDBReset: false,
		},
		{
			Key:             "password_require_lowercase",
			Value:           "true",
			Category:        "security",
			Description:     "Require lowercase letters in passwords",
			RequiresRestart: false,
			RequiresDBReset: false,
		},
		{
			Key:             "password_require_numbers",
			Value:           "true",
			Category:        "security",
			Description:     "Require numbers in passwords",
			RequiresRestart: false,
			RequiresDBReset: false,
		},
		{
			Key:             "password_require_special",
			Value:           "false",
			Category:        "security",
			Description:     "Require special characters in passwords",
			RequiresRestart: false,
			RequiresDBReset: false,
		},
		{
			Key:             "session_timeout",
			Value:           "3600",
			Category:        "security",
			Description:     "Session timeout in seconds",
			RequiresRestart: false,
			RequiresDBReset: false,
		},
	}

	// Execute in transaction using txHandler
	return s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(Service)

		for _, setting := range defaultSettings {
			// Check if setting exists
			existingSetting, err := txService.GetSettingByKeyAndCategory(ctx, setting.Key, setting.Category)
			if err != nil || existingSetting == nil || existingSetting.ID <= 0 {
				// Create if it doesn't exist
				if err := txService.CreateSetting(ctx, setting); err != nil {
					return &ConfigError{Op: "InitializeDefaultSettings", Err: err}
				}
			}
		}
		return nil
	})
}

// RequiresRestart checks if any modified settings require a system restart
func (s *service) RequiresRestart(ctx context.Context) (bool, error) {
	filters := map[string]interface{}{
		"requires_restart": true,
	}

	settings, err := s.settingRepo.List(ctx, filters)
	if err != nil {
		return false, &ConfigError{Op: "RequiresRestart", Err: err}
	}

	return len(settings) > 0, nil
}

// RequiresDatabaseReset checks if any modified settings require a database reset
func (s *service) RequiresDatabaseReset(ctx context.Context) (bool, error) {
	filters := map[string]interface{}{
		"requires_db_reset": true,
	}

	settings, err := s.settingRepo.List(ctx, filters)
	if err != nil {
		return false, &ConfigError{Op: "RequiresDatabaseReset", Err: err}
	}

	return len(settings) > 0, nil
}
