package iot

import (
	"context"
	"time"

	domain "github.com/moto-nrw/project-phoenix/internal/core/domain/iot"
)

type Device = domain.Device
type DeviceStatus = domain.DeviceStatus

// DeviceReadRepository defines basic read operations for IoT devices.
type DeviceReadRepository interface {
	FindByID(ctx context.Context, id interface{}) (*Device, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*Device, error)
}

// DeviceWriteRepository defines write operations for IoT devices.
type DeviceWriteRepository interface {
	Create(ctx context.Context, device *Device) error
	Update(ctx context.Context, device *Device) error
	Delete(ctx context.Context, id interface{}) error
}

// DeviceLookupRepository defines lookups by identifiers and API keys.
type DeviceLookupRepository interface {
	FindByDeviceID(ctx context.Context, deviceID string) (*Device, error)
	FindByAPIKey(ctx context.Context, apiKey string) (*Device, error)
}

// DeviceFilterRepository defines filtered device queries.
type DeviceFilterRepository interface {
	FindByType(ctx context.Context, deviceType string) ([]*Device, error)
	FindByStatus(ctx context.Context, status DeviceStatus) ([]*Device, error)
	FindByRegisteredBy(ctx context.Context, personID int64) ([]*Device, error)
}

// DeviceStatusRepository defines status updates for IoT devices.
type DeviceStatusRepository interface {
	UpdateLastSeen(ctx context.Context, deviceID string, lastSeen time.Time) error
	UpdateStatus(ctx context.Context, deviceID string, status DeviceStatus) error
}

// DeviceAnalyticsRepository defines specialized device queries.
type DeviceAnalyticsRepository interface {
	FindActiveDevices(ctx context.Context) ([]*Device, error)
	FindDevicesRequiringMaintenance(ctx context.Context) ([]*Device, error)
	FindOfflineDevices(ctx context.Context, offlineSince time.Duration) ([]*Device, error)
	CountDevicesByType(ctx context.Context) (map[string]int, error)
}

// DeviceRepository composes all IoT device repository capabilities.
type DeviceRepository interface {
	DeviceReadRepository
	DeviceWriteRepository
	DeviceLookupRepository
	DeviceFilterRepository
	DeviceStatusRepository
	DeviceAnalyticsRepository
}
