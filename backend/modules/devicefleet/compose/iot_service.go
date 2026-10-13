package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/services/iot"
)

// IoTService is the retained device administration service the IoT and
// display routes consume over the Device Fleet capability.
type IoTService = iot.Service

// NewIoTService builds the retained device administration service over the
// Device Fleet capability.
func NewIoTService(devices devicefleet.Capability) IoTService {
	return iot.NewService(devices)
}
