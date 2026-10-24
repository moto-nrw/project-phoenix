package devicefleet

import (
	"testing"
)

func TestDevice_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		device  Device
		wantErr bool
	}{
		{
			name: "Valid device",
			device: Device{
				DeviceID:   "dev-001",
				DeviceType: "terminal",
				Status:     DeviceStatusActive,
			},
			wantErr: false,
		},
		{
			name: "Empty device ID",
			device: Device{
				DeviceID:   "",
				DeviceType: "terminal",
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
				DeviceType: "terminal",
				Status:     "invalid_status",
			},
			wantErr: true,
		},
		{
			name: "Empty status defaulting to active",
			device: Device{
				DeviceID:   "dev-001",
				DeviceType: "terminal",
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
	t.Parallel()

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
				DeviceType: "terminal",
				Status:     tt.status,
			}

			if got := device.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
