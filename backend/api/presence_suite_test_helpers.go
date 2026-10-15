package api

import (
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// The owner compositions the Student Presence, Care Plan and Workforce suites
// bind into the presence and planning graphs they build, as the root binds
// them (#4010). A suite of one owner may not import another owner's
// composition (ADR 0053 point 7); it reaches them through the typed root.
var (
	// WithAttendanceStaff and AttendancePrincipal are the device scan's
	// attendance principal.
	WithAttendanceStaff = devicescanCompose.WithAttendanceStaff
	AttendancePrincipal = devicescanCompose.AttendancePrincipal
	// NewRoomRepository and NewAttendanceRooms are the Facilities room reads.
	NewRoomRepository  = facilitiesCompose.NewRoomRepository
	NewAttendanceRooms = facilitiesLegacy.NewAttendanceRooms
	// PresenceSettings answers the presence questions from the tenant settings.
	PresenceSettings = settingsCompose.PresenceSettings
	// NewAttendanceActivityCategories is the Timetable category total.
	NewAttendanceActivityCategories = timetableCompose.NewCategoryCount
)
