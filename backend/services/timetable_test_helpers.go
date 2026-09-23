package services

import (
	"context"
	"log/slog"
	"time"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type TimetableTestModule struct {
	Instance       timetableplanning.InstanceService
	SchoolCalendar schoolcalendar.Calendar
	// TimetableData carries the Timetable owner's template writes, recurrence
	// gate, planner reads and conflict detection over the same repositories,
	// as services.Factory does.
	TimetableData TimetablePlanning
	// ConflictDetection is the Timetable owner's conflict detection and
	// staffing capability over the same repositories (#3550).
	ConflictDetection timetable.ConflictDetectionCapability
	Materialization   timetable.MaterializationCapability
	RealtimeHub       *realtime.Hub
}

func NewTimetableTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (TimetableTestModule, error) {
	r, err := repositories.NewTimetableTestRepositories(db, clocks...)
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
	recurrenceLock, err := repositories.NewTimetableRecurrenceLock(db)
	if err != nil {
		return TimetableTestModule{}, err
	}
	now := optionalClock(clocks)
	today := timezone.CalendarDateClock(now)
	logger := slog.Default()
	hub := deliveryCompose.NewRealtimeHub(logger)
	students, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return TimetableTestModule{}, err
	}
	carePlan, err := repositories.NewCarePlan(db, students, r.InstanceStudent)
	if err != nil {
		return TimetableTestModule{}, err
	}
	pickup, err := careplanCompose.NewPickupBaselines(carePlan, approvedOfferings, func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	classArrivalQueries, err := arrivalTimetable.NewClassArrivalQueries(db, func(arrivalTimetable.Observation) {})
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
		PrincipalReader: AttendancePrincipal,
		SchoolPresence:  newStudentPresence(db, logger),
		GroupRepo:       sessionGroups, SupervisorRepo: sessionSupervisors,
		StudentRepo: PresenceStudents(db, r.Student), RoomRepo: NewAttendanceRooms(r.Room), ActivityGroupRepo: repositories.NewSessionActivities(r.ActivityGroup),
		EducationGroupRepo: NewAttendanceEducationGroups(r.Group, r.Student), StaffRepo: NewAttendanceStaffDirectory(r.Staff),
		DB: db, Broadcaster: hub, Logger: logger, Now: now,
		AttendanceSyncer:         attendanceMirror,
		TimetableBridgeCompleter: bridge,
	}, presenceservice.WithPresenceSettings(PresenceSettings(settings.Settings)))
	offerings := enrollment.NewCareOfferingService(enrollment.CareOfferingServiceConfig{
		Repo: enrollment.NewCareOfferingRepository(r.CarePlan), Bookings: r.Enrollment(), ActivityGroupRepo: r.ActivityGroup,
		ActivityScheduleRepo: r.ActivitySchedule, CalendarPeriodRepo: r.CalendarPeriod, TimeframeRepo: r.Timeframe,
		ActivityExceptionRepo: r.ActivityException, Phases: r.Enrollment(), Settings: settings.Settings, Today: today,
		LockTemplateRecurrence: recurrenceLock.LockRecurrenceWrites,
		Logger:                 logger,
	})
	series := offerings.(enrollment.CareOfferingSeriesValidator)
	calendarAdministration := schoolCalendarAdministration(settings.Settings,
		recurrenceLock.LockRecurrenceWrites,
		offerings.(enrollment.CareOfferingCalendarPeriodValidator))
	calendar, err := repositories.NewSchoolCalendarWithAdministration(db, func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration })
	if err != nil {
		return TimetableTestModule{}, err
	}
	rows := r.TimetableTemplateRows()
	materialization, err := newTimetableMaterialization(timetableMaterializationInputs{
		Rows: rows, CareBounds: r.Student, RecurrenceLock: recurrenceLock, Broadcaster: hub, DB: db, Logger: logger,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	recovery := repositories.NewActivityRecoveryRepository(db, r.InstanceStudent)
	conflicts, err := NewTimetableConflictDetection(TimetableConflictReaders{
		Instances: r.ActivityInstance, InstanceStaff: r.InstanceStaff, InstanceStudents: r.InstanceStudent,
		Exceptions: r.ActivityException, Schedules: r.ActivitySchedule, Staff: r.Staff, CalendarPeriods: r.CalendarPeriod,
		ArrivalExceptions: r.StudentArrivalException, Sessions: r.ActiveGroup, Shifts: r.StaffShift,
		Presence: newStudentPresence(db, logger), ArrivalBaselines: arrival, Logger: logger,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	substituteConflicts, err := newTimetableSubstituteConflicts(r.OwnerRows())
	if err != nil {
		return TimetableTestModule{}, err
	}
	instance := timetableplanning.NewInstanceService(timetableplanning.InstanceServiceDependencies{
		StartConflicts: conflicts, SubstituteConflicts: substituteConflicts,
		Presence:     newStudentPresence(db, logger),
		InstanceRepo: r.ActivityInstance, IdempotencyRepo: r.InstanceIdempotency, InstanceStaffRepo: r.InstanceStaff,
		InstanceStudents: r.InstanceStudent, ExceptionRepo: r.ActivityException, ActiveGroupRepo: r.ActiveGroup,
		SupervisorRepo: r.GroupSupervisor, RoomRepo: r.Room, ActivityGroupRepo: r.ActivityGroup,
		StaffRepo: r.Staff, StudentRepo: r.Student, CalendarPeriodRepo: r.CalendarPeriod,
		ActiveService: ender, Materialization: materialization, RecurrenceLock: recurrenceLock, CareDayService: timetableplanning.NewInstanceCareDays(careDay, carePlan), DeviationEventRepo: r.DeviationEvent,
		Broadcaster: hub, DB: db, Logger: logger, Settings: settings.Settings, RecoveryRepo: recovery, Now: now,
	})
	dataRows := r.OwnerRows()
	dataRows.Locks = recovery
	timetableData, err := newTimetableData(timetableDataInputs{
		Rows: dataRows, ArrivalBaselines: arrival, PickupBaselines: pickup, Visits: newStudentPresence(db, logger),
		Groups: r.Timetable, Sessions: r.ActiveGroup,
		ConflictAcks: r.Timetable, Transactional: true, Logger: logger,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	templates, err := newTimetableTemplates(timetableTemplateInputs{
		Rows: rows, PlanningTracks: arrivalTimetable.NewPlanningTrackAdministration(r.Timetable, db),
		Materialization: materialization, Instances: instance, CareOfferings: series,
		RecurrenceLock: recurrenceLock, Broadcaster: hub, DB: db, Logger: logger, Today: today,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	deviations, err := newTimetableStaffDeviations(timetableDeviationInputs{
		Rows: r.OwnerRows(), Staff: r.Staff, Supervisions: r.GroupSupervisor,
		Lifecycle: instance, Broadcaster: hub, DB: db, Logger: logger, Now: now,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	// The trail stays unwired, as before: a correction fails closed here.
	corrections, err := newTimetableAttendanceCorrections(timetableCorrectionInputs{
		Rows: dataRows, Logger: logger,
	})
	if err != nil {
		return TimetableTestModule{}, err
	}
	planning := TimetablePlanning{
		Templates: templates, RecurrenceLock: recurrenceLock, Data: timetableData, ConflictDetection: conflicts,
		AttendanceCorrections: corrections, Deviations: deviations,
	}
	return TimetableTestModule{Instance: instance, SchoolCalendar: calendar, TimetableData: planning, ConflictDetection: conflicts, Materialization: materialization, RealtimeHub: hub}, nil
}
