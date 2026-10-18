package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	devicefleetModule "github.com/moto-nrw/project-phoenix/modules/devicefleet"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
)

// serviceGraphInputs are the owners the root composed, the wiring it built
// over them and the process configuration the root's retained services are
// composed over (#2747, ADR 0053). The serving root and the test graphs fill
// them.
type serviceGraphInputs struct {
	DB            transactionruntime.Database
	TenantRuntime tenant.UnitOfWork
	Logger        *slog.Logger
	Config        ServiceConfig
	// AllowAuditRootWrites lets the Audit trails append without a tenant
	// transaction; only test graphs set it.
	AllowAuditRootWrites bool
	// Clock pins the services' clock; nil uses the real one.
	Clock func() time.Time

	CarePlan      careplan.Capability
	Organizations organizationtenancy.Capability
	Persons       peopledirectory.Capability
	Groups        schoolstructure.Capability
	Rooms         facilitiesModule.Capability
	Membership    schoolmembership.Capability
	Calendar      schoolcalendar.Calendar
	Timetable     timetable.Capability
	Appointments  appointments.Capability
	WorkTime      workforceModule.Capability

	MealPlan             parentportalcompose.MealPlanProvider
	BindMealPlanSettings MealPlanSettingsBinder
	FeedbackCounter      studentdeletioncompose.Feedback
	BindFeedbackSettings FeedbackSettingsBinder
	ProductEvents        ProductEventTracker

	ObserveCommunication   func(communicationCompose.Observation)
	ObserveAuditAppend     AuditAppendObserver
	ObserveDelivery        DeliveryObserver
	ObserveDurableDelivery DurableDeliveryObserver
	ObserveDeviceFleet     DeviceFleetObserver
	ObserveIdentityAccess  IdentityAccessObserver

	// StudentPhotoSlot is the runtime slot the People Directory resolves the
	// photo lifecycle's surfaces from, and PhotoUnlinker removes the stored
	// files; without a slot the graph composes no photo lifecycle.
	StudentPhotoSlot *peopleCompose.StudentPhotoRuntime
	PhotoUnlinker    peoplePorts.PhotoUnlinker

	Workforce       WorkforceWiring
	FileStorage     FileStorageWiring
	Identity        IdentityRoot
	People          PeopleWiring
	Platform        PlatformWiring
	Presence        PresenceWiring
	TimetableWiring TimetableWiring
	CarePlanWiring  CarePlanWiring
	Enrollment      EnrollmentWiring
}

// newServeServiceGraph composes the serving root's retained services. Every
// owner, observer and wiring is required, and the process configuration is
// read here.
func newServeServiceGraph(in serviceGraphInputs, publicAPIURL string) (*serviceGraph, error) {
	if err := in.validateServe(); err != nil {
		return nil, err
	}
	communicationCompose.InstallMessageQueryInstrumentation(in.DB.Pool())
	in.Config = CurrentServiceConfig()
	in.Config.PublicAPIURL = publicAPIURL
	in.AllowAuditRootWrites = false
	return newServiceGraph(in)
}

// validateServe refuses a serving graph with a missing owner, observer or
// wiring; test graphs may leave the optional ones out.
func (in serviceGraphInputs) validateServe() error {
	for _, validate := range []func() error{in.People.validate, in.Presence.validate, in.TimetableWiring.validate, in.CarePlanWiring.validate} {
		if err := validate(); err != nil {
			return err
		}
	}
	if in.Identity.Compose == nil || in.Identity.Permissions == nil || in.Identity.ActorScope == nil || in.Identity.AuditActor == nil {
		return errors.New("the Identity & Access composition and the request reads are required")
	}
	if in.CarePlan == nil || in.Organizations == nil || in.Persons == nil || in.Groups == nil || in.Rooms == nil || in.Membership == nil || in.Calendar == nil || in.Timetable == nil || in.Appointments == nil || in.ObserveCommunication == nil || in.MealPlan == nil || in.BindMealPlanSettings == nil || in.FeedbackCounter == nil || in.BindFeedbackSettings == nil || in.ObserveAuditAppend == nil || in.ObserveDelivery == nil || in.ObserveDurableDelivery == nil || in.ObserveDeviceFleet == nil || in.ObserveIdentityAccess == nil || in.WorkTime == nil || in.FileStorage.Objects == nil || in.FileStorage.Observe == nil || in.FileStorage.Settings == nil || in.FileStorage.Events == nil {
		return errors.New("organization tenancy, people directory, school structure, facilities, school membership, school calendar, timetable, appointments, care plan, meal plan, feedback, Communication, Audit, Delivery, Identity & Access, Workforce and File Storage capabilities with their binders and observers are required")
	}
	return nil
}

