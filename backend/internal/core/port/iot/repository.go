package iot

import (
	"context"
	"time"

	domain "github.com/moto-nrw/project-phoenix/internal/core/domain/iot"
)

type Device = domain.Device
type DeviceStatus = domain.DeviceStatus

// DeviceRepository defines operations for managing IoT devices
type DeviceRepository interface {
	// Standard CRUD operations
	Create(ctx context.Context, device *Device) error
	FindByID(ctx context.Context, id interface{}) (*Device, error)
	Update(ctx context.Context, device *Device) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*Device, error)

	// Domain-specific operations
	FindByDeviceID(ctx context.Context, deviceID string) (*Device, error)
	FindByAPIKey(ctx context.Context, apiKey string) (*Device, error)
	FindByType(ctx context.Context, deviceType string) ([]*Device, error)
	FindByStatus(ctx context.Context, status DeviceStatus) ([]*Device, error)
	FindByRegisteredBy(ctx context.Context, personID int64) ([]*Device, error)
	UpdateLastSeen(ctx context.Context, deviceID string, lastSeen time.Time) error
	UpdateStatus(ctx context.Context, deviceID string, status DeviceStatus) error

	// Specialized queries
	FindActiveDevices(ctx context.Context) ([]*Device, error)
	FindDevicesRequiringMaintenance(ctx context.Context) ([]*Device, error)
	FindOfflineDevices(ctx context.Context, offlineSince time.Duration) ([]*Device, error)
	CountDevicesByType(ctx context.Context) (map[string]int, error)
}
