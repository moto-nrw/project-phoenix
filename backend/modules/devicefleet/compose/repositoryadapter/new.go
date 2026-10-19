package repositoryadapter

import (
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	implementation "github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/adapters/repositoryadapter"
)

func NewDeviceRepository(devices devicefleet.Capability) *implementation.DeviceRepository {
	return implementation.NewDeviceRepository(devices)
}
