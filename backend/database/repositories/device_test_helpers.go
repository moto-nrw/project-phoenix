package repositories

import (
	iotRepo "github.com/moto-nrw/project-phoenix/database/repositories/iot"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/uptrace/bun"
)

func NewDeviceTestRepository(db *bun.DB) (iotModels.DeviceRepository, error) {
	rooms, err := NewFacilities(db)
	if err != nil {
		return nil, err
	}
	repo := iotRepo.NewDeviceRepository(db)
	repo.(*iotRepo.DeviceRepository).BindRoomDirectory(iotRoomDirectory{rooms})
	return repo, nil
}