// newServiceGraph composes the root's retained services over the owners and
// the wiring the inputs carry. Each stage binds one group of services over
// what the earlier stages built; a value an earlier service needs from a
// later one is resolved at call time.
func newServiceGraph(in serviceGraphInputs) (*serviceGraph, error) {
	if !in.Workforce.complete() {
		return nil, errors.New("the root's Workforce wiring is required")
	}
	if err := in.Platform.validate(); err != nil {
		return nil, err
	}
	if err := in.Enrollment.validate(); err != nil {
		return nil, err
	}
	b := graphBuild{serviceGraphInputs: in, late: &graphLate{}}
	b.graph.settingsRuntimeDB = in.DB
	stages := []func(graphBuild) (graphBuild, error){
		composeFoundation, composeMail, composeSettings, composeIdentity, composePeople, composeStudentPhotos,
		composeTimeTracking, composeCareDays, composePresence, composeActivities,
		composeShiftPlanning, composeCareSchedules, composeMaterialization, composeLifecycle,
		composeCleanups, composeOperations, composeDelivery, composeCaregivers,
		composeCareLifecycle, composeEnrollmentDecisions, composeEnrollmentRequests,
		composeEnrollmentChanges, composeCareRequests, composeNotifications,
		composeMessaging, composeParentPortal, composeFileStore, composeViews,
		composeProjections, composePlanning, composeParentRequests, composeSettingsWrites,
		composeLateWorkflows,
	}
	var err error
	for _, stage := range stages {
		if b, err = stage(b); err != nil {
			return nil, err
		}
	}
	graph := b.graph
	return &graph, nil
}

// graphBuild carries one composition of the service graph: the inputs, the
// graph it fills and the values the stages hand on that the graph does not
// hold. Each stage takes the build so far and returns it extended instead of
// setting fields on the graph through a receiver. The binding calls the
// Factory made on composed services (the decision notices of the parent event
// emitter, the attachment purger, the care participation, the guardian
// provider, the class-list audit, the settings guards) stay where they were.
// late holds the values a service composed early reads at call time although
// a later stage binds them.
type graphBuild struct {
	serviceGraphInputs
	graph serviceGraph
	late  *graphLate

	now      func() time.Time
	today    func() timezone.Date
	settings settingsCompose.TenantSettingsRuntime
	// settingsService is the graph's Settings with the batch reads the
	// projections prefetch through.
	settingsService settingsCompose.BatchSettingsService

	mailer             email.Mailer
	dispatcher         *email.Dispatcher
	audits             *auditCompose.Persistence
	tenantMailIdentity email.ReplyToResolver
	frontendURL        string
	publicAPIURL       string
	parentsURL         string
	schoolURL          string
	settingsChanged    func(context.Context, int64, string)

	groupSupervisions  studentpresence.SupervisionRecords
	groupSubstitutions GroupSubstitutionRepository
	rosterReconciler   timetablePorts.RosterReconciler
	templateRows       arrivalTimetable.TemplateRows
	timetableRows      arrivalTimetable.OwnerRows

	identity       IdentityComposition
	identityAccess *identityaccess.Module
	userContext    CallerRowReads
	callerContext  identityaccess.CallerContext

	pickupBaselines   careplan.PickupBaselineReader
	arrivalBaselines  careplan.ArrivalBaselineReader
	careOfferings     careplan.CareOfferingCatalogCapability
	pickupAutoExcusal careplan.PickupAutoExcusal
	vapid             deliveryCompose.VAPIDConfig

	timetableConflicts timetable.ConflictDetectionCapability
	instances          timetablePorts.InstanceLifecycle
	staffDeviations    timetable.StaffDeviations
	shiftPlanning      *workforceCompose.ShiftPlanning
	deviceFleet        *devicefleetModule.Module

	enrollmentNotifications enrollmentOwner.Notifications
	enrollmentPhases        *enrollmentCompose.Phases
	enrollmentCareOfferings *enrollmentCompose.CareOfferingRecords
	enrollmentDecisions     *enrollmentCompose.Decisions
	enrollmentCapacity      enrollmentOwner.OfferingCapacity
	notificationConsent     communication.NotificationConsentCapability
	reviewPolicy            CarePlanReviewPolicy
	requestShares           shareVisibilityFunc
	offeringChanges         careplan.OfferingChangeCapability
	excusedRequests         *careplanCompose.ExcusedAbsenceRequests
	staffParentMessages     deliveryCompose.StaffParentMessageNotifier
}

