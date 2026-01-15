package iot

import (
	"errors"
	"fmt"
)

// Common error types
var (
	ErrDeviceNotFound    = errors.New("device not found")
	ErrInvalidDeviceData = errors.New("invalid device data")
	ErrDuplicateDeviceID = errors.New("duplicate device ID")
	ErrInvalidStatus     = errors.New("invalid device status")
	ErrDeviceOffline     = errors.New("device is offline")
	ErrNetworkScanFailed = errors.New("network scan failed")
)

// IoTError wraps IoT service errors with operation context
type IoTError struct {
	Op  string // The operation that failed
	Err error  // The underlying error
}

func (e *IoTError) Error() string {
	return fmt.Sprintf("IoT service error in %s: %v", e.Op, e.Err)
}

func (e *IoTError) Unwrap() error {
	return e.Err
}

// DeviceNotFoundError wraps a device not found error
type DeviceNotFoundError struct {
	DeviceID string
}

func (e *DeviceNotFoundError) Error() string {
	return fmt.Sprintf("device not found: %s", e.DeviceID)
}

func (e *DeviceNotFoundError) Unwrap() error {
	return ErrDeviceNotFound
}

// DuplicateDeviceIDError wraps a duplicate device ID error
type DuplicateDeviceIDError struct {
	DeviceID string
}

func (e *DuplicateDeviceIDError) Error() string {
	return fmt.Sprintf("duplicate device ID: %s", e.DeviceID)
}

func (e *DuplicateDeviceIDError) Unwrap() error {
	return ErrDuplicateDeviceID
}
