package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/analytics"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
	appointmentsModule "github.com/moto-nrw/project-phoenix/modules/appointments"
	appointmentsCompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	dataImportRetained "github.com/moto-nrw/project-phoenix/modules/dataimport/compose/retained"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	feedbackAPI "github.com/moto-nrw/project-phoenix/modules/feedback/http"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanCompose "github.com/moto-nrw/project-phoenix/modules/mealplan/compose"
	mealplanAPI "github.com/moto-nrw/project-phoenix/modules/mealplan/http"
	organizationModule "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	usersAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	schoolCalendarModule "github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	schoolStructureModule "github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	enrollmentProvenanceView "github.com/moto-nrw/project-phoenix/modules/timetableenrollmentprovenance/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/spf13/viper"
)

// recordHTTPRuntimeEvent turns one runtime event into a metric and, for
// failures, one ERROR record carrying method, route, path and status. A
// rolled-back transaction is only a failure when the client got a 5xx: the
// tenant middleware and service-owned transactions also roll back behind
// 401/404/410 responses, and those must not feed the error-spike alert
// (#2953). The slog-chi request line already records every 4xx at WARN.
type httpRuntimeObservation = apiCommon.TenantRuntimeObservation

func recordHTTPRuntimeEvent(tracer *observability.Tracer, observation httpRuntimeObservation) {
	event, r, status := observation.Event, observation.Request, observation.Status
	observability.RecordUnitOfWorkEvent(
		"http",
		string(event.Kind),
		string(event.Result),
		event.Duration,
		event.Retries,
	)
	ctx := r.Context()
	attrs := []slog.Attr{
		slog.String("method", r.Method),
		slog.String("route", observation.Route),
		slog.String("path", customMiddleware.RedactFeedToken(r.URL.Path)),
		slog.Int("status", status),
	}
	switch {
	case event.Kind == apiCommon.TenantRuntimeMissingTenant:
		tracer.Failure(ctx, "http", string(event.Kind), "missing_tenant", event.Err, attrs...)
	case event.Kind == apiCommon.TenantRuntimeTransaction && event.Err != nil && status >= http.StatusInternalServerError:
		attrs = append(attrs, slog.String("result", string(event.Result)))
		tracer.Failure(ctx, "http", string(event.Kind), "transaction_failure", event.Err, attrs...)
	case event.Kind == apiCommon.TenantRuntimeResponseWrite && event.Err != nil:
		tracer.Failure(ctx, "http", string(event.Kind), "response_write_failure", event.Err, attrs...)
	}
}

type moduleServices struct {
	// carePlan is the observed Care Plan owner the retained repositories and
	// the Worker's booking audit read through.
	carePlan *carePlanModule.Module
	// studentStatusDays archives the Worker's status flags over Care Plan.
	studentStatusDays schedulerPorts.StudentStatusFlagArchiver
	// companions is Care Plan's companion graph of the student routes;
	// reviewPolicy is the parent request review reach (#4011).
	companions   carePlanModule.StudentCompanions
	reviewPolicy *ParentRequestReviewPolicy
	// timetableRepositories are the retained Timetable adapters (#2743).
	timetableRepositories timetableCompose.Repositories
	services              *serviceGraph
	// organizations is the Organisation & Tenancy owner the root composed;
	// listExport renders the list documents; imports are the Data Import
	// services; databaseStats counts the Datenverwaltung hub (#4013).
	organizations organizationModule.Capability
	listExport    *lists.RendererService
	imports       imports
	databaseStats DatabaseStatsReader
	// presence is the Student Presence, Facilities, Device Fleet and
	// class-day wiring the root built for the service graph (#4010).
	presence PresenceWiring
	// planningTracks is the planning-track administration the timetable
	// routes and the service graph share (#4009).
	planningTracks timetableModule.PlanningTrackAdministration
	// identity is the Identity & Access module the root composed (#4006);
	// userContext is its caller context with the rows the retained routes
	// read.
	identity      *identityaccess.Module
	userContext   *CallerRows
	demoAccess    authAPI.DemoAccesses
	communication *communicationModule.Module
	mealPlan      *mealplanModule.Module
	feedback      *feedbackModule.Module
	persons       *peopleModule.Module
	rooms         *facilitiesModule.Module
	timetable     *timetableModule.Module
	// calendar owns schedule.calendar_periods, schedule.closing_days and
	// schedule.dateframes and answers the tenant's non-working days.
	calendar *schoolCalendarModule.Module
	// membership owns users.staff, users.teachers and users.guests (#2667).
	membership *schoolMembershipModule.Module
	// workforce owns the work-time templates and staff schedule versions
	// in config.work_time_models, config.work_time_model_entries and
	// config.staff_work_schedules (#2687).
	workforce *workforceModule.Module
	// students is the retained student repository over the People Directory
	// (#2743).
	students StudentRootReads
	// guardianProfiles loads a guardian's profile with the children for the
	// enrollment me/profile routes; birthdays is the birthday display (#4007).
	guardianProfiles *peopleCompose.GuardianProfileLoader
	birthdays        peopleModule.Birthdays
	staff            interface {
		ListAllStaffAccountIDs(ctx context.Context) (map[int64]int64, error)
	}
}

