package iot

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIoTError_Error(t *testing.T) {
	originalErr := errors.New("connection timeout")
	err := &IoTError{
		Op:  "RegisterDevice",
		Err: originalErr,
	}

	expected := "IoT service error in RegisterDevice: connection timeout"
	assert.Equal(t, expected, err.Error())
}

func TestIoTError_Unwrap(t *testing.T) {
	originalErr := errors.New("underlying error")
	err := &IoTError{
		Op:  "UpdateDevice",
		Err: originalErr,
	}

	assert.Equal(t, originalErr, err.Unwrap())
}

func TestIoTError_ErrorsIs(t *testing.T) {
	err := &IoTError{
		Op:  "GetDevice",
		Err: ErrDeviceNotFound,
	}

	assert.True(t, errors.Is(err, ErrDeviceNotFound))
	assert.False(t, errors.Is(err, ErrDuplicateDeviceID))
}

func TestDeviceNotFoundError_Error(t *testing.T) {
	err := &DeviceNotFoundError{DeviceID: "device-123"}
	assert.Equal(t, "device not found: device-123", err.Error())
}

func TestDeviceNotFoundError_Unwrap(t *testing.T) {
	err := &DeviceNotFoundError{DeviceID: "device-123"}
	assert.Equal(t, ErrDeviceNotFound, err.Unwrap())
	assert.True(t, errors.Is(err, ErrDeviceNotFound))
}

func TestDuplicateDeviceIDError_Error(t *testing.T) {
	err := &DuplicateDeviceIDError{DeviceID: "device-456"}
	assert.Equal(t, "duplicate device ID: device-456", err.Error())
}

func TestDuplicateDeviceIDError_Unwrap(t *testing.T) {
	err := &DuplicateDeviceIDError{DeviceID: "device-456"}
	assert.Equal(t, ErrDuplicateDeviceID, err.Unwrap())
	assert.True(t, errors.Is(err, ErrDuplicateDeviceID))
}

func TestIoTError_AllStandardErrors(t *testing.T) {
	tests := []struct {
		name        string
		op          string
		err         error
		wantContain string
	}{
		{
			name:        "device not found",
			op:          "GetDevice",
			err:         ErrDeviceNotFound,
			wantContain: "device not found",
		},
		{
			name:        "invalid device data",
			op:          "CreateDevice",
			err:         ErrInvalidDeviceData,
			wantContain: "invalid device data",
		},
		{
			name:        "duplicate device ID",
			op:          "RegisterDevice",
			err:         ErrDuplicateDeviceID,
			wantContain: "duplicate device ID",
		},
		{
			name:        "invalid status",
			op:          "UpdateStatus",
			err:         ErrInvalidStatus,
			wantContain: "invalid device status",
		},
		{
			name:        "device offline",
			op:          "SendCommand",
			err:         ErrDeviceOffline,
			wantContain: "device is offline",
		},
		{
			name:        "network scan failed",
			op:          "ScanNetwork",
			err:         ErrNetworkScanFailed,
			wantContain: "network scan failed",
		},
		{
			name:        "database operation",
			op:          "SaveDevice",
			err:         ErrDatabaseOperation,
			wantContain: "database operation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &IoTError{
				Op:  tt.op,
				Err: tt.err,
			}
			assert.Contains(t, err.Error(), tt.wantContain)
			assert.Contains(t, err.Error(), tt.op)
		})
	}
}

func TestDeviceNotFoundError_DifferentDeviceIDs(t *testing.T) {
	tests := []struct {
		deviceID string
	}{
		{deviceID: "device-001"},
		{deviceID: "rfid-reader-123"},
		{deviceID: "scanner-abc"},
		{deviceID: ""},
	}

	for _, tt := range tests {
		t.Run(tt.deviceID, func(t *testing.T) {
			err := &DeviceNotFoundError{DeviceID: tt.deviceID}
			assert.Contains(t, err.Error(), tt.deviceID)
			assert.True(t, errors.Is(err, ErrDeviceNotFound))
		})
	}
}

func TestDuplicateDeviceIDError_DifferentDeviceIDs(t *testing.T) {
	tests := []struct {
		deviceID string
	}{
		{deviceID: "device-001"},
		{deviceID: "rfid-reader-123"},
		{deviceID: "scanner-abc"},
		{deviceID: ""},
	}

	for _, tt := range tests {
		t.Run(tt.deviceID, func(t *testing.T) {
			err := &DuplicateDeviceIDError{DeviceID: tt.deviceID}
			assert.Contains(t, err.Error(), tt.deviceID)
			assert.True(t, errors.Is(err, ErrDuplicateDeviceID))
		})
	}
}

func TestIoTError_ChainedWrapping(t *testing.T) {
	// Test multiple levels of error wrapping
	baseErr := errors.New("network connection lost")
	wrapped1 := &IoTError{
		Op:  "ConnectDevice",
		Err: baseErr,
	}
	wrapped2 := &IoTError{
		Op:  "InitializeNetwork",
		Err: wrapped1,
	}

	// Should unwrap through the chain
	assert.True(t, errors.Is(wrapped2, baseErr))
	assert.Contains(t, wrapped2.Error(), "InitializeNetwork")
	assert.Contains(t, wrapped2.Error(), "ConnectDevice")
}

func TestIoTError_NestedDeviceErrors(t *testing.T) {
	// Test wrapping specific device errors in IoTError
	deviceErr := &DeviceNotFoundError{DeviceID: "device-999"}
	wrappedErr := &IoTError{
		Op:  "GetDeviceStatus",
		Err: deviceErr,
	}

	// Should be able to unwrap to both error types
	assert.True(t, errors.Is(wrappedErr, ErrDeviceNotFound))
	assert.Contains(t, wrappedErr.Error(), "device-999")
	assert.Contains(t, wrappedErr.Error(), "GetDeviceStatus")
}
