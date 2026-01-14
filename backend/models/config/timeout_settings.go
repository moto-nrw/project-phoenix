package config

import (
	"errors"
	"time"
)

// TimeoutSettings represents session timeout configuration
type TimeoutSettings struct {
	GlobalTimeoutMinutes    int       `json:"global_timeout_minutes"`           // Default timeout for all sessions
	DeviceTimeoutMinutes    *int      `json:"device_timeout_minutes,omitempty"` // Device-specific override
	WarningThresholdMinutes int       `json:"warning_threshold_minutes"`        // When to warn about upcoming timeout
	CheckIntervalSeconds    int       `json:"check_interval_seconds"`           // How often device should check for timeout
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// Validate ensures timeout settings are valid
func (ts *TimeoutSettings) Validate() error {
	if ts.GlobalTimeoutMinutes <= 0 {
		return errors.New("global timeout minutes must be positive")
	}

	if ts.GlobalTimeoutMinutes > 480 { // Max 8 hours
		return errors.New("global timeout minutes cannot exceed 480 (8 hours)")
	}

	if ts.DeviceTimeoutMinutes != nil {
		if *ts.DeviceTimeoutMinutes <= 0 {
			return errors.New("device timeout minutes must be positive")
		}

		if *ts.DeviceTimeoutMinutes > 480 { // Max 8 hours
			return errors.New("device timeout minutes cannot exceed 480 (8 hours)")
		}
	}

	if ts.WarningThresholdMinutes <= 0 {
		return errors.New("warning threshold minutes must be positive")
	}

	if ts.CheckIntervalSeconds <= 0 {
		return errors.New("check interval seconds must be positive")
	}

	if ts.CheckIntervalSeconds > 300 { // Max 5 minutes
		return errors.New("check interval seconds cannot exceed 300 (5 minutes)")
	}

	// Warning threshold should be less than timeout
	effectiveTimeout := ts.GlobalTimeoutMinutes
	if ts.DeviceTimeoutMinutes != nil {
		effectiveTimeout = *ts.DeviceTimeoutMinutes
	}

	if ts.WarningThresholdMinutes >= effectiveTimeout {
		return errors.New("warning threshold must be less than timeout duration")
	}

	return nil
}

// GetEffectiveTimeoutMinutes returns the effective timeout (device-specific or global)
func (ts *TimeoutSettings) GetEffectiveTimeoutMinutes() int {
	if ts.DeviceTimeoutMinutes != nil {
		return *ts.DeviceTimeoutMinutes
	}
	return ts.GlobalTimeoutMinutes
}

// GetTimeoutDuration returns the effective timeout as a duration
func (ts *TimeoutSettings) GetTimeoutDuration() time.Duration {
	return time.Duration(ts.GetEffectiveTimeoutMinutes()) * time.Minute
}

// GetWarningDuration returns the warning threshold as a duration
func (ts *TimeoutSettings) GetWarningDuration() time.Duration {
	return time.Duration(ts.WarningThresholdMinutes) * time.Minute
}

// GetCheckInterval returns the check interval as a duration
func (ts *TimeoutSettings) GetCheckInterval() time.Duration {
	return time.Duration(ts.CheckIntervalSeconds) * time.Second
}

// SetDeviceTimeout sets a device-specific timeout override
func (ts *TimeoutSettings) SetDeviceTimeout(minutes int) error {
	if minutes <= 0 {
		return errors.New("device timeout minutes must be positive")
	}

	if minutes > 480 {
		return errors.New("device timeout minutes cannot exceed 480 (8 hours)")
	}

	ts.DeviceTimeoutMinutes = &minutes
	ts.UpdatedAt = time.Now()
	return nil
}

// ClearDeviceTimeout removes device-specific timeout override
func (ts *TimeoutSettings) ClearDeviceTimeout() {
	ts.DeviceTimeoutMinutes = nil
	ts.UpdatedAt = time.Now()
}
