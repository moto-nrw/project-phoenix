package config

import (
	"context"
	"fmt"
	"strconv"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// GetTimeoutSettings retrieves the global timeout configuration
func (s *service) GetTimeoutSettings(ctx context.Context) (*config.TimeoutSettings, error) {
	// Get global timeout setting
	globalTimeoutStr, err := s.GetStringValue(ctx, "session_timeout_minutes", "30")
	if err != nil {
		return nil, &ConfigError{Op: "GetTimeoutSettings", Err: fmt.Errorf("failed to get global timeout: %w", err)}
	}

	globalTimeout, err := strconv.Atoi(globalTimeoutStr)
	if err != nil {
		return nil, &ConfigError{Op: "GetTimeoutSettings", Err: fmt.Errorf("invalid global timeout value: %w", err)}
	}

	// Get warning threshold setting
	warningThresholdStr, err := s.GetStringValue(ctx, "session_warning_threshold_minutes", "5")
	if err != nil {
		return nil, &ConfigError{Op: "GetTimeoutSettings", Err: fmt.Errorf("failed to get warning threshold: %w", err)}
	}

	warningThreshold, err := strconv.Atoi(warningThresholdStr)
	if err != nil {
		return nil, &ConfigError{Op: "GetTimeoutSettings", Err: fmt.Errorf("invalid warning threshold value: %w", err)}
	}

	// Get check interval setting
	checkIntervalStr, err := s.GetStringValue(ctx, "session_check_interval_seconds", "30")
	if err != nil {
		return nil, &ConfigError{Op: "GetTimeoutSettings", Err: fmt.Errorf("failed to get check interval: %w", err)}
	}

	checkInterval, err := strconv.Atoi(checkIntervalStr)
	if err != nil {
		return nil, &ConfigError{Op: "GetTimeoutSettings", Err: fmt.Errorf("invalid check interval value: %w", err)}
	}

	settings := &config.TimeoutSettings{
		GlobalTimeoutMinutes:    globalTimeout,
		WarningThresholdMinutes: warningThreshold,
		CheckIntervalSeconds:    checkInterval,
	}

	return settings, nil
}

// UpdateTimeoutSettings updates the global timeout configuration
func (s *service) UpdateTimeoutSettings(ctx context.Context, settings *config.TimeoutSettings) error {
	if err := settings.Validate(); err != nil {
		return &ConfigError{Op: "UpdateTimeoutSettings", Err: fmt.Errorf("invalid timeout settings: %w", err)}
	}

	// Update settings using transactions to ensure atomicity
	return s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(Service)

		// Update global timeout
		if err := txService.UpdateSettingValue(ctx, "session_timeout_minutes", strconv.Itoa(settings.GlobalTimeoutMinutes)); err != nil {
			return fmt.Errorf("failed to update global timeout: %w", err)
		}

		// Update warning threshold
		if err := txService.UpdateSettingValue(ctx, "session_warning_threshold_minutes", strconv.Itoa(settings.WarningThresholdMinutes)); err != nil {
			return fmt.Errorf("failed to update warning threshold: %w", err)
		}

		// Update check interval
		if err := txService.UpdateSettingValue(ctx, "session_check_interval_seconds", strconv.Itoa(settings.CheckIntervalSeconds)); err != nil {
			return fmt.Errorf("failed to update check interval: %w", err)
		}

		return nil
	})
}

// GetDeviceTimeoutSettings retrieves timeout settings for a specific device
func (s *service) GetDeviceTimeoutSettings(ctx context.Context, deviceID int64) (*config.TimeoutSettings, error) {
	// Start with global settings
	settings, err := s.GetTimeoutSettings(ctx)
	if err != nil {
		return nil, &ConfigError{Op: "GetDeviceTimeoutSettings", Err: fmt.Errorf("failed to get global settings: %w", err)}
	}

	// Check for device-specific timeout override
	deviceKey := fmt.Sprintf("device_%d_timeout_minutes", deviceID)
	deviceTimeoutStr, err := s.GetStringValue(ctx, deviceKey, "")
	if err != nil {
		// No device-specific setting found, return global settings
		return settings, nil
	}

	if deviceTimeoutStr != "" {
		deviceTimeout, parseErr := strconv.Atoi(deviceTimeoutStr)
		if parseErr != nil {
			return nil, &ConfigError{Op: "GetDeviceTimeoutSettings", Err: fmt.Errorf("invalid device timeout value: %w", parseErr)}
		}

		// Apply device-specific override
		settings.DeviceTimeoutMinutes = &deviceTimeout
	}

	return settings, nil
}
