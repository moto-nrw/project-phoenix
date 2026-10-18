package api

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	appointmentsModule "github.com/moto-nrw/project-phoenix/modules/appointments"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetModule "github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	exportTransferCompose "github.com/moto-nrw/project-phoenix/modules/exporttransfer/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	schedulerRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/observability"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
)

// newWorker starts the Worker over the owners a root built: the server root
// passes the owners it mounts, the Worker root the owners its jobs need.
// Each argument is one owner's value or a single-owner bundle; the Worker's
// own collaborators (lease, tenant runtime, settings snapshots, retention,
// booking audit, reminder tick, document sweeps) are built here.
func newWorker(
	db transactionruntime.Database, logger *slog.Logger, tracer *observability.Tracer, hub *realtime.Hub,
	settings settingsCompose.BatchSettingsService, audit *auditCompose.Command, fleet devicefleet.Capability,
	carePlan *carePlanModule.Module, persons peopledirectory.Capability, people PeopleRows,
	organizations organizationtenancy.Query, identity *identityaccess.Module, rooms facilitiesModule.Capability,
	timetableRepos timetableCompose.Repositories, workTime workforceModule.Capability, staffAdmin *workforceModule.StaffAdmin,
	workforce WorkforcePlanning, active studentpresence.Presence, reports PresenceReports, lifecycle TimetableLifecycle,
	days TimetableDays, materialization timetable.MaterializationCapability, schedules CarePlanSchedules,
	documents carePlanModule.StudentDocuments, services PeopleServices, studentAudit peoplePorts.StudentAuditService,
	notices EnrollmentNotices, flows EnrollmentFlows, studentDeletion *studentdeletion.Workflow, feedback *feedbackModule.Module,
	calendarApp calendarCompose.Application, notifications DeliveryNotifications, usage deliveryCompose.PWAUsageService,
	staffMessaging communication.StaffMessagingRuntime, outbox *deliveryModule.Worker, reminders reminder.Module,
	fileStore *filestorageModule.Module,
) (schedulerPorts.Worker, error) {
	billing, err := newOperatorBilling(logger)
	if err != nil {
		return nil, fmt.Errorf("compose worker billing: %w", err)
	}
	lease, err := newWorkerLease(db.UnitOfWork())
	if err != nil {
		return nil, fmt.Errorf("compose worker lease: %w", err)
	}
	runtime, err := schedulerRuntimeCompose.NewTenantRuntime(db.UnitOfWork())
	if err != nil {
		return nil, fmt.Errorf("compose worker tenant runtime: %w", err)
	}
	if err := verifySchedulerSettingKeys(); err != nil {
		return nil, err
	}
	resolver, err := schedulerSettings(settings)
	if err != nil {
		return nil, err
	}
	retention, err := workerRetentionOf(db, logger, settings, audit, fleet, carePlan, communicationCompose.NewParentMessageRetention(db.Pool(), settingsValues.NewParentMessageRetentionValues(settings)), notices.RejectedCleanup, studentDeletion)
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
		Logger:                    logger.With("service", "scheduler"),
		Getenv:                    os.Getenv,
		SchoolRepo:                schedulerTenantDirectory{schools: organizations, billing: billing},
		TenantRuntime:             runtime,
		TenantRuntimeObserver:     observability.RecordTenantRuntimeEvent,
		UnitOfWorkObserver:        observability.RecordUnitOfWorkEvent,
		Tracer:                    workerTracer(tracer),
		Settings:                  resolver,
		Lease:                     lease,
		RetentionSteps:            retention.steps,
		RetentionLog:              retention.log,
		CleanupRuns:               retention.runs,
		StaffDocumentCleaner:      staffDocuments,
		StudentDocumentCleaner:    studentDocuments,
		FileStoreCleaner:          fileStore,
		Active:                    newSchedulerSessionMaintenance(active),
		ActiveCleanup:             newSchedulerPresenceCleanup(reports.ActiveCleanup),
		AuthCleanup:               &identityaccessCompose.SessionMaintenance{AccountSessionMaintenance: identity, PasswordResets: identity},
		InvitationCleanup:         identity,
		EmailChangeCleanup:        identity,
		OperatorInvitationCleanup: identity,
		OperatorRetention:         schedulerPorts.OperatorRetention{Ledger: audit, Accounts: identity},
		WorkSessionCleanup:        workforce.TimeTracking.Jobs,
		BreakAutoEnder:            workforce.TimeTracking.Jobs,
		AutoCheckouter:            workforce.TimeTracking.Jobs,
		FeedbackCleaner:           feedback,
		Materializer:              newSchedulerMaterializer(materialization),
		TimetableCleanup:          newSchedulerTimetableCleanup(lifecycle.Cleanup),
		CalendarFeedCleanup:       calendarApp,
		TimeTrackingCleanup:       schedulerTimeTrackingCleanupPort(workforce.TimeTrackingCleanup),
		StudentChangeLogCleanup:   schedulerStudentChangeLogCleanup(services.StudentChangeLogCleanup),
		PWAUsageCleanup:           pwaUsageCleanup(usage),
		StaffMessageCleanup:       staffMessageCleanup(staffMessaging),
		EnrollmentRejectedCleanup: enrollmentRejectedCleanup{cleaner: notices.RejectedCleanup},
		AutoStart:                 newSchedulerAutoStart(lifecycle.AutoStart),
		AutoEnd:                   newSchedulerAutoEnd(lifecycle.AutoEnd),
		TimetableBridge:           days.Bridge,
		StudentLifecycleAudit:     schedulerStudentAudit(studentAudit),
		CareExitEffector:          schedules.Lifecycle,
		OutboxWorker:              outbox,
		AppointmentReminders:      reminders,
		RolloverDeadlineRunner:    rolloverDeadlines{rollovers: flows.Rollovers},
		BookingConsistency:        bookings,
		InstanceRepo:              schedulerDayInstances(timetableRepos),
		InstanceRoomRepo:          schedulerExistingRooms(rooms),
		InstanceStudentRepo:       timetableRepos.InstanceStudent,
		StudentStatusDayRepo:      NewStudentStatusDayRepository(carePlan),
		OverdueBroadcaster:        newSchedulerOverdueBroadcaster(hub),
		StudentLifecycleRepo:      schedulerStudentLifecycle(NewStudentRootReads(people.Students)),
		ReminderNotifications:     reminderNotifications,
	})
}

