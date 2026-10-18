package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	iot "github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/application/deviceservice"
)

// IoTService is the retained device administration service the IoT and
// display routes consume over the Device Fleet capability.
type IoTService = iot.Service

// NewIoTService builds the retained device administration service over the
// Device Fleet capability.
func NewIoTService(devices devicefleet.Capability) IoTService {
	return iot.NewService(devices)
}
