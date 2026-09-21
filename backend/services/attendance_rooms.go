package services

import (
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
)

// NewAttendanceRooms binds the Facilities-owned room projection for attendance.
func NewAttendanceRooms(records facilitiesLegacy.AttendanceRoomRecords) presenceservice.AttendanceRooms {
	return facilitiesLegacy.NewAttendanceRooms(records)
}