// workerRetentionOf builds the nightly cleanup's retention: the student and
// staff deletions and every owner's retention step.
func workerRetentionOf(
	db transactionruntime.Database, logger *slog.Logger, settings settingsCompose.BatchSettingsService, audit *auditCompose.Command,
	fleet devicefleet.Capability, carePlan *carePlanModule.Module, parentMessages communication.ParentMessageCleanup,
	rejected enrollmentOwner.RejectedEnrollmentCleaner, studentDeletion *studentdeletion.Workflow,
) (workerRetention, error) {
	students, err := newStudentRetention(studentDeletion, settings, logger.With("service", "student_retention"))
	if err != nil {
		return workerRetention{}, fmt.Errorf("compose worker student retention: %w", err)
	}
	staff, err := newStaffRetention(db, audit, settings, logger.With("service", "staff_retention"))
	if err != nil {
		return workerRetention{}, fmt.Errorf("compose worker staff retention: %w", err)
	}
	steps := dataRetentionSteps(
		settings, fleet, carePlan, parentMessages, rejected,
		workforceCompose.NewSickNoteRetention(StaffSickNoteRetentionSettings{Settings: settings}),
		audit, deliveryCompose.NewRetention(),
	)
	steps = append(steps, protocolRetentionSteps(
		audit, settingsCompose.NewAuditRetention(), exportTransferCompose.NewJournalRetention(),
	)...)
	steps = append(steps, cleanupJobRetentionSteps(
		filestorageCompose.NewCleanupRetention(), workforceCompose.NewCleanupRetention(),
		carePlanCompose.NewCleanupRetention(), enrollmentCompose.New(),
	)...)
	return newWorkerRetention(audit, settings, students, staff, steps)
}

