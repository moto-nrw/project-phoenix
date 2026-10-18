package compose

import (
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesService "github.com/moto-nrw/project-phoenix/modules/facilities/internal/application/roomservice"
)

func NewRoomNameDirectory(rooms facilitiesService.RoomNameRecords) facilitiesModule.RoomNameDirectory {
	return facilitiesService.NewRoomNameDirectory(rooms)
}
