package iot

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// DeviceRepository defines operations for managing IoT devices
type DeviceRepository interface {
	base.CRUDRepository[*Device]

	// Domain-specific operations
	FindByDeviceID(ctx context.Context, deviceID string) (*Device, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*Device, error)
	FindByAPIKey(ctx context.Context, apiKey string) (*Device, error)
	FindByType(ctx context.Context, deviceType string) ([]*Device, error)
	FindByStatus(ctx context.Context, status DeviceStatus) ([]*Device, error)
	FindByRegisteredBy(ctx context.Context, personID int64) ([]*Device, error)
	UpdateLastSeen(ctx context.Context, id int64, lastSeen time.Time) error
	UpdateRoomID(ctx context.Context, id int64, roomID int64) error
	UpdateStatus(ctx context.Context, deviceID string, status DeviceStatus) error

	// Specialized queries
	FindOfflineDevices(ctx context.Context, offlineSince time.Duration) ([]*Device, error)
	CountDevicesByType(ctx context.Context) (map[string]int, error)
}