// graphLate are the values a later stage binds and the services composed
// before it read on every call: Delivery's outbox and state for Identity &
// Access, Care Plan's participation for the person directory, the sick
// cascade for time tracking, the booking materialization for the offering
// catalog and the phase administration, the cancellation notice for the
// instance lifecycle and the parents portal's sharing rules for the request
// queues.
type graphLate struct {
	emailOutbox             *deliveryCompose.EmailOutbox
	delivery                *deliveryCompose.Runtime
	careParticipation       careplan.CareParticipation
	shiftPlanSyncer         shiftplansync.SickCascade
	careBookings            careplan.BookingMaterializationCapability
	guardianNoticePublisher communication.CareCancellationPublisher
	phaseSourcedTemplates   enrollmentOwner.SourcedTemplateResyncer
	requestShareVisibility  communicationCompose.ParentShareVisibilityResolver
}

// composeFoundation binds the clocks, the shared rows and the settings
// runtime every later stage reads.
func composeFoundation(b graphBuild) (graphBuild, error) {
	b.now = b.Clock
	b.today = timezone.CalendarDateClock(b.now)
	b.groupSubstitutions = NewGroupSubstitutionRepository(b.People.Groups, b.Membership, b.WorkTime)
	b.groupSupervisions = b.People.SupervisionRecords(b.Presence.SessionRecords)
	b.settings = settingsCompose.NewTenantSettingsRuntime(b.DB.Pool(), nil)
	repos := b.TimetableWiring.Repositories
	calendarPeriods := arrivalTimetable.NewCalendarPeriodRepository(b.Calendar, arrivalTimetable.NewPlanningCalendarPeriodUsage(b.Enrollment.Bookings.Module, b.Timetable))
	b.templateRows = arrivalTimetable.TemplateRows{
		Groups: repos.ActivityGroup, Categories: repos.ActivityCategory, Schedules: repos.ActivitySchedule,
		Enrollments: repos.StudentEnrollment, Supervisors: repos.ActivitySupervisor, Instances: repos.ActivityInstance,
		InstanceStaff: repos.InstanceStaff, Participants: repos.InstanceStudent, Timeframes: repos.Timeframe,
		CalendarPeriods: calendarPeriods, Exceptions: repos.ActivityException, EducationGroups: b.People.Groups,
	}
	// Reconciles already-materialized future timetable rosters when a grade
	// transition graduates or restores students (#405).
	b.rosterReconciler = arrivalTimetable.NewRosterReconciler(repos.ActivityInstance, repos.InstanceStudent, repos.StudentEnrollment, b.Logger, b.now)
	return b, nil
}