// workerDocumentSweeps build the Worker's recovery passes over the staff and
// child document objects (#2745): Workforce's personnel documents and Care
// Plan's child documents, each over File Storage's object removal.
func workerDocumentSweeps(db transactionruntime.Database, logger *slog.Logger, users peoplePorts.PersonDirectory, staffAdmin *workforceModule.StaffAdmin, timeTracking *workforceCompose.TimeTracking, documents carePlanModule.StudentDocuments) (schedulerPorts.StaffDocumentFileCleaner, schedulerPorts.StudentDocumentFileCleaner, error) {
	cleanup, err := workforceCompose.NewDocumentCleanup(db.Pool(), nil)
	if err != nil {
		return nil, nil, err
	}
	staffLogger := logger.With("handler", "staff")
	staff, err := workforceCompose.NewStaffDocumentFileRecovery(NewWorkforceAdminCapabilities(users, staffAdmin, timeTracking).Documents, cleanup, nativeUploads.UploadFiles{}, staffLogger)
	if err != nil {
		return nil, nil, err
	}
	students, err := carePlanModule.NewStudentDocumentFileCleanup(documents, studentDocumentFileRemoval, logger.With("handler", "students"))
	if err != nil {
		return nil, nil, err
	}
	return staff, students, nil
}

// studentDocumentFileRemoval opens File Storage's removal of stored child
// document objects; the storage backend is looked up on each pass.
func studentDocumentFileRemoval(kind string, documents carePlanModule.StudentDocuments, logger *slog.Logger) (carePlanModule.StudentDocumentFileRemover, error) {
	backend, err := nativeUploads.PrivateUploadsBackend()
	if err != nil {
		return nil, err
	}
	return filestorageCompose.NewDocumentCoordinator(kind, backend, documents, logger).Remove, nil
}

// rolloverDeadlines runs Enrollment's rollover deadline job for the Worker,
// which discards the job's summary.
type rolloverDeadlines struct {
	rollovers enrollmentOwner.Rollovers
}

func (r rolloverDeadlines) RunDeadlineWorker(ctx context.Context, asOf time.Time) (any, error) {
	return r.rollovers.RunDeadlineWorker(ctx, asOf)
}

func staffMessageCleanup(service communication.StaffMessagingRuntime) schedulerPorts.StaffMessageCleanup {
	return func(ctx context.Context) (schedulerPorts.StaffMessageCleanupResult, error) {
		result, err := service.CleanupExpiredStaffMessages(ctx)
		return schedulerPorts.StaffMessageCleanupResult{
			MessagesDeleted: result.MessagesDeleted,
			ThreadsDeleted:  result.ThreadsDeleted,
			RetentionDays:   result.RetentionDays,
		}, err
	}
}

func workerTracer(tracer *observability.Tracer) schedulerPorts.WorkerTracer {
	return schedulerPorts.WorkerTracer{
		StartJob: func(ctx context.Context, operation string) (context.Context, error) {
			ctx, _, err := tracer.StartJob(ctx, operation)
			return ctx, err
		},
		Logger: tracer.Logger,
		Failure: func(ctx context.Context, operation, outcome string, err error) {
			tracer.Failure(ctx, "worker", operation, outcome, err)
		},
		Run: func(jobID schedulerPorts.JobID, outcome string, duration time.Duration) {
			observability.RecordWorkerRunEvent(string(jobID), outcome, duration)
		},
		Batch: func(event schedulerPorts.TenantBatchEvidence) {
			observability.RecordWorkerTenantBatchEvent(
				string(event.JobID),
				event.Duration,
				event.Processed,
				event.Failed,
				event.Retries,
				event.Backlog,
				event.PoolWait,
			)
		},
		Backlog: func(jobID schedulerPorts.JobID, backlog int) {
			observability.SetWorkerTenantBatchBacklog(string(jobID), backlog)
		},
	}
}

