package api

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
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
	holidayClosureHTTP "github.com/moto-nrw/project-phoenix/workflows/holidayclosurenotification/http"
)

// mountTraegerClosingDayRoutes serves the closing days of a Träger (#4702) at
// /traeger/closing-days: reading behind "Übersicht sehen", planning behind
// "Einstellungen verwalten". A saved closing day tells the parents of the
// holiday bookings it affects (#6434); /traeger/holiday-closures previews
// them and lists the closed days nobody told yet, behind the planning right.
func mountTraegerClosingDayRoutes(root chi.Router, schools traegercalendar.Schools, organizations HolidayOrganizations,
	mail HolidayClosureMail, holidayCalendar enrollmentOwner.HolidaySchoolCalendar,
) {
	closures := newHolidayClosures(organizations, mail, holidayCalendar)
	requireManage := common.RequiresPermission(identityaccess.TraegerRightSettingsManage)
	resource := schoolCalendarHTTP.NewTraegerClosingDayResource(
		schoolCalendarCompose.NewTraegerClosingDays(schools, owners.NewTraegerClosingDayHolidayNotices(closures)),
		common.RequiresPermission(identityaccess.TraegerRightOverviewRead), requireManage)
	root.Mount("/traeger/closing-days", resource.TraegerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/holiday-closures", enrollmentAPI.NewHolidayClosureResource(closures, requireManage).TraegerRouter(ProtectedTraegerRoutes))
}

// HolidayClosureMail is the Träger outbox of the closing day notices with
// the transport outcome and the protected repetition of their mails
// (ADR 0196).
type HolidayClosureMail struct {
	Delivery deliveryRuntime.Capability
	Mails    deliveryRuntime.TraegerMails
}

func (m HolidayClosureMail) EnqueueTraegerMail(ctx context.Context, mail enrollmentOwner.TraegerMail) error {
	return holidayTraegerMail{delivery: m.Delivery}.EnqueueTraegerMail(ctx, mail)
}

// TraegerMailStates names the transport outcome of each mail: accepted is
// "sent", whatever the mailbox did later; a dead letter whose worker
// stopped during the provider call is "unknown"; any other dead letter or a
// mail the lists held back is "failed".
func (m HolidayClosureMail) TraegerMailStates(ctx context.Context, organizationID int64, keys []string) ([]enrollmentOwner.TraegerMailState, error) {
	statuses, err := m.Mails.TraegerEmailStatusesByKey(ctx, organizationID, keys)
	if err != nil {
		return nil, err
	}
	states := make([]enrollmentOwner.TraegerMailState, 0, len(statuses))
	for _, status := range statuses {
		if status.IdempotencyKey == nil {
			continue
		}
		outcome := enrollmentOwner.TraegerMailQueued
		switch status.State {
		case deliveryContract.StateSent:
			outcome = enrollmentOwner.TraegerMailSent
		case deliveryContract.StateDeadLetter:
			outcome = enrollmentOwner.TraegerMailFailed
			if status.LastError != nil && *status.LastError == deliveryContract.DispatchOutcomeUnknown {
				outcome = enrollmentOwner.TraegerMailUnknown
			}
		case deliveryContract.StateCancelled:
			outcome = enrollmentOwner.TraegerMailFailed
		}
		states = append(states, enrollmentOwner.TraegerMailState{IdempotencyKey: *status.IdempotencyKey, Outcome: outcome})
	}
	return states, nil
}

func (m HolidayClosureMail) RetryTraegerMails(ctx context.Context, organizationID int64, keys []string) (int, error) {
	retried, err := m.Mails.RetryFailedTraegerEmails(ctx, organizationID, keys)
	return int(retried), err
}

// newHolidayClosures builds Enrollment's closing day notices over the
// Träger directory, the outbox and the host schools' calendar.
func newHolidayClosures(organizations HolidayOrganizations, mail HolidayClosureMail, holidayCalendar enrollmentOwner.HolidaySchoolCalendar) enrollmentOwner.HolidayClosures {
	return enrollmentCompose.NewHolidayClosures(holidayCalendar, newHolidayOfficeTraeger(organizations), mail, nil)
}

// mountHolidayClosureNotificationRoutes serves the notices after a school
// closing day changed (#6442, #6457, ADR 0196): the school's proposals and
// click at /api/holiday-closure-notices behind the rights that change
// closing days, and the Träger-Büro's list "Eltern noch nicht informiert"
// at /traeger/holiday-closures/pending behind "Einstellungen verwalten".
func mountHolidayClosureNotificationRoutes(root, tenant chi.Router, organizations HolidayOrganizations, mail HolidayClosureMail,
	holidayCalendar enrollmentOwner.HolidaySchoolCalendar, holidays owners.HolidayClosureHolidays, logger *slog.Logger,
) {
	notices := owners.NewHolidayClosureNotifications(newHolidayClosures(organizations, mail, holidayCalendar), holidays,
		logger.With("workflow", "holiday-closure-notification"))
	resource := holidayClosureHTTP.NewResource(notices, common.RequiresPermission(permissions.SchedulesUpdate),
		common.RequiresPermission(permissions.SchedulesUpdate), common.RequiresPermission(identityaccess.TraegerRightSettingsManage))
	tenant.Mount("/holiday-closure-notices", resource.SchoolRouter(ProtectedTenantRoutes))
	root.Mount("/traeger/holiday-closures/pending", resource.TraegerRouter(ProtectedTraegerRoutes))
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
