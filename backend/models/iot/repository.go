package iot

import "context"

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
	UpdateRoomID(ctx context.Context, id int64, roomID int64) error

	// Specialized queries
	CountDevicesByType(ctx context.Context) (map[string]int, error)
}
