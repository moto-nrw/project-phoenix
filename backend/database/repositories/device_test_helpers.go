package repositories

import (
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	"github.com/uptrace/bun"
)

// NewDeviceRepository serves the retained device repository shape from an
// unobserved owner. Production graphs compose the owner once and wrap it
// themselves (NewFactory, NewSessionCleanupRepositories); test graphs that
// only need the repository shape use this.
func NewDeviceRepository(db *bun.DB) (iotModels.DeviceRepository, error) {
	fleet, err := NewDeviceFleet(db)
	if err != nil {
		return nil, err
	}
	return devicefleetRepositoryAdapter.NewDeviceRepository(fleet), nil
}
