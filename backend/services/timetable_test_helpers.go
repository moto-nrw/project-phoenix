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
	TimetableData  *timetableplanning.TimetableDataService
	// ConflictDetection is the Timetable owner's conflict detection and
	// staffing capability over the same repositories (#3550).
	ConflictDetection timetable.ConflictDetectionCapability
	Materialization   timetableplanning.MaterializationService
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
	bridge := timetableplanning.NewTimetableBridgeService(timetableplanning.TimetableBridgeDependencies{
		Instances: r.ActivityInstance, InstanceStudents: r.InstanceStudent, CareDays: careDay,
	})
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
		AttendanceSyncer:         timetableplanning.NewAttendanceSyncService(r.ActivityInstance, r.InstanceStudent, logger),
		TimetableBridgeCompleter: bridge,
	}, presenceservice.WithPresenceSettings(PresenceSettings(settings.Settings)))
	offerings := enrollment.NewCareOfferingService(enrollment.CareOfferingServiceConfig{
		Repo: enrollment.NewCareOfferingRepository(r.CarePlan), Bookings: r.Enrollment(), ActivityGroupRepo: r.ActivityGroup,
		ActivityScheduleRepo: r.ActivitySchedule, CalendarPeriodRepo: r.CalendarPeriod, TimeframeRepo: r.Timeframe,
		ActivityExceptionRepo: r.ActivityException, Phases: r.Enrollment(), Settings: settings.Settings, Today: today,
		LockTemplateRecurrence: func(ctx context.Context) error { return timetableplanning.LockTenantRecurrenceWrites(ctx, db) },
		Logger:                 logger,
	})
	series := offerings.(enrollment.CareOfferingSeriesValidator)
	calendarAdministration := schoolCalendarAdministration(settings.Settings,
		func(ctx context.Context) error { return timetableplanning.LockTenantRecurrenceWrites(ctx, db) },
		offerings.(enrollment.CareOfferingCalendarPeriodValidator))
	calendar, err := repositories.NewSchoolCalendarWithAdministration(db, func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration })
	if err != nil {
		return TimetableTestModule{}, err
	}
	materialization := timetableplanning.NewMaterializationService(r.ActivityGroup, r.ActivitySchedule, r.StudentEnrollment,
		r.ActivitySupervisor, r.CalendarPeriod, r.ActivityInstance, r.InstanceStaff, r.InstanceStudent,
		r.ActivityException, r.Timeframe, db, hub, logger,
		timetableplanning.WithCareBoundReader(r.Student))
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
	instance := timetableplanning.NewInstanceService(timetableplanning.InstanceServiceDependencies{
		StartConflicts: conflicts,
		Presence:       newStudentPresence(db, logger),
		InstanceRepo:   r.ActivityInstance, IdempotencyRepo: r.InstanceIdempotency, InstanceStaffRepo: r.InstanceStaff,
		InstanceStudents: r.InstanceStudent, ExceptionRepo: r.ActivityException, ActiveGroupRepo: r.ActiveGroup,
		SupervisorRepo: r.GroupSupervisor, RoomRepo: r.Room, ActivityGroupRepo: r.ActivityGroup,
		StaffRepo: r.Staff, StudentRepo: r.Student, CalendarPeriodRepo: r.CalendarPeriod,
		ActiveService: ender, Materialization: materialization, CareDayService: timetableplanning.NewInstanceCareDays(careDay, carePlan), DeviationEventRepo: r.DeviationEvent,
		Broadcaster: hub, DB: db, Logger: logger, Settings: settings.Settings, RecoveryRepo: recovery, Now: now,
	})
	data := timetableplanning.NewTimetableDataService(timetableplanning.TimetableDataDependencies{
		InstanceStudentRepo: r.InstanceStudent, ActivityInstanceRepo: r.ActivityInstance, ActivityExceptionRepo: r.ActivityException,
		ActivityScheduleRepo: r.ActivitySchedule, InstanceStaffRepo: r.InstanceStaff,
		ActiveGroupRepo: r.ActiveGroup, SupervisorRepo: r.GroupSupervisor,
		ArrivalBaselines: arrival, ArrivalExceptionRepo: r.StudentArrivalException,
		PickupScheduleRepo: r.StudentPickupSchedule, PickupBaselines: pickup, PickupExceptionRepo: r.StudentPickupException,
		Presence: newStudentPresence(db, logger), RoomRepo: r.Room, ActivityCategoryRepo: r.ActivityCategory, PlanningTracks: arrivalTimetable.NewPlanningTrackAdministration(r.Timetable, db),
		ActivityGroupRepo: r.ActivityGroup, ActivitySupervisorRepo: r.ActivitySupervisor, StudentEnrollmentRepo: r.StudentEnrollment,
		TimeframeRepo: r.Timeframe, EducationGroupRepo: r.Group,
		ValidateCareOfferingSeries: series.ValidateTemplateSeries, ValidateOfferingSource: series.ValidateTemplateOfferingSource,
		DeviationEventRepo: r.DeviationEvent, ConflictAcks: r.Timetable, ConflictDetection: conflicts, RecoveryRepo: recovery,
		Broadcaster: hub, Logger: logger, DB: db, Today: today,
	})
	return TimetableTestModule{Instance: instance, SchoolCalendar: calendar, TimetableData: data, ConflictDetection: conflicts, Materialization: materialization, RealtimeHub: hub}, nil
}
