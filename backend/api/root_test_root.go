//go:build testhooks

package api

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/realtime"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	devicefleetModule "github.com/moto-nrw/project-phoenix/modules/devicefleet"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// composeTestRoot builds the owners of the server root for a test, as local
// values in the server root's build order, with no-op observers and the
// Audit trails appending outside a tenant transaction. It returns the
// single-owner values the test roots hand out.
func composeTestRoot(db transactionruntime.Database, logger *slog.Logger, cfg owners.ServiceConfig, clocks ...func() time.Time) (
	env owners.RootEnvironment, mailSender email.Mailer, mailDispatcher email.Dispatcher, mailFrom email.Email, pushVAPID deliveryCompose.VAPIDConfig, hub *realtime.Hub, identity *identityaccess.Module, caller *owners.CallerRows, settings owners.SettingsOwner,
	persons *peopledirectory.Module, services owners.PeopleServices, schedules owners.CarePlanSchedules, requests owners.CarePlanRequests,
	active studentpresence.Presence, reports owners.PresenceReports, lifecycle owners.TimetableLifecycle, planning TimetablePlanning,
	materialization timetable.MaterializationCapability, provisioning organizationtenancy.Provisioning,
	activities activityContract.ActivityService, structureViews StructureServices, facilitiesRooms FacilitiesRooms,
	deviceFleet *devicefleetModule.Module, notices owners.EnrollmentNotices, settingsWrites SettingsWrites, outbox *deliveryModule.Worker, statusDays studentpresence.StatusDays, err error,
) {
	return composeTestRootWith(db, logger, cfg, testRootObservers(), clocks...)
}

