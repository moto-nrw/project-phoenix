package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
)

func NewRoomSnapshotPeople(source application.RoomSnapshotSource) application.RoomSnapshotPeople {
	return application.NewRoomSnapshotPeople(source)
}
