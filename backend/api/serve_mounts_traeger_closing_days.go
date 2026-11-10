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
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationTenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
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
	closures := enrollmentCompose.NewHolidayClosures(holidayCalendar, newHolidayOfficeTraeger(organizations), holidayTraegerMail{delivery: delivery}, nil)
	requireManage := common.RequiresPermission(identityaccess.TraegerRightSettingsManage)
	resource := schoolCalendarHTTP.NewTraegerClosingDayResource(
		schoolCalendarCompose.NewTraegerClosingDays(schools, owners.NewTraegerClosingDayHolidayNotices(closures)),
		common.RequiresPermission(identityaccess.TraegerRightOverviewRead), requireManage)
	root.Mount("/traeger/closing-days", resource.TraegerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/holiday-closures", enrollmentAPI.NewHolidayClosureResource(closures, requireManage).TraegerRouter(ProtectedTraegerRoutes))
}

// holidayOfficeTraeger names the Träger and its host schools in a mail the
// Träger-Büro triggers: the notice of a closing day (#6434) and the mail that
// a booking moved up the waiting list after a cancellation, a rebooking or a
// higher capacity (#4699, #6437). It reads both in the caller's Träger
// transaction: the schools like the host-school check of the holiday
// calendar, the name through Organisation & Tenancy's read of it, because that
// transaction reads no row of the organization itself.
type holidayOfficeTraeger struct {
	organizations HolidayOrganizations
	names         organizationtenancy.TraegerNames
}

func newHolidayOfficeTraeger(organizations HolidayOrganizations) holidayOfficeTraeger {
	return holidayOfficeTraeger{organizations: organizations, names: organizationTenancyCompose.NewTraegerMasterData()}
}

func (d holidayOfficeTraeger) HolidayTraeger(ctx context.Context, organizationID int64) (enrollmentOwner.HolidayTraeger, error) {
	name, err := d.names.TraegerName(ctx, organizationID)
	if err != nil {
		return enrollmentOwner.HolidayTraeger{}, err
	}
	schools, err := d.organizations.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return enrollmentOwner.HolidayTraeger{}, err
	}
	traeger := enrollmentOwner.HolidayTraeger{Name: name, Schools: []enrollmentOwner.HolidaySchool{}}
	for _, school := range schools {
		if !school.IsDeleted() {
			traeger.Schools = append(traeger.Schools, enrollmentOwner.HolidaySchool{ID: school.ID, Name: school.Name})
		}
	}
	return traeger, nil
}