// composeMail binds the mail transport, the Audit trails and the portal
// links the configuration names.
func composeMail(b graphBuild) (graphBuild, error) {
	cfg := b.Config
	mailer, err := email.NewMailer(email.MailerConfig{
		Host: cfg.EmailSMTPHost, Port: cfg.EmailSMTPPort, User: cfg.EmailSMTPUser, Password: cfg.EmailSMTPPassword,
		DefaultFrom: email.NewEmail(cfg.EmailFromName, cfg.EmailFromAddress), TemplateDir: "./templates",
		Logger: b.Logger.With("service", "email"), AppEnv: cfg.AppEnv,
	})
	if err != nil {
		return b, fmt.Errorf("initialize email transport: %w", err)
	}
	if _, ok := mailer.(*email.MockMailer); ok {
		b.Logger.Warn("SMTP mailer not configured; using mock mailer (tokens will not be sent via SMTP)")
	}
	b.mailer = mailer
	b.graph.Mailer = mailer
	if b, err = composeAudit(b); err != nil {
		return b, err
	}
	b.dispatcher = email.NewDispatcher(mailer, b.Logger.With("component", "email"), email.DeliveryObserver(b.ObserveDelivery))
	b.graph.DefaultFrom = email.NewEmail(deliveryModule.SenderOrDefault(cfg.EmailFromName, cfg.EmailFromAddress))
	if b, err = resolveLinks(b); err != nil {
		return b, err
	}
	b.graph.InvitationTokenExpiry = identityaccess.InvitationTokenExpiry(cfg.InvitationTokenExpiryHours)
	b.graph.PasswordResetTokenExpiry = identityaccess.PasswordResetTokenExpiry(cfg.PasswordResetExpiryMinutes)
	return b, nil
}

// composeAudit binds every retained Audit query adapter to the caller's
// transaction, tenant and read-only root DB fallback, and the append store
// to the authoritative transaction (ADR 0049); the composition reads the
// tenant runtime's ambient transaction itself (#4013). Writes are routed
// through the fail-closed command before services capture them.
func composeAudit(b graphBuild) (graphBuild, error) {
	audits, err := auditCompose.NewTenantPersistence(b.DB.Pool(), b.AllowAuditRootWrites)
	if err != nil {
		return b, err
	}
	command, err := auditCompose.NewObservedCommand(audits.AppendStore, b.Logger.With("component", "audit-command"), b.ObserveAuditAppend)
	if err != nil {
		return b, err
	}
	b.audits = audits.WithRoutedWrites(command)
	b.graph.Audit = command
	if err := b.Presence.BindClassListEntries(command); err != nil {
		return b, err
	}
	b.graph.StudentConsents = b.People.StudentConsents(b.audits.StudentConsentChange)
	return b, nil
}

// resolveLinks validates the staff, API, parents-portal and school-portal
// bases the mails and the CalDAV feed link to: production requires https.
func resolveLinks(b graphBuild) (graphBuild, error) {
	production := strings.ToLower(b.Config.AppEnv) == "production"
	links := []struct {
		name   string
		raw    string
		target *string
	}{
		{"FRONTEND_URL", b.Config.FrontendURL, &b.frontendURL},
		{"NEXT_PUBLIC_API_URL", b.Config.PublicAPIURL, &b.publicAPIURL},
		// Every parent-facing email link (status, decision emails,
		// guardian invitation accept).
		{"PARENTS_URL", b.Config.ParentsURL, &b.parentsURL},
		// Lehrkraft invitation links land on the school portal, where the
		// accept flow lives (#2207).
		{"SCHOOL_URL", b.Config.SchoolURL, &b.schoolURL},
	}
	for _, link := range links {
		value, err := requiredLink(link.name, link.raw, production)
		if err != nil {
			return b, err
		}
		*link.target = value
	}
	b.graph.FrontendURL = b.frontendURL
	return b, nil
}

// requiredLink trims a configured base URL and refuses an empty one, and in
// production one without https.
func requiredLink(name, raw string, production bool) (string, error) {
	value := strings.TrimRight(raw, "/")
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if production && !strings.HasPrefix(value, "https://") {
		return "", fmt.Errorf("%s must use https:// in production (received %q)", name, raw)
	}
	return value, nil
}

