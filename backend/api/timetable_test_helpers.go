package api

import (
	"context"
	"log/slog"
	"time"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	blockDisplayCompose "github.com/moto-nrw/project-phoenix/modules/timetableblockdisplay/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type TimetableTestModule struct {
	Instance       *arrivalTimetable.InstanceLifecycleService
	SchoolCalendar schoolcalendar.Calendar
	// TimetableData carries the Timetable owner's template writes, recurrence
	// gate, planner reads and conflict detection over the same repositories,
	// as services.Factory does.
	TimetableData arrivalTimetable.Planning
	// ConflictDetection is the Timetable owner's conflict detection and
	// staffing capability over the same repositories (#3550).
	ConflictDetection timetable.ConflictDetectionCapability
	Materialization   timetable.MaterializationCapability
	RealtimeHub       *realtime.Hub
}

func NewTimetableTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (TimetableTestModule, error) {
	r, err := NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return TimetableTestModule{}, err
	}
	approvedOfferings, err := NewApprovedOfferingTestProjection(db, r.Enrollment())
	if err != nil {
		return TimetableTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return TimetableTestModule{}, err
	}
	recurrenceLock, err := newTimetableTestRecurrenceLock(db)
	if err != nil {
		return TimetableTestModule{}, err
	}
	now := services.OptionalClock(clocks)
	today := timezone.CalendarDateClock(now)
	logger := slog.Default()
	hub := deliveryCompose.NewRealtimeHub(logger)
	students, err := NewPeopleDirectory(db)
	if err != nil {
		return TimetableTestModule{}, err
	}
	carePlan, err := NewCarePlan(db, students, r.InstanceStudent)
	if err != nil {
		return TimetableTestModule{}, err
	}
	pickup, err := careplanCompose.NewPickupBaselines(carePlan, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	classArrivalQueries, err := arrivalTimetable.NewClassArrivalQueries(db.Pool(), func(arrivalTimetable.Observation) {})
	if err != nil {
		return TimetableTestModule{}, err
	}
	arrival, err := NewArrivalBaselines(carePlan, students, classArrivalQueries, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	careDay := careplanCompose.NewCareDays(careplanCompose.CareDayDependencies{
		ArrivalBaselines: arrival, Records: carePlan,
		PickupBaselines: pickup,
	})
	care, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return TimetableTestModule{}, err
	}
	careplanCompose.WireCareParticipation(careDay, care.CareLifecycle)
	bridge, err := NewTimetableEndedSessionCompletion(r.OwnerRows(), careDay)
	if err != nil {
		return TimetableTestModule{}, err
	}
	attendanceMirror, err := NewTimetableAttendanceMirror(r.OwnerRows(), logger)
	if err != nil {
		return TimetableTestModule{}, err
	}
	// Instance completion consumes only the active-session end capability.
	// Retain its real transaction, visit sync, supervision and SSE paths.
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(r.ActiveGroup)
	ender := presenceservice.NewPresence(presenceservice.PresenceDependencies{
		PrincipalReader: devicescanCompose.AttendancePrincipal,
		SchoolPresence:  NewLoggedStudentPresence(db, logger),
		GroupRepo:       sessionGroups, SupervisorRepo: sessionSupervisors,
		StudentRepo: PresenceStudents(db, r.Student), RoomRepo: facilitiesLegacy.NewAttendanceRooms(r.Room), ActivityGroupRepo: NewSessionActivities(r.GroupReads()),
		EducationGroupRepo: NewAttendanceEducationGroups(r.Group, r.Student), StaffRepo: NewAttendanceStaffDirectory(r.Staff),
		DB: db.Pool(), Broadcaster: hub, Logger: logger, Now: now,
		AttendanceSyncer:         attendanceMirror,
		TimetableBridgeCompleter: bridge,
	}, presenceservice.WithPresenceSettings(settingsCompose.PresenceSettings(settings.Settings)))
	offerings, err := newTestCareOfferingCatalog(r, settings.Settings, CareOfferingCatalogTestOptions{
		LockRecurrence: recurrenceLock.LockRecurrenceWrites, Today: today,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	calendarAdministration := SchoolCalendarAdministration(settings.Settings,
		recurrenceLock.LockRecurrenceWrites,
		offerings)
	calendar, err := NewSchoolCalendarWithAdministration(db, func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration })
	if err != nil {
		return TimetableTestModule{}, err
	}
	rows := r.TimetableTemplateRows()
	materializationDeps := TimetableMaterializationDependencies(services.TimetableMaterializationInputs{
		Rows: rows, CareBounds: r.Student, NonWorkingDays: calendar, RecurrenceLock: recurrenceLock, Broadcaster: hub, Logger: logger,
	})
	materializationDeps.DB = db.Pool()
	materialization, err := arrivalTimetable.NewMaterialization(materializationDeps)
	if err != nil {
		return TimetableTestModule{}, err
	}
	recovery := arrivalTimetable.NewRetainedActivityRecoveryRepository(db.Pool(), r.InstanceStudent)
	conflicts, err := NewTimetableConflictDetection(services.TimetableConflictReaders{
		Instances: r.ActivityInstance, InstanceStaff: r.InstanceStaff, InstanceStudents: r.InstanceStudent,
		Exceptions: r.ActivityException, Schedules: r.ActivitySchedule, Staff: r.Staff, CalendarPeriods: r.CalendarPeriod,
		ArrivalExceptions: r.StudentArrivalException, Sessions: r.ActiveGroup, Shifts: r.StaffShift,
		Presence: NewLoggedStudentPresence(db, logger), ArrivalBaselines: arrival, Logger: logger,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	substituteConflicts, err := arrivalTimetable.NewSubstituteConflictsOverRows(r.OwnerRows())
	if err != nil {
		return TimetableTestModule{}, err
	}
	lifecycleRows := r.OwnerRows()
	lifecycleRows.Locks = recovery
	instance, err := NewTimetableLifecycle(services.TimetableLifecycleInputs{
		Rows: lifecycleRows, Planning: arrivalTimetable.InstanceLifecycleDependencies{
			IdempotencyRepo: r.InstanceIdempotency, ExceptionRepo: r.ActivityException, CalendarPeriodRepo: r.CalendarPeriod, DB: db.Pool(),
		},
		Staff: r.Staff, Sessions: r.ActiveGroup, Supervisions: r.GroupSupervisor,
		Presence: NewLoggedStudentPresence(db, logger), SessionEnder: ender, CareDays: careDay, CareDayLocks: carePlan,
		Settings: settings.Settings, Materialization: materialization, RecurrenceLock: recurrenceLock,
		StartConflicts: conflicts, SubstituteConflicts: substituteConflicts, Broadcaster: hub, Logger: logger, Now: now,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	dataRows := r.OwnerRows()
	dataRows.Locks = recovery
	timetableData, err := NewTimetableData(services.TimetableDataInputs{
		Rows: dataRows, ArrivalBaselines: arrival, PickupBaselines: pickup, Visits: NewLoggedStudentPresence(db, logger),
		Groups: r.Timetable, Sessions: r.ActiveGroup,
		ConflictAcks: r.Timetable, BlockMetadata: arrivalTimetable.NewBlockDisplayMetadata(db.Pool(), blockDisplayCompose.List), Transactional: true, Logger: logger,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	templateDeps, err := TimetableTemplateDependencies(services.TimetableTemplateInputs{
		Rows: rows, PlanningTracks: arrivalTimetable.NewPlanningTrackAdministration(r.Timetable, db.Pool()),
		Materialization: materialization, Deviations: instance.SeriesDeviations(), CareOfferings: offerings,
		RecurrenceLock: recurrenceLock, Broadcaster: hub, Logger: logger, Today: today,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	templateDeps.DB = db.Pool()
	templates, err := arrivalTimetable.NewTemplateAdministration(templateDeps)
	if err != nil {
		return TimetableTestModule{}, err
	}
	deviationDeps, err := TimetableStaffDeviationDependencies(services.TimetableDeviationInputs{
		Rows: r.OwnerRows(), Staff: r.Staff, Supervisions: r.GroupSupervisor,
		Lifecycle: instance.DeviationLifecycle(), Broadcaster: hub, Logger: logger, Now: now,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	deviationDeps.DB = db.Pool()
	deviations, err := arrivalTimetable.NewStaffDeviations(deviationDeps)
	if err != nil {
		return TimetableTestModule{}, err
	}
	// The trail stays unwired, as before: a correction fails closed here.
	corrections, err := NewTimetableAttendanceCorrections(services.TimetableCorrectionInputs{
		Rows: dataRows, Logger: logger,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	planning := arrivalTimetable.Planning{
		Templates: templates, RecurrenceLock: recurrenceLock, Data: timetableData, ConflictDetection: conflicts,
		AttendanceCorrections: corrections, Deviations: deviations,
	}
	return TimetableTestModule{Instance: instance, SchoolCalendar: calendar, TimetableData: planning, ConflictDetection: conflicts, Materialization: materialization, RealtimeHub: hub}, nil
}