func composeTestRootWith(db transactionruntime.Database, logger *slog.Logger, cfg owners.ServiceConfig, observe owners.RootObservers, clocks ...func() time.Time) (
	env owners.RootEnvironment, mailSender email.Mailer, mailDispatcher email.Dispatcher, mailFrom email.Email, pushVAPID deliveryCompose.VAPIDConfig, hub *realtime.Hub, identity *identityaccess.Module, caller *owners.CallerRows, settings owners.SettingsOwner,
	persons *peopledirectory.Module, services owners.PeopleServices, schedules owners.CarePlanSchedules, requests owners.CarePlanRequests,
	active studentpresence.Presence, reports owners.PresenceReports, lifecycle owners.TimetableLifecycle, planning TimetablePlanning,
	materialization timetable.MaterializationCapability, provisioning organizationtenancy.Provisioning,
	activities activityContract.ActivityService, structureViews StructureServices, facilitiesRooms FacilitiesRooms,
	deviceFleet *devicefleetModule.Module, notices owners.EnrollmentNotices, settingsWrites SettingsWrites, outbox *deliveryModule.Worker, statusDays studentpresence.StatusDays, err error,
) {
	env, err = newRootEnvironment(cfg, logger, OptionalClock(clocks), observe)
	if err != nil {
		return
	}
	// Parallel test roots share the SQL pool, not a mutable query-hook list.
	messageDB, err := transactionruntime.NewDatabase(communicationCompose.InstrumentMessageQueries(db.Pool(), true))
	if err != nil {
		return
	}
	env.AllowAuditRootWrites = true
	hub = deliveryCompose.NewRealtimeHub(logger.With("component", "sse-hub"))
	unit := db.UnitOfWork()
	var (
		yard             FacilitiesYard
		tokens           owners.TokenAdapter
		objects          filestorageCompose.ObjectBackend
		audit            owners.AuditOwner
		organizations    organizationtenancy.Capability
		groups           *schoolstructure.Module
		rooms            *facilitiesModule.Rooms
		membership       *schoolMembershipModule.Module
		bookings         *enrollmentOwner.BookingProjection
		calendar         *schoolcalendar.Records
		workTime         *workforceModule.Module
		people           owners.PeopleRows
		peopleIdentity   *identityaccess.Module
		guardian         peoplePorts.GuardianService
		photoRuntime     owners.StudentPhotoRuntime
		timetableOwner   *timetable.Module
		timetableRepos   owners.TimetableRepositories
		carePlan         *carePlanModule.Module
		operatorAuditLog owners.OperatorAuditEntries
		structure        owners.StructureRows
		presencePeople   owners.PresencePeopleReads
		careRows         owners.TimetableCareRows
		devices          devicefleetModule.Capability
		presence         owners.PresenceOwner
		substitutions    owners.GroupSubstitutions
		supervisions     studentpresence.SupervisionRecords
		timetableRows    owners.TimetableOwner
		classArrivals    timetable.ClassArrivals
		studentAudit     peoplePorts.StudentAuditService
		carePlanRows     owners.CarePlanRows
		enrollment       owners.EnrollmentRows
		delivery         owners.DeliveryRuntime
		mailIdentity     email.ReplyToResolver
		portalReads      carePlanCompose.ParentPortalRepositories
		messaging        parentStore.MessagingRepositories
		consent          communicationModule.NotificationConsentCapability
		notifications    owners.DeliveryNotifications
		emitter          eventContract.Emitter
		purger           filestorageModule.AttachmentPurger
		announcements    communicationModule.ParentAnnouncementCapability
		days             owners.TimetableDays
		payroll          settingsCompose.PayrollStatusGetter
		side             *workforceCompose.WorkSessionSide
		offerings        owners.CarePlanOfferings
		reviewPolicy     *owners.ParentRequestReviewPolicy
		staffGroups      schoolstructure.StaffGroupQuery
	)
	err = owners.RunRootSteps(
		func() (err error) {
			tokens, err = tokenAdapterFor(cfg.JWTSecret, cfg.JWTExpiry, cfg.JWTRefreshExpiry)
			return
		},
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
			settings, err = owners.BuildSettingsOwner(db, unit, env, organizations, hub, bookings)
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
		func() error { operatorAuditLog = owners.NewOperatorAuditLog(db); return nil },
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
				structure.Groups, staffGroups, arrivalTimetable.NewCallerActivityGroups(timetableRepos.ActivityGroup),
				presence.SessionRecords, presence.Presence)
			return
		},
		func() (err error) {
			messaging = owners.NewParentMessagingRepositories(messageDB, groups, membership, peopleIdentity, enrollmentCompose.New(), nil, organizations)
			consent, err = communicationCompose.NewNotificationConsent(communicationCompose.NotificationConsentConfig{
				DB: messageDB.Pool(), Observe: env.Observe.Communication,
			})
			return
		},
		func() (err error) {
			notifications, err = owners.BuildDeliveryNotifications(db, unit, env, pushVAPID, hub, owners.DeliveryModuleOf(delivery), settings.Service,
				consent, identity, people, structure, workTime, messaging)
			return
		},
		func() (err error) {
			emitter = owners.BuildParentEventEmitter(messageDB, messageDB.UnitOfWork(), env, hub, settings.Service, messaging, notifications, NewCommunicationEventRuntime(messageDB.Pool() != nil, messageDB.UnitOfWork()))
			purger, err = owners.BuildAttachmentPurger(db, env, objects)
			return
		},
		func() (err error) {
			announcements, err = owners.BuildParentAnnouncements(env, settings.Service, delivery.Outbox, owners.DeliveryModuleOf(delivery),
				notifications, messaging, purger, CommunicationRequestRuntime{})
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
				services.Users, rooms, devices, timetableRows, days, side.AutoCheckIn, emitter, nil, AttendancePrincipal)
			return
		},
		func() (err error) {
			activities, err = owners.BuildActivities(timetableRows, timetableOwner, presence.SessionRecords, people)
			yard = buildFacilitiesYard(env, facilitiesRooms, activities)
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
				caller.Caller(), presence.SessionRecords, supervisions, active, lifecycle.Deviations, timetableRows, audit, settings.Service)
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
		func() error {
			notices = owners.BuildEnrollmentNotices(env, enrollment, settings.Service, delivery.Enqueuer(), organizations, audit, carePlan, owners.DeliveryModuleOf(delivery))
			return nil
		},
		func() (err error) {
			settingsWrites, err = buildSettingsWrites(env, settings, payroll, yard, schedules.Lifecycle, services.StudentPhotos, hub)
			return
		},
		func() error {
			reviewPolicy = owners.NewParentRequestReviewPolicy(caller.Caller().ParentRequestReviews)
			return nil
		},
		func() (err error) {
			requests, err = owners.BuildCarePlanRequests(db, env, hub, carePlan, persons, people, schedules, offerings, enrollment,
				timetableOwner, settings.Service, audit, services, emitter, caller, reviewPolicy, tokens.Permissions, notifications.AbsenceNotifier)
			return
		},
		func() (err error) {
			provisioning, err = owners.BuildProvisioning(db, env, organizations, identity, persons, membership, people, devices,
				presence.SessionRecords, supervisions, timetableRows, operatorAuditLog, settings.Service)
			return
		},
		func() (err error) {
			planning, err = buildTimetablePlanning(db, env, hub, timetableRows, lifecycle, schedules, presence, timetableOwner,
				offerings, audit, people)
			return
		},
		func() (err error) {
			outbox, err = owners.BuildDeliveryWorker(db, env, mailSender, mailFrom, pushVAPID, mailIdentity, delivery, owners.BuildCalendarAppointmentRenderer(db, mailFrom, people),
				owners.IdentityDeliveryRenderers(identity, mailFrom, mailIdentity, delivery.Runtime.EmailStatus, env), nil)
			return
		},
		func() error { statusDays = presence.StatusDays; return nil },
	)
	return
}