// NewCleanupTimetable composes the unobserved Timetable owner for CLI roots.
// The serving root uses initializeModuleServices so it can attach metrics.
func NewCleanupTimetable(db transactionruntime.Database) (timetableModule.Capability, error) {
	students, err := peopleCompose.New(peopleCompose.Dependencies{StudentDirectory: studentDirectoryView.NewReader(), DB: db.Pool(), Observe: func(peopleCompose.Observation) {}, GuardianLinkRows: guardianLinkView.NewReader()})
	if err != nil {
		return nil, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return nil, err
	}
	return NewUnobservedTimetable(db, students, rooms)
}

func composeTimetable(db transactionruntime.Database, persons *peopleModule.Module, rooms *facilitiesModule.Module, membership *schoolMembershipModule.Module) (*timetableModule.Module, error) {
	return newTimetable(db, persons, rooms, membership, func(observation timetableCompose.Observation) {
		observability.ObserveTimetableActivitiesOperation(
			observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows,
			observation.Stats.DuplicatePreventionConflicts, observation.Stats.StatementDuration,
			timetableModule.ErrorCode(observation.Err), observation.Err,
		)
	})
}

// newTimetable composes the Timetable owner. A staff assignment locks the
// School Membership row; the session facts come from Student Presence over
// the same database (#2762).
func newTimetable(db transactionruntime.Database, students peopleModule.StudentQuery, rooms facilitiesModule.Query, membership schoolMembershipModule.Capability, observe func(timetableCompose.Observation)) (*timetableModule.Module, error) {
	return timetableCompose.New(timetableCompose.Dependencies{
		LockStaffAssignment: func(ctx context.Context, staffID int64) error {
			_, err := membership.FindStaffForMutation(ctx, staffID)
			return err
		},
		DB: db.Pool(), Students: timetableStudents(students), Rooms: timetableRooms(rooms), Sessions: NewPresenceFacts(db),
		Observe: observe, Provenance: enrollmentProvenanceView.NewReader(),
	})
}

