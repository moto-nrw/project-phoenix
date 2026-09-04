package facilities

import facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"

const (
	RoomColorUniqueConstraintName   = facilitiesModule.RoomColorUniqueConstraintName
	RoomWCAliasUniqueConstraintName = facilitiesModule.RoomWCAliasUniqueConstraintName
)

var IsReservedRoomColor = facilitiesModule.IsReservedRoomColor
