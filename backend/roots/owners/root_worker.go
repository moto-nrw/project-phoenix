package owners

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	appointmentsModule "github.com/moto-nrw/project-phoenix/modules/appointments"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
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
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/realtime"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"

	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
)

// workerRetentionOf builds the nightly cleanup's retention: the student and
// staff deletions and every owner's retention step.
func workerRetentionOf(
	db transactionruntime.Database, logger *slog.Logger, settings settingsCompose.BatchSettingsService, audit auditCommand,
	fleet devicefleet.Capability, carePlan *carePlanModule.Module, parentMessages communication.ParentMessageCleanup,
	rejected enrollmentOwner.RejectedEnrollmentCleaner, studentDeletion *studentdeletion.Workflow, identity *identityaccess.Module,
	announcements communication.ParentAnnouncementCapability,
) (workerRetention, error) {
	announcementRetention, ok := announcements.(communication.ParentAnnouncementRetention)
	if !ok {
		return workerRetention{}, fmt.Errorf("compose worker retention: parent announcements %T lack their retention", announcements)
	}
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
	contentSteps, err := newSchoolContentRetentionSteps(db, settings, announcementRetention)
	if err != nil {
		return workerRetention{}, err
	}
	steps = append(steps, contentSteps...)
	attendance, err := presenceCompose.New(presenceCompose.Dependencies{DB: db.Pool(), Observe: presenceCompose.FailureLogger()})
	if err != nil {
		return workerRetention{}, fmt.Errorf("compose worker attendance retention: %w", err)
	}
	return newWorkerRetention(audit, settings, students, staff, steps, attendance, newGuardianRetention(db, identity))
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
	if service == nil {
		return nil
	}
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

// ComposeWorker builds the owners the Worker's jobs need as local values,
// in the build order of the owner graph, without the HTTP router, and starts
// the Worker over them through newWorker, the function the server root
// starts its embedded Worker with (#4083).
func ComposeWorker(db transactionruntime.Database, env RootEnvironment, hub *realtime.Hub, tracer *observability.Tracer, tracker ProductEventTracker, tokensFor func() (TokenAdapter, error), communicationSession interface {
	AccountID(context.Context) int64
	Permissions(context.Context) []string
}, timetableSession timetablePorts.RequestSession, principal func(context.Context) studentpresence.RequestPrincipal, withStaff func(context.Context, int64, int64) context.Context, documentPermissions carePlanCompose.StudentDocumentPermissions) (schedulerPorts.Worker, error) {
	unit := db.UnitOfWork()
	communicationCompose.InstrumentMessageQueries(db.Pool(), false)
	renderer := lists.NewRendererService()
	var (
		tokens           TokenAdapter
		objects          filestorageCompose.ObjectBackend
		audit            AuditOwner
		mailSender       email.Mailer
		mailDispatcher   email.Dispatcher
		mailFrom         email.Email
		pushVAPID        deliveryCompose.VAPIDConfig
		organizations    organizationtenancy.Capability
		groups           schoolstructure.Owner
		rooms            *facilitiesModule.Rooms
		membership       *schoolMembershipModule.Module
		bookings         *enrollmentOwner.BookingProjection
		settings         SettingsOwner
		calendar         *schoolcalendar.Records
		workTime         *workforceCompose.WorkTime
		people           PeopleRows
		peopleIdentity   *identityaccess.Module
		guardian         peoplePorts.GuardianService
		photoRuntime     StudentPhotoRuntime
		persons          *peopleCompose.Directory
		timetableOwner   *timetable.Module
		timetableRepos   TimetableRepositories
		carePlan         *carePlanModule.Module
		appointments     appointmentsModule.Capability
		operatorAuditLog OperatorAuditEntries
		mealPlanSettings *mealplanCompose.Settings
		mealPlan         *mealplanModule.Module
		feedbackSettings *feedbackCompose.Settings
		feedback         *feedbackModule.Module
		structure        StructureRows
		presencePeople   PresencePeopleReads
		careRows         TimetableCareRows
		devices          devicefleet.Capability
		presence         PresenceOwner
		substitutions    GroupSubstitutions
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
		consent          communication.NotificationConsentCapability
		notifications    DeliveryNotifications
		emitter          eventContract.Emitter
		purger           filestorageModule.AttachmentPurger
		announcements    communication.ParentAnnouncementCapability
		schedules        CarePlanSchedules
		services         PeopleServices
		days             TimetableDays
		payroll          settingsCompose.PayrollStatusGetter
		side             *workforceCompose.WorkSessionSide
		active           studentpresence.Presence
		activities       activityContract.ActivityService
		shifts           *workforceCompose.ShiftPlanning
		deviceFleet      *devicefleet.Module
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
		fileStore        filestorageModule.Capability
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
	err := RunRootSteps(
		func() (err error) { tokens, err = tokensFor(); return },
		func() (err error) {
			objects, err = nativeUploads.PrivateUploadsBackend()
			return StepError("resolve uploads backend", err)
		},
		func() (err error) { audit, err = BuildAuditOwner(db, env); return },
		func() (err error) {
			mailSender, mailDispatcher, mailFrom, pushVAPID, err = BuildMailTransport(env)
			return
		},
		func() (err error) { organizations, err = BuildServeOrganizations(db); return },
		func() (err error) { groups, err = BuildServeSchoolStructure(db); return },
		func() (err error) { rooms, err = BuildServeRooms(db); return },
		func() (err error) { membership, err = ComposeSchoolMembership(db); return },
		func() error { bookings = NewEnrollmentBookings(); return nil },
		func() (err error) {
			settings, err = BuildSettingsOwner(db, unit, env, organizations, hub, bookings)
			return
		},
		func() (err error) {
			calendar, err = BuildSchoolCalendarRecords(db, ObserveSchoolCalendar, settings.Service)
			return
		},
		func() (err error) { workTime, err = BuildServeWorkforce(db, membership, calendar); return },
		func() error {
			people, peopleIdentity = BuildPeopleRows(db, env.Observe.IdentityAccess, groups, rooms, membership, workTime)
			return nil
		},
		func() (err error) { guardian, err = BuildGuardianService(db, people, audit); return },
		func() error {
			unlinker := peopledirectory.NewPhotoUnlinker(env.Logger.With("component", "student-photo-unlinker"), "public", nativeUploads.UploadFiles{})
			photoRuntime = BuildStudentPhotoRuntime(db, env, settings.Service, hub, audit, unlinker, func(ctx context.Context) int64 { claims, _ := tokens.Claims(ctx); return claims.AccountID })
			return nil
		},
		func() (err error) {
			persons, err = BuildPeopleDirectory(db, ObservePeopleDirectory, photoRuntime, GuardianDirectoryProvider(db, guardian))
			return
		},
		func() (err error) { timetableOwner, err = BuildServeTimetable(db, persons, rooms, membership); return },
		func() error {
			timetableRepos = NewTimetableRepositories(db, timetableOwner, persons, groups, rooms, calendar, membership, workTime)
			return nil
		},
		func() (err error) {
			carePlan, err = ComposeCarePlan(db, persons, CarePlanStatusSlots(timetableRepos.InstanceStudent))
			return
		},
		func() (err error) { appointments, err = BuildServeAppointments(db); return },
		func() error { operatorAuditLog = NewOperatorAuditLog(db); return nil },
		func() (err error) { mealPlanSettings, err = BuildMealPlanSettings(settings.Service); return },
		func() (err error) {
			mealPlan, err = BuildServeMealPlan(db, mealPlanSettings, NewStudentRootReads(people.Students))
			return
		},
		func() (err error) { feedbackSettings, err = BuildFeedbackSettings(settings.Service); return },
		func() (err error) { feedback, err = BuildServeFeedback(db, feedbackSettings); return },
		func() (err error) {
			structure = BuildStructureRows(db, persons, people, peopleIdentity, rooms, membership, workTime)
			presencePeople = BuildPresencePeopleReads(db, env.Logger, persons, groups, organizations, people)
			careRows = BuildTimetableCareRows(carePlan)
			devices, err = NewDeviceFleet(db)
			return StepError("compose device fleet", err)
		},
		func() error {
			presence = BuildPresenceOwner(db, env.Logger, env.Now, devices, persons, rooms, membership, carePlan, timetableOwner, timetableRepos, people, structure)
			substitutions = NewGroupSubstitutionRepository(structure.Groups, membership, workTime)
			supervisions = NewGroupSupervisionRecords(presence.SessionRecords, groups)
			return nil
		},
		func() (err error) {
			timetableRows, err = BuildTimetableOwner(db, env.Logger, env.Now, timetableOwner, timetableRepos, calendar, bookings, structure.Groups)
			return
		},
		func() (err error) { classArrivals, err = BuildClassArrivals(db, env.Logger); return },
		func() (err error) {
			studentAudit = peopleCompose.NewStudentAuditService(tokens.AuditActor, NewStudentAuditFor(persons))
			carePlanRows, err = BuildCarePlanRows(db, env.Logger, studentAudit, carePlan, persons, membership, timetableOwner, calendar, classArrivals, people)
			return
		},
		func() (err error) {
			enrollment = BuildEnrollmentRows(bookings, carePlan, persons)
			delivery, err = BuildDeliveryRuntime(db, env, guardian)
			return
		},
		func() (err error) {
			staffGroups, err = education.NewStaffGroups(education.StaffGroupsDependencies{Groups: groups, Assignments: membership, Substitutions: workTime})
			return
		},
		func() (err error) {
			mailIdentity = TenantMailIdentity(organizations, settings.Service, env)
			portalReads = NewParentPortalReads(db, persons, peopleIdentity, enrollmentCompose.New(), organizations)
			identity, caller, err = BuildIdentityAccess(db, unit, env, tokens, organizations, persons, membership, settings.Service,
				audit, operatorAuditLog, mailSender, mailDispatcher, mailFrom, delivery.Runtime, mailIdentity, delivery.Enqueuer(), people, portalReads.ParentEnrollmentRequest,
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
			notifications, err = BuildDeliveryNotifications(db, unit, env, pushVAPID, hub, DeliveryModuleOf(delivery), settings.Service,
				consent, identity, people, structure, workTime, messaging)
			return
		},
		func() (err error) {
			emitter = BuildParentEventEmitter(db, unit, env, hub, settings.Service, messaging, notifications)
			purger, err = BuildAttachmentPurger(db, env, objects)
			return
		},
		func() (err error) {
			announcements, err = BuildParentAnnouncements(env, settings.Service, delivery.Outbox, DeliveryModuleOf(delivery),
				notifications, messaging, purger)
			return
		},
		func() (err error) {
			schedules, err = BuildCarePlanSchedules(db, env.Logger, carePlan, persons, carePlanRows, classArrivals,
				enrollment.ApprovedOfferings, settings.Service, timetableRows.RecurrenceLock.LockRecurrenceWrites, presence.Presence)
			return
		},
		func() error {
			services = BuildPeopleServices(db, env, persons, people, peopleIdentity, identity, schedules.Lifecycle, audit, tokens,
				guardian, studentAudit, photoRuntime, NewCaregiverBindingLocker(membership, workTime, presence.Presence, timetableOwner),
				supervisions, substitutions, structure,
				CaregiverPlannedSupervisionBlockers{Supervisors: timetableOwner}, settings.Service)
			payroll = PayrollStatus(settings.Service, people)
			return nil
		},
		func() (err error) {
			days, err = BuildTimetableDays(timetableRows, people, rooms, careRows, audit, schedules.CareDay)
			return
		},
		func() (err error) {
			side, err = BuildWorkSessionSide(db, env, mailFrom, hub, mailIdentity, workTime, membership, persons, organizations, people,
				services.Users, audit, supervisions, presence.SessionRecords, calendar, settings.Service, payroll, delivery.Enqueuer())
			return
		},
		func() (err error) {
			active, err = BuildActivePresence(db, unit, env, hub, settings.Service, presence, presencePeople, carePlanRows, people,
				services.Users, rooms, devices, timetableRows, days, side.AutoCheckIn, emitter, tracker, principal)
			return
		},
		func() (err error) {
			activities, err = BuildActivities(timetableRows, timetableOwner, presence.SessionRecords, people)
			return
		},
		func() (err error) {
			shifts, err = BuildShiftPlanning(db, env, hub, workTime, people, calendar, audit, timetableRows, rooms, activities)
			return
		},
		func() (err error) {
			deviceFleet, err = BuildDeviceFleet(db, env, settings.Service, rooms, presence, timetableRows, schedules.Pickups, organizations)
			return
		},
		func() (err error) {
			materialization, err = BuildTimetableMaterialization(db, env, hub, timetableRows, people, calendar)
			return
		},
		func() (err error) {
			lifecycle, err = BuildTimetableLifecycle(db, env, hub, timetableRows, days, people, careRows, presence, supervisions,
				WorkforceShiftRows(workTime), schedules, carePlan, active, materialization, announcements, settings.Service)
			return
		},
		func() (err error) {
			lifecycle, err = BuildTimetableDay(env, hub, lifecycle, timetableOwner, audit, settings.Service, presence, supervisions,
				services.Users, membership, active, schedules, timetableSession, withStaff)
			return
		},
		func() error {
			reports = BuildPresenceReports(db, env, presence, carePlanRows, carePlan, services.Users, timetableRows, rooms, calendar,
				people, audit, settings.Service)
			return nil
		},
		func() (err error) {
			offerings, err = BuildCarePlanOfferings(env, carePlan, persons, people, timetableOwner, calendar, timetableRows, enrollment,
				settings.Service, schedules, audit, identity, hub, emitter)
			return
		},
		func() (err error) {
			notices = BuildEnrollmentNotices(env, enrollment, settings.Service, delivery.Enqueuer(), organizations, audit, carePlan, DeliveryModuleOf(delivery))
			phases, err = BuildEnrollmentPhases(env, enrollment, carePlan, persons, calendar, timetableRows, offerings, settings.Service)
			return
		},
		func() error {
			decisions = BuildEnrollmentDecisions(db, env, hub, enrollment, notices, audit, people, careRows, persons, carePlan,
				offerings, identity, services, schedules, carePlanRows, timetableRows, settings.Service)
			return nil
		},
		func() error {
			flows = BuildEnrollmentFlows(db, env, enrollment, notices, decisions, phases, offerings, consent, identity, people, structure,
				carePlan, carePlanRows, schedules, membership, organizations, audit, settings.Service, delivery.Enqueuer())
			reviewPolicy = NewParentRequestReviewPolicy(caller.Caller().ParentRequestReviews)
			return nil
		},
		func() (err error) {
			requests, err = BuildCarePlanRequests(db, env, hub, carePlan, persons, people, schedules, offerings, enrollment,
				timetableOwner, settings.Service, audit, services, emitter, caller, reviewPolicy, tokens.Permissions, notifications.AbsenceNotifier, documentPermissions)
			return
		},
		func() error {
			chat = BuildCommunicationMessaging(db, env, hub, settings.Service, messaging, services.Users, StaffStudentReads{Caller: caller.Caller()},
				notifications, delivery.Outbox, people, SchoolNameDirectory{Schools: organizations}, communicationSession)
			calendarApp = BuildCalendarApp(db, env, people, structure, portalReads, timetableRows, rooms, workTime, organizations,
				identity, caller.Caller(), appointments, calendar, delivery, notifications, settings.Service, persons)
			return nil
		},
		func() (err error) {
			parent, err = BuildParentPortal(db, env, hub, settings.Service, people, portalReads, messaging, presence, carePlanRows,
				carePlan, persons, mealPlan, schedules, requests, services, emitter, notifications, identity, enrollment, notices)
			return
		},
		func() (err error) {
			fileStore, err = BuildFileStore(db, env, objects, identity, persons, settings.Service, audit, announcements, parent)
			return
		},
		func() (err error) {
			planExport = BuildPlanExport(env, shifts, workTime, timetableRows, rooms, people, timetableOwner, calendar, renderer)
			workforce, err = BuildWorkforcePlanning(db, env, hub, workTime, membership, persons, shifts, planExport, side,
				lifecycle, timetableRows, audit, settings.Service)
			return
		},
		func() (err error) {
			staffAdmin, err = BuildWorkforceStaffAdmin(db, env, persons, membership, people, audit)
			return
		},
		func() (err error) {
			provisioning, err = BuildProvisioning(db, env, organizations, identity, persons, membership, people, devices,
				presence.SessionRecords, supervisions, timetableRows, operatorAuditLog, settings.Service)
			return
		},
		func() (err error) {
			deliveryDevices, err = BuildDeliveryDevices(db, unit, env, pushVAPID, identity, provisioning, settings.Service)
			return
		},
		func() error {
			reminders = BuildReminders(db, env, caller, settings.Service, schedules, active, supervisions, timetableOwner, rooms, people,
				calendarApp, announcements, delivery)
			return nil
		},
		func() (err error) {
			studentDeletion, err = BuildStudentDeletion(db, env, hub, persons, carePlan, timetableOwner, timetableRows, feedback,
				caller.HasCurrentStaff, services.StudentPhotos, audit)
			return
		},
		func() (err error) {
			facilities, err = BuildFacilitiesModule(db, ObserveFacilities, timetableRows.RecurrenceLock, presence.Presence, requests.CareOfferings)
			return
		},
		func() (err error) {
			outbox, err = BuildDeliveryWorker(db, env, mailSender, mailFrom, pushVAPID, mailIdentity, delivery, BuildCalendarAppointmentRenderer(db, mailFrom, people),
				IdentityDeliveryRenderers(identity, mailFrom, mailIdentity, delivery.Runtime.EmailStatus, env), calendarApp)
			return
		},
	)
	if err != nil {
		return nil, err
	}
	return NewWorker(db, env.Logger, tracer, hub, settings.Service, audit.Command, deviceFleet, carePlan, persons, people,
		organizations, identity, facilities, timetableRepos, workTime, staffAdmin, workforce, active, reports, lifecycle, days,
		materialization, schedules, requests.Documents, services, studentAudit, notices, flows, studentDeletion, feedback, calendarApp,
		notifications, deliveryDevices.PWAUsage, chat.StaffMessaging, outbox, reminders, fileStore, announcements)
}