// composeSettings binds the realtime hub, the group management, the
// settings service with its binders and the tenant mail identity.
func composeSettings(b graphBuild) (graphBuild, error) {
	hub := deliveryCompose.NewRealtimeHub(b.Logger.With("component", "sse-hub"))
	b.graph.RealtimeHub = hub
	b.graph.Education = education.NewGroupManagement(
		b.People.Groups, b.People.GroupTeachers, b.People.ClassTeachers, b.People.GroupRooms, b.People.GroupTeacherDirectory,
		b.People.StaffChecks, b.People.Students, b.groupSubstitutions, education.NewLegacyRepositoryRuntime(b.DB.Pool()),
		education.GroupServiceOptions{Broadcaster: NewGroupEvents(hub, nil), Audit: b.People.ClassAssignmentAudit(b.audits.StaffMasterDataChange)},
	)
	// Start page composition (#2875) shares the settings tenant runtime: the
	// personal layout and the school's prescription are read inside the same
	// tenant transaction as every other tenant-scoped setting.
	b.settingsChanged = func(_ context.Context, tenantID int64, key string) {
		event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key})
		_ = hub.BroadcastToTenant(tenantID, event)
	}
	homeLayouts := settingsCompose.NewHomeLayoutService(settingsCompose.NewHomeLayoutRepository(b.settings), b.settings, b.Logger, b.settingsChanged)
	settings := settingsCompose.NewSettingsServiceWithHomeLayouts(
		settingsCompose.NewSettingValueRepository(b.settings), settingsCompose.NewSettingAuditRepository(b.settings),
		b.Platform.SchoolSettings, b.settings, b.Logger, homeLayouts,
	)
	b.graph.Settings, b.settingsService = settings, settings
	b.bindOwnerSettings(settings)
	// Replies to tenant-bound mail belong to the OGS, not to moto (#1936).
	// Built once here and shared by every tenant-bound sender.
	b.tenantMailIdentity = deliveryCompose.NewTenantMailIdentity(b.People.SchoolContacts, func(ctx context.Context, tenantID int64) (string, error) {
		return settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, b.Logger)
	return b, nil
}

// bindOwnerSettings hands the Meal Plan, Feedback and Enrollment owners the
// settings they resolve per call.
func (b *graphBuild) bindOwnerSettings(settings settingsCompose.BatchSettingsService) {
	if b.MealPlan != nil {
		b.BindMealPlanSettings(
			func(ctx context.Context) (bool, error) {
				return settings.ResolveBool(ctx, configModels.KeyMealPlanEnabled)
			},
			func(ctx context.Context) (bool, error) {
				return settings.ResolveBool(ctx, configModels.KeyMealRegistrationEnabled)
			},
			func(ctx context.Context) (string, error) {
				return settings.ResolveString(ctx, configModels.KeyMealRegistrationCutoffTime)
			},
		)
	}
	if b.BindFeedbackSettings != nil {
		b.BindFeedbackSettings(
			func(ctx context.Context) (bool, error) {
				return settings.ResolveBool(ctx, configModels.KeyFeedbackEnabled)
			},
			func(ctx context.Context) (int, error) {
				return settings.ResolveInt(ctx, configModels.KeyFeedbackDataRetentionDays)
			},
		)
	}
	// Enrollment's active phase restrictions guard the collection settings
	// (#1663).
	b.Enrollment.BindSettingsGuards(settings)
}

