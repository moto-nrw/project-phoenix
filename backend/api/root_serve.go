package api

import (
	"net/http"

	birthdaysAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/birthdays"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/analytics"
	appointmentsModule "github.com/moto-nrw/project-phoenix/modules/appointments"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
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

// composeServeRoot builds every owner the server mounts as a local value, in
// the build order of the owner graph (ADR 0058, ADR 0059), then mounts each
// route resource where the values exist.
func composeServeRoot(
	db transactionruntime.Database, env owners.RootEnvironment, hub *realtime.Hub, config ServeConfig, tracker analytics.Tracker,
	metrics *httpMetrics, tracer *observability.Tracer, metricsToken string, embedWorker bool, documentPermissions carePlanCompose.StudentDocumentPermissions,
) (http.Handler, schedulerPorts.Worker, error) {
	unit := db.UnitOfWork()
	communicationCompose.InstrumentMessageQueries(db.Pool(), false)
	renderer := lists.NewRendererService()
	var (
		tokens           owners.TokenAdapter
		objects          filestorageCompose.ObjectBackend
		audit            owners.AuditOwner
		mailSender       email.Mailer
		mailDispatcher   email.Dispatcher
		mailFrom         email.Email
		pushVAPID        deliveryCompose.VAPIDConfig
		organizations    organizationtenancy.Capability
		groups           schoolstructure.Owner
		rooms            facilitiesModule.Catalog
		membership       schoolMembershipModule.Owner
		classListEntries schoolMembershipModule.Owner
		bookings         *enrollmentOwner.BookingProjection
		settings         owners.SettingsOwner
		calendar         schoolcalendar.Days
		workTime         workforceCompose.WorkTime
		people           owners.PeopleRows
		peopleIdentity   identityaccess.Owner
		guardian         peoplePorts.GuardianService
		photoRuntime     owners.StudentPhotoRuntime
		persons          *peopleCompose.Directory
		timetableOwner   timetable.Owner
		timetableRepos   owners.TimetableRepositories
		carePlan         carePlanModule.Owner
		appointments     appointmentsModule.Capability
		operatorAuditLog owners.OperatorAuditEntries
		communication    communicationModule.Capability
		mealPlanSettings *mealplanCompose.Settings
		mealPlan         mealplanModule.Capability
		feedbackSettings *feedbackCompose.Settings
		feedback         feedbackModule.Capability
		structure        owners.StructureRows
		presencePeople   owners.PresencePeopleReads
		careRows         owners.TimetableCareRows
		devices          devicefleetModule.Capability
		presence         owners.PresenceOwner
		facilitiesRooms  FacilitiesRooms
		substitutions    owners.GroupSubstitutions
		supervisions     studentpresence.SupervisionRecords
		timetableRows    owners.TimetableOwner
		classArrivals    timetable.ClassArrivals
		studentAudit     peoplePorts.StudentAuditService
		carePlanRows     owners.CarePlanRows
		enrollment       owners.EnrollmentRows
		delivery         owners.DeliveryRuntime
		mailIdentity     email.ReplyToResolver
		identity         identityaccess.Owner
		caller           *owners.CallerRows
		portalReads      carePlanCompose.ParentPortalRepositories
		messaging        parentStore.MessagingRepositories
		consent          communicationModule.NotificationConsentCapability
		notifications    owners.DeliveryNotifications
		emitter          eventContract.Emitter
		purger           filestorageModule.AttachmentPurger
		announcements    communicationModule.ParentAnnouncementCapability
		schedules        owners.CarePlanSchedules
		services         owners.PeopleServices
		days             owners.TimetableDays
		payroll          settingsCompose.PayrollStatusGetter
		side             *workforceCompose.WorkSessionSide
		active           studentpresence.Presence
		activities       activityContract.ActivityService
		yard             FacilitiesYard
		shifts           *workforceCompose.ShiftPlanning
		deviceFleet      devicefleetModule.Capability
		materialization  timetable.MaterializationCapability
		lifecycle        owners.TimetableLifecycle
		structureViews   StructureServices
		reports          owners.PresenceReports
		offerings        owners.CarePlanOfferings
		notices          owners.EnrollmentNotices
		phases           *enrollmentCompose.Phases
		decisions        *enrollmentCompose.Decisions
		gradeTransition  *gradetransition.Workflow
		flows            owners.EnrollmentFlows
		reviewPolicy     *owners.ParentRequestReviewPolicy
		requests         owners.CarePlanRequests
		classDay         ClassDayViews
		chat             owners.CommunicationMessaging
		calendarApp      calendarCompose.Application
		parent           *parentportal.Portal
		fileStore        filestorageModule.Capability
		planExport       planexport.Service
		workforce        owners.WorkforcePlanning
		staffAdmin       *workforceModule.StaffAdmin
		provisioning     organizationtenancy.Provisioning
		deliveryDevices  owners.DeliveryDevices
		emergency        emergencysnapshot.Query
		reminders        reminder.Module
		groupLive        grouplive.Query
		dashboard        supervisiondashboard.Query
		planning         TimetablePlanning
		settingsWrites   SettingsWrites
		studentDeletion  *studentdeletion.Workflow
		facilities       facilitiesModule.Capability
		calendarAdmin    schoolcalendar.Calendar
		outbox           *deliveryModule.Worker
		iot              devicefleetCompose.IoTService
		guardianProfiles *peopleCompose.GuardianProfileLoader
		birthdays        birthdaysAPI.Service
		dataImports      imports
		databaseStats    DatabaseStatsReader
		staffGroups      schoolstructure.StaffGroupQuery
	)
	err := owners.RunRootSteps(
		func() (err error) { tokens, err = newLegacyTokenAdapter(); return },
		func() (err error) {
			objects, err = nativeUploads.PrivateUploadsBackend()
			return owners.StepError("resolve uploads backend", err)
		},
		func() (err error) { audit, err = owners.BuildAuditOwner(db, env); return },
		func() (err error) {
			mailSender, mailDispatcher, mailFrom, pushVAPID, err = owners.BuildMailTransport(env)
			return
		},
		func() (err error) { organizations, err = owners.BuildServeOrganizations(db); return },
		func() (err error) { groups, err = owners.BuildServeSchoolStructure(db); return },
		func() (err error) { rooms, err = owners.BuildServeRooms(db); return },
		func() (err error) { membership, err = owners.ComposeSchoolMembership(db); return },
		func() error { bookings = owners.NewEnrollmentBookings(); return nil },
		func() (err error) {
			settings, tracker, err = buildServeAnalyticsSettings(db, unit, env, organizations, hub, bookings, tracker)
			return
		},
		func() (err error) {
			calendar, err = owners.BuildSchoolCalendarRecords(db, owners.ObserveSchoolCalendar, settings.Service)
			return
		},
		func() (err error) { workTime, err = owners.BuildServeWorkforce(db, membership, calendar); return },
		func() error {
			people, peopleIdentity = owners.BuildPeopleRows(db, env.Observe.IdentityAccess, groups, rooms, membership, workTime)
			return nil
		},
		func() (err error) { guardian, err = owners.BuildGuardianService(db, people, audit); return },
		func() error {
			unlinker := peopledirectory.NewPhotoUnlinker(env.Logger.With("component", "student-photo-unlinker"), "public", nativeUploads.UploadFiles{})
			photoRuntime = owners.BuildStudentPhotoRuntime(db, env, settings.Service, hub, audit, unlinker, actingAccountID)
			return nil
		},
		func() (err error) {
			persons, err = owners.BuildPeopleDirectory(db, owners.ObservePeopleDirectory, photoRuntime, owners.GuardianDirectoryProvider(db, guardian))
			return
		},
		func() (err error) {
			classListEntries, err = composeClassListEntries(db, persons, audit.Command)
			return
		},
		func() (err error) {
			timetableOwner, err = owners.BuildServeTimetable(db, persons, rooms, membership)
			return
		},
		func() error {
			timetableRepos = owners.NewTimetableRepositories(db, timetableOwner, persons, groups, rooms, calendar, membership, workTime)
			return nil
		},
		func() (err error) {
			carePlan, err = owners.ComposeCarePlan(db, persons, owners.CarePlanStatusSlots(timetableRepos.InstanceStudent))
			return
		},
		func() (err error) { appointments, err = owners.BuildServeAppointments(db); return },
		func() (err error) {
			operatorAuditLog = owners.NewOperatorAuditLog(db)
			communication, err = buildCommunicationModule(db, env, organizations, persons, operatorAuditLog)
			return
		},
		func() (err error) { mealPlanSettings, err = owners.BuildMealPlanSettings(settings.Service); return },
		func() (err error) {
			mealPlan, err = owners.BuildServeMealPlan(db, mealPlanSettings, calendar, owners.NewStudentRootReads(people.Students))
			return
		},
		func() (err error) { feedbackSettings, err = owners.BuildFeedbackSettings(settings.Service); return },
		func() (err error) { feedback, err = owners.BuildServeFeedback(db, feedbackSettings); return },
		func() (err error) {
			structure = owners.BuildStructureRows(db, persons, people, peopleIdentity, rooms, membership, workTime)
			presencePeople = owners.BuildPresencePeopleReads(db, env.Logger, persons, groups, organizations, people)
			careRows = owners.BuildTimetableCareRows(carePlan)
			devices, err = owners.NewDeviceFleet(db)
			return owners.StepError("compose device fleet", err)
		},
		func() error {
			presence = owners.BuildPresenceOwner(db, env.Logger, env.Now, devices, persons, rooms, membership, carePlan, timetableOwner, timetableRepos, people, structure)
			facilitiesRooms = buildFacilitiesRooms(presence.Presence, rooms, timetableOwner, membership, persons)
			substitutions = owners.NewGroupSubstitutionRepository(structure.Groups, membership, workTime)
			supervisions = owners.NewGroupSupervisionRecords(presence.SessionRecords, groups)
			return nil
		},
		func() (err error) {
			timetableRows, err = owners.BuildTimetableOwner(db, env.Logger, env.Now, timetableOwner, timetableRepos, calendar, bookings, structure.Groups)
			return
		},
		func() (err error) { classArrivals, err = owners.BuildClassArrivals(db, env.Logger); return },
		func() (err error) {
			studentAudit = peopleCompose.NewStudentAuditService(tokens.AuditActor, owners.NewStudentAuditFor(persons))
			carePlanRows, err = owners.BuildCarePlanRows(db, env.Logger, studentAudit, carePlan, persons, membership, timetableOwner, calendar, classArrivals, people)
			return
		},
		func() (err error) {
			enrollment = owners.BuildEnrollmentRows(bookings, carePlan, persons)
			delivery, err = owners.BuildDeliveryRuntime(db, env, guardian)
			return
		},
		func() (err error) {
			staffGroups, err = education.NewStaffGroups(education.StaffGroupsDependencies{Groups: groups, Assignments: membership, Substitutions: workTime})
			return
		},
		func() (err error) {
			mailIdentity = owners.TenantMailIdentity(organizations, settings.Service, env)
			portalReads = owners.NewParentPortalReads(db, persons, peopleIdentity, enrollmentCompose.New(), organizations)
			identity, caller, err = owners.BuildIdentityAccess(db, unit, env, tokens, organizations, persons, membership, settings.Service,
				audit, operatorAuditLog, mailSender, mailDispatcher, mailFrom, delivery.Runtime, mailIdentity, delivery.Enqueuer(), people, portalReads.ParentEnrollmentRequest,
				structure.Groups, staffGroups, arrivalTimetable.NewCallerActivityGroups(timetableRepos.Timetable),
				presence.SessionRecords, presence.Presence, workTime, calendar)
			return
		},
		func() (err error) {
			messaging = owners.NewParentMessagingRepositories(db, groups, membership, peopleIdentity, enrollmentCompose.New(), nil, organizations)
			consent, err = communicationCompose.NewNotificationConsent(communicationCompose.NotificationConsentConfig{
				DB: db.Pool(), Observe: env.Observe.Communication,
			})
			return
		},
		func() (err error) {
			notifications, err = owners.BuildDeliveryNotifications(db, unit, env, pushVAPID, hub, owners.DeliveryModuleOf(delivery), settings.Service,
				consent, identity, people, structure, workTime, messaging)
			return
		},
		func() (err error) {
			emitter = owners.BuildParentEventEmitter(db, unit, env, hub, settings.Service, messaging, notifications)
			purger, err = owners.BuildAttachmentPurger(db, env, objects)
			return
		},
		func() (err error) {
			announcements, err = owners.BuildParentAnnouncements(env, settings.Service, delivery.Outbox, owners.DeliveryModuleOf(delivery),
				notifications, messaging, purger)
			return
		},
		func() (err error) {
			schedules, err = owners.BuildCarePlanSchedules(db, env.Logger, carePlan, persons, carePlanRows, classArrivals,
				enrollment.ApprovedOfferings, settings.Service, timetableRows.RecurrenceLock.LockRecurrenceWrites, presence.Presence)
			return
		},
		func() error {
			services = owners.BuildPeopleServices(db, env, persons, people, peopleIdentity, identity, schedules.Lifecycle, audit, tokens,
				guardian, studentAudit, photoRuntime, owners.NewCaregiverBindingLocker(membership, workTime, presence.Presence, timetableOwner),
				supervisions, substitutions, structure,
				owners.CaregiverPlannedSupervisionBlockers{Supervisors: timetableOwner}, settings.Service)
			payroll = owners.PayrollStatus(settings.Service, people)
			return nil
		},
		func() (err error) {
			days, err = owners.BuildTimetableDays(timetableRows, people, rooms, careRows, audit, schedules.CareDay)
			return
		},
		func() (err error) {
			side, err = owners.BuildWorkSessionSide(db, env, mailFrom, hub, mailIdentity, workTime, membership, persons, organizations, people,
				services.Users, audit, supervisions, presence.SessionRecords, calendar, settings.Service, payroll, delivery.Enqueuer())
			return
		},
		func() (err error) {
			active, err = owners.BuildActivePresence(db, unit, env, hub, settings.Service, presence, presencePeople, carePlanRows, people,
				services.Users, rooms, devices, timetableRows, days, side.AutoCheckIn, emitter, tracker, AttendancePrincipal)
			return
		},
		func() (err error) {
			activities, err = owners.BuildActivities(timetableRows, timetableOwner, presence.SessionRecords, people)
			yard = buildFacilitiesYard(env, facilitiesRooms, activities)
			return
		},
		func() (err error) {
			shifts, err = owners.BuildShiftPlanning(db, env, hub, workTime, people, calendar, audit, timetableRows, rooms, activities)
			return
		},
		func() (err error) {
			deviceFleet, err = owners.BuildDeviceFleet(db, env, settings.Service, rooms, presence, timetableRows, schedules.Pickups, organizations)
			return
		},
		func() (err error) {
			materialization, err = owners.BuildTimetableMaterialization(db, env, hub, timetableRows, people, calendar)
			return
		},
		func() (err error) {
			lifecycle, err = owners.BuildTimetableLifecycle(db, env, hub, timetableRows, days, people, careRows, presence, supervisions,
				owners.WorkforceShiftRows(workTime), schedules, carePlan, active, materialization, announcements, settings.Service)
			return
		},
		func() (err error) {
			lifecycle, err = owners.BuildTimetableDay(env, hub, lifecycle, timetableOwner, audit, settings.Service, presence, supervisions,
				services.Users, membership, active, schedules, TimetableRequestSession{}, deviceauth.WithAttendanceStaff)
			return
		},
		func() (err error) {
			structureViews.Education = buildGroupManagement(db, hub, structure, people, substitutions, audit)
			structureViews.Substitution, err = buildSubstitutions(db, env, hub, structure, people, substitutions, workTime,
				caller.Caller(), presence.SessionRecords, supervisions, active, lifecycle.Deviations, timetableRows, audit, settings.Service, identity)
			return
		},
		func() error {
			reports = owners.BuildPresenceReports(db, env, presence, carePlanRows, carePlan, services.Users, timetableRows, rooms, calendar,
				people, audit, settings.Service)
			return nil
		},
		func() (err error) {
			offerings, err = owners.BuildCarePlanOfferings(env, carePlan, persons, people, timetableOwner, calendar, timetableRows, enrollment,
				settings.Service, schedules, audit, identity, hub, emitter)
			return
		},
		func() (err error) {
			notices = owners.BuildEnrollmentNotices(env, enrollment, settings.Service, delivery.Enqueuer(), organizations, audit, carePlan, owners.DeliveryModuleOf(delivery))
			phases, err = owners.BuildEnrollmentPhases(env, enrollment, carePlan, persons, calendar, timetableRows, offerings, settings.Service)
			return
		},
		func() (err error) {
			decisions = owners.BuildEnrollmentDecisions(db, env, hub, enrollment, notices, audit, people, careRows, persons, carePlan,
				offerings, identity, services, schedules, carePlanRows, timetableRows, settings.Service)
			gradeTransition, err = buildGradeTransition(db, env, persons, membership, timetableRows, offerings.Bookings.ResyncOfferingSourcedTemplates, audit)
			return
		},
		func() error {
			flows = owners.BuildEnrollmentFlows(db, env, enrollment, notices, decisions, phases, offerings, consent, identity, people, structure,
				carePlan, carePlanRows, schedules, membership, organizations, audit, settings.Service, delivery.Enqueuer())
			reviewPolicy = owners.NewParentRequestReviewPolicy(caller.Caller().ParentRequestReviews)
			return nil
		},
		func() (err error) {
			requests, err = owners.BuildCarePlanRequests(db, env, hub, carePlan, persons, people, schedules, offerings, enrollment,
				timetableOwner, settings.Service, audit, services, emitter, caller, reviewPolicy, tokens.Permissions, notifications.AbsenceNotifier, documentPermissions)
			return
		},
		func() (err error) {
			classDay, err = buildClassDayViews(db, env, hub, caller, flows.Reports, carePlanRows, schedules, carePlan, people, audit,
				settings.Service, lifecycle.Operations, presence, persons, groups, rooms, renderer, timetableOwner, emitter)
			return
		},
		func() error {
			chat = owners.BuildCommunicationMessaging(db, env, hub, settings.Service, messaging, services.Users, owners.StaffStudentReads{Caller: caller.Caller()},
				notifications, delivery.Outbox, people, owners.SchoolNameDirectory{Schools: organizations}, CommunicationSession{})
			calendarApp = owners.BuildCalendarApp(db, env, people, structure, portalReads, timetableRows, rooms, workTime, organizations,
				identity, caller.Caller(), appointments, calendar, delivery, notifications, settings.Service, persons)
			return nil
		},
		func() (err error) {
			parent, err = owners.BuildParentPortal(db, env, hub, settings.Service, people, portalReads, messaging, presence, carePlanRows,
				carePlan, persons, mealPlan, schedules, requests, services, emitter, notifications, identity, enrollment, notices)
			return
		},
		func() (err error) {
			fileStore, err = owners.BuildFileStore(db, env, objects, identity, persons, settings.Service, audit, announcements)
			return
		},
		func() (err error) {
			planExport = owners.BuildPlanExport(env, shifts, workTime, timetableRows, rooms, people, timetableOwner, calendar, renderer)
			workforce, err = owners.BuildWorkforcePlanning(db, env, hub, workTime, membership, persons, shifts, planExport, side,
				lifecycle, timetableRows, audit, settings.Service)
			return
		},
		func() (err error) {
			staffAdmin, err = owners.BuildWorkforceStaffAdmin(db, env, persons, membership, people, audit)
			return
		},
		func() (err error) {
			provisioning, err = owners.BuildProvisioning(db, env, organizations, identity, persons, membership, people, devices,
				presence.SessionRecords, supervisions, timetableRows, operatorAuditLog, settings.Service)
			return
		},
		func() (err error) {
			deliveryDevices, err = owners.BuildDeliveryDevices(db, unit, env, pushVAPID, identity, provisioning, settings.Service)
			return
		},
		func() (err error) {
			emergency, err = buildEmergencySnapshot(env, presence, active, people, persons, rooms, renderer)
			return
		},
		func() error {
			reminders = owners.BuildReminders(db, env, caller, settings.Service, schedules, active, supervisions, timetableOwner, rooms, people,
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
				structureViews, lifecycle, schedules, services.Users)
			return
		},
		func() (err error) {
			planning, err = buildTimetablePlanning(db, env, hub, timetableRows, lifecycle, schedules, presence, timetableOwner,
				offerings, audit, people)
			return
		},
		func() (err error) {
			settingsWrites, err = buildSettingsWrites(env, settings, payroll, yard, schedules.Lifecycle, services.StudentPhotos, owners.BuildPresenceCleanup(env, presence, audit, settings.Service), hub)
			return
		},
		func() (err error) {
			studentDeletion, err = owners.BuildStudentDeletion(db, env, hub, persons, carePlan, timetableOwner, timetableRows, feedback,
				caller.HasCurrentStaff, services.StudentPhotos, audit)
			return
		},
		func() (err error) {
			facilities, err = owners.BuildFacilitiesModule(db, owners.ObserveFacilities, timetableRows.RecurrenceLock, presence.Presence, requests.CareOfferings)
			return
		},
		func() (err error) {
			calendarAdmin, err = buildCalendarAdministration(db, owners.ObserveSchoolCalendar, calendar, timetableRows.RecurrenceLock, requests.CareOfferings)
			return
		},
		func() (err error) {
			iot = devicefleetCompose.NewIoTService(deviceFleet)
			guardianProfiles = peopleCompose.NewGuardianProfileLoader(people.GuardianProfiles, db.Pool(), env.Logger.With("service", "guardian-profile-loader"))
			birthdays, err = buildBirthdayEntry(db, membership, BirthdayRepositories{Students: people.Students, Staff: people.Staff, Persons: people.Persons},
				settings.Service, env.Logger.With("service", "birthdays"), nil)
			dataImports = newServeImports(audit, identity, groups, rooms, persons, carePlan, presence, organizations, membership, workTime, workforce)
			databaseStats = newDatabaseStats(people.Repositories, structure.Groups, facilities, timetableRepos, identity, devices, env.Logger)
			return err
		},
		func() (err error) {
			outbox, err = owners.BuildDeliveryWorker(db, env, mailSender, mailFrom, pushVAPID, mailIdentity, delivery, owners.BuildCalendarAppointmentRenderer(db, mailFrom, people),
				owners.IdentityDeliveryRenderers(identity, mailFrom, mailIdentity, delivery.Runtime.EmailStatus, env), calendarApp, settings.Service)
			return
		},
	)
	if err != nil {
		return nil, nil, err
	}
	registerRuntimeStatsProviders(hub, deliveryDevices.PWAUsage)
	router := chi.NewRouter()
	sessionAuth, err := mountServeMiddleware(router, unit, env.Logger, config, tracer, metrics, tracker, settings.Service, analyticsCollectionPolicy(IsDemoEnvironment(env.Config.AppEnv), settings.Service, identity, organizations), identity, identity)
	if err != nil {
		return nil, nil, err
	}
	logger, limiters, tenant := env.Logger, authRateLimitersFromEnv(), chi.NewRouter()
	deviceAuth, studentPresence := newDeviceAuthentication(iot, organizations, settings.Service, env.Logger), owners.NewLoggedStudentPresence(db, env.Logger)
	careOfferings := requests.CareOfferings
	err = owners.RunRootSteps(
		func() error {
			return mountDemoAccess(router, authAPI.ComposedDemoAccess(identity.DemoAccess()), viper.GetString("app_env"), config.FrontendURL, viper.GetString("tenant_domain"))
		},
		func() error { mountPublicRoutes(router, metricsToken, db.Pool().PingContext); return nil },
		func() error { return mountRequestFeed(router, tenant, db, logger, config.FrontendURL) },
		func() error { return mountModuleRoutes(tenant, settings.Service) },
		func() error {
			return mountStudentRoutes(tenant, db, logger, deviceAuth, studentPresence, persons, services, membership, caller, reviewPolicy,
				settings.Service, hub, carePlan, carePlanRows.Companions, schedules, requests, planning, emitter, activities,
				lifecycle, reports, structureViews, organizations, presence.StatusDays, notifications.AbsenceNotifier, studentDeletion,
				groupLive, enrollmentCompose.PublicDecisions(decisions), active, owners.NewCareResumption(carePlanRows.LifecycleOwners.Students, carePlan, carePlanRows.LifecycleOwners.ChangeTrail, nil, logger.With("service", "care_lifecycle")).Resume)
		},
		func() error {
			return mountPresenceRoutes(tenant, db, logger, config.SentryPyrePortalDSN, deviceAuth, studentPresence, active, facilities,
				timetableOwner, timetableRepos, days, persons, services.Users, caller, settings.Service, hub, emitter, groupLive,
				dashboard, yard, iot, activities, structureViews.Education, schedules.Pickups, workforce.TimeTracking, facilitiesRooms,
				feedback, organizations, identity)
		},
		func() error {
			return mountStaffRoutes(tenant, db, logger, membership, workTime, services.Users, staffAdmin, workforce, shifts,
				structureViews.Education, structureViews.Substitution, persons, settings.Service, hub, identity, caller, calendarAdmin)
		},
		func() error {
			return mountSchoolPortalRoutes(router, tenant, db, logger, limiters, studentPresence, timetableOwner, calendarAdmin,
				materialization, lifecycle, planning, timetableRows.PlanningTracks, services.Users, schedules, caller, settings.Service,
				careOfferings, planExport, chat, persons, notifications, deliveryDevices, classDay, identity, calendarApp)
		},
		func() error {
			enrollmentResource := newEnrollmentResource(logger, settings.Service, config.Turnstile, careOfferings, flows, phases,
				enrollmentCompose.PublicDecisions(decisions), notices.Deletions, identity, guardianProfiles, organizations, renderer, persons, carePlan)
			mountSchoolRoutes(tenant, db, logger, facilities, facilitiesRooms, settings.Service, caller, active, services.Users,
				structureViews.Education, renderer, activities, timetableOwner, iot, settingsWrites,
				membership, classListEntries, databaseStats, dataImports, persons, gradeTransition, enrollmentResource)
			mountPeopleRoutes(tenant, db, logger, reports, renderer, persons, services.Guardian, identity, caller, feedback, mealPlan,
				birthdays, emergency)
			mountCommunicationRoutes(tenant, chat, announcements, reminders, deliveryDevices, communication)
			mountCalendarRoutes(router, tenant, logger, calendarApp)
			return mountDeliveryRoutes(router, tenant, logger, fileStore, parent, hub, caller, identity)
		},
		func() error {
			operator, err := newOperatorRouter(db, logger, sessionAuth, limiters, iot, organizations, settings.Service, hub,
				active, settingsWrites, identity, provisioning, services.CaregiverCapability, schedules.Lifecycle, communication)
			if err != nil {
				return err
			}
			mountPortalRoutes(router, limiters, newAccountResource(db, identity, organizations, services.CaregiverCapability, settings.Service),
				operator, newParentResource(logger, identity, parent, calendarApp, flows, guardianProfiles, organizations, deliveryDevices, notifications))
			return mountTraegerPortalRoutes(router, limiters, identity, organizations, settings.Traeger)
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
	worker, err := owners.NewWorker(db, env.Logger, tracer, hub, settings.Service, audit.Command, deviceFleet, carePlan, persons, people,
		organizations, identity, facilities, timetableRepos, workTime, staffAdmin, workforce, active, reports, lifecycle, days,
		materialization, schedules, requests.Documents, services, studentAudit, notices, flows, studentDeletion, feedback, calendarApp,
		notifications, deliveryDevices.PWAUsage, chat.StaffMessaging, outbox, reminders, fileStore, announcements, analyticsSchoolNames(tracker))
	if err != nil {
		return nil, nil, err
	}
	return &serveHandler{router: router, settings: settings.Service}, worker, nil
}