func initializeModuleServices(db transactionruntime.Database, publicAPIURL string, logger *slog.Logger, productEvents ProductEventTracker) (moduleServices, error) {
	pool, tenantRuntime := db.Pool(), db.UnitOfWork()
	organizations, err := organizationCompose.New(organizationCompose.Dependencies{
		DB: pool,
		Observe: func(observation organizationCompose.Observation) {
			observability.ObserveOrganizationTenancyOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, organizationModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return moduleServices{}, err
	}
	// The photo lifecycle's runtime (feature gate, caller access, file
	// cleanup, live refresh, consent trail) only exists once the service
	// graph is up, so the owner resolves it from a slot the graph fills.
	persons, studentPhotoRuntime, err := NewPeopleDirectoryWithPhotosAndObserver(db, func(observation peopleCompose.Observation) {
		observability.ObservePeopleDirectoryOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, peopleModule.ErrorCode(observation.Err), observation.Err)
	})
	if err != nil {
		return moduleServices{}, err
	}
	groups, err := schoolStructureCompose.New(schoolStructureCompose.Dependencies{
		DB: pool,
		Observe: func(observation schoolStructureCompose.Observation) {
			observability.ObserveSchoolStructureOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolStructureModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return moduleServices{}, err
	}
	// The room deletion asks Student Presence and Care Plan through the
	// Facilities room usage port; the root binds both once the service graph
	// exists (#4010).
	roomUsage := &RoomUsage{}
	rooms, err := composeFacilities(db, roomUsage)
	if err != nil {
		return moduleServices{}, err
	}
	membership, err := composeSchoolMembership(db)
	if err != nil {
		return moduleServices{}, err
	}
	// The period administration resolves the recurrence gate, the
	// care-offering guard and the federal-state setting on every call from
	// the runtime the service graph answers with below; the guard is an
	// Enrollment service that itself reads this calendar, so it cannot exist
	// before the service graph does.
	var calendarAdministration schoolCalendarCompose.AdministrationRuntime
	calendar, err := schoolCalendarCompose.New(schoolCalendarCompose.Dependencies{
		DB: pool,
		Observe: func(observation schoolCalendarCompose.Observation) {
			observability.ObserveSchoolCalendarOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolCalendarModule.ErrorCode(observation.Err), observation.Err)
		},
		Administration: func() schoolCalendarCompose.AdministrationRuntime { return calendarAdministration },
	})
	if err != nil {
		return moduleServices{}, err
	}
	timetableCapability, err := composeTimetable(db, persons, rooms, membership)
	if err != nil {
		return moduleServices{}, err
	}
	workTime, err := workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: workforceStaffAssignmentLock(membership),
		DB:                  pool,
		LiveStaffIDs:        workforceLiveStaffIDs(membership),
		// A Sonderarbeitszeit never sets a target on a statutory holiday.
		StatutoryHolidays: calendar.TenantHolidayDates,
		Observe: func(observation workforceCompose.Observation) {
			observability.ObserveWorkforceOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, workforceModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return moduleServices{}, err
	}
	// The retained activities and schedule repositories belong to Timetable
	// (#2743); the Care Plan slots, the kiosk session mirror and the Worker
	// read the same adapters.
	timetableRepositories := NewTimetableRepositories(db, timetableCapability, persons, groups, rooms, calendar, membership, workTime)
	appointmentCapability, err := appointmentsCompose.New(appointmentsCompose.Dependencies{
		DB: pool,
		Observe: func(observation appointmentsCompose.Observation) {
			observability.ObserveAppointmentsOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.DuplicatePreventionConflicts, observation.Stats.StatementDuration, appointmentsModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return moduleServices{}, err
	}
	communicationCapability, err := communicationCompose.New(communicationCompose.Dependencies{
		DB:            pool,
		Organizations: organizations,
		Viewers:       communicationViewerNames{query: persons},
		Audit:         NewCommunicationAnnouncementAudit(NewOperatorAuditLog(db).Create),
		Observe:       observeCommunication,
	})
	if err != nil {
		return moduleServices{}, err
	}
	carePlan, err := composeCarePlan(db, persons, CarePlanStatusSlots(timetableRepositories.InstanceStudent))
	if err != nil {
		return moduleServices{}, err
	}
	// The retained People Directory, School Membership and School Structure
	// repositories and their adapters are built here and handed to the
	// service graph (#4007), and so are Student Presence, its session
	// records and the room service (#4010). The meal plan's participants, the
	// Worker's activation tick and its reminder recipients read the same
	// student and staff repositories.
	people, presence, timetableWiring, platform, err := newServiceGraphWiring(PeopleWiringOwners{
		DB: db, Logger: logger, Persons: persons, Groups: groups, Rooms: rooms, Membership: membership,
		Workforce: workTime, CarePlan: carePlan, Organizations: organizations, Timetable: timetableCapability,
		ObserveIdentityAccess: observability.ObserveIdentityAccessOperation,
	}, PresenceWiringOwners{
		DB: db, Logger: logger, Persons: persons, Rooms: rooms, Membership: membership, CarePlan: carePlan,
		Calendar: calendar, Timetable: timetableCapability, TimetableRows: timetableRepositories,
	}, organizations)
	if err != nil {
		return moduleServices{}, err
	}
	students := NewStudentRootReads(people.Students)
	staff := people.Staff
	mealPlanSettings := mealplanCompose.NewSettings()
	mealPlan, err := mealplanCompose.New(mealplanCompose.Dependencies{
		DB:       pool,
		Settings: mealPlanSettings,
		Observe: func(observation mealplanCompose.Observation) {
			observability.ObserveMealPlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, observation.Err)
		},
		Now:          time.Now,
		Participants: mealPlanParticipantFinder(students, time.Now),
	})
	if err != nil {
		return moduleServices{}, err
	}
	feedbackSettings := feedbackCompose.NewSettings()
	feedbackCapability, err := composeFeedback(db, feedbackSettings)
	if err != nil {
		return moduleServices{}, err
	}
	// Care Plan's bindings to the owners composed here are built by the root
	// and handed to the service graph (#4011); the student routes
	// mount the same companion graph.
	carePlanOwners := carePlanWiringOwners{
		DB: db, Logger: logger, CarePlan: carePlan, Persons: persons, Membership: membership,
		Timetable: timetableCapability, Calendar: calendar, People: people, ObserveCarePlan: observeCarePlan,
	}
	factory, identity, carePlanBindings, err := withRootWiring(db, carePlanOwners, serviceGraphInputs{
		DB: db, TenantRuntime: tenantRuntime, Logger: logger,
		CarePlan: carePlan, Organizations: organizations, Persons: persons, Groups: groups, Rooms: rooms,
		Membership: membership, Calendar: calendar, Timetable: timetableCapability, Appointments: appointmentCapability,
		WorkTime: workTime, MealPlan: mealPlan, BindMealPlanSettings: mealPlanSettings.Bind,
		FeedbackCounter: feedbackCapability, BindFeedbackSettings: feedbackSettings.Bind, ProductEvents: productEvents,
		ObserveCommunication: observeCommunication, ObserveAuditAppend: observability.ObserveAuditAppend,
		ObserveDelivery: observability.ObserveSynchronousDelivery, ObserveDurableDelivery: observability.ObserveDurableDelivery,
		ObserveDeviceFleet: observability.ObserveDeviceFleetOperation, ObserveIdentityAccess: observability.ObserveIdentityAccessOperation,
		StudentPhotoSlot: studentPhotoRuntime,
		PhotoUnlinker:    peopleModule.NewPhotoUnlinker(logger.With("component", "student-photo-unlinker"), "public", nativeUploads.UploadFiles{}),
		Workforce:        newWorkforceWiring(db, workTime, membership, persons, organizations, staff),
		People:           people, Platform: platform, Presence: presence, TimetableWiring: timetableWiring,
		Enrollment: newEnrollmentWiring(enrollmentWiringOwners{
			DB: db, CarePlan: carePlan, Persons: persons, Calendar: calendar,
			Timetable: timetableCapability, Schools: people.EnrollmentSchools,
		}),
	}, publicAPIURL)
	if err != nil {
		return moduleServices{}, err
	}
	roomUsage.Sessions, roomUsage.Offerings = presence.Presence, factory.EnrollmentCareOffering
	// The guardian profile loader backs the public and parent enrollment
	// me/profile reads; the birthday display (#1542) reads the same retained
	// rows. The root builds both where it mounts them (#4007).
	guardianProfiles := peopleCompose.NewGuardianProfileLoader(people.GuardianProfiles, pool, logger.With("service", "guardian-profile-loader"))
	birthdays := NewBirthdays(BirthdayRepositories{Students: people.Students, Staff: people.Staff, Persons: people.Persons},
		factory.Settings, logger.With("service", "birthdays"), nil)
	calendarAdministration = GuardedCalendarAdministration(factory.Settings, timetableWiring.RecurrenceLock, factory.EnrollmentCareOffering)
	identityModule, userContext := identity()
	studentStatusDays := NewStudentStatusDayRepository(carePlan)
	// The parent request review policy of the request queues, for the review
	// projection and the student routes (#4011).
	reviewPolicy := NewParentRequestReviewPolicy(userContext.Caller().ParentRequestReviews)
	// The Data Import (#2708) commits every accepted row through the owner
	// commands; the root builds it where it mounts the import routes (#4013).
	importRuntime := newImportRuntime(factory.Audit, observeDataImport)
	imports := imports{
		Student: newStudentImport(
			importRuntime, identityModule, groups, rooms, persons, persons, carePlan, presence.Presence,
			auditCompose.NewConsentRecorder(factory.Audit),
		),
		Staff:     newStaffImport(importRuntime, identityModule, identityModule, organizations, persons, membership, workTime),
		ClassList: newClassListImport(importRuntime, membership, persons),
		OpeningBalance: newOpeningBalanceImport(importRuntime, dataImportRetained.OpeningBalanceImportDeps{
			BalanceAdjustService: factory.WorkSession.OpeningBalances,
			StaffAbsenceService:  factory.WorkSession.VacationTakeovers,
		}, membership, persons, workTime),
	}
	databaseStats := newDatabaseStats(people.Repositories, people.Groups, rooms, timetableRepositories, identityModule, platform.DeviceFleet, logger)
	return moduleServices{organizations: organizations, listExport: platform.ListExport, imports: imports, databaseStats: databaseStats, carePlan: carePlan, studentStatusDays: studentStatusDays, presence: presence, companions: carePlanBindings.companions, reviewPolicy: reviewPolicy, timetableRepositories: timetableRepositories, planningTracks: timetableWiring.PlanningTracks, services: factory, identity: identityModule, userContext: userContext, demoAccess: authAPI.ComposedDemoAccess(identityModule.DemoAccess()), communication: communicationCapability, mealPlan: mealPlan, feedback: feedbackCapability, persons: persons, rooms: rooms, timetable: timetableCapability, calendar: calendar, membership: membership, workforce: workTime, students: students, staff: staff, guardianProfiles: guardianProfiles, birthdays: birthdays}, nil
}

// newServiceGraphWiring builds the wiring the service graph composes over:
// the People Directory, School Membership and School Structure wiring
// (#4007) and, over its retained rows, the Student Presence, Facilities,
// Device Fleet and class-day wiring (#4010), together with the Timetable,
// School Calendar and reminder wiring (#4009) and the platform,
// Communication and Organisation & Tenancy wiring (#4013).
func newServiceGraphWiring(people PeopleWiringOwners, presence PresenceWiringOwners, organizations organizationModule.Capability) (PeopleWiring, PresenceWiring, TimetableWiring, PlatformWiring, error) {
	peopleWiring, err := NewPeopleWiring(people)
	if err != nil {
		return PeopleWiring{}, PresenceWiring{}, TimetableWiring{}, PlatformWiring{}, err
	}
	presence.People = peopleWiring
	presenceWiring, err := NewPresenceWiring(presence)
	if err != nil {
		return PeopleWiring{}, PresenceWiring{}, TimetableWiring{}, PlatformWiring{}, err
	}
	timetableWiring, err := NewTimetableWiring(TimetableWiringOwners{
		DB: people.DB, Logger: people.Logger, Repositories: presence.TimetableRows, Timetable: people.Timetable,
		Calendar: presence.Calendar, Rooms: people.Rooms, Membership: people.Membership, Persons: people.Persons, People: peopleWiring,
	})
	if err != nil {
		return PeopleWiring{}, PresenceWiring{}, TimetableWiring{}, PlatformWiring{}, err
	}
	platform, err := NewPlatformWiring(PlatformWiringOwners{
		DB: people.DB, Organizations: organizations, Persons: people.Persons, Groups: people.Groups,
		Membership: people.Membership, People: peopleWiring,
	})
	return peopleWiring, presenceWiring, timetableWiring, platform, err
}

// composeSchoolMembership composes the observed School Membership owner. A
// staff member is School Membership's membership row plus Workforce's
// employment profile (#2753); the membership owner composes the two.
func composeSchoolMembership(db transactionruntime.Database) (*schoolMembershipModule.Module, error) {
	pool := db.Pool()
	staffEmployment, err := workforceCompose.NewStaffEmployment(pool, func(observation workforceCompose.Observation) {
		observability.ObserveWorkforceOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, workforceModule.ErrorCode(observation.Err), observation.Err)
	})
	if err != nil {
		return nil, err
	}
	return schoolMembershipCompose.New(schoolMembershipCompose.Dependencies{
		DB: pool,
		Observe: func(observation schoolMembershipCompose.Observation) {
			observability.ObserveSchoolMembershipOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, schoolMembershipModule.ErrorCode(observation.Err), observation.Err)
		},
		Employment: MembershipStaffEmployment(staffEmployment),
		// Every counting membership write is checked against the school's
		// Kinderkontingent, which Organisation & Tenancy owns (#3567).
		ChildQuota: organizationCompose.NewChildQuotaLimits(),
	})
}

// composeFeedback composes the observed Feedback owner over its settings.
func composeFeedback(db transactionruntime.Database, settings *feedbackCompose.Settings) (*feedbackModule.Module, error) {
	return feedbackCompose.New(feedbackCompose.Dependencies{
		DB:       db.Pool(),
		Settings: settings,
		Today:    feedbackModule.Today,
		Observe: func(observation feedbackCompose.Observation) {
			observability.ObserveFeedbackOperation(
				observation.Operation,
				observation.Duration,
				observation.Stats.Queries,
				observation.Stats.Rows,
				observation.Stats.StatementDuration,
				feedbackModule.ErrorCode(observation.Err),
				observation.Err,
			)
		},
	})
}

// withRootWiring resolves what the service graph needs from the root and
// cannot compose itself. File Storage (#2707) needs the uploads object store,
// rooted at the uploads directory every document feature shares, and the
// metrics sink; the service graph composes the module where the announcement ports
// live. Identity & Access is composed by the root over the values the
// factory builds, with the session codec of Identity's session token adapter
// (#2747, #4006). Care Plan's bindings to the root's owners come built
// (#4011); their change history names the actor through the same token
// adapter.
func withRootWiring(db transactionruntime.Database, carePlanOwners carePlanWiringOwners, in serviceGraphInputs, publicAPIURL string) (*serviceGraph, composedIdentity, carePlanRoot, error) {
	uploads, err := nativeUploads.PrivateUploadsBackend()
	if err != nil {
		return nil, nil, carePlanRoot{}, fmt.Errorf("resolve uploads backend: %w", err)
	}
	tokens, err := newLegacyTokenAdapter()
	if err != nil {
		return nil, nil, carePlanRoot{}, err
	}
	wiring, identity := newIdentityRoot(db, tokens)
	carePlanOwners.AuditActor = wiring.AuditActor
	carePlan, err := newCarePlanWiring(carePlanOwners)
	if err != nil {
		return nil, nil, carePlanRoot{}, err
	}
	in.FileStorage = FileStorageWiring{
		Objects: uploads, Observe: observeFileStorage,
		Settings: func(settings settingsCompose.SettingsService) filestorageCompose.Settings {
			return FileStorageSettings{Service: settings}
		},
		Events: func(events FileEventAppender) filestorageCompose.Events {
			return FileStorageEvents{Repo: events}
		},
	}
	in.Identity, in.CarePlanWiring = wiring, carePlan.wiring
	graph, err := newServeServiceGraph(in, publicAPIURL)
	if err != nil {
		return nil, nil, carePlanRoot{}, err
	}
	return graph, identity, carePlan, nil
}

// observeCommunication records one Communication operation.
func observeCommunication(observation communicationCompose.Observation) {
	observability.ObserveCommunicationOperation(
		observation.Operation, observation.Duration, int64(observation.Stats.Queries), observation.Stats.Rows,
		int64(observation.Stats.DuplicatePreventionConflicts), observation.Stats.StatementDuration,
		communicationModule.ErrorCode(observation.Err), observation.Err,
	)
}

func observeFileStorage(observation filestorageCompose.Observation) {
	observability.ObserveFileStorageOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, filestorageModule.ErrorCode(observation.Err), observation.Err)
}

