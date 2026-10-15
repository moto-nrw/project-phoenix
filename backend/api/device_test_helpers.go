package api

import (
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewDeviceRepository serves the retained device repository shape from an
// unobserved owner. Production graphs compose the owner once and wrap it
// themselves (NewFactory, NewSessionCleanupRepositories); test graphs that
// only need the repository shape use this.
func NewDeviceRepository(db transactionruntime.Database) (*devicefleetRepositoryAdapter.DeviceRepository, error) {
	fleet, err := NewDeviceFleet(db)
	if err != nil {
		return nil, err
	}
	return devicefleetRepositoryAdapter.NewDeviceRepository(fleet), nil
}
