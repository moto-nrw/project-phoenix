package owners

import (
	"context"
	"fmt"
	"log/slog"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/realtime"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CarePlanOfferings are Care Plan's care-offering catalog and its booking
// materialization (#3560), which reference each other: a catalog edit
// resyncs the bookings it feeds, the bookings read the catalog. Care Plan
// composes both together.
type CarePlanOfferings struct {
	Catalog  careplan.CareOfferingCatalogCapability
	Bookings careplan.BookingMaterializationCapability
}

// BuildCarePlanOfferings binds the care-offering catalog and the booking
// materialization: the rosters an approval derives, the offering-sourced
// Regeltermine, the dated offering adjustments and the offering pickup
// times. Offering-sourced weekly Gehzeit changes re-derive the auto excusals
// of the students' future day exceptions (#2360) in Care Plan's own unit of
// work.
func BuildCarePlanOfferings(
	env RootEnvironment, carePlan careplan.Capability, persons peopledirectory.Capability, people PeopleRows,
	timetableOwner timetable.Capability, calendar schoolcalendar.Query, owner TimetableOwner, enrollment EnrollmentRows,
	settings settingsCompose.SettingsService, schedules CarePlanSchedules, audit AuditOwner, identity identityaccess.Owner,
	hub *realtime.Hub, guardians eventContract.Emitter,
) (CarePlanOfferings, error) {
	lock := owner.RecurrenceLock.LockRecurrenceWrites
	bookings, err := BookingMaterializationOwners(
		timetableOwner, owner.RosterReconciler, persons, calendar, enrollment.Bookings, enrollment.ApprovedOfferings,
		audit.Persistence.EnrollmentOfferingAdjustment, people.Persons, identity, schedules.PickupBaselines, carePlan,
		owner.StudentTemplateDays,
	)
	if err != nil {
		return CarePlanOfferings{}, fmt.Errorf("compose care plan booking materialization: %w", err)
	}
	bookings.Settings = settingsModule.NewCareBookingValues(settings)
	bookings.Bookings = carePlan
	bookings.Withdrawals = schedules.Lifecycle
	bookings.LockTemplateRecurrence = lock
	bookings.ResyncPickupAutoExcusals = schedules.AutoExcusal.ResyncFutureExceptions
	bookings.ClearPickupWeekdayExtension = timetableOwner.ClearPickupWeekdayExtension
	bookings.PickupGuardians = guardians
	bookings.PickupStaff = PickupScheduleBroadcaster{Broadcaster: hub}
	bookings.Today = env.Today
	bookings.Logger = env.Logger.With("service", "care-plan-bookings")
	catalog, materialization, err := carePlanCompose.NewCareOfferingCatalogWithBookings(
		careOfferingCatalogDependencies(carePlan, enrollment, timetableOwner, calendar, settings, lock, env.Today, env.Logger), bookings,
	)
	if err != nil {
		return CarePlanOfferings{}, fmt.Errorf("compose care offering catalog: %w", err)
	}
	return CarePlanOfferings{Catalog: catalog, Bookings: materialization}, nil
}

// careOfferingCatalogDependencies binds the care-offering validation to
// Enrollment's phases and bookings, the Timetable owner, the School Calendar
// and the settings.
func careOfferingCatalogDependencies(
	carePlan careplan.Capability, enrollment EnrollmentRows, timetableOwner timetable.Capability, calendar schoolcalendar.Query,
	settings settingsCompose.SettingsService, lock func(context.Context) error, today func() timezone.Date, logger *slog.Logger,
) carePlanCompose.CareOfferingCatalogDependencies {
	return carePlanCompose.CareOfferingCatalogDependencies{
		Records:                carePlan,
		Phases:                 CareOfferingCatalogPhases{Phases: enrollment.Bookings},
		Timetable:              CareOfferingCatalogTimetable{Timetable: timetableOwner},
		Calendar:               CareOfferingCatalogCalendar{Calendar: calendar},
		Bookings:               CareOfferingCatalogBookings{Bookings: enrollment.Bookings},
		Settings:               CareOfferingCatalogSettings{Settings: settings},
		Translations:           CareOfferingCatalogTranslations{},
		SourceRules:            CareOfferingSourceRules{},
		LockTemplateRecurrence: lock,
		Today:                  today,
		Logger:                 logger.With("service", "enrollment-care-offering"),
	}
}