func observeDataImport(observation dataImportRetained.ImportObservation) {
	observability.ObserveDataImport(observation.Entity, observation.DryRun, observation.Rows, observation.Accepted, observation.Rejected, observation.Created, observation.Updated, observation.Duration)
	if observation.DryRun {
		return
	}
	observability.ObserveDataImportRuntime(observation.Entity, observation.BatchesCommitted, observation.BatchesRetried, observation.CheckpointLag, observation.Deadlocks, observation.PoolWait, observation.LockWait)
	for _, command := range observation.Commands {
		observability.ObserveDataImportCommand(observation.Entity, command.Owner, command.Operation, command.Duration, command.Failed)
	}
}

func composeFacilities(db transactionruntime.Database, usage facilitiesCompose.RoomUsage) (*facilitiesModule.Module, error) {
	recurrenceLock, err := timetableCompose.NewRecurrenceWriteLock(timetableCompose.RecurrenceLockDependencies{
		DB: db.Pool(), GradeTransitionLockKey: schoolStructureModule.TransitionsLockKey,
	})
	if err != nil {
		return nil, err
	}
	return facilitiesCompose.New(facilitiesCompose.Dependencies{
		DB:           db.Pool(),
		DeletionLock: recurrenceLock.LockRecurrenceWrites,
		Usage:        usage,
		Observe: func(observation facilitiesCompose.Observation) {
			observability.ObserveFacilitiesOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, facilitiesModule.ErrorCode(observation.Err), observation.Err)
		},
	})
}

