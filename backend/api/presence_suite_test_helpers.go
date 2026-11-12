//go:build testhooks

package api

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/devicefleet/deviceauth"
	facilitiesCompose "github.com/moto-ogs/project-phoenix/modules/facilities/compose"
	settingsCompose "github.com/moto-ogs/project-phoenix/modules/settings/compose"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	timetableCompose "github.com/moto-ogs/project-phoenix/modules/timetable/compose"
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

// CreateSupervisorForTest stores a supervision of the staff member on the
// active group through Student Presence's supervisor write, for suites whose
// test role may not name Student Presence's types (#6702).
func CreateSupervisorForTest(ctx context.Context, presence interface {
	CreateGroupSupervisor(context.Context, *studentpresence.GroupSupervision) error
}, staffID, groupID int64, startDate string) error {
	return presence.CreateGroupSupervisor(ctx, &studentpresence.GroupSupervision{StaffID: staffID, GroupID: groupID, Role: "supervisor", StartDate: startDate})
}
