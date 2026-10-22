package application

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
)

// NewWorker constructs and validates the complete embedded worker before the
// Serve root can become ready.
func NewWorker(deps ports.WorkerDependencies) (*Scheduler, error) {
	if err := validateWorkerDependencies(deps); err != nil {
		return nil, err
	}

	retention, err := newRetentionRun(deps.RetentionSteps, deps.RetentionLog, deps.Logger)
	if err != nil {
		return nil, err
	}
	worker := newScheduler(deps)
	worker.retention = retention
	registry, err := NewRegistry(requiredWorkerJobIDs(), worker.jobDefinitions()...)
	if err != nil {
		return nil, fmt.Errorf("build worker registry: %w", err)
	}
	worker.registry = registry
	return worker, nil
}

func validateWorkerDependencies(deps ports.WorkerDependencies) error {
	required := []struct {
		name  string
		value any
	}{
		{name: "logger", value: deps.Logger},
		{name: "tenant directory", value: deps.SchoolRepo},
		{name: "tenant runtime", value: deps.TenantRuntime},
		{name: "settings", value: deps.Settings},
		{name: "auth cleanup", value: deps.AuthCleanup},
		{name: "invitation cleanup", value: deps.InvitationCleanup},
		{name: "email change cleanup", value: deps.EmailChangeCleanup},
		{name: "operator invitation cleanup", value: deps.OperatorInvitationCleanup},
		{name: "operator ledger retention", value: deps.OperatorRetention.Ledger},
		{name: "operator account retention", value: deps.OperatorRetention.Accounts},
		{name: "account retention", value: deps.AccountRetention},
		{name: "feedback cleanup", value: deps.FeedbackCleaner},
		{name: "cleanup run ledger", value: deps.CleanupRuns},
		{name: "export transfer recovery", value: deps.SettleExpiredTransfers},
	}
	for _, dependency := range required {
		if isNilDependency(dependency.value) {
			return fmt.Errorf("worker dependency %s is required", dependency.name)
		}
	}
	return validateWorkerLease(deps.Lease)
}

func requiredWorkerJobIDs() []ports.JobID {
	return []ports.JobID{
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
		"announcement-reminders",
		"billing-key-dates",
		"analytics-school-names",
	}
}

func (s *Scheduler) jobDefinitions() []Job {
	jobs := make([]Job, 0, len(requiredWorkerJobIDs()))
	add := func(ready bool, id ports.JobID, start func()) {
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
	add(s.reminderNotifications != nil, "reminder-notifications", s.scheduleReminderNotificationTask)
	add(!isNilDependency(s.autoStart), "timetable-auto-start", s.scheduleAutoStartTask)
	add(!isNilDependency(s.autoEnd), "timetable-auto-end", s.scheduleAutoEndTask)
	add(!isNilDependency(s.studentLifecycleRepo), "activate-students", s.scheduleActivateStudentsTask)
	add(!isNilDependency(s.outboxWorker), "email-outbox", s.scheduleOutboxWorkerTask)
	add(!isNilDependency(s.rolloverDeadlineRunner), "rollover-deadline", s.scheduleRolloverDeadlineTask)
	add(!isNilDependency(s.appointmentReminders), "appointment-reminders", s.scheduleAppointmentReminderTask)
	add(!isNilDependency(s.announcementReminders), "announcement-reminders", s.scheduleAnnouncementReminderTask)
	add(!isNilDependency(s.schoolRepo), "billing-key-dates", s.scheduleBillingKeyDatesTask)
	add(!isNilDependency(s.schoolRepo), "analytics-school-names", s.scheduleSchoolNamesTask)
	return jobs
}

type schedulerJob struct {
	id    ports.JobID
	start func()
}

func (job schedulerJob) ID() ports.JobID { return job.id }

func (job schedulerJob) Start() { job.start() }
