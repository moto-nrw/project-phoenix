//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"time"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	blockDisplayCompose "github.com/moto-nrw/project-phoenix/modules/timetableblockdisplay/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type TimetableTestModule struct {
	Instance       timetablePorts.InstanceLifecycle
	SchoolCalendar schoolcalendar.Calendar
	// TimetableData carries the Timetable owner's template writes, recurrence
	// gate, planner reads and conflict detection over the same repositories,
	// as serviceGraph does.
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
	now := OptionalClock(clocks)
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
		DB: db.Pool(), Broadcaster: NewPresenceEvents(hub, logger), Logger: logger, Now: now,
		AttendanceSyncer:         attendanceMirror,
		TimetableBridgeCompleter: bridge,
	}, presenceservice.WithPresenceSettings(settingsCompose.PresenceSettings(settings.Settings)))
	offerings, err := newTestCareOfferingCatalog(r, settings.Settings, CareOfferingCatalogTestOptions{
		LockRecurrence: recurrenceLock.LockRecurrenceWrites, Today: today,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	calendar, err := NewSchoolCalendarWithAdministration(db, settings.Settings,
		recurrenceLock.LockRecurrenceWrites,
		offerings)
	if err != nil {
		return TimetableTestModule{}, err
	}
	rows := r.TimetableTemplateRows()
	materializationDeps := TimetableMaterializationDependencies(rows, r.Student, calendar, recurrenceLock, hub, logger)
	materializationDeps.Transactions = arrivalTimetable.NewTenantTransactions(db.Pool())
	materialization, err := arrivalTimetable.NewMaterialization(materializationDeps)
	if err != nil {
		return TimetableTestModule{}, err
	}
	recovery := arrivalTimetable.NewRetainedActivityRecoveryRepository(db.Pool(), r.InstanceStudent)
	conflicts, err := NewTimetableConflictDetection(timetablePorts.ConflictDetectionDependencies{
		Instances: r.ActivityInstance, InstanceStaff: r.InstanceStaff, InstanceStudents: r.InstanceStudent,
		Exceptions: r.ActivityException, Schedules: r.ActivitySchedule, Staff: NewTimetableStaff(r.Staff), CalendarPeriods: r.CalendarPeriod,
		ArrivalExceptions: r.StudentArrivalException, Sessions: r.ActiveGroup, Shifts: r.StaffShift,
		Presence: NewLoggedStudentPresence(db, logger), Logger: logger,
	}, arrival)
	if err != nil {
		return TimetableTestModule{}, err
	}
	substituteConflicts, err := arrivalTimetable.NewSubstituteConflictsOverRows(r.OwnerRows())
	if err != nil {
		return TimetableTestModule{}, err
	}
	lifecycleRows := r.OwnerRows()
	lifecycleRows.Locks = recovery
	instance, err := NewTimetableLifecycle(timetablePorts.InstanceLifecycleDependencies{
		IdempotencyRepo: r.InstanceIdempotency, ExceptionRepo: r.ActivityException, CalendarPeriodRepo: r.CalendarPeriod, Database: arrivalTimetable.NewTenantDatabase(db.Pool()),
		StaffRepo: NewTimetableStaff(r.Staff), ActiveGroupRepo: r.ActiveGroup, SupervisorRepo: r.GroupSupervisor,
		Presence: NewLoggedStudentPresence(db, logger), ActiveService: ender, CareDayLocks: carePlan,
		Materialization: materialization, RecurrenceLock: recurrenceLock,
		StartConflicts: conflicts, SubstituteConflicts: substituteConflicts, Logger: logger, Now: now,
	}, lifecycleRows, careDay, nil, settings.Settings, hub)
	if err != nil {
		return TimetableTestModule{}, err
	}
	dataRows := r.OwnerRows()
	dataRows.Locks = recovery
	timetableData, err := NewTimetableData(dataRows, arrival, pickup, NewLoggedStudentPresence(db, logger), r.Timetable, r.ActiveGroup,
		r.Timetable, arrivalTimetable.NewBlockDisplayMetadata(db.Pool(), blockDisplayCompose.List), true, logger)
	if err != nil {
		return TimetableTestModule{}, err
	}
	templateDeps, err := TimetableTemplateDependencies(rows, arrivalTimetable.NewPlanningTrackAdministration(r.Timetable, db.Pool()),
		materialization, instance.SeriesDeviations(), offerings, nil, recurrenceLock, hub, logger, today)
	if err != nil {
		return TimetableTestModule{}, err
	}
	templateDeps.Transactions = arrivalTimetable.NewTenantTransactions(db.Pool())
	templates, err := arrivalTimetable.NewTemplateAdministration(templateDeps)
	if err != nil {
		return TimetableTestModule{}, err
	}
	deviationDeps, err := TimetableStaffDeviationDependencies(r.OwnerRows(), NewTimetableStaff(r.Staff), r.GroupSupervisor,
		instance.DeviationLifecycle(), hub, logger, now)
	if err != nil {
		return TimetableTestModule{}, err
	}
	deviationDeps.Locks = arrivalTimetable.NewTenantDatabase(db.Pool())
	deviations, err := arrivalTimetable.NewStaffDeviations(deviationDeps)
	if err != nil {
		return TimetableTestModule{}, err
	}
	// The trail stays unwired, as before: a correction fails closed here.
	corrections, err := NewTimetableAttendanceCorrections(dataRows, nil, nil, logger)
	if err != nil {
		return TimetableTestModule{}, err
	}
	planning := arrivalTimetable.Planning{
		Templates: templates, RecurrenceLock: recurrenceLock, Data: timetableData, ConflictDetection: conflicts,
		AttendanceCorrections: corrections, Deviations: deviations,
	}
	return TimetableTestModule{Instance: instance, SchoolCalendar: calendar, TimetableData: planning, ConflictDetection: conflicts, Materialization: materialization, RealtimeHub: hub}, nil
}
