//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
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
	WithAttendanceStaff = deviceauth.WithAttendanceStaff
	// NewRoomRepository and NewAttendanceRooms are the Facilities room reads.
	NewRoomRepository  = facilitiesCompose.NewRoomRepository
	NewAttendanceRooms = facilitiesCompose.NewAttendanceRooms
	// PresenceSettings answers the presence questions from the tenant settings.
	PresenceSettings = settingsCompose.PresenceSettings
	// NewAttendanceActivityCategories is the Timetable category total.
	NewAttendanceActivityCategories = timetableCompose.NewCategoryCount
)
