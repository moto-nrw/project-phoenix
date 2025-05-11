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
	ErrDatabaseOperation = errors.New("database operation failed")
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

// InvalidDeviceDataError wraps a validation error for a device
type InvalidDeviceDataError struct {
	Err error
}

func (e *InvalidDeviceDataError) Error() string {
	return fmt.Sprintf("invalid device data: %v", e.Err)
}

func (e *InvalidDeviceDataError) Unwrap() error {
	return ErrInvalidDeviceData
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

// DeviceOfflineError wraps a device offline error
type DeviceOfflineError struct {
	DeviceID string
	LastSeen *string
}

func (e *DeviceOfflineError) Error() string {
	if e.LastSeen != nil {
		return fmt.Sprintf("device %s is offline (last seen: %s)", e.DeviceID, *e.LastSeen)
	}
	return fmt.Sprintf("device %s is offline (never seen)", e.DeviceID)
}

func (e *DeviceOfflineError) Unwrap() error {
	return ErrDeviceOffline
}

// NetworkScanError wraps a network scan error
type NetworkScanError struct {
	Err error
}

func (e *NetworkScanError) Error() string {
	return fmt.Sprintf("network scan failed: %v", e.Err)
}

func (e *NetworkScanError) Unwrap() error {
	return ErrNetworkScanFailed
}
