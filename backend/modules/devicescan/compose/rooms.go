package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
)

type RoomAvailability = devicescan.RoomAvailability

// NewRoomAvailability composes the kiosk view over the existing room projection.
func NewRoomAvailability(rooms application.RoomAvailabilitySource) devicescan.RoomAvailability {
	return application.NewRoomAvailability(rooms, principals{})
}