func composeCarePlan(db transactionruntime.Database, persons *peopleModule.Module, slots carePlanCompose.StatusSlotDirectory) (*carePlanModule.Module, error) {
	profiles, err := carePlanCompose.NewStudentProfiles(db.Pool(), studentDirectoryView.NewReader(), func(carePlanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	statusStudents, err := CarePlanStatusStudents(persons, profiles)
	if err != nil {
		return nil, err
	}
	return carePlanCompose.New(carePlanCompose.Dependencies{
		StudentDirectory: studentDirectoryView.NewReader(),
		DB:               db.Pool(), AmbientDB: carePlanCompose.TenantAmbientDatabase(db.Pool()),
		StatusStudents: statusStudents, StatusSlots: slots,
		People: carePlanStudentNames(func(ctx context.Context, ids []int64) ([]carePlanPorts.StudentName, error) {
			values, err := persons.ListStudentNamesByID(ctx, ids)
			if err != nil {
				return nil, err
			}
			result := make([]carePlanPorts.StudentName, 0, len(values))
			for _, value := range values {
				result = append(result, carePlanPorts.StudentName{StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName})
			}
			return result, nil
		}),
		StudentLock: persons.LockStudent, StudentNotFound: peopleModule.ErrStudentNotFound,
		Observe: func(observation carePlanCompose.Observation) {
			observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, carePlanModule.ErrorCode(observation.Err), observation.Err)
		},
	})
}

func mealPlanParticipantFinder(students StudentRootReads, now func() time.Time) mealplanCompose.ParticipantFinder {
	return func(ctx context.Context, date string) ([]mealplanCompose.ParticipantCandidate, error) {
		rows, err := students.OverlappingOnDate(ctx, date, now())
		if err != nil {
			return nil, err
		}
		candidates := make([]mealplanCompose.ParticipantCandidate, 0, len(rows))
		for _, row := range rows {
			candidates = append(candidates, mealplanCompose.ParticipantCandidate{
				StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass,
			})
		}
		return candidates, nil
	}
}

var mealPlanErrorRules = []apiCommon.ErrorRule{
	{Target: mealplanModule.ErrDisabled, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorForbiddenWithCode, errors.New(mealplanModule.ErrDisabled.Detail), apiCommon.CodeMealsPlanDisabled)},
	{Target: mealplanModule.ErrInvalidMealDate, Render: apiCommon.FixedRenderer(apiCommon.ErrorInvalidRequest, errors.New(mealplanModule.ErrInvalidMealDate.Detail))},
	{Target: mealplanModule.ErrInvalidDishes, Render: apiCommon.ErrorInvalidRequest},
	{Target: mealplanModule.ErrRegistrationDisabled, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorForbiddenWithCode, errors.New(mealplanModule.ErrRegistrationDisabled.Detail), apiCommon.CodeMealsRegistrationDisabled)},
	{Target: mealplanModule.ErrInvalidParticipation, Render: apiCommon.ErrorInvalidRequest},
	{Target: mealplanModule.ErrParticipationCutoff, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorConflictWithCode, errors.New(mealplanModule.ErrParticipationCutoff.Detail), apiCommon.CodeMealsParticipationCutoffPassed)},
}

