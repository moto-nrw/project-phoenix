package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/application"
)

// NewSessionDeviceIDs binds the native attendance device port.
func NewSessionDeviceIDs(records devicefleet.SessionDeviceRecords) devicefleet.SessionDeviceDirectory {
	return application.SessionDeviceIDs{SessionDeviceRecords: records}
}
