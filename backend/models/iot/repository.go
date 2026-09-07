package iot

import (
	"context"
	"time"
)

// DeviceRepository is the retained repository shape for iot.devices. The
// concrete implementation delegates to the Device Fleet owner; the interface
// spells its CRUD block out so this package stays free of ORM plumbing.
type DeviceRepository interface {
	Create(ctx context.Context, entity *Device) error
	FindByID(ctx context.Context, id any) (*Device, error)
	Update(ctx context.Context, entity *Device) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*Device, error)

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
