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
	"github.com/moto-nrw/project-phoenix/tenant"
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

// holidayClosureTraeger names the Träger and its schools in the closure
// notice (#6434). The notice is written in the Träger transaction, whose row
// security hides the organization, so the names are read in an
// administrative transaction of their own that shares nothing with it.
type holidayClosureTraeger struct {
	organizations HolidayOrganizations
}

func (d holidayClosureTraeger) HolidayTraeger(ctx context.Context, organizationID int64) (enrollmentOwner.HolidayTraeger, error) {
	var traeger enrollmentOwner.HolidayTraeger
	err := tenant.WithinAdmin(tenant.ContextWithoutTransaction(ctx), func(ctx context.Context) error {
		var err error
		traeger, err = holidayTraegerDirectory{organizations: d.organizations}.HolidayTraeger(ctx, organizationID)
		return err
	})
	return traeger, err
}
