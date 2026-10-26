package api

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HolidayHostSchools is the Organisation & Tenancy read that names the
// schools of the hosting school's Träger.
type HolidayHostSchools interface {
	FindSchool(ctx context.Context, id int64) (organizationtenancy.School, error)
	ListSchoolsByOrganization(ctx context.Context, organizationID int64) ([]organizationtenancy.School, error)
}

// mountHolidayHostRoutes serves the participant lists of the school that
// hosts a holiday week (#4665, ADR 0116) at /api/holiday-weeks, behind the
// school session and config:read like the other enrollment lists.
func mountHolidayHostRoutes(tenant chi.Router, schools HolidayHostSchools, accessLog owners.DataAccessAppender) {
	tenant.Mount("/holiday-weeks", NewHolidayHostHTTP(schools, accessLog, nil).Router(ProtectedTenantRoutes))
}

// NewHolidayHostHTTP binds the participant lists to Enrollment, the school
// names to Organisation & Tenancy and every read to the school's access log
// at the Audit Platform. A nil today uses the Berlin calendar day.
func NewHolidayHostHTTP(schools HolidayHostSchools, accessLog owners.DataAccessAppender, today func() calendar.Date) *enrollmentAPI.HolidayHostResource {
	return enrollmentAPI.NewHolidayHostResource(
		enrollmentCompose.NewHolidayHostParticipants(owners.NewHolidayParticipantAccessLog(accessLog), today),
		holidayHostSchoolNames{schools: schools}, common.RequiresPermission(permissions.ConfigRead))
}

// holidayHostSchoolNames names the schools of the hosting school's Träger,
// deleted ones included, for the home schools of a participant list.
type holidayHostSchoolNames struct {
	schools HolidayHostSchools
}

func (n holidayHostSchoolNames) HolidayHostSchoolNames(ctx context.Context, schoolID int64) (map[int64]string, error) {
	school, err := n.schools.FindSchool(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	schools, err := n.schools.ListSchoolsByOrganization(ctx, school.OrganizationID)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(schools))
	for _, sibling := range schools {
		names[sibling.ID] = sibling.Name
	}
	return names, nil
}
