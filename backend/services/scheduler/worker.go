package scheduler

import (
	"fmt"
	"log/slog"
	"time"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	pwaSvc "github.com/moto-nrw/project-phoenix/modules/delivery/application/pwa"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// WorkerDependencies is the complete typed input to the embedded Worker root.
// Construction replaces the former post-construction Set* graph.
type WorkerDependencies struct {
	Logger                    *slog.Logger
	Getenv                    func(string) string
	DB                        *bun.DB
	SchoolRepo                platform.SchoolRepository
	TenantRuntime             *tenant.UnitOfWork
	TenantRuntimeObserver     func(entryPoint, outcome string)
	UnitOfWorkObserver        func(entryPoint, kind, result string, duration time.Duration, retries int)
	Tracer                    WorkerTracer
	Settings                  SettingsResolver
	Active                    active.Service
	ActiveCleanup             active.CleanupService
	AuthCleanup               AuthCleanup
	InvitationCleanup         InvitationCleaner
	EmailChangeCleanup        EmailChangeTokenCleaner
	OperatorInvitationCleanup OperatorInvitationCleaner
	WorkSessionCleanup        WorkSessionCleaner
	BreakAutoEnder            BreakAutoEnder
	AutoCheckouter            AutoCheckouter
	FeedbackCleaner           FeedbackCleaner
	UnregisteredScanCleaner   UnregisteredTagScanCleaner
	StaffDocumentCleaner      StaffDocumentFileCleaner
	StudentDocumentCleaner    StudentDocumentFileCleaner
	FileStoreCleaner          FileStoreCleaner
	Materializer              scheduleSvc.MaterializationService
	TimetableCleanup          scheduleSvc.TimetableCleanupService
	CalendarFeedCleanup       CalendarFeedCleaner
	TimeTrackingCleanup       active.TimeTrackingCleanupService
	StudentChangeLogCleanup   usersSvc.StudentChangeLogCleanupService
	PWAUsageCleanup           pwaSvc.UsageService
	StaffMessageCleanup       communication.StaffMessageCleanup
	BookingConsistency        auditModel.BookingConsistencyRepository
	EnrollmentRejectedCleanup enrollmentSvc.RejectedEnrollmentCleaner
	AutoStart                 scheduleSvc.AutoStartService
	AutoEnd                   scheduleSvc.AutoEndService
	InstanceRepo              scheduleModel.ActivityInstanceRepository
	InstanceRoomRepo          facilitiesModel.RoomRepository
	InstanceStudentRepo       scheduleModel.InstanceStudentRepository
	TimetableBridge           TimetableBridgeCompleter
	StudentStatusDayRepo      activeModel.StudentStatusDayRepository
	OverdueBroadcaster        realtime.Broadcaster
	StudentLifecycleRepo      StudentLifecycleRepository
	StudentLifecycleAudit     StudentLifecycleAuditor
	CareExitEffector          CareExitEffector
	OutboxWorker              OutboxWorkerRunner
	RolloverDeadlineRunner    RolloverDeadlineRunner
	ReminderNotifications     ReminderNotificationDeps
	AppointmentReminders      AppointmentReminderQueuer
}

// NewWorker constructs and validates the complete embedded worker before the
// Serve root can become ready.
func NewWorker(deps WorkerDependencies) (*Scheduler, error) {
	if err := validateWorkerDependencies(deps); err != nil {
		return nil, err
	}

	worker := newScheduler(deps)
	registry, err := NewRegistry(requiredWorkerJobIDs(), worker.jobDefinitions()...)
	if err != nil {
		return nil, fmt.Errorf("build worker registry: %w", err)
	}
	worker.registry = registry
	return worker, nil
}

func validateWorkerDependencies(deps WorkerDependencies) error {
	required := []struct {
		name  string
		value any
	}{
		{name: "logger", value: deps.Logger},
		{name: "database", value: deps.DB},
		{name: "school repository", value: deps.SchoolRepo},
		{name: "tenant runtime", value: deps.TenantRuntime},
		{name: "settings", value: deps.Settings},
		{name: "auth cleanup", value: deps.AuthCleanup},
		{name: "invitation cleanup", value: deps.InvitationCleanup},
		{name: "email change cleanup", value: deps.EmailChangeCleanup},
		{name: "operator invitation cleanup", value: deps.OperatorInvitationCleanup},
		{name: "feedback cleanup", value: deps.FeedbackCleaner},
	}
	for _, dependency := range required {
		if isNilDependency(dependency.value) {
			return fmt.Errorf("worker dependency %s is required", dependency.name)
		}
	}
	return nil
}

func requiredWorkerJobIDs() []JobID {
	return []JobID{
		"visit-cleanup",
		"timetable-cleanup",
		"time-tracking-cleanup",
		"student-change-log-cleanup",
		"pwa-usage-cleanup",
		"staff-message-cleanup",
		"booking-consistency-audit",
		"session-end",
		"token-cleanup",
		"staff-document-file-cleanup",
		"student-document-file-cleanup",
		"file-store-cleanup",
		"session-cleanup",
		"break-auto-end",
		"auto-checkout",
		"status-flag-clear",
		"timetable-materialization",
		"instance-overdue",
		"reminder-notifications",
		"timetable-auto-start",
		"timetable-auto-end",
		"activate-students",
		"email-outbox",
		"rollover-deadline",
		"appointment-reminders",
	}
}

func (s *Scheduler) jobDefinitions() []Job {
	jobs := make([]Job, 0, len(requiredWorkerJobIDs()))
	add := func(ready bool, id JobID, start func()) {
		if ready {
			jobs = append(jobs, schedulerJob{id: id, start: start})
		}
	}
	add(!isNilDependency(s.cleanupService), "visit-cleanup", s.scheduleCleanupTask)
	add(!isNilDependency(s.timetableCleanup), "timetable-cleanup", s.scheduleTimetableCleanupTask)
	add(!isNilDependency(s.timeTrackingCleanup), "time-tracking-cleanup", s.scheduleTimeTrackingCleanupTask)
	add(!isNilDependency(s.studentChangeLogCleanup), "student-change-log-cleanup", s.scheduleStudentChangeLogCleanupTask)
	add(!isNilDependency(s.pwaUsageCleanup), "pwa-usage-cleanup", s.schedulePWAUsageCleanupTask)
	add(!isNilDependency(s.staffMessageCleanup), "staff-message-cleanup", s.scheduleStaffMessageCleanupTask)
	add(!isNilDependency(s.bookingConsistency), "booking-consistency-audit", s.scheduleBookingConsistencyAuditTask)
	add(!isNilDependency(s.activeService), "session-end", s.scheduleSessionEndTask)
	add(true, "token-cleanup", s.scheduleTokenCleanupTask)
	add(!isNilDependency(s.staffDocumentFileCleaner), "staff-document-file-cleanup", s.scheduleStaffDocumentFileCleanupTask)
	add(!isNilDependency(s.studentDocumentFileCleaner), "student-document-file-cleanup", s.scheduleStudentDocumentFileCleanupTask)
	add(!isNilDependency(s.fileStoreCleaner), "file-store-cleanup", s.scheduleFileStoreCleanupTask)
	add(!isNilDependency(s.activeService), "session-cleanup", s.scheduleSessionCleanupTask)
	add(!isNilDependency(s.breakAutoEnder), "break-auto-end", s.scheduleBreakAutoEndTask)
	add(!isNilDependency(s.autoCheckouter), "auto-checkout", s.scheduleAutoCheckoutTask)
	add(!isNilDependency(s.studentStatusDayRepo), "status-flag-clear", s.scheduleStatusFlagClearTask)
	add(!isNilDependency(s.materializer), "timetable-materialization", s.scheduleMaterializationTask)
	add(!isNilDependency(s.instanceRepo) && !isNilDependency(s.instanceRoomRepo) && !isNilDependency(s.overdueBroadcaster), "instance-overdue", s.scheduleInstanceOverdueTask)
	add(s.reminderNotifications.complete(), "reminder-notifications", s.scheduleReminderNotificationTask)
	add(!isNilDependency(s.autoStart), "timetable-auto-start", s.scheduleAutoStartTask)
	add(!isNilDependency(s.autoEnd), "timetable-auto-end", s.scheduleAutoEndTask)
	add(!isNilDependency(s.studentLifecycleRepo), "activate-students", s.scheduleActivateStudentsTask)
	add(!isNilDependency(s.outboxWorker), "email-outbox", s.scheduleOutboxWorkerTask)
	add(!isNilDependency(s.rolloverDeadlineRunner), "rollover-deadline", s.scheduleRolloverDeadlineTask)
	add(!isNilDependency(s.appointmentReminders), "appointment-reminders", s.scheduleAppointmentReminderTask)
	return jobs
}

type schedulerJob struct {
	id    JobID
	start func()
}

func (job schedulerJob) ID() JobID { return job.id }

func (job schedulerJob) Start() { job.start() }