// testRootObservers record nothing.
func testRootObservers() owners.RootObservers {
	return owners.RootObservers{
		Communication:             func(communicationCompose.Observation) {},
		AuditAppend:               func(string, time.Duration, int, error) {},
		Delivery:                  func(string, string, string, time.Duration, error) {},
		DurableDelivery:           func(string, string, string, time.Duration, int, error) {},
		DeviceFleet:               func(string, time.Duration, int64, int64, time.Duration, string, error) {},
		IdentityAccess:            func(string, time.Duration, int64, int64, time.Duration, string, error) {},
		FileStorage:               func(filestorageCompose.Observation) {},
		SettingsLookup:            func(string, string, string, time.Duration) {},
		SettingsSideEffectFailure: func(string) {},
		CarePlan:                  func(carePlanCompose.Observation) {},
	}
}

// The test roots below build the server root's owners over a test database
// with the configuration of the test process, and hand out the values one
// suite drives. Every one composes the whole root, as the serving root
// does, so a suite exercises the production wiring.

// NewTestIdentityAccess builds the Identity & Access module of a test root.
func NewTestIdentityAccess(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (*identityaccess.Module, error) {
	return NewTestIdentityAccessWithConfig(db, logger, currentTestServiceConfig(), clocks...)
}

// NewTestIdentityAccessWithConfig builds the Identity & Access module of a
// test root over the given configuration.
func NewTestIdentityAccessWithConfig(db transactionruntime.Database, logger *slog.Logger, cfg owners.ServiceConfig, clocks ...func() time.Time) (*identityaccess.Module, error) {
	_, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(db, logger, cfg, clocks...)
	return identity, err
}

// NewTestActivePresence builds the active service of a test root.
func NewTestActivePresence(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (studentpresence.Presence, error) {
	_, _, _, _, _, _, _, _, _, _, _, _, _, active, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(db, logger, currentTestServiceConfig(), clocks...)
	return active, err
}

// NewTestPresenceCleanup builds the presence cleanup of a test root.
func NewTestPresenceCleanup(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (studentpresence.PresenceCleanup, error) {
	_, _, _, _, _, _, _, _, _, _, _, _, _, _, reports, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(db, logger, currentTestServiceConfig(), clocks...)
	return reports.ActiveCleanup, err
}

// NewTestCarePlanSchedules builds Care Plan's lifecycle and schedules of a
// test root.
func NewTestCarePlanSchedules(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (owners.CarePlanSchedules, error) {
	_, _, _, _, _, _, _, _, _, _, _, schedules, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(db, logger, currentTestServiceConfig(), clocks...)
	return schedules, err
}

// NewTestCareRequests builds the parent request queues of a test root with
// the schedules, the People Directory and its services, the caller rows and
// the settings they read.
func NewTestCareRequests(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (
	owners.CarePlanRequests, owners.CarePlanSchedules, owners.PeopleServices, *owners.CallerRows, *peopledirectory.Module, settingsCompose.BatchSettingsService, error,
) {
	_, _, _, _, _, _, _, caller, settings, persons, services, schedules, requests, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(db, logger, currentTestServiceConfig(), clocks...)
	return requests, schedules, services, caller, persons, settings.Service, err
}

// NewTestPeopleServices builds the People Directory services of a test root
// with the Identity & Access module they are composed over.
func NewTestPeopleServices(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (owners.PeopleServices, *identityaccess.Module, error) {
	_, _, _, _, _, _, identity, _, _, _, services, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(db, logger, currentTestServiceConfig(), clocks...)
	return services, identity, err
}

// NewTestTimetable builds the Timetable owner's lifecycle, planning and
// materialization of a test root with the active service and the care days
// they run over.
func NewTestTimetable(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (
	owners.TimetableLifecycle, TimetablePlanning, timetable.MaterializationCapability, studentpresence.Presence, owners.CarePlanSchedules,
	carePlanModule.CareOfferingCapability, studentpresence.StatusDays, error,
) {
	_, _, _, _, _, _, _, _, _, _, _, schedules, requests, active, _, lifecycle, planning, materialization, _, _, _, _, _, _, _, _, statusDays, err := composeTestRoot(db, logger, currentTestServiceConfig(), clocks...)
	return lifecycle, planning, materialization, active, schedules, requests.CareOfferings, statusDays, err
}
func currentTestServiceConfig() owners.ServiceConfig {
	cfg := CurrentServiceConfig()
	cfg.PublicAPIURL = "http://api.test"
	return cfg
}

// caregiverProfilesForTests binds the Lehrkraft guard to real People
// Directory and School Membership compositions over db, as the service graph does.
func caregiverProfilesForTests(db transactionruntime.Database) (owners.CaregiverProfiles, error) {
	persons, err := owners.NewPeopleDirectory(db)
	if err != nil {
		return owners.CaregiverProfiles{}, err
	}
	membership, err := owners.NewSchoolMembership(db)
	if err != nil {
		return owners.CaregiverProfiles{}, err
	}
	return owners.CaregiverProfiles{Persons: persons, Membership: membership}, nil
}

// OptionalClock is the clock a test graph pins, or nil for the real one.
func OptionalClock(clocks []func() time.Time) func() time.Time {
	if len(clocks) == 0 {
		return nil
	}
	if len(clocks) > 1 {
		panic("a test graph accepts at most one clock")
	}
	return clocks[0]
}