// composeWorkerRoot builds the owners the Worker's jobs need as local values,
// in the build order of the owner graph, without the HTTP router, and starts
// the Worker over them through newWorker, the function the server root
// starts its embedded Worker with (#4083).
func composeWorkerRoot(db transactionruntime.Database, env RootEnvironment, hub *realtime.Hub, tracer *observability.Tracer, tracker ProductEventTracker) (schedulerPorts.Worker, error) {
	unit := db.UnitOfWork()
	communicationCompose.InstallMessageQueryInstrumentation(db.Pool())
	renderer := lists.NewRendererService()
	var (
		tokens           TokenAdapter
		objects          filestorageCompose.ObjectBackend
		audit            AuditOwner
		transport        MailTransport
		organizations    organizationtenancy.Capability
		groups           *schoolstructure.Module
		rooms            *facilitiesModule.Rooms
		membership       *schoolMembershipModule.Module
		bookings         *enrollmentOwner.BookingProjection
		settings         SettingsOwner
		calendar         *schoolcalendar.Records
		workTime         *workforceModule.Module
		people           PeopleRows
		peopleIdentity   *identityaccess.Module
		guardian         peoplePorts.GuardianService
		photoRuntime     studentPhotoRuntime
		persons          *peopledirectory.Module
		timetableOwner   *timetable.Module
		timetableRepos   TimetableRepositories
		carePlan         *carePlanModule.Module
		appointments     appointmentsModule.Capability
		operatorAuditLog operatorAuditEntries
		mealPlanSettings *mealplanCompose.Settings
		mealPlan         *mealplanModule.Module
		feedbackSettings *feedbackCompose.Settings
		feedback         *feedbackModule.Module
		structure        StructureRows
		presencePeople   PresencePeopleReads
		careRows         TimetableCareRows
		devices          devicefleetModule.Capability
		presence         PresenceOwner
		substitutions    GroupSubstitutionRepository
		supervisions     studentpresence.SupervisionRecords
		timetableRows    TimetableOwner
		classArrivals    timetable.ClassArrivals
		studentAudit     peoplePorts.StudentAuditService
		carePlanRows     CarePlanRows
		enrollment       EnrollmentRows
		delivery         DeliveryRuntime
		mailIdentity     email.ReplyToResolver
		identity         *identityaccess.Module
		caller           *CallerRows
		portalReads      carePlanCompose.ParentPortalRepositories
		messaging        parentStore.MessagingRepositories
		consent          communicationModule.NotificationConsentCapability
		notifications    DeliveryNotifications
		emitter          *communicationCompose.ParentEventEmitter
		purger           filestorageModule.AttachmentPurger
		announcements    communicationModule.ParentAnnouncementCapability
		schedules        CarePlanSchedules
		services         PeopleServices
		days             TimetableDays
		payroll          settingsCompose.PayrollStatusGetter
		side             *workforceCompose.WorkSessionSide
		active           studentpresence.Presence
		activities       arrivalTimetable.ActivityService
		shifts           *workforceCompose.ShiftPlanning
		deviceFleet      *devicefleetModule.Module
		materialization  timetable.MaterializationCapability
		lifecycle        TimetableLifecycle
		reports          PresenceReports
		offerings        CarePlanOfferings
		notices          EnrollmentNotices
		phases           *enrollmentCompose.Phases
		decisions        *enrollmentCompose.Decisions
		flows            EnrollmentFlows
		reviewPolicy     *ParentRequestReviewPolicy
		requests         CarePlanRequests
		chat             CommunicationMessaging
		calendarApp      calendarCompose.Application
		parent           *parentportal.Portal
		fileStore        *filestorageModule.Module
		planExport       planexport.Service
		workforce        WorkforcePlanning
		staffAdmin       *workforceModule.StaffAdmin
		provisioning     organizationtenancy.Provisioning
		deliveryDevices  DeliveryDevices
		reminders        reminder.Module
		studentDeletion  *studentdeletion.Workflow
		facilities       *facilitiesModule.Module
		outbox           *deliveryModule.Worker
		staffGroups      schoolstructure.StaffGroupQuery
	)
	err := runRootSteps(
		func() (err error) { tokens, err = newLegacyTokenAdapter(); return },
		func() (err error) {
			objects, err = nativeUploads.PrivateUploadsBackend()
			return stepError("resolve uploads backend", err)
		},
		func() (err error) { audit, err = buildAuditOwner(db, env); return },
		func() (err error) { transport, err = buildMailTransport(env, hub); return },
		func() (err error) { organizations, err = buildServeOrganizations(db); return },
		func() (err error) { groups, err = buildServeSchoolStructure(db); return },
		func() (err error) { rooms, err = buildServeRooms(db); return },
		func() (err error) { membership, err = composeSchoolMembership(db); return },
		func() error { bookings = NewEnrollmentBookings(); return nil },
		func() (err error) {
			settings, err = buildSettingsOwner(db, unit, env, organizations, transport.Hub, bookings)
			return
		},
		func() (err error) {
			calendar, err = buildSchoolCalendarRecords(db, observeSchoolCalendar, settings.Service)
			return
		},
		func() (err error) { workTime, err = buildServeWorkforce(db, membership, calendar); return },
		func() error {
			people, peopleIdentity = buildPeopleRows(db, env.Observe.IdentityAccess, groups, rooms, membership, workTime)
			return nil
		},
		func() (err error) { guardian, err = buildGuardianService(db, people, audit); return },
		func() error {
			unlinker := peopledirectory.NewPhotoUnlinker(env.Logger.With("component", "student-photo-unlinker"), "public", nativeUploads.UploadFiles{})
			photoRuntime = buildStudentPhotoRuntime(env, settings.Service, transport, audit, unlinker, actingAccountID)
			return nil
		},
		func() (err error) {
			persons, err = buildPeopleDirectory(db, observePeopleDirectory, photoRuntime, guardianDirectoryProvider(db, guardian))
			return
		},
		func() (err error) { timetableOwner, err = buildServeTimetable(db, persons, rooms, membership); return },
		func() error {
			timetableRepos = NewTimetableRepositories(db, timetableOwner, persons, groups, rooms, calendar, membership, workTime)
			return nil
		},
		func() (err error) {
			carePlan, err = composeCarePlan(db, persons, CarePlanStatusSlots(timetableRepos.InstanceStudent))
			return
		},
		func() (err error) { appointments, err = buildServeAppointments(db); return },
		func() error { operatorAuditLog = NewOperatorAuditLog(db); return nil },
		func() (err error) { mealPlanSettings, err = buildMealPlanSettings(settings.Service); return },
		func() (err error) {
			mealPlan, err = buildServeMealPlan(db, mealPlanSettings, NewStudentRootReads(people.Students))
			return
		},
		func() (err error) { feedbackSettings, err = buildFeedbackSettings(settings.Service); return },
		func() (err error) { feedback, err = buildServeFeedback(db, feedbackSettings); return },
		func() (err error) {
			structure = buildStructureRows(db, persons, people, peopleIdentity, rooms, membership, workTime)
			presencePeople = buildPresencePeopleReads(db, env.Logger, persons, groups, organizations, people)
			careRows = buildTimetableCareRows(carePlan)
			devices, err = NewDeviceFleet(db)
			return stepError("compose device fleet", err)
		},
		func() error {
			presence = buildPresenceOwner(db, env.Logger, env.Now, devices, persons, rooms, membership, carePlan, timetableOwner, timetableRepos, people, structure)
			substitutions = NewGroupSubstitutionRepository(structure.Groups, membership, workTime)
			supervisions = NewGroupSupervisionRecords(presence.SessionRecords, groups)
			return nil
		},
		func() (err error) {
			timetableRows, err = buildTimetableOwner(db, env.Logger, env.Now, timetableOwner, timetableRepos, calendar, bookings, structure.Groups)
			return
		},
		func() (err error) { classArrivals, err = buildClassArrivals(db, env.Logger); return },
		func() (err error) {
			studentAudit = peopleCompose.NewStudentAuditService(tokens.AuditActor, NewStudentAuditFor(persons))
			carePlanRows, err = buildCarePlanRows(db, env.Logger, studentAudit, carePlan, persons, membership, timetableOwner, calendar, classArrivals, people)
			return
		},
		func() (err error) {
			enrollment = buildEnrollmentRows(bookings, carePlan, persons)
			delivery, err = buildDeliveryRuntime(db, env, people.GuardianProfiles)
			return
		},
		func() (err error) {
			staffGroups, err = education.NewStaffGroups(education.StaffGroupsDependencies{Groups: groups, Assignments: membership, Substitutions: workTime})
			return
		},
		func() (err error) {
			mailIdentity = tenantMailIdentity(organizations, settings.Service, env)
			portalReads = NewParentPortalReads(db, persons, peopleIdentity, enrollmentCompose.New(), organizations)
			identity, caller, err = buildIdentityAccess(db, unit, env, tokens, organizations, persons, membership, settings.Service,
				audit, operatorAuditLog, transport, mailIdentity, delivery.Enqueuer(), people, portalReads.ParentEnrollmentRequest,
				structure.Groups, staffGroups, arrivalTimetable.NewCallerActivityGroups(timetableRepos.ActivityGroup),
				presence.SessionRecords, presence.Presence)
			return
		},
		func() (err error) {
			messaging = NewParentMessagingRepositories(db, groups, membership, peopleIdentity, enrollmentCompose.New(), nil, organizations)
			consent, err = communicationCompose.NewNotificationConsent(communicationCompose.NotificationConsentConfig{
				DB: db.Pool(), Observe: env.Observe.Communication,
			})
			return
		},
		func() (err error) {
			notifications, err = buildDeliveryNotifications(db, unit, env, transport, deliveryModuleOf(delivery), settings.Service,
				consent, identity, people, structure, workTime, messaging.ParentMessageThread)
			return
		},
		func() (err error) {
			emitter = buildParentEventEmitter(db, unit, env, transport, settings.Service, messaging.ParentMessageThread, notifications)
			purger, err = buildAttachmentPurger(db, env, objects)
			return
		},
		func() (err error) {
			announcements, err = buildParentAnnouncements(env, settings.Service, delivery.Outbox, deliveryModuleOf(delivery),
				notifications, messaging.ParentAnnouncement, purger)
			return
		},
		func() (err error) {
			schedules, err = buildCarePlanSchedules(db, env.Logger, carePlan, persons, carePlanRows, classArrivals,
				enrollment.ApprovedOfferings, settings.Service, timetableRows.RecurrenceLock.LockRecurrenceWrites, presence.Presence)
			return
		},
		func() error {
			services = buildPeopleServices(db, env, persons, people, peopleIdentity, identity, schedules.Lifecycle, audit, tokens,
				guardian, studentAudit, photoRuntime, NewCaregiverBindingLocker(membership, workTime, presence.Presence, timetableOwner),
				supervisions, substitutions, structure,
				arrivalTimetable.CaregiverPlannedSupervisionBlockers{Supervisors: timetableRepos.ActivitySupervisor}, settings.Service)
			payroll = payrollStatus(settings.Service, people.Staff)
			return nil
		},
		func() (err error) {
			days, err = buildTimetableDays(timetableRows, people, rooms, careRows, audit, schedules.CareDay)
			return
		},
		func() (err error) {
			side, err = buildWorkSessionSide(db, env, transport, mailIdentity, workTime, membership, persons, organizations, people,
				services.Users, audit, supervisions, presence.SessionRecords, calendar, settings.Service, payroll)
			return
		},
		func() (err error) {
			active, err = buildActivePresence(db, unit, env, transport, settings.Service, presence, presencePeople, carePlanRows, people,
				services.Users, rooms, devices, timetableRows, days, side.AutoCheckIn, emitter, tracker)
			return
		},
		func() (err error) {
			activities, err = buildActivities(timetableRows, timetableOwner, presence.SessionRecords, people)
			return
		},
		func() (err error) {
			shifts, err = buildShiftPlanning(db, env, transport, workTime, people, calendar, audit, timetableRows, rooms, activities)
			return
		},
		func() (err error) {
			deviceFleet, err = buildDeviceFleet(db, env, settings.Service, rooms, presence, timetableRows, schedules.Pickups, organizations)
			return
		},
		func() (err error) {
			materialization, err = buildTimetableMaterialization(db, env, transport, timetableRows, people, calendar)
			return
		},
		func() (err error) {
			lifecycle, err = buildTimetableLifecycle(db, env, transport, timetableRows, days, people, careRows, presence, supervisions,
				workforceShiftRows(workTime), schedules, carePlan, active, materialization, announcements, settings.Service)
			return
		},
		func() (err error) {
			lifecycle, err = buildTimetableDay(env, transport, lifecycle, timetableOwner, audit, settings.Service, presence, supervisions,
				services.Users, membership, active, schedules)
			return
		},
		func() error {
			reports = buildPresenceReports(db, env, presence, carePlanRows, carePlan, services.Users, timetableRows, rooms, calendar,
				people, audit, settings.Service)
			return nil
		},
		func() (err error) {
			offerings, err = buildCarePlanOfferings(env, carePlan, persons, people, timetableOwner, calendar, timetableRows, enrollment,
				settings.Service, schedules, audit, identity, transport.Hub, emitter)
			return
		},
		func() (err error) {
			notices = buildEnrollmentNotices(env, enrollment, settings.Service, delivery.Enqueuer(), organizations, audit, carePlan, deliveryModuleOf(delivery))
			phases, err = buildEnrollmentPhases(env, enrollment, carePlan, persons, calendar, timetableRows, offerings, settings.Service)
			return
		},
		func() error {
			decisions = buildEnrollmentDecisions(db, env, transport, enrollment, notices, audit, people, careRows, persons, carePlan,
				offerings, identity, services, schedules, carePlanRows, timetableRows, settings.Service)
			return nil
		},
		func() error {
			flows = buildEnrollmentFlows(db, env, enrollment, notices, decisions, phases, offerings, consent, identity, people, structure,
				carePlan, carePlanRows, schedules, membership, organizations, audit, settings.Service, delivery.Enqueuer())
			reviewPolicy = NewParentRequestReviewPolicy(caller.Caller().ParentRequestReviews)
			return nil
		},
		func() (err error) {
			requests, err = buildCarePlanRequests(db, env, transport, carePlan, persons, people, schedules, offerings, enrollment,
				timetableOwner, settings.Service, audit, services, emitter, caller, reviewPolicy, tokens.Permissions, notifications.AbsenceNotifier)
			return
		},
		func() error {
			chat = buildCommunicationMessaging(db, env, transport, settings.Service, messaging, services.Users, StaffStudentReads{Caller: caller.Caller()},
				notifications, delivery.Outbox, people, schoolNameDirectory{schools: organizations})
			calendarApp = buildCalendarApp(db, env, people, structure, portalReads, timetableRows, rooms, workTime, organizations,
				identity, caller.Caller(), appointments, calendar, delivery, notifications, settings.Service, persons)
			return nil
		},
		func() (err error) {
			parent, err = buildParentPortal(db, env, transport, settings.Service, people, portalReads, messaging, presence, carePlanRows,
				carePlan, persons, mealPlan, schedules, requests, services, emitter, notifications, identity, enrollment, notices)
			return
		},
		func() (err error) {
			fileStore, err = buildFileStore(db, env, objects, identity, persons, settings.Service, audit, announcements, parent)
			return
		},
		func() (err error) {
			planExport = buildPlanExport(env, shifts, workTime, timetableRows, rooms, people, timetableOwner, calendar, renderer)
			workforce, err = buildWorkforcePlanning(db, env, transport, workTime, membership, persons, shifts, planExport, side,
				lifecycle, timetableRows, audit, settings.Service)
			return
		},
		func() (err error) {
			staffAdmin, err = buildWorkforceStaffAdmin(db, env, persons, membership, people, audit)
			return
		},
		func() (err error) {
			provisioning, err = buildProvisioning(db, env, organizations, identity, persons, membership, people, devices,
				presence.SessionRecords, supervisions, timetableRows, operatorAuditLog, settings.Service)
			return
		},
		func() (err error) {
			deliveryDevices, err = buildDeliveryDevices(db, unit, env, transport, identity, provisioning, settings.Service)
			return
		},
		func() error {
			reminders = buildReminders(db, env, caller, settings.Service, schedules, active, supervisions, timetableOwner, rooms, people,
				calendarApp, announcements)
			return nil
		},
		func() (err error) {
			studentDeletion, err = buildStudentDeletion(db, env, transport, persons, carePlan, timetableOwner, timetableRows, feedback,
				caller.HasCurrentStaff, services.StudentPhotos, audit)
			return
		},
		func() (err error) {
			facilities, err = buildFacilitiesModule(db, observeFacilities, timetableRows.RecurrenceLock, presence.Presence, requests.CareOfferings)
			return
		},
		func() (err error) {
			outbox, err = buildDeliveryWorker(db, env, transport, mailIdentity, delivery, buildCalendarAppointmentRenderer(db, transport, people),
				identityDeliveryRenderers(identity, transport.DefaultFrom, mailIdentity, delivery.Runtime.EmailStatus, env))
			return
		},
	)
	if err != nil {
		return nil, err
	}
	return newWorker(db, env.Logger, tracer, hub, settings.Service, audit.Command, deviceFleet, carePlan, persons, people,
		organizations, identity, facilities, timetableRepos, workTime, staffAdmin, workforce, active, reports, lifecycle, days,
		materialization, schedules, requests.Documents, services, studentAudit, notices, flows, studentDeletion, feedback, calendarApp,
		notifications, deliveryDevices.PWAUsage, chat.StaffMessaging, outbox, reminders, fileStore)
}
