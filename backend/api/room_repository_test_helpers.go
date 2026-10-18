//go:build testhooks

package api

import (
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// NewTimetableRoomNames names and locks the Facilities rooms for the
// Timetable owner rows (#3551).
func NewTimetableRoomNames(rooms facilitiesModule.Capability) timetableCompose.OwnerRooms {
	return facilitiesCompose.NewRoomNameDirectory(facilitiesCompose.NewRoomRecords(rooms))
}
