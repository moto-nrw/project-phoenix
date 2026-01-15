package iot

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/uptrace/bun"
)

// Error message constants to avoid string duplication
const (
	errDeviceIDEmpty = "device ID cannot be empty"
)

// service implements the Service interface
type service struct {
	deviceRepo iot.DeviceRepository
	db         *bun.DB
	txHandler  *base.TxHandler
}

// NewService creates a new IoT service
func NewService(deviceRepo iot.DeviceRepository, db *bun.DB) Service {
	return &service{
		deviceRepo: deviceRepo,
		db:         db,
		txHandler:  base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *service) WithTx(tx bun.Tx) any {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var deviceRepo = s.deviceRepo

	// Try to cast repository to TransactionalRepository and apply the transaction
	if txRepo, ok := s.deviceRepo.(base.TransactionalRepository); ok {
		deviceRepo = txRepo.WithTx(tx).(iot.DeviceRepository)
	}

	// Return a new service with the transaction
	return &service{
		deviceRepo: deviceRepo,
		db:         s.db,
		txHandler:  s.txHandler.WithTx(tx),
	}
}

// generateAPIKey generates a secure random API key for device authentication
func (s *service) generateAPIKey() (string, error) {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Convert to hex string and add prefix
	return fmt.Sprintf("dev_%s", hex.EncodeToString(bytes)), nil
}

// CreateDevice creates a new IoT device
func (s *service) CreateDevice(ctx context.Context, device *iot.Device) error {
	if device == nil {
		return &IoTError{Op: "CreateDevice", Err: ErrInvalidDeviceData}
	}

	// Validate device data
	if err := device.Validate(); err != nil {
		return &IoTError{Op: "CreateDevice", Err: err}
	}

	// Check if a device with the same device ID exists
	existingDevice, err := s.deviceRepo.FindByDeviceID(ctx, device.DeviceID)
	if err == nil && existingDevice != nil && existingDevice.ID > 0 {
		return &IoTError{Op: "CreateDevice", Err: &DuplicateDeviceIDError{DeviceID: device.DeviceID}}
	}

	// Generate API key if not provided
	if device.APIKey == nil || *device.APIKey == "" {
		apiKey, err := s.generateAPIKey()
		if err != nil {
			return &IoTError{Op: "CreateDevice", Err: fmt.Errorf("failed to generate API key: %w", err)}
		}
		device.APIKey = &apiKey
	}

	// Set default status if not provided
	if device.Status == "" {
		device.Status = iot.DeviceStatusActive
	}

	// Set initial last seen time if not provided
	if device.LastSeen == nil {
		now := time.Now()
		device.LastSeen = &now
	}

	// Create the device
	if err := s.deviceRepo.Create(ctx, device); err != nil {
		return &IoTError{Op: "CreateDevice", Err: err}
	}

	return nil
}

// GetDeviceByID retrieves a device by its ID
func (s *service) GetDeviceByID(ctx context.Context, id int64) (*iot.Device, error) {
	if id <= 0 {
		return nil, &IoTError{Op: "GetDeviceByID", Err: errors.New("invalid ID")}
	}

	device, err := s.deviceRepo.FindByID(ctx, id)
	if err != nil {
		// Check if this is a "not found" error
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &IoTError{Op: "GetDeviceByID", Err: ErrDeviceNotFound}
		}
		return nil, &IoTError{Op: "GetDeviceByID", Err: err}
	}

	if device == nil {
		return nil, &IoTError{Op: "GetDeviceByID", Err: ErrDeviceNotFound}
	}

	return device, nil
}

// GetDeviceByDeviceID retrieves a device by its device ID
func (s *service) GetDeviceByDeviceID(ctx context.Context, deviceID string) (*iot.Device, error) {
	if deviceID == "" {
		return nil, &IoTError{Op: "GetDeviceByDeviceID", Err: errors.New(errDeviceIDEmpty)}
	}

	device, err := s.deviceRepo.FindByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, &IoTError{Op: "GetDeviceByDeviceID", Err: err}
	}

	if device == nil || device.ID <= 0 {
		return nil, &IoTError{Op: "GetDeviceByDeviceID", Err: &DeviceNotFoundError{DeviceID: deviceID}}
	}

	return device, nil
}

