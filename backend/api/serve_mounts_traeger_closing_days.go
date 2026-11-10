package api

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	schoolCalendarHTTP "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/http"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/traegercalendar"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

// mountTraegerClosingDayRoutes serves the closing days of a Träger (#4702) at
// /traeger/closing-days: reading behind "Übersicht sehen", planning behind
// "Einstellungen verwalten". A saved closing day tells the parents of the
// holiday bookings it affects (#6434); /traeger/holiday-closures previews
// them and lists the closed days nobody told yet, behind the planning right.
func mountTraegerClosingDayRoutes(root chi.Router, schools traegercalendar.Schools, organizations HolidayOrganizations,
	delivery deliveryRuntime.Capability, holidayCalendar enrollmentOwner.HolidaySchoolCalendar,
) {
	closures := enrollmentCompose.NewHolidayClosures(holidayCalendar, holidayClosureTraeger{organizations: organizations}, holidayTraegerMail{delivery: delivery}, nil)
	requireManage := common.RequiresPermission(identityaccess.TraegerRightSettingsManage)
	resource := schoolCalendarHTTP.NewTraegerClosingDayResource(
		schoolCalendarCompose.NewTraegerClosingDays(schools, owners.NewTraegerClosingDayHolidayNotices(closures)),
		common.RequiresPermission(identityaccess.TraegerRightOverviewRead), requireManage)
	root.Mount("/traeger/closing-days", resource.TraegerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/holiday-closures", enrollmentAPI.NewHolidayClosureResource(closures, requireManage).TraegerRouter(ProtectedTraegerRoutes))
}

// holidayClosureTraeger names the host schools in the closure notice
// (#6434), read in the caller's Träger transaction like the host-school check
// of the holiday calendar. That transaction hides the organization itself, so
// the notice names no Träger, like the mail that a booking moved up after a
// change in the Büro (#4699).
type holidayClosureTraeger struct {
	organizations HolidayOrganizations
}

func (d holidayClosureTraeger) HolidayTraeger(ctx context.Context, organizationID int64) (enrollmentOwner.HolidayTraeger, error) {
	schools, err := d.organizations.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return enrollmentOwner.HolidayTraeger{}, err
	}
	traeger := enrollmentOwner.HolidayTraeger{Schools: []enrollmentOwner.HolidaySchool{}}
	for _, school := range schools {
		if !school.IsDeleted() {
			traeger.Schools = append(traeger.Schools, enrollmentOwner.HolidaySchool{ID: school.ID, Name: school.Name})
		}
	}
	return traeger, nil
}
