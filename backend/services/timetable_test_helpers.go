package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type TimetableTestModule struct {
	Instance        schedule.InstanceService
	CalendarPeriod  schedule.CalendarPeriodService
	TimetableData   *schedule.TimetableDataService
	Materialization schedule.MaterializationService
	RealtimeHub     *realtime.Hub
}

func NewTimetableTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (TimetableTestModule, error) {
	r, err := repositories.NewTimetableTestRepositories(db, clocks...)
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
	pickup := schedule.NewPickupBaselineServiceWithSettings(r.StudentPickupSchedule, r.RequestChildOffering, r.CareOffering, settings.Settings)
	arrival := schedule.NewArrivalBaselineService(r.StudentArrivalSchedule, r.Student,
		r.ClassArrivalTime, r.ClassArrivalException, r.RequestChildOffering, r.CareOffering, settings.Settings)
	careDay := schedule.NewCareDayService(schedule.CareDayDependencies{
		ArrivalBaselines: arrival, ArrivalSchedules: r.StudentArrivalSchedule, ArrivalExceptions: r.StudentArrivalException,
		PickupBaselines: pickup, PickupExceptions: r.StudentPickupException,
	})
	care, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return TimetableTestModule{}, err
	}
	schedule.WireCareParticipation(careDay, care.CareLifecycle)
	bridge := schedule.NewTimetableBridgeService(schedule.TimetableBridgeDependencies{
		Instances: r.ActivityInstance, InstanceStudents: r.InstanceStudent, CareDays: careDay,
	})
	// Instance completion consumes only the active-session end capability.
	// Retain its real transaction, visit sync, supervision and SSE paths.
	ender := active.NewService(active.ServiceDependencies{
		GroupRepo: r.ActiveGroup, VisitRepo: r.ActiveVisit, SupervisorRepo: r.GroupSupervisor,
		StudentRepo: r.Student, PersonRepo: r.Person, RoomRepo: r.Room, ActivityGroupRepo: r.ActivityGroup,
		EducationGroupRepo: r.Group, StaffRepo: r.Staff, TeacherRepo: r.Teacher,
		DB: db, Broadcaster: hub, Logger: logger, Now: now,
		AttendanceSyncer:         schedule.NewAttendanceSyncService(r.ActivityInstance, r.InstanceStudent, logger),
		TimetableBridgeCompleter: bridge,
	})
	ender.SetSettingsService(settings.Settings)
	offerings := enrollment.NewCareOfferingService(enrollment.CareOfferingServiceConfig{
		Repo: r.CareOffering, RequestChildOfferingRepo: r.RequestChildOffering, ActivityGroupRepo: r.ActivityGroup,
		ActivityScheduleRepo: r.ActivitySchedule, CalendarPeriodRepo: r.CalendarPeriod, TimeframeRepo: r.Timeframe,
		ActivityExceptionRepo: r.ActivityException, PhaseRepo: r.Phase, Settings: settings.Settings, Today: today,
		LockTemplateRecurrence: func(ctx context.Context) error { return schedule.LockTenantRecurrenceWrites(ctx, db) },
		Logger:                 logger,
	})
	series := offerings.(enrollment.CareOfferingSeriesValidator)
	periods := schedule.NewCalendarPeriodServiceWithConfig(schedule.CalendarPeriodServiceConfig{
		Repo: r.CalendarPeriod, DB: db, Logger: logger,
		ValidateCareOfferingChange: offerings.(enrollment.CareOfferingCalendarPeriodValidator).ValidateCalendarPeriodChange,
	})
	materialization := schedule.NewMaterializationService(r.ActivityGroup, r.ActivitySchedule, r.StudentEnrollment,
		r.ActivitySupervisor, r.CalendarPeriod, r.ActivityInstance, r.InstanceStaff, r.InstanceStudent,
		r.ActivityException, r.Timeframe, periods, db, hub, logger)
	schedule.WireMaterializationCareBounds(materialization, r.Student)
	recovery := repositories.NewActivityRecoveryRepository(db, r.InstanceStudent)
	instance := schedule.NewInstanceService(schedule.InstanceServiceDependencies{
		InstanceRepo: r.ActivityInstance, IdempotencyRepo: r.InstanceIdempotency, InstanceStaffRepo: r.InstanceStaff,
		InstanceStudents: r.InstanceStudent, ExceptionRepo: r.ActivityException, ActiveGroupRepo: r.ActiveGroup,
		SupervisorRepo: r.GroupSupervisor, VisitRepo: r.ActiveVisit, RoomRepo: r.Room, ActivityGroupRepo: r.ActivityGroup,
		StaffRepo: r.Staff, StudentRepo: r.Student, CalendarPeriodRepo: r.CalendarPeriod,
		ActiveService: ender, Materialization: materialization, CareDayService: careDay, DeviationEventRepo: r.DeviationEvent,
		Broadcaster: hub, DB: db, Logger: logger, Settings: settings.Settings, RecoveryRepo: recovery, Now: now,
	})
	data := schedule.NewTimetableDataService(schedule.TimetableDataDependencies{
		InstanceStudentRepo: r.InstanceStudent, ActivityInstanceRepo: r.ActivityInstance, ActivityExceptionRepo: r.ActivityException,
		ActivityScheduleRepo: r.ActivitySchedule, InstanceStaffRepo: r.InstanceStaff, StaffShiftRepo: r.StaffShift,
		StaffRepo: r.Staff, CalendarPeriodRepo: r.CalendarPeriod, ActiveGroupRepo: r.ActiveGroup, SupervisorRepo: r.GroupSupervisor,
		ArrivalScheduleRepo: r.StudentArrivalSchedule, ArrivalBaselines: arrival, ArrivalExceptionRepo: r.StudentArrivalException,
		PickupScheduleRepo: r.StudentPickupSchedule, PickupBaselines: pickup, PickupExceptionRepo: r.StudentPickupException,
		VisitRepo: r.ActiveVisit, RoomRepo: r.Room, ActivityCategoryRepo: r.ActivityCategory, PlanningTrackRepo: r.PlanningTrack,
		ActivityGroupRepo: r.ActivityGroup, ActivitySupervisorRepo: r.ActivitySupervisor, StudentEnrollmentRepo: r.StudentEnrollment,
		TimeframeRepo: r.Timeframe, EducationGroupRepo: r.Group,
		ValidateCareOfferingSeries: series.ValidateTemplateSeries, ValidateOfferingSource: series.ValidateTemplateOfferingSource,
		DeviationEventRepo: r.DeviationEvent, ConflictAckRepo: r.TimetableConflictAck, RecoveryRepo: recovery,
		Broadcaster: hub, Logger: logger, DB: db, Today: today,
	})
	return TimetableTestModule{Instance: instance, CalendarPeriod: periods, TimetableData: data, Materialization: materialization, RealtimeHub: hub}, nil
}
