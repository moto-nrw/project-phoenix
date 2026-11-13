//go:build testhooks

package owners

import (
	facilitiesModule "github.com/moto-ogs/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-ogs/project-phoenix/modules/facilities/compose"
	timetableCompose "github.com/moto-ogs/project-phoenix/modules/timetable/compose"
)

// NewTimetableRoomNames names and locks the Facilities rooms for the
// Timetable owner rows (#3551).
func NewTimetableRoomNames(rooms facilitiesModule.Capability) timetableCompose.OwnerRooms {
	return facilitiesCompose.NewRoomNameDirectory(facilitiesCompose.NewRoomRecords(rooms))
}
