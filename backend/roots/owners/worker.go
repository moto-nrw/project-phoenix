package owners

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"

	schedulerRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"

	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	exportTransferCompose "github.com/moto-nrw/project-phoenix/modules/exporttransfer/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// NewWorker starts the Worker over the owners a root built: the server root
// passes the owners it mounts, the Worker root the owners its jobs need.
// Each argument is one owner's value or a single-owner bundle; the Worker's
// own collaborators (lease, tenant runtime, settings snapshots, retention,
// booking audit, reminder tick, document sweeps) are built here.
func NewWorker(
	db transactionruntime.Database, logger *slog.Logger, tracer *observability.Tracer, hub *realtime.Hub,
	settings settingsCompose.BatchSettingsService, audit auditCommand, accessLog DataAccessAppender, fleet devicefleet.Capability,
	carePlan carePlanModule.Owner, persons peopledirectory.Capability, people PeopleRows,
	organizations organizationtenancy.Query, identity identityaccess.Owner, rooms facilitiesModule.Capability,
	timetableRepos timetableCompose.Repositories, workTime workforceModule.Capability, staffAdmin *workforceModule.StaffAdmin,
	workforce WorkforcePlanning, active studentpresence.Presence, reports PresenceReports, lifecycle TimetableLifecycle,
	days TimetableDays, materialization timetable.MaterializationCapability, schedules CarePlanSchedules,
	documents carePlanModule.StudentDocuments, services PeopleServices, studentAudit peoplePorts.StudentAuditService,
	notices EnrollmentNotices, flows EnrollmentFlows, studentDeletion *studentdeletion.Workflow, feedback feedbackModule.Capability,
	calendarApp calendarCompose.Application, notifications DeliveryNotifications, usage deliveryCompose.PWAUsageService,
	staffMessaging communication.StaffMessagingRuntime, outbox *deliveryModule.Worker, reminders reminder.Module,
	fileStore filestorageModule.Capability, announcements communication.ParentAnnouncementCapability, mealPlan mealplanModule.Capability, schoolNames SchoolNamesPublisher,
	plannedExits PlannedStaffExitsBuilder, dayLog WeekProtocolDayLogBuilder,
) (schedulerPorts.Worker, error) {
	billing, err := NewOperatorBilling(logger)
	if err != nil {
		return nil, fmt.Errorf("compose worker billing: %w", err)
	}
	lease, runtime, err := newWorkerRuntime(db.UnitOfWork())
	if err != nil {
		return nil, err
	}
	resolver, err := schedulerSettings(settings)
	if err != nil {
		return nil, err
	}
	retention, err := workerRetentionOf(db, logger, settings, audit, fleet, carePlan, communicationCompose.NewParentMessageRetention(db.Pool(), settingsValues.NewParentMessageRetentionValues(settings)), notices.RejectedCleanup, studentDeletion, identity, announcements, timetableRepos.Timetable)
	if err != nil {
		return nil, err
	}
	bookings, err := schedulerBookingConsistency(db, persons, carePlan)
	if err != nil {
		return nil, err
	}
	reminderNotifications, err := newPersonalReminderTick(reminders, notifications, people.Staff, identity, workTime, runtime, logger.With("service", "scheduler"))
	if err != nil {
		return nil, fmt.Errorf("compose personal reminder notifications: %w", err)
	}
	staffDocuments, studentDocuments, err := workerDocumentSweeps(db, logger, services.Users, staffAdmin, workforce.TimeTracking, documents)
	if err != nil {
		return nil, err
	}
	return schedulerRuntimeCompose.NewWorker(schedulerPorts.WorkerDependencies{
		Logger:                       logger.With("service", "scheduler"),
		Getenv:                       os.Getenv,
		SchoolRepo:                   schedulerTenantDirectory{schools: organizations, billing: billing, schoolNames: schoolNames},
		TenantRuntime:                runtime,
		TenantRuntimeObserver:        observability.RecordTenantRuntimeEvent,
		UnitOfWorkObserver:           observability.RecordUnitOfWorkEvent,
		Tracer:                       workerTracer(tracer),
		Settings:                     resolver,
		Lease:                        lease,
		RetentionSteps:               retention.steps,
		RetentionLog:                 retention.log,
		CleanupRuns:                  retention.runs,
		StaffDocumentCleaner:         staffDocuments,
		StudentDocumentCleaner:       studentDocuments,
		FileStoreCleaner:             newFileStoreCleaner(db, fileStore, logger),
		Active:                       newSchedulerSessionMaintenance(active),
		ActiveCleanup:                newSchedulerPresenceCleanup(reports.ActiveCleanup),
		AuthCleanup:                  &identityaccessCompose.SessionMaintenance{AccountSessionMaintenance: identity, PasswordResets: identity},
		InvitationCleanup:            identity,
		EmailChangeCleanup:           identity,
		OperatorInvitationCleanup:    identity,
		OperatorRetention:            schedulerPorts.OperatorRetention{Ledger: audit, Accounts: identity},
		AccountRetention:             identity,
		HolidayRegistrationRetention: enrollmentCompose.NewHolidayRegistrationRetention(nil),
		WorkSessionCleanup:           workforce.TimeTracking.Jobs,
		BreakAutoEnder:               workforce.TimeTracking.Jobs,
		AutoCheckouter:               workforce.TimeTracking.Jobs,
		FeedbackCleaner:              feedback,
		Materializer:                 newSchedulerMaterializer(materialization),
		TimetableCleanup:             newSchedulerTimetableCleanup(lifecycle.Cleanup),
		CalendarFeedCleanup:          calendarApp,
		TimeTrackingCleanup:          schedulerTimeTrackingCleanupPort(workforce.TimeTrackingCleanup),
		StudentChangeLogCleanup:      schedulerStudentChangeLogCleanup(services.StudentChangeLogCleanup),
		PWAUsageCleanup:              pwaUsageCleanup(usage),
		StaffMessageCleanup:          staffMessageCleanup(staffMessaging),
		EnrollmentRejectedCleanup:    enrollmentRejectedCleanup{cleaner: notices.RejectedCleanup},
		AutoStart:                    newSchedulerAutoStart(lifecycle.AutoStart),
		AutoEnd:                      newSchedulerAutoEnd(lifecycle.AutoEnd),
		TimetableBridge:              days.Bridge,
		StudentLifecycleAudit:        schedulerStudentAudit(studentAudit),
		CareExitEffector:             schedules.Lifecycle,
		OutboxWorker:                 outbox,
		SettleExpiredTransfers:       exportTransferCompose.NewDispatchRecovery(),
		AppointmentReminders:         reminders,
		RolloverDeadlineRunner:       rolloverDeadlines{rollovers: flows.Rollovers},
		BookingConsistency:           bookings,
		MealOrders:                   mealPlan.SettleOrders,
		InstanceRepo:                 schedulerDayInstances(timetableRepos),
		InstanceRoomRepo:             schedulerExistingRooms(rooms),
		InstanceStudentRepo:          timetableRepos.ParticipantCheckouts(),
		StudentStatusDayRepo:         NewStudentStatusDayRepository(carePlan),
		OverdueBroadcaster:           newSchedulerOverdueBroadcaster(hub),
		StudentLifecycleRepo:         schedulerStudentLifecycle(NewStudentRootReads(people.Students)), WeekProtocols: NewWeekProtocolEvenings(db, logger, settings, persons, services, reports, schedules, carePlan, timetableRepos, dayLog),
		PayrollDispatch:       workforce.PayrollDispatch.RunDue,
		ReminderNotifications: reminderNotifications, HolidayGuests: schedulerHolidayGuests(persons, carePlan, services.Users, accessLog), PlannedStaffExits: plannedExits(identity, notifications, people.Staff), SchoolVacations: schoolVacationSync(),
	})
}

// schoolVacationSync binds the Worker's school vacation job to the School
// Calendar's platform-wide sync (#5345). The composition needs no
// collaborator that can be missing; a failure still surfaces on every run.
func schoolVacationSync() schedulerPorts.SchoolVacationSync {
	sync, composeErr := schoolCalendarCompose.NewSchoolVacations(nil, "", nil)
	return func(ctx context.Context) (int, int, error) {
		if composeErr != nil {
			return 0, 0, fmt.Errorf("compose school vacation sync: %w", composeErr)
		}
		result, err := sync.SyncSchoolVacations(ctx)
		return result.Regions, result.Vacations, err
	}
}
