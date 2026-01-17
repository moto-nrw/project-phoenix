package config

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/config"
	"github.com/uptrace/bun"
)

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
