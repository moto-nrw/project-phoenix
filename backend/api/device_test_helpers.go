//go:build testhooks

package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

// NewDeviceRepository serves the retained device repository shape from an
// unobserved owner. Production graphs compose the owner once and wrap it
// themselves (NewFactory, NewSessionCleanupRepositories); test graphs that
// only need the repository shape use this.
func NewDeviceRepository(db transactionruntime.Database) (DeviceTestRecords, error) {
	fleet, err := owners.NewDeviceFleet(db)
	if err != nil {
		return nil, err
	}
	return devicefleetRepositoryAdapter.NewDeviceRepository(fleet), nil
}

// DeviceTestRecords is the device-only fixture capability consumed by HTTP suites.
type DeviceTestRecords interface {
	Create(ctx context.Context, entity *devicefleet.Device) error
	Update(ctx context.Context, entity *devicefleet.Device) error
	Delete(ctx context.Context, id any) error
	FindByID(ctx context.Context, id any) (*devicefleet.Device, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*devicefleet.Device, error)
	FindByDeviceID(ctx context.Context, deviceID string) (*devicefleet.Device, error)
	LookupSessionDevice(ctx context.Context, code string) (int64, bool, error)
	FindByAPIKey(ctx context.Context, apiKey string) (*devicefleet.Device, error)
	List(ctx context.Context, filters map[string]any) ([]*devicefleet.Device, error)
	UpdateRoomID(ctx context.Context, id int64, roomID int64) error
	CountDevicesByType(ctx context.Context) (map[string]int, error)
}
