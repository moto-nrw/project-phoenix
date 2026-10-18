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
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
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

// TimetableTestModule is the Timetable owner's instance lifecycle, planning,
// conflict detection and materialization over the test database.
type TimetableTestModule struct {
	Instance timetablePorts.InstanceLifecycle
	// TimetableData carries the Timetable owner's template writes, recurrence
	// gate, planner reads and conflict detection over the same repositories,
	// as serviceGraph does.
	TimetableData arrivalTimetable.Planning
	// ConflictDetection is the Timetable owner's conflict detection and
	// staffing capability over the same repositories (#3550).
	ConflictDetection timetable.ConflictDetectionCapability
	Materialization   timetable.MaterializationCapability
}

// NewTimetableTestModule composes the Timetable owner over db and returns it
// with the School Calendar it plans over and the realtime hub it publishes
// on.
func NewTimetableTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (TimetableTestModule, schoolcalendar.Calendar, *realtime.Hub, error) {
	r, err := NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	members := MustNewMembershipTestRepositories(db)
	presence, err := NewTimetablePresenceRows(db, r, members, clocks...)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	rooms, err := NewTimetableRoomRows(db)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	approvedOfferings, err := NewApprovedOfferingTestProjection(db, r.Enrollment())
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	recurrenceLock, err := newTimetableTestRecurrenceLock(db)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	now := OptionalClock(clocks)
	today := timezone.CalendarDateClock(now)
	logger := slog.Default()
	hub := deliveryCompose.NewRealtimeHub(logger)
	students, err := NewPeopleDirectory(db)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	carePlan, err := NewCarePlan(db, students, r.InstanceStudent)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	pickup, err := careplanCompose.NewPickupBaselines(carePlan, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	classArrivalQueries, err := arrivalTimetable.NewClassArrivalQueries(db.Pool(), func(arrivalTimetable.Observation) {})
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	arrival, err := NewArrivalBaselines(carePlan, students, classArrivalQueries, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	careDay := careplanCompose.NewCareDays(careplanCompose.CareDayDependencies{
		ArrivalBaselines: arrival, Records: carePlan,
		PickupBaselines: pickup,
	})
	care, _, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	careplanCompose.WireCareParticipation(careDay, care.CareLifecycle)
	bridge, err := NewTimetableEndedSessionCompletion(r.OwnerRows(), careDay)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	attendanceMirror, err := NewTimetableAttendanceMirror(r.OwnerRows(), logger)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	// Instance completion consumes only the active-session end capability.
	// Retain its real transaction, visit sync, supervision and SSE paths.
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(presence.ActiveGroup)
	ender := presenceservice.NewPresence(presenceservice.PresenceDependencies{
		PrincipalReader: devicescanCompose.AttendancePrincipal,
		SchoolPresence:  NewLoggedStudentPresence(db, logger),
		GroupRepo:       sessionGroups, SupervisorRepo: sessionSupervisors,
		StudentRepo: PresenceStudents(db, members.Student), RoomRepo: facilitiesLegacy.NewAttendanceRooms(rooms.Room), ActivityGroupRepo: NewSessionActivities(r.GroupReads()),
		EducationGroupRepo: NewAttendanceEducationGroups(NewSchoolStructureSuiteRepositories(db).Group, members.Student), StaffRepo: NewAttendanceStaffDirectory(members.Staff),
		DB: db.Pool(), Broadcaster: NewPresenceEvents(hub, logger), Logger: logger, Now: now,
		AttendanceSyncer:         attendanceMirror,
		TimetableBridgeCompleter: bridge,
	}, presenceservice.WithPresenceSettings(settingsCompose.PresenceSettings(settings.Settings)))
	offerings, err := newTestCareOfferingCatalog(db, r, settings.Settings, CareOfferingCatalogTestOptions{
		LockRecurrence: recurrenceLock.LockRecurrenceWrites, Today: today,
	})
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	calendarAdministration := SchoolCalendarAdministration(settings.Settings,
		recurrenceLock.LockRecurrenceWrites,
		offerings)
	calendar, err := NewSchoolCalendarWithAdministration(db, func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration })
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	rows := r.TimetableTemplateRows()
	materializationDeps := TimetableMaterializationDependencies(rows, members.Student, calendar, recurrenceLock, hub, logger)
	materializationDeps.Transactions = arrivalTimetable.NewTenantTransactions(db.Pool())
	materialization, err := arrivalTimetable.NewMaterialization(materializationDeps)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	recovery := arrivalTimetable.NewRetainedActivityRecoveryRepository(db.Pool(), r.InstanceStudent)
	conflicts, err := NewTimetableConflictDetection(timetablePorts.ConflictDetectionDependencies{
		Instances: r.ActivityInstance, InstanceStaff: r.InstanceStaff, InstanceStudents: r.InstanceStudent,
		Exceptions: r.ActivityException, Schedules: r.ActivitySchedule, Staff: NewTimetableStaff(members.Staff), CalendarPeriods: r.CalendarPeriod,
		ArrivalExceptions: r.StudentArrivalException, Sessions: presence.ActiveGroup, Shifts: r.StaffShift,
		Presence: NewLoggedStudentPresence(db, logger), Logger: logger,
	}, arrival)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	substituteConflicts, err := arrivalTimetable.NewSubstituteConflictsOverRows(r.OwnerRows())
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	lifecycleRows := r.OwnerRows()
	lifecycleRows.Locks = recovery
	instance, err := NewTimetableLifecycle(timetablePorts.InstanceLifecycleDependencies{
		IdempotencyRepo: r.InstanceIdempotency, ExceptionRepo: r.ActivityException, CalendarPeriodRepo: r.CalendarPeriod, Database: arrivalTimetable.NewTenantDatabase(db.Pool()),
		StaffRepo: NewTimetableStaff(members.Staff), ActiveGroupRepo: presence.ActiveGroup, SupervisorRepo: presence.GroupSupervisor,
		Presence: NewLoggedStudentPresence(db, logger), ActiveService: ender, CareDayLocks: carePlan,
		Materialization: materialization, RecurrenceLock: recurrenceLock,
		StartConflicts: conflicts, SubstituteConflicts: substituteConflicts, Logger: logger, Now: now,
	}, lifecycleRows, careDay, nil, settings.Settings, hub)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	dataRows := r.OwnerRows()
	dataRows.Locks = recovery
	timetableData, err := NewTimetableData(dataRows, arrival, pickup, NewLoggedStudentPresence(db, logger), r.Timetable, presence.ActiveGroup,
		r.Timetable, arrivalTimetable.NewBlockDisplayMetadata(db.Pool(), blockDisplayCompose.List), true, logger)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	templateDeps, err := TimetableTemplateDependencies(rows, arrivalTimetable.NewPlanningTrackAdministration(r.Timetable, db.Pool()),
		materialization, instance.SeriesDeviations(), offerings, nil, recurrenceLock, hub, logger, today)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	templateDeps.Transactions = arrivalTimetable.NewTenantTransactions(db.Pool())
	templates, err := arrivalTimetable.NewTemplateAdministration(templateDeps)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	deviationDeps, err := TimetableStaffDeviationDependencies(r.OwnerRows(), NewTimetableStaff(members.Staff), presence.GroupSupervisor,
		instance.DeviationLifecycle(), hub, logger, now)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	deviationDeps.Locks = arrivalTimetable.NewTenantDatabase(db.Pool())
	deviations, err := arrivalTimetable.NewStaffDeviations(deviationDeps)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	// The trail stays unwired, as before: a correction fails closed here.
	corrections, err := NewTimetableAttendanceCorrections(dataRows, nil, nil, logger)
	if err != nil {
		return TimetableTestModule{}, nil, nil, err
	}
	planning := arrivalTimetable.Planning{
		Templates: templates, RecurrenceLock: recurrenceLock, Data: timetableData, ConflictDetection: conflicts,
		AttendanceCorrections: corrections, Deviations: deviations,
	}
	return TimetableTestModule{Instance: instance, TimetableData: planning, ConflictDetection: conflicts, Materialization: materialization}, calendar, hub, nil
}