func renderMealPlanFailure(w http.ResponseWriter, r *http.Request, err error, internalMessage string) {
	renderer := apiCommon.RulesRenderer(mealPlanErrorRules, apiCommon.ErrorInternalServerRenderer(internalMessage))
	apiCommon.RenderError(w, r, renderer(err))
}

func newMealPlanResource(module *mealplanModule.Module, renderer SimpleListRenderer) *mealplanAPI.Resource {
	return mealplanAPI.NewResource(module, mealplanAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, mealplanAPI.Middleware)) {
			ProtectedTenantRoutes(router, register)
		},
		Permission: func(access mealplanAPI.Access) mealplanAPI.Middleware {
			return requireMealPlanAccess(access)
		},
		Success: apiCommon.Respond,
		InvalidRequest: func(w http.ResponseWriter, r *http.Request, err error) {
			apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
		},
		ModuleFailure: func(w http.ResponseWriter, r *http.Request, err error, internalMessage string) {
			renderMealPlanFailure(w, r, err, internalMessage)
		},
		ExportDailyList: func(list mealplanModule.DailyList, format string) (mealplanAPI.ExportFile, error) {
			return renderMealParticipationExport(renderer, list, format)
		},
	})
}

func renderMealParticipationExport(renderer SimpleListRenderer, list mealplanModule.DailyList, format string) (mealplanAPI.ExportFile, error) {
	export, err := list.Export()
	if err != nil {
		return mealplanAPI.ExportFile{}, err
	}
	columns := make([]SimpleListColumn, 0, len(export.Columns))
	for _, column := range export.Columns {
		columns = append(columns, SimpleListColumn{ID: column.ID, Label: column.Label})
	}
	file, err := renderer(SimpleListDocument{
		Title: export.Title, Subtitle: export.Subtitle, GeneratedAt: time.Now(), Filters: export.Filters,
		Columns: columns, Rows: export.Lines, Footer: export.Footer,
	}, format, export.Filename)
	if err != nil {
		return mealplanAPI.ExportFile{}, err
	}
	return mealplanAPI.ExportFile{Data: file.Data, ContentType: file.ContentType, Filename: file.Filename}, nil
}

