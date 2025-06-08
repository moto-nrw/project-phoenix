package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeoutSettings_Validate(t *testing.T) {
	tests := []struct {
		name        string
		settings    *TimeoutSettings
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid default settings",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes:    30,
				WarningThresholdMinutes: 5,
				CheckIntervalSeconds:    30,
			},
			expectError: false,
		},
		{
			name: "valid with device override",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes:    30,
				DeviceTimeoutMinutes:    func(v int) *int { return &v }(45),
				WarningThresholdMinutes: 10,
				CheckIntervalSeconds:    60,
			},
			expectError: false,
		},
		{
			name: "invalid global timeout - zero",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes:    0,
				WarningThresholdMinutes: 5,
				CheckIntervalSeconds:    30,
			},
			expectError: true,
			errorMsg:    "global timeout minutes must be positive",
		},
		{
			name: "invalid global timeout - too high",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes:    500,
				WarningThresholdMinutes: 5,
				CheckIntervalSeconds:    30,
			},
			expectError: true,
			errorMsg:    "global timeout minutes cannot exceed 480",
		},
		{
			name: "invalid device timeout - zero",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes:    30,
				DeviceTimeoutMinutes:    func(v int) *int { return &v }(0),
				WarningThresholdMinutes: 5,
				CheckIntervalSeconds:    30,
			},
			expectError: true,
			errorMsg:    "device timeout minutes must be positive",
		},
		{
			name: "invalid device timeout - too high",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes:    30,
				DeviceTimeoutMinutes:    func(v int) *int { return &v }(500),
				WarningThresholdMinutes: 5,
				CheckIntervalSeconds:    30,
			},
			expectError: true,
			errorMsg:    "device timeout minutes cannot exceed 480",
		},
		{
			name: "invalid warning threshold - zero",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes:    30,
				WarningThresholdMinutes: 0,
				CheckIntervalSeconds:    30,
			},
			expectError: true,
			errorMsg:    "warning threshold minutes must be positive",
		},
		{
			name: "invalid check interval - zero",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes:    30,
				WarningThresholdMinutes: 5,
				CheckIntervalSeconds:    0,
			},
			expectError: true,
			errorMsg:    "check interval seconds must be positive",
		},
		{
			name: "invalid check interval - too high",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes:    30,
				WarningThresholdMinutes: 5,
				CheckIntervalSeconds:    400,
			},
			expectError: true,
			errorMsg:    "check interval seconds cannot exceed 300",
		},
		{
			name: "warning threshold >= global timeout",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes:    30,
				WarningThresholdMinutes: 30,
				CheckIntervalSeconds:    30,
			},
			expectError: true,
			errorMsg:    "warning threshold must be less than timeout duration",
		},
		{
			name: "warning threshold >= device timeout",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes:    30,
				DeviceTimeoutMinutes:    func(v int) *int { return &v }(20),
				WarningThresholdMinutes: 25,
				CheckIntervalSeconds:    30,
			},
			expectError: true,
			errorMsg:    "warning threshold must be less than timeout duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTimeoutSettings_GetEffectiveTimeoutMinutes(t *testing.T) {
	tests := []struct {
		name               string
		settings           *TimeoutSettings
		expectedTimeout    int
	}{
		{
			name: "use global timeout when no device override",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes: 30,
			},
			expectedTimeout: 30,
		},
		{
			name: "use device timeout when override provided",
			settings: &TimeoutSettings{
				GlobalTimeoutMinutes: 30,
				DeviceTimeoutMinutes: func(v int) *int { return &v }(45),
			},
			expectedTimeout: 45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.settings.GetEffectiveTimeoutMinutes()
			assert.Equal(t, tt.expectedTimeout, result)
		})
	}
}

func TestTimeoutSettings_GetTimeoutDuration(t *testing.T) {
	settings := &TimeoutSettings{
		GlobalTimeoutMinutes: 45,
	}

	duration := settings.GetTimeoutDuration()
	assert.Equal(t, 45*time.Minute, duration)
}

func TestTimeoutSettings_GetWarningDuration(t *testing.T) {
	settings := &TimeoutSettings{
		WarningThresholdMinutes: 10,
	}

	duration := settings.GetWarningDuration()
	assert.Equal(t, 10*time.Minute, duration)
}

func TestTimeoutSettings_GetCheckInterval(t *testing.T) {
	settings := &TimeoutSettings{
		CheckIntervalSeconds: 120,
	}

	duration := settings.GetCheckInterval()
	assert.Equal(t, 120*time.Second, duration)
}

func TestTimeoutSettings_SetDeviceTimeout(t *testing.T) {
	tests := []struct {
		name        string
		minutes     int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid device timeout",
			minutes:     60,
			expectError: false,
		},
		{
			name:        "invalid device timeout - zero",
			minutes:     0,
			expectError: true,
			errorMsg:    "device timeout minutes must be positive",
		},
		{
			name:        "invalid device timeout - negative",
			minutes:     -10,
			expectError: true,
			errorMsg:    "device timeout minutes must be positive",
		},
		{
			name:        "invalid device timeout - too high",
			minutes:     500,
			expectError: true,
			errorMsg:    "device timeout minutes cannot exceed 480",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &TimeoutSettings{
				GlobalTimeoutMinutes: 30,
			}

			err := settings.SetDeviceTimeout(tt.minutes)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, settings.DeviceTimeoutMinutes)
			} else {
				require.NoError(t, err)
				require.NotNil(t, settings.DeviceTimeoutMinutes)
				assert.Equal(t, tt.minutes, *settings.DeviceTimeoutMinutes)
			}
		})
	}
}

func TestTimeoutSettings_ClearDeviceTimeout(t *testing.T) {
	settings := &TimeoutSettings{
		GlobalTimeoutMinutes: 30,
		DeviceTimeoutMinutes: func(v int) *int { return &v }(45),
	}

	// Verify device timeout is set
	require.NotNil(t, settings.DeviceTimeoutMinutes)
	assert.Equal(t, 45, *settings.DeviceTimeoutMinutes)

	// Clear device timeout
	settings.ClearDeviceTimeout()

	// Verify device timeout is cleared
	assert.Nil(t, settings.DeviceTimeoutMinutes)
}

func TestNewDefaultTimeoutSettings(t *testing.T) {
	settings := NewDefaultTimeoutSettings()

	assert.Equal(t, 30, settings.GlobalTimeoutMinutes)
	assert.Nil(t, settings.DeviceTimeoutMinutes)
	assert.Equal(t, 5, settings.WarningThresholdMinutes)
	assert.Equal(t, 30, settings.CheckIntervalSeconds)
	assert.False(t, settings.CreatedAt.IsZero())
	assert.False(t, settings.UpdatedAt.IsZero())

	// Validate the default settings
	err := settings.Validate()
	assert.NoError(t, err)
}