// UpdateDevice updates an existing IoT device
func (s *service) UpdateDevice(ctx context.Context, device *iot.Device) error {
	if device == nil || device.ID <= 0 {
		return &IoTError{Op: "UpdateDevice", Err: ErrInvalidDeviceData}
	}

	// Validate device data
	if err := device.Validate(); err != nil {
		return &IoTError{Op: "UpdateDevice", Err: err}
	}

	// Check if device exists
	existingDevice, err := s.deviceRepo.FindByID(ctx, device.ID)
	if err != nil {
		// Check if this is a "not found" error
		if errors.Is(err, sql.ErrNoRows) {
			return &IoTError{Op: "UpdateDevice", Err: ErrDeviceNotFound}
		}
		return &IoTError{Op: "UpdateDevice", Err: err}
	}

	if existingDevice == nil || existingDevice.ID <= 0 {
		return &IoTError{Op: "UpdateDevice", Err: ErrDeviceNotFound}
	}

	// Check for duplicate device ID if changed
	if existingDevice.DeviceID != device.DeviceID {
		duplicateCheck, err := s.deviceRepo.FindByDeviceID(ctx, device.DeviceID)
		if err == nil && duplicateCheck != nil && duplicateCheck.ID > 0 && duplicateCheck.ID != device.ID {
			return &IoTError{Op: "UpdateDevice", Err: &DuplicateDeviceIDError{DeviceID: device.DeviceID}}
		}
	}

	// Update the device
	if err := s.deviceRepo.Update(ctx, device); err != nil {
		return &IoTError{Op: "UpdateDevice", Err: err}
	}

	return nil
}

// DeleteDevice deletes an IoT device by its ID
func (s *service) DeleteDevice(ctx context.Context, id int64) error {
	if id <= 0 {
		return &IoTError{Op: "DeleteDevice", Err: errors.New("invalid ID")}
	}

	// Check if device exists
	device, err := s.deviceRepo.FindByID(ctx, id)
	if err != nil {
		// Check if this is a "not found" error
		if errors.Is(err, sql.ErrNoRows) {
			return &IoTError{Op: "DeleteDevice", Err: ErrDeviceNotFound}
		}
		return &IoTError{Op: "DeleteDevice", Err: err}
	}

	if device == nil || device.ID <= 0 {
		return &IoTError{Op: "DeleteDevice", Err: ErrDeviceNotFound}
	}

	// Delete the device
	if err := s.deviceRepo.Delete(ctx, id); err != nil {
		return &IoTError{Op: "DeleteDevice", Err: err}
	}

	return nil
}

// ListDevices retrieves devices based on filters
func (s *service) ListDevices(ctx context.Context, filters map[string]interface{}) ([]*iot.Device, error) {
	devices, err := s.deviceRepo.List(ctx, filters)
	if err != nil {
		return nil, &IoTError{Op: "ListDevices", Err: err}
	}
	return devices, nil
}

// UpdateDeviceStatus updates the status of a device
func (s *service) UpdateDeviceStatus(ctx context.Context, deviceID string, status iot.DeviceStatus) error {
	if deviceID == "" {
		return &IoTError{Op: "UpdateDeviceStatus", Err: errors.New(errDeviceIDEmpty)}
	}

	// Validate the status
	device := &iot.Device{Status: status}
	if err := device.SetStatus(status); err != nil {
		return &IoTError{Op: "UpdateDeviceStatus", Err: err}
	}

	// Check if device exists
	existingDevice, err := s.deviceRepo.FindByDeviceID(ctx, deviceID)
	if err != nil {
		return &IoTError{Op: "UpdateDeviceStatus", Err: err}
	}

	if existingDevice == nil || existingDevice.ID <= 0 {
		return &IoTError{Op: "UpdateDeviceStatus", Err: &DeviceNotFoundError{DeviceID: deviceID}}
	}

	// Update the device status
	if err := s.deviceRepo.UpdateStatus(ctx, deviceID, status); err != nil {
		return &IoTError{Op: "UpdateDeviceStatus", Err: err}
	}

	return nil
}

// PingDevice updates the last seen time for a device
func (s *service) PingDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return &IoTError{Op: "PingDevice", Err: errors.New(errDeviceIDEmpty)}
	}

	// Check if device exists
	existingDevice, err := s.deviceRepo.FindByDeviceID(ctx, deviceID)
	if err != nil {
		return &IoTError{Op: "PingDevice", Err: err}
	}

	if existingDevice == nil || existingDevice.ID <= 0 {
		return &IoTError{Op: "PingDevice", Err: &DeviceNotFoundError{DeviceID: deviceID}}
	}

	// Update the last seen time
	now := time.Now()
	if err := s.deviceRepo.UpdateLastSeen(ctx, deviceID, now); err != nil {
		return &IoTError{Op: "PingDevice", Err: err}
	}

	return nil
}

// GetDevicesByType retrieves devices by their type
func (s *service) GetDevicesByType(ctx context.Context, deviceType string) ([]*iot.Device, error) {
	if deviceType == "" {
		return nil, &IoTError{Op: "GetDevicesByType", Err: errors.New("device type cannot be empty")}
	}

	devices, err := s.deviceRepo.FindByType(ctx, deviceType)
	if err != nil {
		return nil, &IoTError{Op: "GetDevicesByType", Err: err}
	}

	return devices, nil
}