func newUsersResource(module peopleModule.Capability, accountEmails func(context.Context, []int64) (map[int64]string, error), tagExists func(context.Context, string) (bool, error)) *usersAPI.Resource {
	return usersAPI.NewResource(module, usersAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, usersAPI.Middleware)) {
			ProtectedTenantRoutes(router, register)
		},
		Permission: func(permission string) usersAPI.Middleware {
			return apiCommon.RequiresPermission(permission)
		},
		ParsePagination: apiCommon.ParsePagination,
		Success:         apiCommon.Respond,
		SuccessPaginated: func(w http.ResponseWriter, r *http.Request, status int, data any, pagination usersAPI.Pagination, message string) {
			apiCommon.RespondPaginated(w, r, status, data, apiCommon.PaginationParams{Page: pagination.Page, PageSize: pagination.PageSize, Total: pagination.Total}, message)
		},
		NoContent: apiCommon.RespondNoContent,
		Failure: func(w http.ResponseWriter, r *http.Request, kind usersAPI.FailureKind, err error) {
			switch kind {
			case usersAPI.FailureInvalidRequest:
				apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
			case usersAPI.FailureNotFound:
				apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
			case usersAPI.FailureConflict:
				apiCommon.RenderError(w, r, apiCommon.ErrorConflict(err))
			default:
				apiCommon.RenderError(w, r, apiCommon.ErrorInternalServerWrap("Internal server error", err))
			}
		},
		AccountEmails: accountEmails,
		TagExists:     tagExists,
		ObserveResponse: func(status int, code string) {
			observability.ObservePeopleDirectoryHTTPResponse(status, code)
		},
	})
}

func newFeedbackResource(module *feedbackModule.Module) *feedbackAPI.Resource {
	return feedbackAPI.NewResource(module, feedbackAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, feedbackAPI.Middleware)) {
			ProtectedTenantRoutes(router, register)
		},
		Permission: func(permission string) feedbackAPI.Middleware {
			return apiCommon.RequiresPermission(permission)
		},
		Success: apiCommon.Respond,
		Failure: func(w http.ResponseWriter, r *http.Request, failure feedbackAPI.Failure) {
			response := &apiCommon.ErrResponse{Err: failure.Err, HTTPStatusCode: failure.Status, Detail: failure.Err.Error()}
			if errors.Is(failure.Err, feedbackAPI.ErrFeatureDisabled) {
				response.Code = apiCommon.CodeFeedbackDisabled
			}
			apiCommon.RenderError(w, r, response)
		},
		ObserveResponse: func(status int, code string) {
			observability.ObserveFeedbackHTTPResponse("staff", status, code)
		},
	})
}

// serveGraph is the composed Serve root: its router, the process-scoped pool
// and the retained composition the embedded Worker still reads until the
// service Factory is deleted (#2747). Each route resource is mounted where it
// is built (#2745); only the two document sweeps the Worker runs outlive their
// mount.
type serveGraph struct {
	router             chi.Router
	db                 transactionruntime.Database
	services           *serviceGraph
	metrics            *httpMetrics
	tracer             *observability.Tracer
	metricsBearerToken string
	feedback           *feedbackModule.Module
	documents          documentSweeps
}

// documentSweeps are the Worker's file sweeps that route resources serve.
type documentSweeps struct {
	staff   schedulerPorts.StaffDocumentFileCleaner
	student schedulerPorts.StudentDocumentFileCleaner
}

// routeInputs is what the route composition reads. It hands back none of the
// resources it builds.
type routeInputs struct {
	modules        moduleServices
	db             transactionruntime.Database
	logger         *slog.Logger
	sessionAuth    *sessiontoken.TokenAuth
	frontendURL    string
	errorReportDSN string
	metricsToken   string
}

type apiBuildResources struct {
	db       transactionruntime.Database
	tracker  analytics.Tracker
	released bool
}

func (resources *apiBuildResources) close() error {
	if resources.released {
		return nil
	}
	var err error
	if resources.tracker != nil {
		err = resources.tracker.Close()
	}
	return errors.Join(err, resources.db.Close())
}

