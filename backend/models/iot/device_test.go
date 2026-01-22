package iot

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestDevice_Validate(t *testing.T) {
	tests := []struct {
		name    string
		device  Device
		wantErr bool
	}{
		{
			name: "Valid device",
			device: Device{
				DeviceID:   "dev-001",
				DeviceType: "sensor",
				Status:     DeviceStatusActive,
			},
			wantErr: false,
		},
		{
			name: "Empty device ID",
			device: Device{
				DeviceID:   "",
				DeviceType: "sensor",
				Status:     DeviceStatusActive,
			},
			wantErr: true,
		},
		{
			name: "Empty device type",
			device: Device{
				DeviceID:   "dev-001",
				DeviceType: "",
				Status:     DeviceStatusActive,
			},
			wantErr: true,
		},
		{
			name: "Invalid status",
			device: Device{
				DeviceID:   "dev-001",
				DeviceType: "sensor",
				Status:     "invalid_status",
			},
			wantErr: true,
		},
		{
			name: "Empty status defaulting to active",
			device: Device{
				DeviceID:   "dev-001",
				DeviceType: "sensor",
				Status:     "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.device.Validate()

			// Check error condition
			if (err != nil) != tt.wantErr {
				t.Errorf("Device.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check default status assignment
			if tt.name == "Empty status defaulting to active" && tt.device.Status != DeviceStatusActive {
				t.Errorf("Status was not defaulted to active, got %s", tt.device.Status)
			}
		})
	}
}

func TestDevice_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   DeviceStatus
		expected bool
	}{
		{
			name:     "Active device",
			status:   DeviceStatusActive,
			expected: true,
		},
		{
			name:     "Inactive device",
			status:   DeviceStatusInactive,
			expected: false,
		},
		{
			name:     "Maintenance device",
			status:   DeviceStatusMaintenance,
			expected: false,
		},
		{
			name:     "Offline device",
			status:   DeviceStatusOffline,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device := Device{
				DeviceID:   "dev-001",
				DeviceType: "sensor",
				Status:     tt.status,
			}

			if got := device.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestDevice_UpdateLastSeen(t *testing.T) {
	device := Device{
		DeviceID:   "dev-001",
		DeviceType: "sensor",
	}

	// Verify LastSeen is initially nil
	if device.LastSeen != nil {
		t.Error("LastSeen should initially be nil")
	}

	// Update LastSeen
	before := time.Now()
	device.UpdateLastSeen()
	after := time.Now()

	// Verify LastSeen was updated and falls within expected range
	if device.LastSeen == nil {
		t.Error("UpdateLastSeen() should set the LastSeen field")
	} else {
		if device.LastSeen.Before(before) || device.LastSeen.After(after) {
			t.Errorf("LastSeen time %v should be between %v and %v",
				device.LastSeen, before, after)
		}
	}
}

func TestDevice_SetStatus(t *testing.T) {
	device := Device{
		DeviceID:   "dev-001",
		DeviceType: "sensor",
		Status:     DeviceStatusActive,
	}

	// Test setting to a valid status
	err := device.SetStatus(DeviceStatusMaintenance)
	if err != nil {
		t.Errorf("SetStatus() returned unexpected error: %v", err)
	}
	if device.Status != DeviceStatusMaintenance {
		t.Errorf("Status should be %s, got %s", DeviceStatusMaintenance, device.Status)
	}

	// Test setting to an invalid status
	err = device.SetStatus("invalid_status")
	if err == nil {
		t.Error("SetStatus() with invalid status should return an error")
	}
	if device.Status != DeviceStatusMaintenance {
		t.Error("Status should not change when SetStatus fails")
	}
}

func TestDevice_GetLastSeenDuration(t *testing.T) {
	// Test with nil LastSeen
	device := Device{
		DeviceID:   "dev-001",
		DeviceType: "sensor",
		LastSeen:   nil,
	}

	duration := device.GetLastSeenDuration()
	if duration != nil {
		t.Errorf("GetLastSeenDuration() with nil LastSeen should return nil, got %v", duration)
	}

	// Test with a specific LastSeen time
	pastTime := time.Now().Add(-1 * time.Hour)
	device.LastSeen = &pastTime

	duration = device.GetLastSeenDuration()
	if duration == nil {
		t.Error("GetLastSeenDuration() should return a duration, got nil")
	} else {
		// The duration should be approximately 1 hour, but allow some flexibility
		// since there's elapsed time between setting pastTime and calling GetLastSeenDuration
		if *duration < 59*time.Minute || *duration > 61*time.Minute {
			t.Errorf("Expected duration ~1 hour, got %v", *duration)
		}
	}
}

func TestDevice_IsOnline(t *testing.T) {
	// Test with nil LastSeen
	device := Device{
		DeviceID:   "dev-001",
		DeviceType: "sensor",
		LastSeen:   nil,
	}

	if device.IsOnline() {
		t.Error("Device with nil LastSeen should not be online")
	}

	// Test with recent LastSeen (within 5 minutes)
	recentTime := time.Now().Add(-3 * time.Minute)
	device.LastSeen = &recentTime

	if !device.IsOnline() {
		t.Error("Device seen 3 minutes ago should be online")
	}

	// Test with old LastSeen (more than 5 minutes ago)
	oldTime := time.Now().Add(-10 * time.Minute)
	device.LastSeen = &oldTime

	if device.IsOnline() {
		t.Error("Device seen 10 minutes ago should not be online")
	}
}

func TestDevice_HasAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   *string
		expected bool
	}{
		{
			name:     "nil API key",
			apiKey:   nil,
			expected: false,
		},
		{
			name:     "empty API key",
			apiKey:   base.StringPtr(""),
			expected: false,
		},
		{
			name:     "valid API key",
			apiKey:   base.StringPtr("abc123xyz"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device := &Device{
				DeviceID:   "dev-001",
				DeviceType: "sensor",
				APIKey:     tt.apiKey,
			}

			if got := device.HasAPIKey(); got != tt.expected {
				t.Errorf("Device.HasAPIKey() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDevice_BeforeAppendModel(t *testing.T) {
	t.Run("handles nil query", func(t *testing.T) {
		device := &Device{
			DeviceID:   "dev-001",
			DeviceType: "sensor",
			Status:     DeviceStatusActive,
		}
		err := device.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		device := &Device{
			DeviceID:   "dev-001",
			DeviceType: "sensor",
			Status:     DeviceStatusActive,
		}
		err := device.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestDevice_TableName(t *testing.T) {
	device := &Device{}
	if got := device.TableName(); got != "iot.devices" {
		t.Errorf("TableName() = %v, want iot.devices", got)
	}
}

func TestDevice_IsOffline(t *testing.T) {
	tests := []struct {
		name     string
		status   DeviceStatus
		expected bool
	}{
		{
			name:     "Offline device",
			status:   DeviceStatusOffline,
			expected: true,
		},
		{
			name:     "Active device",
			status:   DeviceStatusActive,
			expected: false,
		},
		{
			name:     "Inactive device",
			status:   DeviceStatusInactive,
			expected: false,
		},
		{
			name:     "Maintenance device",
			status:   DeviceStatusMaintenance,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device := Device{
				DeviceID:   "dev-001",
				DeviceType: "sensor",
				Status:     tt.status,
			}

			if got := device.IsOffline(); got != tt.expected {
				t.Errorf("IsOffline() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestDevice_GetID(t *testing.T) {
	device := &Device{
		TenantModel: base.TenantModel{Model: base.Model{ID: 42}},
		DeviceID:    "dev-001",
		DeviceType:  "sensor",
		Status:      DeviceStatusActive,
	}

	if got, ok := device.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", device.GetID())
	}
}

func TestDevice_GetCreatedAt(t *testing.T) {
	now := time.Now()
	device := &Device{
		TenantModel: base.TenantModel{Model: base.Model{CreatedAt: now}},
		DeviceID:    "dev-001",
		DeviceType:  "sensor",
		Status:      DeviceStatusActive,
	}

	if got := device.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestDevice_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	device := &Device{
		TenantModel: base.TenantModel{Model: base.Model{UpdatedAt: now}},
		DeviceID:    "dev-001",
		DeviceType:  "sensor",
		Status:      DeviceStatusActive,
	}

	if got := device.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