// composeIdentity hands the root's Identity & Access composition the values
// this graph built (#4006): sessions, the account lifecycle, the second
// factor, the operator flows and the caller context. Delivery is composed
// later, so the guardian and staff mails read its outbox and its delivery
// state at call time.
func composeIdentity(b graphBuild) (graphBuild, error) {
	staffGroups, err := education.NewStaffGroups(education.StaffGroupsDependencies{
		Groups: b.Groups, Assignments: b.Membership, Substitutions: b.WorkTime,
	})
	if err != nil {
		return b, err
	}
	people := b.People
	identity, err := b.Identity.Compose(IdentityMaterial{
		Config: b.Config, Logger: b.Logger, Observe: b.ObserveIdentityAccess,
		Organizations: b.Organizations, Persons: b.Persons, Membership: b.Membership,
		Settings: b.settingsService, Audit: b.graph.Audit, AuthEvents: b.audits.AuthEvent,
		OperatorAuditLog: b.Platform.OperatorAuditLog, PushSubscriptions: deliveryCompose.NewPushSubscriptionRepository(b.DB.Pool()),
		Dispatcher: b.dispatcher, DefaultFrom: b.graph.DefaultFrom, MailIdentity: b.tenantMailIdentity,
		FrontendURL: b.frontendURL, ParentsURL: b.parentsURL, SchoolURL: b.schoolURL,
		InvitationExpiry: b.graph.InvitationTokenExpiry, PasswordResetExpiry: b.graph.PasswordResetTokenExpiry,
		Outbox:      func() MailResultOutbox { return b.outbox() },
		EmailStatus: b.late.emailStatus,
		PersonRows:  people.Persons, StaffRows: people.Staff, TeacherRows: people.Teachers, StudentRows: people.Students,
		GuardianProfiles: people.GuardianProfiles, StudentGuardians: people.StudentGuardians,
		GuardianEnrollments: b.Platform.ParentPortal.ParentEnrollmentRequest,
		Groups:              people.Groups, StaffGroups: staffGroups,
		ActivityGroups: arrivalTimetable.NewCallerActivityGroups(b.TimetableWiring.Repositories.ActivityGroup),
		Sessions:       b.Presence.SessionRecords, Presence: b.Presence.Presence,
	})
	if err != nil {
		return b, err
	}
	b.identity = identity
	b.identityAccess = identity.Access
	b.userContext = identity.Caller
	b.callerContext = identity.Caller.Caller()
	return b, nil
}

// emailStatus reads a mail's delivery state from Delivery, which is composed
// after Identity & Access.
func (late *graphLate) emailStatus(ctx context.Context, tenantID, id int64) (deliveryModule.State, bool, error) {
	if late.delivery == nil {
		return "", false, errors.New("delivery module is not initialized")
	}
	return late.delivery.EmailStatus(ctx, tenantID, id)
}

// composePeople binds the person directory, the personnel record and the
// guardian service over the People Directory, School Membership and
// Workforce.
func composePeople(b graphBuild) (graphBuild, error) {
	people := b.People
	// The staff and teacher lookups and the staff writes are School
	// Membership and Workforce territory (#3752); the person service takes
	// them through the StaffDirectory port.
	staffDirectory := peopleCompose.NewStaffDirectory(peoplePorts.StaffDirectoryDependencies{
		Runtime: peopleCompose.NewLegacyRepositoryRuntime(b.DB.Pool()), Persons: people.Persons, Staff: people.Staff,
		Teachers: people.Teachers, LehrkraftRoles: b.identityAccess, MayManageStaff: staffAdministrationPermitted,
	})
	// Care Plan is composed later and reads the person directory, so the
	// directory reaches its participation decision at call time.
	b.graph.Users = peopleCompose.NewPersonDirectory(peoplePorts.PersonDirectoryDependencies{
		PersonDirectory:  peopleCompose.NewPersonWrites(b.Persons),
		StudentDirectory: peopleCompose.NewStudentDirectory(b.Persons),
		PersonRepo:       people.Persons, RFIDRepo: people.Identity,
		AccountExists: identityaccessCompose.AccountExists(people.Identity),
		StudentRepo:   people.Students, TeacherRepo: people.Teachers, StaffDirectory: staffDirectory,
		CareParticipation: b.CarePlanWiring.CareParticipation(func() careplan.CareParticipation {
			return b.late.careParticipation
		}),
	})
	// Personnel record (#1417, #1423, #1424): the payroll number, the
	// Stammdaten sections and the Dokumente tab are Workforce's own
	// administration; the staff and person rows and the trail are bound here.
	staffAdmin, err := b.Workforce.StaffAdmin(WorkforceStaffAdminSources{
		Staff: people.Staff, Persons: people.Persons, Audits: b.audits,
		Logger: b.Logger.With("service", "staff_documents"), Now: b.now,
	})
	if err != nil {
		return b, fmt.Errorf("staff admin: %w", err)
	}
	b.graph.StaffDocuments = staffAdmin
	b.graph.Guardian = peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{
		GuardianProfileRepo: people.GuardianProfiles, GuardianPhoneNumberRepo: peopleCompose.NewGuardianPhoneNumberRepository(b.DB.Pool()),
		StudentGuardianRepo: people.StudentGuardians, GuardianInvitations: b.identity.GuardianInvitations,
		StudentRepo: people.Students, PersonRepo: people.Persons,
		GuardianFinancialRepo:  peopleCompose.NewGuardianFinancialDataRepository(b.DB.Pool()),
		GuardianFinancialAudit: people.GuardianFinancialAudit(b.audits.GuardianFinancialChange),
		DataAccessLog:          people.GuardianDataAccessLog(b.audits.DataAccessLog),
		RolePresets: peoplePorts.GuardianRolePresetFuncs{
			Default: securityruntime.DefaultStudentGuardianRole, Presets: securityruntime.StudentGuardianRolePreset,
		}}, b.DB.Pool())
	b.graph.PeopleDirectory = b.Persons
	return b, nil
}