// newServeGraph composes the production HTTP graph of one Serve root, the
// Worker's retained reads over the same owner modules and the product
// analytics tracker the runtime closes on shutdown.
func newServeGraph(config ServeConfig) (result *serveGraph, reads workerReads, tracker analytics.Tracker, resultErr error) {
	logger := config.Logger
	metricsBearerToken, err := observability.MetricsBearerTokenFromEnv(os.Getenv)
	if err != nil {
		return nil, workerReads{}, nil, err
	}

	// Get database connection as phoenix_auth (least-privilege for serve)
	db, err := transactionruntime.OpenServeDatabase()
	if err != nil {
		return nil, workerReads{}, nil, err
	}
	buildResources := apiBuildResources{db: db}
	defer func() {
		resultErr = errors.Join(resultErr, buildResources.close())
	}()
	tenantRuntime := db.UnitOfWork()

	// Fail fast on a partially migrated schema: the student repository selects
	// and writes its departure columns unconditionally (no per-request
	// information_schema probes, #2059), so the server must only start against
	// a fully migrated database.
	if err := peopleCompose.VerifyRetainedStudentSchema(context.Background(), db.Pool()); err != nil {
		return nil, workerReads{}, nil, err
	}

	if viper.GetBool("db_debug") {
		db.LogQueries(logger.With("component", "database"))
	}

	tracker, err = newProductAnalytics(logger)
	if err != nil {
		return nil, workerReads{}, nil, err
	}
	buildResources.tracker = tracker

	// Compose one authoritative instance of each migrated module.
	modules, err := initializeModuleServices(db, config.PublicAPIURL, logger, tracker)
	if err != nil {
		return nil, workerReads{}, nil, err
	}
	retained := modules.services
	if err := retained.bindTenantRuntime(tenantRuntime); err != nil {
		return nil, workerReads{}, nil, err
	}
	// Identity & Access is composed by the root (#4006), so the root binds
	// its unit of work as it does for the retained services.
	if err := modules.identity.SetTenantRuntime(tenantRuntime); err != nil {
		return nil, workerReads{}, nil, err
	}
	retained.SetSettingsObservers(
		observability.ObserveSettingsLookup,
		observability.RecordSettingsSideEffectFailure,
	)
	registerRuntimeStatsProviders(db, modules)

	httpMetrics := newHTTPMetrics()
	tracer := newRuntimeTracer(logger)
	graph := &serveGraph{
		router:             chi.NewRouter(),
		db:                 db,
		services:           retained,
		metrics:            httpMetrics,
		tracer:             tracer,
		metricsBearerToken: metricsBearerToken,
		feedback:           modules.feedback,
	}

	// Setup router middleware
	graph.router.Use(requestIDs(tracer))
	graph.router.Use(apiCommon.ProblemResponseMiddleware)
	// chi answers an unknown path with a plain-text body; the shared envelope
	// replaces it, so ProblemResponseMiddleware only repairs real handler
	// regressions. chi's 405 has no body and keeps its Allow header.
	graph.router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		apiCommon.RenderStatus(w, r, http.StatusNotFound, "")
	})
	graph.router.Use(apiCommon.TenantRuntimeMiddleware(tenantRuntime))
	graph.router.Use(AuthorizationObserverMiddleware(recordAuthorizationEvent))
	graph.router.Use(apiCommon.TenantRuntimeObserverMiddleware(func(observation apiCommon.TenantRuntimeObservation) {
		recordHTTPRuntimeEvent(tracer, observation)
	}))
	graph.router.Use(apiCommon.TenantRequestObserverMiddleware(func(event apiCommon.TenantRequestEvent) {
		observability.ObserveTenantRequest(
			event.TenantID,
			event.Scope,
			event.Request.Method,
			apiCommon.RoutePattern(event.Request),
			event.Status,
			event.Duration,
			event.Outcome,
		)
	}))
	setupBasicMiddleware(graph.router, logger, httpMetrics)

	// Setup CORS, security logging, and rate limiting
	setupCORSIfEnabled(graph.router, config.EnableCORS)
	securityLogger := setupSecurityLogging(graph.router)
	sessionAuth, err := newSessionTokenAuth()
	if err != nil {
		return nil, workerReads{}, nil, err
	}
	setupRateLimiting(graph.router, securityLogger, sessionAuth, demoLoopbackExempt())
	// One verifier serves every route: it only parses the presented token, and
	// each protected group still rejects through the Authenticator and its
	// scope gate. A group mounted without it fails closed.
	graph.router.Use(sessionAuth.Verifier())
	// Sentry events name the session they affect (#3643).
	graph.router.Use(SentrySessionContext)
	// Core actions of the portals reach the usage analytics once their
	// response is 2xx (#3602). After the verifier, which names the session.
	graph.router.Use(coreActionAnalytics(tracker, sessionAuth, settingsCompose.NewAnalyseFreigabe(retained.Settings, logger)))

	documents, err := mountRoutes(graph.router, routeInputs{
		modules:        modules,
		db:             db,
		logger:         logger,
		sessionAuth:    sessionAuth,
		frontendURL:    config.FrontendURL,
		errorReportDSN: config.SentryPyrePortalDSN,
		metricsToken:   metricsBearerToken,
	}, authRateLimitersFromEnv())
	if err != nil {
		return nil, workerReads{}, nil, err
	}
	graph.documents = documents

	buildResources.released = true
	return graph, newWorkerReads(modules), tracker, nil
}

// registerRuntimeStatsProviders hands the pool, the live streams and the PWA
// usage to the metrics endpoint.
func registerRuntimeStatsProviders(db transactionruntime.Database, modules moduleServices) {
	retained := modules.services
	observability.RegisterDBStatsProvider(func() observability.DBStats {
		stats := db.Capacity()
		return observability.DBStats{
			OpenConnections:   stats.OpenConnections,
			InUse:             stats.InUse,
			Idle:              stats.Idle,
			WaitCount:         stats.WaitCount,
			WaitDuration:      stats.WaitDuration,
			MaxIdleClosed:     stats.MaxIdleClosed,
			MaxLifetimeClosed: stats.MaxLifetimeClosed,
		}
	})
	observability.RegisterSSEStatsProvider(retained.RealtimeHub)
	observability.RegisterPWAUsageStatsProvider(observability.PWAUsageStatsProviderFunc(func() ([]observability.PWAUsageStat, error) {
		rows, err := retained.PWAUsage.SnapshotUsage()
		if err != nil {
			return nil, err
		}
		stats := make([]observability.PWAUsageStat, 0, len(rows))
		for _, row := range rows {
			stats = append(stats, observability.PWAUsageStat{
				TenantID:        row.TenantID,
				Portal:          row.Portal,
				StandaloneUsers: row.StandaloneUsers,
				EligibleUsers:   row.EligibleUsers,
			})
		}
		return stats, nil
	}))
}

// ServeHTTP routes a request through the composed graph. CalDAV extension
// methods reach their handler under a routable stand-in method.
func (graph *serveGraph) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = normalizeCalDAVMethodForRouting(r)
	if graph.services != nil && graph.services.Settings != nil {
		// A weekend that follows Friday's plan reads the tenant setting only
		// for Saturday and Sunday dates (#3921).
		r = WithWeekendPlan(r, graph.services.Settings)
	}
	graph.router.ServeHTTP(w, r)
}
