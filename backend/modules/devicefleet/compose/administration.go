package compose

import "github.com/moto-nrw/project-phoenix/modules/devicefleet"

// NewAdministration binds administrative commands to the native fleet.
func NewAdministration(fleet devicefleet.Capability) devicefleet.Administration {
	return devicefleet.NewAdministration(fleet)
}
