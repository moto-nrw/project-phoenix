package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
)

type RoomAvailability = devicescan.RoomAvailability

// NewRoomAvailability composes the kiosk view over the existing room projection.
func NewRoomAvailability(rooms application.RoomAvailabilitySource, principal devicescan.DevicePrincipal) devicescan.RoomAvailability {
	if principal == nil {
		panic("device scan composition: principal is required")
	}
	return application.NewRoomAvailability(rooms, principal)
}
