package api

import (
	"fmt"
	"log/slog"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	reminderPorts "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

// TimetableWiringOwners are the owners and retained rows the root composed
// that the Timetable, School Calendar and reminder wiring binds.
type TimetableWiringOwners struct {
	DB     transactionruntime.Database
	Logger *slog.Logger
	// Repositories are the retained Timetable repositories the root built
	// over the same owners (#2743).
	Repositories TimetableRepositories
	Timetable    timetable.Capability
	Calendar     schoolcalendar.Calendar
	Rooms        facilities.Query
	Membership   schoolmembership.Capability
	Persons      peopledirectory.GuardianPortalContacts
	// People are the retained People Directory and School Structure rows the
	// root built for the service graph (#4007).
	People PeopleWiring
}

// NewTimetableWiring builds what the root binds of Timetable & Activities,
// the School Calendar and Reminder Delivery into the service graph (#4009,
// ADR 0053 point 8): the adapters over the owners the root composes, and the
// constructors the graph calls with the values only it composes.
func NewTimetableWiring(owners TimetableWiringOwners) (TimetableWiring, error) {
	pool := owners.DB.Pool()
	classDayReads, err := timetableCompose.NewPresenceReads(pool)
	if err != nil {
		return TimetableWiring{}, fmt.Errorf("compose class day timetable reads: %w", err)
	}
	recurrenceLock, err := timetableCompose.NewRecurrenceWriteLock(timetableCompose.RecurrenceLockDependencies{
		DB: pool, GradeTransitionLockKey: schoolstructure.TransitionsLockKey,
	})
	if err != nil {
		return TimetableWiring{}, fmt.Errorf("compose timetable recurrence lock: %w", err)
	}
	people := owners.People
	return TimetableWiring{
		Repositories:          owners.Repositories,
		EducationGroupNames:   TimetableEducationGroupNames(people.Groups),
		PlanningTracks:        timetableCompose.NewPlanningTrackAdministration(owners.Timetable, pool),
		RecurrenceLock:        recurrenceLock,
		ClassDayTimetable:     NewClassDayTimetableRows(classDayReads),
		HistoryRooms:          historyRoomNames(owners.Rooms),
		HistorySlots:          NewHistorySlots(owners.Repositories.InstanceStudent),
		PlanExportClosingDays: planExportClosingDays{calendar: owners.Calendar},
		PlanExportHolidays:    planExportHolidays{calendar: owners.Calendar},
		CalendarGuardians:     NewCalendarGuardianDirectory(people.GuardianProfiles, owners.Persons),

		AttendanceMirror:       NewTimetableAttendanceMirror,
		EndedSessionCompletion: NewTimetableEndedSessionCompletion,
		DeviationPorts:         TimetableDeviationPorts,
		ConflictDetection:      NewTimetableConflictDetection,
		Lifecycle:              NewTimetableLifecycle,
		StaffDeviations:        TimetableStaffDeviationDependencies,
		Cleanup:                NewTimetableCleanup,
		// The operational day resolves the acting account's staff ID through
		// School Membership alone (#2753).
		Operations: func(in TimetableOperationInputs) (timetable.OperationCapability, error) {
			in.People = TimetableOperationPeople{OperationPeople: in.People, Membership: owners.Membership}
			return NewTimetableOperations(in)
		},
		Materialization:       TimetableMaterializationDependencies,
		Data:                  NewTimetableData,
		Templates:             TimetableTemplateDependencies,
		AttendanceCorrections: NewTimetableAttendanceCorrections,
		Calendar:              NewCalendarPortal,
		// Calendar appointments share one renderer for all four kinds.
		AppointmentRenderer: func(defaultFrom email.Email) deliveryCompose.MailRenderer {
			return deliveryCompose.MailRendererFunc(NewCalendarAppointmentRenderer(defaultFrom, calendarCompose.NewTenantRuntime(pool), people.StudentGuardians))
		},
		ReminderQuery: newReminderQuery(owners),
		ReminderCommand: func(calendar calendarCompose.Application) reminder.Command {
			return reminderCompose.NewCommand(pool, CalendarReminderDelivery(calendar))
		},
	}, nil
}

// newReminderQuery binds the reminder evaluation's readers to the owners the
// root composed and to the values the service graph hands over: the acting
// staff member, the tenant settings, the effective pickups, the live
// sessions and the active supervisions.
func newReminderQuery(owners TimetableWiringOwners) func(CareRequestStaff, settingsCompose.SettingsService, EffectivePickupTimes, studentpresence.Presence, studentpresence.SupervisionRecords, *slog.Logger) reminder.Query {
	presence := NewLoggedStudentPresence(owners.DB, owners.Logger)
	return func(currentStaff CareRequestStaff, settings settingsCompose.SettingsService, pickups EffectivePickupTimes, sessions studentpresence.Presence, supervisions studentpresence.SupervisionRecords, logger *slog.Logger) reminder.Query {
		return reminderCompose.NewQuery(reminderPorts.QueryDependencies{
			Clock:        ReminderClock(),
			CurrentStaff: ReminderStaffIdentity(currentStaff),
			Settings:     ReminderSettings{settings},
			Attendance:   presence,
			Pickup:       ReminderPickupReader{Source: pickups},
			Instance:     ReminderTimetableReader{Source: owners.Timetable},
			Room:         ReminderRoomReader{Source: owners.Rooms},
			Student:      ReminderStudentReader{Source: owners.People.Students},
			Person:       ReminderPersonReader{Source: owners.People.Persons},
			Supervision:  ReminderSupervisionReader{Source: sessions, Presence: presence},
			Visits:       ReminderVisitReader{Source: presence},
			Logger:       logger,

			// Bulk readers for ComputeBatch. They answer the three genuinely
			// per-person facts for the whole tenant in one query each, which is what
			// keeps the per-minute cost flat in the number of staff.
			BulkSupervision:   ReminderBulkSupervisionReader{Source: supervisions},
			BulkInstanceStaff: ReminderTimetableReader{Source: owners.Timetable},
		})
	}
}
