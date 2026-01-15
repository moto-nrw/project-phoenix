package iot

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/iot"
)

// DeviceCRUD handles basic device CRUD operations
type DeviceCRUD interface {
	CreateDevice(ctx context.Context, device *iot.Device) error
	GetDeviceByID(ctx context.Context, id int64) (*iot.Device, error)
	GetDeviceByDeviceID(ctx context.Context, deviceID string) (*iot.Device, error)
	UpdateDevice(ctx context.Context, device *iot.Device) error
	DeleteDevice(ctx context.Context, id int64) error
	ListDevices(ctx context.Context, filters map[string]any) ([]*iot.Device, error)
}

// DeviceStatusOperations handles device status updates
type DeviceStatusOperations interface {
	UpdateDeviceStatus(ctx context.Context, deviceID string, status iot.DeviceStatus) error
	PingDevice(ctx context.Context, deviceID string) error
}

// DeviceFinder handles filtered device lookups
type DeviceFinder interface {
	GetDevicesByType(ctx context.Context, deviceType string) ([]*iot.Device, error)
	GetDevicesByStatus(ctx context.Context, status iot.DeviceStatus) ([]*iot.Device, error)
	GetDevicesByRegisteredBy(ctx context.Context, personID int64) ([]*iot.Device, error)
}

// DeviceMonitoring handles device health monitoring and reporting
type DeviceMonitoring interface {
	GetActiveDevices(ctx context.Context) ([]*iot.Device, error)
	GetDevicesRequiringMaintenance(ctx context.Context) ([]*iot.Device, error)
	GetOfflineDevices(ctx context.Context, offlineDuration time.Duration) ([]*iot.Device, error)
	GetDeviceTypeStatistics(ctx context.Context) (map[string]int, error)
}

// DeviceNetworkOperations handles network discovery operations
type DeviceNetworkOperations interface {
	DetectNewDevices(ctx context.Context) ([]*iot.Device, error)
	ScanNetwork(ctx context.Context) (map[string]string, error)
}

// DeviceAuthOperations handles device authentication
type DeviceAuthOperations interface {
	GetDeviceByAPIKey(ctx context.Context, apiKey string) (*iot.Device, error)
}

// Service composes all IoT-related operations.
// Existing callers can continue using this full interface.
// New code can depend on smaller sub-interfaces for better decoupling.
type Service interface {
	base.TransactionalService
	DeviceCRUD
	DeviceStatusOperations
	DeviceFinder
	DeviceMonitoring
	DeviceNetworkOperations
	DeviceAuthOperations
}
