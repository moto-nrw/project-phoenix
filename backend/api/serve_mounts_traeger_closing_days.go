package api

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	enrollmentOwner "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	schoolCalendarCompose "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/compose"
	schoolCalendarHTTP "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/http"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar/traegercalendar"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	holidayClosureHTTP "github.com/moto-ogs/project-phoenix/workflows/holidayclosurenotification/http"
)

// mountTraegerClosingDayRoutes serves the closing days of a Träger (#4702) at
// /traeger/closing-days: reading behind "Übersicht sehen", planning behind
// "Einstellungen verwalten". A saved closing day tells the parents of the
// holiday bookings it affects (#6434); /traeger/holiday-closures previews
// them and lists the closed days nobody told yet, behind the planning right.
func mountTraegerClosingDayRoutes(root chi.Router, schools traegercalendar.Schools, organizations HolidayOrganizations,
	mail owners.HolidayClosureMail, holidayCalendar enrollmentOwner.HolidaySchoolCalendar, holidays owners.HolidayClosureHolidays,
) {
	closures := newHolidayClosures(organizations, mail, holidayCalendar, holidays)
	requireManage := common.RequiresPermission(identityaccess.TraegerRightSettingsManage)
	resource := schoolCalendarHTTP.NewTraegerClosingDayResource(
		schoolCalendarCompose.NewTraegerClosingDays(schools, owners.NewTraegerClosingDayHolidayNotices(closures)),
		common.RequiresPermission(identityaccess.TraegerRightOverviewRead), requireManage)
	root.Mount("/traeger/closing-days", resource.TraegerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/holiday-closures", enrollmentAPI.NewHolidayClosureResource(closures, requireManage).TraegerRouter(ProtectedTraegerRoutes))
}

// newHolidayClosures builds Enrollment's closing day notices over the
// Träger directory, the outbox and the host schools' calendar.
func newHolidayClosures(organizations HolidayOrganizations, mail owners.HolidayClosureMail, holidayCalendar enrollmentOwner.HolidaySchoolCalendar,
	holidays owners.HolidayClosureHolidays,
) enrollmentOwner.HolidayClosures {
	return owners.NewHolidayClosures(holidayCalendar, holidays, newHolidayOfficeTraeger(organizations), mail)
}

// mountHolidayClosureNotificationRoutes serves the notices after a school
// closing day changed (#6442, #6457, ADR 0196): the school's proposals and
// click at /api/holiday-closure-notices behind the rights that change
// closing days, and the Träger-Büro's list "Eltern noch nicht informiert"
// at /traeger/holiday-closures/pending behind "Einstellungen verwalten".
func mountHolidayClosureNotificationRoutes(root, tenant chi.Router, organizations HolidayOrganizations,
	mail owners.HolidayClosureMail, holidayCalendar enrollmentOwner.HolidaySchoolCalendar, holidays owners.HolidayClosureHolidays, logger *slog.Logger,
) {
	notices := owners.NewHolidayClosureNotifications(newHolidayClosures(organizations, mail, holidayCalendar, holidays), organizations, mail,
		logger.With("workflow", "holiday-closure-notification"))
	resource := holidayClosureHTTP.NewResource(notices, common.RequiresPermission(permissions.SchedulesUpdate),
		common.RequiresPermission(permissions.SchedulesUpdate), common.RequiresPermission(identityaccess.TraegerRightSettingsManage))
	tenant.Mount("/holiday-closure-notices", resource.SchoolRouter(ProtectedTenantRoutes))
	root.Mount("/traeger/holiday-closures/pending", resource.TraegerRouter(ProtectedTraegerRoutes))
}

// newHolidayOfficeTraeger names the Träger and its host schools in a mail
// the Träger-Büro triggers: the notice of a closing day (#6434) and the mail
// that a booking moved up the waiting list after a cancellation, a rebooking
// or a higher capacity (#4699, #6437).
func newHolidayOfficeTraeger(organizations HolidayOrganizations) enrollmentOwner.HolidayTraegerDirectory {
	return owners.NewHolidayOfficeTraeger(organizations)
}
