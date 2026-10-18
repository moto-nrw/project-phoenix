//go:build testhooks

package api

import (
	"log/slog"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
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
func composeTestRoot(db transactionruntime.Database, logger *slog.Logger, cfg ServiceConfig, clocks ...func() time.Time) (
	env RootEnvironment, transport MailTransport, identity *identityaccess.Module, caller *CallerRows, settings SettingsOwner,
	persons *peopledirectory.Module, services PeopleServices, schedules CarePlanSchedules, requests CarePlanRequests,
	active studentpresence.Presence, reports PresenceReports, lifecycle TimetableLifecycle, planning TimetablePlanning,
	materialization timetable.MaterializationCapability, provisioning organizationtenancy.Provisioning,
	activities activityContract.ActivityService, structureViews StructureServices, facilitiesRooms FacilitiesRooms,
	deviceFleet *devicefleetModule.Module, notices EnrollmentNotices, settingsWrites SettingsWrites, outbox *deliveryModule.Worker, statusDays studentpresence.StatusDays, err error,
) {
	return composeTestRootWith(db, logger, cfg, testRootObservers(), clocks...)
}

func composeTestRootWith(db transactionruntime.Database, logger *slog.Logger, cfg ServiceConfig, observe RootObservers, clocks ...func() time.Time) (
	env RootEnvironment, transport MailTransport, identity *identityaccess.Module, caller *CallerRows, settings SettingsOwner,
	persons *peopledirectory.Module, services PeopleServices, schedules CarePlanSchedules, requests CarePlanRequests,
	active studentpresence.Presence, reports PresenceReports, lifecycle TimetableLifecycle, planning TimetablePlanning,
	materialization timetable.MaterializationCapability, provisioning organizationtenancy.Provisioning,
	activities activityContract.ActivityService, structureViews StructureServices, facilitiesRooms FacilitiesRooms,
	deviceFleet *devicefleetModule.Module, notices EnrollmentNotices, settingsWrites SettingsWrites, outbox *deliveryModule.Worker, statusDays studentpresence.StatusDays, err error,
) {
	env, err = newRootEnvironment(cfg, logger, OptionalClock(clocks), observe)
	if err != nil {
		return
	}
	env.AllowAuditRootWrites = true
	hub := deliveryCompose.NewRealtimeHub(logger.With("component", "sse-hub"))
	unit := db.UnitOfWork()
	communicationCompose.InstallMessageQueryInstrumentation(db.Pool())
	var (
		yard             FacilitiesYard
		tokens           TokenAdapter
		objects          filestorageCompose.ObjectBackend
		audit            AuditOwner
		organizations    organizationtenancy.Capability
		groups           *schoolstructure.Module
		rooms            *facilitiesModule.Rooms
		membership       *schoolMembershipModule.Module
		bookings         *enrollmentOwner.BookingProjection
		calendar         *schoolcalendar.Records
		workTime         *workforceModule.Module
		people           PeopleRows
		peopleIdentity   *identityaccess.Module
		guardian         peoplePorts.GuardianService
		photoRuntime     studentPhotoRuntime
		timetableOwner   *timetable.Module
		timetableRepos   TimetableRepositories
		carePlan         *carePlanModule.Module
		operatorAuditLog operatorAuditEntries
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
		portalReads      carePlanCompose.ParentPortalRepositories
		messaging        parentStore.MessagingRepositories
		consent          communicationModule.NotificationConsentCapability
		notifications    DeliveryNotifications
		emitter          eventContract.Emitter
		purger           filestorageModule.AttachmentPurger
		announcements    communicationModule.ParentAnnouncementCapability
		days             TimetableDays
		payroll          settingsCompose.PayrollStatusGetter
		side             *workforceCompose.WorkSessionSide
		offerings        CarePlanOfferings
		reviewPolicy     *ParentRequestReviewPolicy
		staffGroups      schoolstructure.StaffGroupQuery
	)
	err = runRootSteps(
		func() (err error) {
			tokens, err = tokenAdapterFor(cfg.JWTSecret, cfg.JWTExpiry, cfg.JWTRefreshExpiry)
			return
		},
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
		func() (err error) { timetableOwner, err = buildServeTimetable(db, persons, rooms, membership); return },
		func() error {
			timetableRepos = NewTimetableRepositories(db, timetableOwner, persons, groups, rooms, calendar, membership, workTime)
			return nil
		},
		func() (err error) {
			carePlan, err = composeCarePlan(db, persons, CarePlanStatusSlots(timetableRepos.InstanceStudent))
			return
		},
		func() error { operatorAuditLog = NewOperatorAuditLog(db); return nil },
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
				services.Users, rooms, devices, timetableRows, days, side.AutoCheckIn, emitter, nil)
			return
		},
		func() (err error) {
			activities, err = buildActivities(timetableRows, timetableOwner, presence.SessionRecords, people)
			yard = buildFacilitiesYard(env, facilitiesRooms, activities)
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
		func() error {
			notices = buildEnrollmentNotices(env, enrollment, settings.Service, delivery.Enqueuer(), organizations, audit, carePlan, deliveryModuleOf(delivery))
			return nil
		},
		func() (err error) {
			settingsWrites, err = buildSettingsWrites(env, settings, payroll, yard, schedules.Lifecycle, services.StudentPhotos, transport.Hub)
			return
		},
		func() error {
			reviewPolicy = NewParentRequestReviewPolicy(caller.Caller().ParentRequestReviews)
			return nil
		},
		func() (err error) {
			requests, err = buildCarePlanRequests(db, env, transport, carePlan, persons, people, schedules, offerings, enrollment,
				timetableOwner, settings.Service, audit, services, emitter, caller, reviewPolicy, tokens.Permissions, notifications.AbsenceNotifier)
			return
		},
		func() (err error) {
			provisioning, err = buildProvisioning(db, env, organizations, identity, persons, membership, people, devices,
				presence.SessionRecords, supervisions, timetableRows, operatorAuditLog, settings.Service)
			return
		},
		func() (err error) {
			planning, err = buildTimetablePlanning(db, env, transport, timetableRows, lifecycle, schedules, presence, timetableOwner,
				offerings, audit, people)
			return
		},
		func() (err error) {
			outbox, err = buildDeliveryWorker(db, env, transport, mailIdentity, delivery, buildCalendarAppointmentRenderer(db, transport, people),
				identityDeliveryRenderers(identity, transport.DefaultFrom, mailIdentity, delivery.Runtime.EmailStatus, env), nil)
			return
		},
		func() error { statusDays = presence.StatusDays; return nil },
	)
	return
}

