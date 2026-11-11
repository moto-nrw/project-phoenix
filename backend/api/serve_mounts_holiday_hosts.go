package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HolidayHostSchools is the Organisation & Tenancy read that names the
// schools of the hosting school's Träger.
type HolidayHostSchools = owners.HolidayHostSchools

// mountHolidayHostRoutes serves the participant lists of the school that
// hosts a holiday week (#4665, ADR 0116) at /api/holiday-weeks, behind the
// school session and config:read like the other enrollment lists, the group
// choice for the week's holiday guests behind users:create, and the sheet of
// a holiday guest (#4666, ADR 0120) at /api/holiday-guests behind users:read
// with the hints „Gast aus …“ of its lists behind students:read (#5630).
func mountHolidayHostRoutes(tenant chi.Router, schools HolidayHostSchools, people owners.HolidayGuestDirectory, care owners.HolidayGuestCarePlan, accessLog owners.DataAccessAppender, healthData enrollmentOwner.HolidayHealthCipher) {
	guests := owners.NewHolidayGuests(people, schools, care, accessLog, healthData, nil)
	tenant.Mount("/holiday-weeks", NewHolidayHostGuestHTTP(NewHolidayHostHTTP(schools, people, accessLog, healthData, nil), guests).Router(ProtectedTenantRoutes))
	tenant.Mount("/holiday-guests", NewHolidayGuestHTTP(guests).Router(ProtectedTenantRoutes))
}

// NewHolidayGuestHTTP serves the sheet of a holiday guest behind users:read,
// the permission that opens a child's contacts, the hints „Gast aus …“
// behind students:read, the permission of the child list they stand in
// (#5630), and the Feriengruppen behind groups:read, the permission of the
// group lists they stand in (ADR 0120, Nachtrag 2026-11-11).
func NewHolidayGuestHTTP(guests enrollmentOwner.HolidayGuests) *enrollmentAPI.HolidayGuestResource {
	return enrollmentAPI.NewHolidayGuestResource(guests, common.RequiresPermission(permissions.UsersRead),
		common.RequiresPermission(permissions.StudentsRead), common.RequiresPermission(permissions.GroupsRead))
}

// NewHolidayHostHTTP binds the participant lists to Enrollment, the school
// names to Organisation & Tenancy, the school's own children to the People
// Directory and every read to the school's access log at the Audit Platform.
// The health information of a child of the school needs users:read, like its
// own health field (#4722). A nil today uses the Berlin calendar day.
func NewHolidayHostHTTP(schools HolidayHostSchools, people owners.HolidayGuestDirectory, accessLog owners.DataAccessAppender, healthData enrollmentOwner.HolidayHealthCipher, today func() calendar.Date) *enrollmentAPI.HolidayHostResource {
	return enrollmentAPI.NewHolidayHostResource(
		enrollmentCompose.NewHolidayHostParticipants(owners.NewHolidayParticipantAccessLog(accessLog), healthData,
			owners.NewHolidayStudentIdentities(people), today),
		owners.NewHolidayHostSchoolNames(schools), common.RequiresPermission(permissions.ConfigRead),
		common.RequiresPermission(permissions.UsersRead))
}

// NewHolidayHostGuestHTTP adds to the participant lists the group choice for
// the week's holiday guests, behind users:create: it creates children in the
// school.
func NewHolidayHostGuestHTTP(participants *enrollmentAPI.HolidayHostResource, guests enrollmentOwner.HolidayGuests) *enrollmentAPI.HolidayHostResource {
	return enrollmentAPI.NewHolidayHostGuestResource(participants, guests, common.RequiresPermission(permissions.UsersCreate))
}
