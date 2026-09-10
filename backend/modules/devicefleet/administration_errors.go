package devicefleet

import (
	"errors"
	"fmt"
)

// Common error types
var (
	ErrAdministrationDeviceNotFound    = errors.New("device not found")
	ErrAdministrationInvalidDeviceData = errors.New("invalid device data")
	ErrAdministrationDuplicateDeviceID = errors.New("Diese Geräte-ID ist bereits vergeben") //nolint:staticcheck // ST1005: user-facing German message
	ErrAdministrationInvalidStatus     = errors.New("invalid device status")
	ErrAdministrationDeviceOffline     = errors.New("device is offline")
	ErrAdministrationNetworkScanFailed = errors.New("network scan failed")
	ErrAdministrationDatabaseOperation = errors.New("database operation failed")
	ErrAdministrationDeviceProtected   = errors.New("Systemgerät kann nicht geändert oder gelöscht werden") //nolint:staticcheck // ST1005: user-facing German message
)

// AdministrationError wraps IoT service errors with operation context
type AdministrationError struct {
	Op  string // The operation that failed
	Err error  // The underlying error
}

func (e *AdministrationError) Error() string {
	return fmt.Sprintf("IoT service error in %s: %v", e.Op, e.Err)
}

func (e *AdministrationError) Unwrap() error {
	return e.Err
}

// AdministrationDeviceNotFoundError wraps a device not found error
type AdministrationDeviceNotFoundError struct {
	DeviceID string
}

func (e *AdministrationDeviceNotFoundError) Error() string {
	return fmt.Sprintf("device not found: %s", e.DeviceID)
}

func (e *AdministrationDeviceNotFoundError) Unwrap() error {
	return ErrAdministrationDeviceNotFound
}

// AdministrationDuplicateDeviceIDError wraps a duplicate device ID error
type AdministrationDuplicateDeviceIDError struct {
	DeviceID string
}

func (e *AdministrationDuplicateDeviceIDError) Error() string {
	return fmt.Sprintf("Die Geräte-ID %q ist bereits vergeben", e.DeviceID)
}

func (e *AdministrationDuplicateDeviceIDError) Unwrap() error {
	return ErrAdministrationDuplicateDeviceID
}