// GetDevicesByStatus retrieves devices by their status
func (s *service) GetDevicesByStatus(ctx context.Context, status iot.DeviceStatus) ([]*iot.Device, error) {
	// Validate the status
	device := &iot.Device{Status: status}
	if err := device.SetStatus(status); err != nil {
		return nil, &IoTError{Op: "GetDevicesByStatus", Err: err}
	}

	devices, err := s.deviceRepo.FindByStatus(ctx, status)
	if err != nil {
		return nil, &IoTError{Op: "GetDevicesByStatus", Err: err}
	}

	return devices, nil
}

// GetDevicesByRegisteredBy retrieves devices registered by a specific person
func (s *service) GetDevicesByRegisteredBy(ctx context.Context, personID int64) ([]*iot.Device, error) {
	if personID <= 0 {
		return nil, &IoTError{Op: "GetDevicesByRegisteredBy", Err: errors.New("invalid person ID")}
	}

	devices, err := s.deviceRepo.FindByRegisteredBy(ctx, personID)
	if err != nil {
		return nil, &IoTError{Op: "GetDevicesByRegisteredBy", Err: err}
	}

	return devices, nil
}

// GetActiveDevices retrieves all active devices
func (s *service) GetActiveDevices(ctx context.Context) ([]*iot.Device, error) {
	devices, err := s.deviceRepo.FindActiveDevices(ctx)
	if err != nil {
		return nil, &IoTError{Op: "GetActiveDevices", Err: err}
	}

	return devices, nil
}

// GetDevicesRequiringMaintenance retrieves all devices requiring maintenance
func (s *service) GetDevicesRequiringMaintenance(ctx context.Context) ([]*iot.Device, error) {
	devices, err := s.deviceRepo.FindDevicesRequiringMaintenance(ctx)
	if err != nil {
		return nil, &IoTError{Op: "GetDevicesRequiringMaintenance", Err: err}
	}

	return devices, nil
}

// GetOfflineDevices retrieves devices that have been offline for a specified duration
func (s *service) GetOfflineDevices(ctx context.Context, offlineDuration time.Duration) ([]*iot.Device, error) {
	if offlineDuration <= 0 {
		return nil, &IoTError{Op: "GetOfflineDevices", Err: errors.New("invalid offline duration")}
	}

	devices, err := s.deviceRepo.FindOfflineDevices(ctx, offlineDuration)
	if err != nil {
		return nil, &IoTError{Op: "GetOfflineDevices", Err: err}
	}

	return devices, nil
}

// GetDeviceTypeStatistics retrieves a count of devices by type
func (s *service) GetDeviceTypeStatistics(ctx context.Context) (map[string]int, error) {
	stats, err := s.deviceRepo.CountDevicesByType(ctx)
	if err != nil {
		return nil, &IoTError{Op: "GetDeviceTypeStatistics", Err: err}
	}

	return stats, nil
}

// DetectNewDevices scans the network for new devices and returns them
// This is a placeholder implementation and would need to be extended
// with actual IoT device discovery logic
func (s *service) DetectNewDevices(_ context.Context) ([]*iot.Device, error) {
	// This would be implemented with actual network scanning logic in a real system
	// For now, just return an error to indicate this is not implemented
	return nil, &IoTError{Op: "DetectNewDevices", Err: errors.New("device auto-discovery not implemented")}
}

// ScanNetwork scans the network for all IoT devices and returns a map of device IDs to device types
// This is a placeholder implementation and would need to be extended
// with actual network scanning logic
func (s *service) ScanNetwork(_ context.Context) (map[string]string, error) {
	// This would be implemented with actual network scanning logic in a real system
	// For now, just return an error to indicate this is not implemented
	return nil, &IoTError{Op: "ScanNetwork", Err: errors.New("network scanning not implemented")}
}

// GetDeviceByAPIKey retrieves a device by its API key for authentication
func (s *service) GetDeviceByAPIKey(ctx context.Context, apiKey string) (*iot.Device, error) {
	if apiKey == "" {
		return nil, &IoTError{Op: "GetDeviceByAPIKey", Err: errors.New("API key cannot be empty")}
	}

	device, err := s.deviceRepo.FindByAPIKey(ctx, apiKey)
	if err != nil {
		return nil, &IoTError{Op: "GetDeviceByAPIKey", Err: err}
	}

	if device == nil {
		return nil, &IoTError{Op: "GetDeviceByAPIKey", Err: &DeviceNotFoundError{DeviceID: "unknown"}}
	}

	return device, nil
}