// testRootObservers record nothing.
func testRootObservers() RootObservers {
	return RootObservers{
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
func NewTestIdentityAccessWithConfig(db transactionruntime.Database, logger *slog.Logger, cfg ServiceConfig, clocks ...func() time.Time) (*identityaccess.Module, error) {
	_, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(db, logger, cfg, clocks...)
	return identity, err
}

// NewTestActivePresence builds the active service of a test root.
func NewTestActivePresence(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (studentpresence.Presence, error) {
	_, _, _, _, _, _, _, _, _, active, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(db, logger, currentTestServiceConfig(), clocks...)
	return active, err
}

// NewTestPresenceCleanup builds the presence cleanup of a test root.
func NewTestPresenceCleanup(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (studentpresence.PresenceCleanup, error) {
	_, _, _, _, _, _, _, _, _, _, reports, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(db, logger, currentTestServiceConfig(), clocks...)
	return reports.ActiveCleanup, err
}

// NewTestCarePlanSchedules builds Care Plan's lifecycle and schedules of a
// test root.
func NewTestCarePlanSchedules(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (CarePlanSchedules, error) {
	_, _, _, _, _, _, _, schedules, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(db, logger, currentTestServiceConfig(), clocks...)
	return schedules, err
}

// NewTestCareRequests builds the parent request queues of a test root with
// the schedules, the People Directory and its services, the caller rows and
// the settings they read.
func NewTestCareRequests(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (
	CarePlanRequests, CarePlanSchedules, PeopleServices, *CallerRows, *peopledirectory.Module, settingsCompose.BatchSettingsService, error,
) {
	_, _, _, caller, settings, persons, services, schedules, requests, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(db, logger, currentTestServiceConfig(), clocks...)
	return requests, schedules, services, caller, persons, settings.Service, err
}

// NewTestPeopleServices builds the People Directory services of a test root
// with the Identity & Access module they are composed over.
func NewTestPeopleServices(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (PeopleServices, *identityaccess.Module, error) {
	_, _, identity, _, _, _, services, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(db, logger, currentTestServiceConfig(), clocks...)
	return services, identity, err
}

// NewTestTimetable builds the Timetable owner's lifecycle, planning and
// materialization of a test root with the active service and the care days
// they run over.
func NewTestTimetable(db transactionruntime.Database, logger *slog.Logger, clocks ...func() time.Time) (
	TimetableLifecycle, TimetablePlanning, timetable.MaterializationCapability, studentpresence.Presence, CarePlanSchedules,
	carePlanModule.CareOfferingCapability, studentpresence.StatusDays, error,
) {
	_, _, _, _, _, _, _, schedules, requests, active, _, lifecycle, planning, materialization, _, _, _, _, _, _, _, _, statusDays, err := composeTestRoot(db, logger, currentTestServiceConfig(), clocks...)
	return lifecycle, planning, materialization, active, schedules, requests.CareOfferings, statusDays, err
}
func currentTestServiceConfig() ServiceConfig {
	cfg := CurrentServiceConfig()
	cfg.PublicAPIURL = "http://api.test"
	return cfg
}

// caregiverProfilesForTests binds the Lehrkraft guard to real People
// Directory and School Membership compositions over db, as the service graph does.
func caregiverProfilesForTests(db transactionruntime.Database) (CaregiverProfiles, error) {
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return CaregiverProfiles{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return CaregiverProfiles{}, err
	}
	return CaregiverProfiles{Persons: persons, Membership: membership}, nil
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