// composeStudentPhotos binds the photo lifecycle over the People Directory,
// the settings, the realtime hub and the consents (#4007); the settings
// writes register its feature toggle.
func composeStudentPhotos(b graphBuild) (graphBuild, error) {
	if b.StudentPhotoSlot == nil {
		return b, nil
	}
	b.graph.StudentPhotos = NewStudentPhotos(b.Persons, b.StudentPhotoSlot, StudentPhotoRuntimeDependencies{
		Settings: b.settingsService, Broadcaster: b.graph.RealtimeHub, Unlinker: b.PhotoUnlinker,
		Consents: b.graph.StudentConsents, Logger: b.Logger.With("service", "student-photo"), ActingAccount: actingAccountID,
	})
	return b, nil
}

// composeTimeTracking binds the payroll status and the time-tracking side of
// Workforce (#3413): stamps, history and edits, absences, month balances and
// their close, overview, exports and the audit feed.
func composeTimeTracking(b graphBuild) (graphBuild, error) {
	staff := b.People.Staff
	// One payroll-status instance: the /payroll page and the DATEV writers
	// must judge completeness identically.
	b.graph.PayrollStatus = settingsCompose.NewPayrollStatusService(b.settingsService, func(ctx context.Context) (int, int, error) {
		members, err := staff.List(ctx, nil)
		if err != nil {
			return 0, 0, err
		}
		numbers := make([]*string, len(members))
		for i, member := range members {
			numbers[i] = member.PersonnelNumber
		}
		return len(members), workforceModule.MissingPersonnelNumbers(numbers), nil
	})
	// The #1843 sick cascade is the shift-plan-sync workflow, which needs the
	// planning capability and the timetable services built long after the
	// absence service; the deferred binding resolves it on every call.
	timeTracking, err := b.Workforce.TimeTracking(WorkforceTimeTrackingSources{
		Audits: b.audits, Supervisions: b.groupSupervisions, SessionRecords: b.Presence.SessionRecords,
		Staff: staff, People: b.graph.Users, Holidays: schoolCalendarCompose.NonWorkingDays{Calendar: b.Calendar},
		Settings: b.settingsService, Payroll: b.graph.PayrollStatus, Broadcaster: b.graph.RealtimeHub,
		Mail:          WorkforceMail{Dispatcher: b.dispatcher, From: b.graph.DefaultFrom, ReplyTo: b.tenantMailIdentity, FrontendURL: b.frontendURL},
		ShiftPlanSync: shiftplansyncCompose.DeferredSickCascade(func() shiftplansync.SickCascade { return b.late.shiftPlanSyncer }),
		Logger:        b.Logger.With("service", "active"),
	})
	if err != nil {
		return b, fmt.Errorf("compose time tracking: %w", err)
	}
	b.graph.WorkSession = timeTracking
	return b, nil
}
