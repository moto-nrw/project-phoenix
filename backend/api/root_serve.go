package api

import (
	"fmt"
	"net/http"

	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/analytics"
	appointmentsModule "github.com/moto-nrw/project-phoenix/modules/appointments"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	devicefleetModule "github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
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
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	"github.com/spf13/viper"
)

// runRootSteps runs a root's build steps in order and stops at the first
// failure. Each step assigns locals of the root function that declares it,
// so a later step only reads values an earlier one built.
func runRootSteps(steps ...func() error) error {
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// stepError names the value a failed root step was building.
func stepError(what string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", what, err)
}

// composeServeRoot builds every owner the server mounts as a local value, in
// the build order of the owner graph (ADR 0058, ADR 0059), then mounts each
// route resource where the values exist.
func composeServeRoot(
	db transactionruntime.Database, env RootEnvironment, hub *realtime.Hub, config ServeConfig, tracker analytics.Tracker,
	metrics *httpMetrics, tracer *observability.Tracer, metricsToken string, embedWorker bool,
) (http.Handler, schedulerPorts.Worker, error) {
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
		classListEntries *schoolMembershipModule.Module
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
		communication    *communicationModule.Module
		mealPlanSettings *mealplanCompose.Settings
		mealPlan         *mealplanModule.Module
		feedbackSettings *feedbackCompose.Settings
		feedback         *feedbackModule.Module
		structure        StructureRows
		presencePeople   PresencePeopleReads
		careRows         TimetableCareRows
		devices          devicefleetModule.Capability
		presence         PresenceOwner
		facilitiesRooms  FacilitiesRooms
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
		yard             FacilitiesYard
		shifts           *workforceCompose.ShiftPlanning
		deviceFleet      *devicefleetModule.Module
		materialization  timetable.MaterializationCapability
		lifecycle        TimetableLifecycle
		structureViews   StructureServices
		reports          PresenceReports
		offerings        CarePlanOfferings
		notices          EnrollmentNotices
		phases           *enrollmentCompose.Phases
		decisions        *enrollmentCompose.Decisions
		gradeTransition  *gradetransition.Workflow
		flows            EnrollmentFlows
		reviewPolicy     *ParentRequestReviewPolicy
		requests         CarePlanRequests
		classDay         ClassDayViews
		chat             CommunicationMessaging
		calendarApp      calendarCompose.Application
		parent           *parentportal.Portal
		fileStore        *filestorageModule.Module
		planExport       planexport.Service
		workforce        WorkforcePlanning
		staffAdmin       *workforceModule.StaffAdmin
		provisioning     organizationtenancy.Provisioning
		deliveryDevices  DeliveryDevices
		emergency        emergencysnapshot.Query
		reminders        reminder.Module
		groupLive        grouplive.Query
		dashboard        supervisiondashboard.Query
		planning         TimetablePlanning
		settingsWrites   SettingsWrites
		studentDeletion  *studentdeletion.Workflow
		facilities       *facilitiesModule.Module
		calendarAdmin    *schoolcalendar.Module
		outbox           *deliveryModule.Worker
		iot              devicefleetCompose.IoTService
		guardianProfiles *peopleCompose.GuardianProfileLoader
		birthdays        peopledirectory.Birthdays
		dataImports      imports
		databaseStats    DatabaseStatsReader
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
			photoRuntime = buildStudentPhotoRuntime(db, env, settings.Service, transport, audit, unlinker, actingAccountID)
			return nil
		},
		func() (err error) {
			persons, err = buildPeopleDirectory(db, observePeopleDirectory, photoRuntime, guardianDirectoryProvider(db, guardian))
			return
		},
		func() (err error) {
			classListEntries, err = composeClassListEntries(db, persons, audit.Command)
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
		func() (err error) {
			operatorAuditLog = NewOperatorAuditLog(db)
			communication, err = buildCommunicationModule(db, env, organizations, persons, operatorAuditLog)
			return
		},
		func() (err error) { mealPlanSettings, err = buildMealPlanSettings(settings.Service); return },
		func() (err error) {
			mealPlan, err = buildServeMealPlan(db, mealPlanSettings, NewStudentRootReads(people.Students), calendar)
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
			facilitiesRooms = buildFacilitiesRooms(presence.Presence, rooms, timetableOwner, membership, persons)
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
			delivery, err = buildDeliveryRuntime(db, env, guardian)
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
				audit, operatorAuditLog, transport, delivery.Runtime, mailIdentity, delivery.Enqueuer(), people, portalReads.ParentEnrollmentRequest,
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
				consent, identity, people, structure, workTime, messaging)
			return
		},
		func() (err error) {
			emitter = buildParentEventEmitter(db, unit, env, transport, settings.Service, messaging, notifications)
			purger, err = buildAttachmentPurger(db, env, objects)
			return
		},
		func() (err error) {
			announcements, err = buildParentAnnouncements(env, settings.Service, delivery.Outbox, deliveryModuleOf(delivery),
				notifications, messaging, purger)
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
				CaregiverPlannedSupervisionBlockers{Supervisors: timetableOwner}, settings.Service)
			payroll = payrollStatus(settings.Service, people)
			return nil
		},
		func() (err error) {
			days, err = buildTimetableDays(timetableRows, people, rooms, careRows, audit, schedules.CareDay)
			return
		},
		func() (err error) {
			side, err = buildWorkSessionSide(db, env, transport, mailIdentity, workTime, membership, persons, organizations, people,
				services.Users, audit, supervisions, presence.SessionRecords, calendar, settings.Service, payroll, delivery.Enqueuer())
			return
		},
		func() (err error) {
			active, err = buildActivePresence(db, unit, env, transport, settings.Service, presence, presencePeople, carePlanRows, people,
				services.Users, rooms, devices, timetableRows, days, side.AutoCheckIn, emitter, tracker)
			return
		},
		func() (err error) {
			activities, err = buildActivities(timetableRows, timetableOwner, presence.SessionRecords, people)
			yard = buildFacilitiesYard(env, facilitiesRooms, activities)
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
		func() (err error) {
			structureViews.Education = buildGroupManagement(db, transport, structure, people, substitutions, audit)
			structureViews.Substitution, err = buildSubstitutions(db, env, transport, structure, people, substitutions, workTime,
				caller.Caller(), presence.SessionRecords, supervisions, active, lifecycle.Deviations, timetableRows, audit, settings.Service)
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
		func() (err error) {
			decisions = buildEnrollmentDecisions(db, env, transport, enrollment, notices, audit, people, careRows, persons, carePlan,
				offerings, identity, services, schedules, carePlanRows, timetableRows, settings.Service)
			gradeTransition, err = buildGradeTransition(db, env, persons, membership, timetableRows, offerings.Bookings.ResyncOfferingSourcedTemplates, audit)
			return
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
		func() (err error) {
			classDay, err = buildClassDayViews(db, env, transport, caller, flows.Reports, carePlanRows, schedules, carePlan, people, audit,
				settings.Service, lifecycle.Operations, presence, persons, groups, rooms, renderer)
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
		func() (err error) {
			emergency, err = buildEmergencySnapshot(env, presence, active, people, persons, rooms, renderer)
			return
		},
		func() error {
			reminders = buildReminders(db, env, caller, settings.Service, schedules, active, supervisions, timetableOwner, rooms, people,
				calendarApp, announcements, delivery)
			return nil
		},
		func() (err error) {
			groupLive, err = buildGroupLive(env, presence, services.Users, structureViews, caller, active, settings.Service, schedules,
				lifecycle, requests)
			return
		},
		func() (err error) {
			dashboard, err = buildSupervisionDashboard(env, settings.Service, caller, active, presence, rooms, timetableOwner, yard,
				structureViews, lifecycle, schedules)
			return
		},
		func() (err error) {
			planning, err = buildTimetablePlanning(db, env, transport, timetableRows, lifecycle, schedules, presence, timetableOwner,
				offerings, audit, people)
			return
		},
		func() (err error) {
			settingsWrites, err = buildSettingsWrites(env, settings, payroll, yard, schedules.Lifecycle, services.StudentPhotos, transport.Hub)
			return
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
			calendarAdmin, err = buildCalendarAdministration(db, observeSchoolCalendar, calendar, timetableRows.RecurrenceLock, requests.CareOfferings)
			return
		},
		func() error {
			iot = devicefleetCompose.NewIoTService(deviceFleet)
			guardianProfiles = peopleCompose.NewGuardianProfileLoader(people.GuardianProfiles, db.Pool(), env.Logger.With("service", "guardian-profile-loader"))
			birthdays = NewBirthdays(BirthdayRepositories{Students: people.Students, Staff: people.Staff, Persons: people.Persons},
				settings.Service, env.Logger.With("service", "birthdays"), nil)
			dataImports = newServeImports(audit, identity, groups, rooms, persons, carePlan, presence, organizations, membership, workTime, workforce)
			databaseStats = newDatabaseStats(people.Repositories, structure.Groups, facilities, timetableRepos, identity, devices, env.Logger)
			return nil
		},
		func() (err error) {
			outbox, err = buildDeliveryWorker(db, env, transport, mailIdentity, delivery, buildCalendarAppointmentRenderer(db, transport, people),
				identityDeliveryRenderers(identity, transport.DefaultFrom, mailIdentity, delivery.Runtime.EmailStatus, env), calendarApp)
			return
		},
	)
	if err != nil {
		return nil, nil, err
	}
	registerRuntimeStatsProviders(hub, deliveryDevices.PWAUsage)
	router := chi.NewRouter()
	sessionAuth, err := mountServeMiddleware(router, unit, env.Logger, config, tracer, metrics, tracker, settings.Service)
	if err != nil {
		return nil, nil, err
	}
	logger, limiters, tenant := env.Logger, authRateLimitersFromEnv(), chi.NewRouter()
	deviceAuth, studentPresence := newDeviceAuthentication(iot, organizations, settings.Service), NewLoggedStudentPresence(db, env.Logger)
	careOfferings := requests.CareOfferings
	err = runRootSteps(
		func() error {
			return mountDemoAccess(router, authAPI.ComposedDemoAccess(identity.DemoAccess()), viper.GetString("app_env"), config.FrontendURL, viper.GetString("tenant_domain"))
		},
		func() error { mountPublicRoutes(router, metricsToken); return nil },
		func() error { return mountRequestFeed(router, tenant, db, logger, config.FrontendURL) },
		func() error { return mountModuleRoutes(tenant, settings.Service) },
		func() error {
			return mountStudentRoutes(tenant, db, logger, deviceAuth, studentPresence, persons, services, membership, caller, reviewPolicy,
				settings.Service, transport.Hub, carePlan, carePlanRows.Companions, schedules, requests, planning, emitter, activities,
				lifecycle, reports, structureViews, organizations, presence.StatusDays, notifications.AbsenceNotifier, studentDeletion,
				groupLive, enrollmentCompose.PublicDecisions(decisions), active)
		},
		func() error {
			return mountPresenceRoutes(tenant, db, logger, config.SentryPyrePortalDSN, deviceAuth, studentPresence, active, facilities,
				timetableOwner, timetableRepos, days, persons, services.Users, caller, settings.Service, transport.Hub, emitter, groupLive,
				dashboard, yard, iot, activities, structureViews.Education, schedules.Pickups, workforce.TimeTracking, facilitiesRooms,
				feedback, organizations)
		},
		func() error {
			return mountStaffRoutes(tenant, db, logger, membership, workTime, services.Users, staffAdmin, workforce, shifts,
				structureViews.Education, structureViews.Substitution, persons, settings.Service, transport.Hub, identity, caller, calendarAdmin)
		},
		func() error {
			return mountSchoolPortalRoutes(router, tenant, db, logger, limiters, studentPresence, timetableOwner, calendarAdmin,
				materialization, lifecycle, planning, timetableRows.PlanningTracks, services.Users, schedules, caller, settings.Service,
				careOfferings, planExport, chat, persons, notifications, deliveryDevices, classDay, identity)
		},
		func() error {
			enrollmentResource := newEnrollmentResource(logger, settings.Service, careOfferings, flows, phases,
				enrollmentCompose.PublicDecisions(decisions), notices.Deletions, identity, guardianProfiles, organizations, renderer, persons, carePlan)
			mountSchoolRoutes(tenant, db, logger, facilities, facilitiesRooms, settings.Service, caller, active, services.Users,
				structureViews.Education, renderer, activities, timetableOwner, iot, calendarAdmin, planning, careOfferings, settingsWrites,
				membership, classListEntries, databaseStats, dataImports, persons, gradeTransition, enrollmentResource)
			mountPeopleRoutes(tenant, db, logger, reports, renderer, persons, services.Guardian, identity, caller, feedback, mealPlan,
				birthdays, emergency)
			mountCommunicationRoutes(tenant, chat, announcements, reminders, deliveryDevices, communication)
			mountCalendarRoutes(router, tenant, logger, calendarApp)
			return mountDeliveryRoutes(router, tenant, logger, fileStore, transport.Hub, caller, identity)
		},
		func() error {
			operator, err := newOperatorRouter(db, logger, sessionAuth, limiters, iot, organizations, settings.Service, transport.Hub,
				active, settingsWrites, identity, provisioning, services.CaregiverCapability, schedules.Lifecycle, communication)
			if err != nil {
				return err
			}
			mountPortalRoutes(router, limiters, newAccountResource(db, identity, organizations, services.CaregiverCapability, settings.Service),
				operator, newParentResource(identity, parent, calendarApp, flows, guardianProfiles, organizations, deliveryDevices, notifications))
			return nil
		},
	)
	if err != nil {
		return nil, nil, err
	}
	router.Mount("/api", tenant)
	if err := requireCoreActionClassification(router); err != nil {
		return nil, nil, err
	}
	if !embedWorker {
		return &serveHandler{router: router, settings: settings.Service}, nil, nil
	}
	worker, err := newWorker(db, env.Logger, tracer, hub, settings.Service, audit.Command, deviceFleet, carePlan, persons, people,
		organizations, identity, facilities, timetableRepos, workTime, staffAdmin, workforce, active, reports, lifecycle, days,
		materialization, schedules, requests.Documents, services, studentAudit, notices, flows, studentDeletion, feedback, calendarApp,
		notifications, deliveryDevices.PWAUsage, chat.StaffMessaging, outbox, reminders, fileStore)
	if err != nil {
		return nil, nil, err
	}
	return &serveHandler{router: router, settings: settings.Service}, worker, nil
}
