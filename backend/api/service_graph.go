package api

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"

	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
	"github.com/spf13/viper"
)

type SubstitutionActorResolver struct {
	Identity identityaccess.CallerIdentities
}

func (r SubstitutionActorResolver) ResolveActor(ctx context.Context) (*education.Actor, error) {
	staffID, err := r.Identity.StaffID(ctx)
	if err != nil {
		if expectedMissingSubstitutionIdentity(err) {
			return nil, nil
		}
		return nil, err
	}
	teacherID, err := r.Identity.TeacherID(ctx)
	if err != nil {
		if expectedMissingSubstitutionIdentity(err) {
			return nil, nil
		}
		return nil, err
	}
	return &education.Actor{StaffID: staffID, TeacherID: teacherID}, nil
}

func expectedMissingSubstitutionIdentity(err error) bool {
	return errors.Is(err, identityaccess.ErrCallerNotLinkedToPerson) ||
		errors.Is(err, identityaccess.ErrCallerNotLinkedToStaff) ||
		errors.Is(err, identityaccess.ErrCallerNotLinkedToTeacher)
}

// serviceGraph holds the retained services the root composes over its owners
// (#2747). The serving root mounts them; test graphs drive them by name.
type serviceGraph struct {
	// settingsRuntimeDB is the root database the settings runtime binds the
	// unit of work over.
	settingsRuntimeDB transactionruntime.Database
	Audit             *auditCompose.Command
	Active            studentpresence.Presence
	ActiveCleanup     studentpresence.PresenceCleanup
	// WorkSession is the composed time-tracking side of Workforce (#3413):
	// the public stamp, session, absence, month, ledger, month-close,
	// overview, audit-log and export contracts. It replaces the eight
	// retained service fields under the oldest of their names.
	WorkSession  *workforceCompose.TimeTracking
	Activities   arrivalTimetable.ActivityService
	Education    schoolstructure.GroupManagement
	Substitution education.SubstitutionModule
	// GradeTransition is the owner workflow behind the school-year rollover
	// (#2711): the admin HTTP surface calls exactly its public commands.
	GradeTransition *gradetransition.Workflow
	Schulhof        facilitiesCompose.SchulhofService
	WC              facilitiesCompose.WCService
	IoT             devicefleetCompose.IoTService
	Settings        settingsCompose.SettingsService
	TenantSettings  *settingsCompose.TenantOperations
	PayrollStatus   settingsCompose.PayrollStatusGetter
	// The Dienstplan side of Workforce (#3418): the planning capability the
	// staff-shift route mounts, the self-service assignments and the week
	// overview, and the shift-type administration, all public contracts.
	StaffShifts             workforceModule.StaffShiftPlanning
	StaffAssignments        workforceModule.StaffAssignmentQuery
	StaffScheduleOverview   workforceModule.StaffScheduleOverviewQuery
	ShiftTypes              workforceModule.ShiftTypeAdministration
	PickupSchedule          careplan.PickupScheduleService
	PartialAbsence          careplan.PartialAbsenceService
	ArrivalSchedule         careplan.ArrivalScheduleService
	CareDay                 careplan.CareDayQuery
	TimetableBridge         timetable.EndedSessionCompletion
	Materialization         timetable.MaterializationCapability
	TimetableCleanup        timetable.TimetableCleanup
	TimeTrackingCleanup     workforceCompose.TimeTrackingCleanupService
	StudentChangeLogCleanup peoplePorts.StudentChangeLogCleanupService
	Instance                timetable.InstanceLifecycleCapability
	AutoStart               timetable.InstanceAutoStart
	AutoEnd                 timetable.InstanceAutoEnd
	TimetableOperations     timetable.OperationCapability
	Users                   peoplePorts.PersonDirectory
	// StaffDocuments is Workforce's personnel-record administration: the
	// Dokumente tab, the Stammdaten sections and the payroll number (#3752).
	StaffDocuments           *workforceModule.StaffAdmin
	StudentDocuments         careplan.StudentDocuments
	FileStore                *filestorageModule.Module
	CaregiverCapability      peoplePorts.CaregiverCapabilityService
	Guardian                 peoplePorts.GuardianService
	PeopleDirectory          peopledirectory.Capability
	Emergency                emergencysnapshot.Query
	SlotLists                classday.SlotLists
	PlanExport               planexport.Service
	Reminders                reminder.Capability
	Notifications            deliveryCompose.Notifier
	PushSubscriptions        deliveryCompose.PushSubscriptionService
	PWAUsage                 deliveryCompose.PWAUsageService
	NotificationPreferences  deliveryCompose.NotificationPreferenceService
	AbsenceNotifier          deliveryCompose.AbsenceNotifier
	RealtimeHub              *realtime.Hub // SSE event hub (shared by services and API)
	Mailer                   email.Mailer
	DefaultFrom              email.Email
	FrontendURL              string
	InvitationTokenExpiry    time.Duration
	PasswordResetTokenExpiry time.Duration

	// Platform domain (operator dashboard)
	OperatorProvisioning organizationtenancy.Provisioning
	StudentDeletion      *studentdeletion.Workflow
	CareLifecycle        careplan.CareLifecycle
	StudentAudit         peoplePorts.StudentAuditService
	MasterDataReview     masterdatarequests.Decisions
	CareRequests         carerequests.Service
	ExcusedRequests      careplan.ExcusedAbsenceRequests
	ParentRequests       parentrequests.Coordinator
	AbsenceOverview      studentpresence.StatusDayOverviews
	StudentHistory       studentpresence.StudentHistory
	// Statistics is the Statistik report (#2606).
	Statistics              studentpresence.StatisticsReports
	OGSGroupLive            grouplive.Query
	SupervisionDashboard    supervisiondashboard.Query
	TimetableData           arrivalTimetable.Planning
	InstanceSeriesConverter timetable.InstanceSeriesConversion

	// Delivery owns the leased email and push outboxes; EmailOutboxWorker
	// drains both transports.
	EmailOutboxWorker *deliveryModule.Worker
	Delivery          *deliveryModule.Module

	// Enrollment domain (parent-enrollment PR 5+). The root builds the form
	// schemas, the captcha and the phase expiry warnings where it mounts
	// them (#4012).
	EnrollmentCareOffering careplan.CareOfferingCapability
	EnrollmentRequest      *enrollmentCompose.Intake
	EnrollmentPhase        enrollmentOwner.PhaseAdministration
	EnrollmentDecision     enrollmentOwner.Decisions
	EnrollmentReport       enrollmentOwner.Reports
	// ClassDayArrivalExceptions carries the school portal's whole class-day
	// capability: the day report, the supervision sheet and the arrival
	// exception write seam (#2970, #3563). The field keeps its name because
	// the composition surface only shrinks (#2747).
	ClassDayArrivalExceptions classday.ClassDay
	EnrollmentRollover        enrollmentOwner.Rollovers
	EnrollmentChangeRequest   enrollmentOwner.ChangeRequests
	EnrollmentDeletion        enrollmentOwner.EnrollmentDeletions
	EnrollmentRejectedCleanup enrollmentOwner.RejectedEnrollmentCleaner

	// Parent (cross-tenant guardian portal - PR 9)
	Parent *parentportal.Portal

	// Messaging (staff-side parent-OGS inbox / threads)
	Messaging communication.ParentMessagingCapability

	// StaffMessaging (OGS-internal colleague chat, #2598)
	StaffMessaging communication.StaffMessagingRuntime

	// ParentEventEmitter is the chat-pill + guardian-wake emitter (#1803/#1845).
	// Exposed so the API layer can wake a child's guardians (its message-
	// independent parent_child_updated SSE fan-out) after staff-side care writes,
	// so an open parents-app tab refetches the child's care state live (#1725).
	ParentEventEmitter *communicationCompose.ParentEventEmitter

	// Calendar (staff and parent personal calendars)
	Calendar            calendarCompose.Application
	CalendarFeedCleanup calendarService.FeedCleanupService

	// ParentAnnouncement (staff-side parent broadcast news authoring, #1669)
	ParentAnnouncement communication.ParentAnnouncementCapability

	// SettingsSideEffects is the per-key handler registry the settings
	// routes dispatch a value change to; Facilities, Care Plan and the photo
	// lifecycle register their handlers while the graph is composed.
	SettingsSideEffects *settingsCompose.SideEffectRegistry
	// StudentPhotos is the photo lifecycle; nil in a graph composed without
	// the photo slot and the file unlinker.
	StudentPhotos   peoplePorts.StudentPhotoService
	StudentConsents StudentConsentRecords
}

// SetSettingsObservers wires delivery-owned metrics without coupling the
// settings application layer to a metrics implementation.
func (f *serviceGraph) SetSettingsObservers(
	lookup settingsCompose.SettingsLookupObserver,
	sideEffectFailure settingsCompose.SideEffectFailureObserver,
) {
	if observable, ok := f.Settings.(interface {
		SetLookupObserver(settingsCompose.SettingsLookupObserver)
	}); ok {
		observable.SetLookupObserver(lookup)
	}
	if f.SettingsSideEffects != nil {
		f.SettingsSideEffects.SetFailureObserver(sideEffectFailure)
	}
}

type MealPlanSettingsBinder func(
	func(context.Context) (bool, error),
	func(context.Context) (bool, error),
	func(context.Context) (string, error),
)

type FeedbackSettingsBinder func(
	func(context.Context) (bool, error),
	func(context.Context) (int, error),
)

// ServiceConfig is the process configuration snapshot consumed by one service
// graph. Capturing it at the composition root keeps service construction pure
// and lets independent graphs coexist in the same process.
type ServiceConfig struct {
	EmailSMTPHost              string
	EmailSMTPPort              int
	EmailSMTPUser              string
	EmailSMTPPassword          string
	EmailFromName              string
	EmailFromAddress           string
	FrontendURL                string
	PublicAPIURL               string
	ParentsURL                 string
	SchoolURL                  string
	AppEnv                     string
	InvitationTokenExpiryHours int
	PasswordResetExpiryMinutes int
	PostHogAPIKey              string
	PostHogHost                string
	RateLimitEnabled           bool
	JWTSecret                  string
	JWTExpiry                  time.Duration
	JWTRefreshExpiry           time.Duration
	TenantDomain               string
	OperatorHostname           string
	VAPIDPublicKey             string
	VAPIDPrivateKey            string
	VAPIDSubscriber            string
	StudentDailyCheckoutTime   string
}

func CurrentServiceConfig() ServiceConfig {
	return ServiceConfig{
		EmailSMTPHost:              viper.GetString("email_smtp_host"),
		EmailSMTPPort:              viper.GetInt("email_smtp_port"),
		EmailSMTPUser:              viper.GetString("email_smtp_user"),
		EmailSMTPPassword:          viper.GetString("email_smtp_password"),
		EmailFromName:              viper.GetString("email_from_name"),
		EmailFromAddress:           viper.GetString("email_from_address"),
		FrontendURL:                viper.GetString("frontend_url"),
		PublicAPIURL:               viper.GetString("next_public_api_url"),
		ParentsURL:                 viper.GetString("parents_url"),
		SchoolURL:                  viper.GetString("school_url"),
		AppEnv:                     viper.GetString("app_env"),
		InvitationTokenExpiryHours: viper.GetInt("invitation_token_expiry_hours"),
		PasswordResetExpiryMinutes: viper.GetInt("password_reset_token_expiry_minutes"),
		PostHogAPIKey:              viper.GetString("posthog_api_key"),
		PostHogHost:                viper.GetString("posthog_host"),
		RateLimitEnabled:           viper.GetBool("rate_limit_enabled"),
		JWTSecret:                  viper.GetString("auth_jwt_secret"),
		JWTExpiry:                  viper.GetDuration("auth_jwt_expiry"),
		JWTRefreshExpiry:           viper.GetDuration("auth_jwt_refresh_expiry"),
		TenantDomain:               viper.GetString("tenant_domain"),
		OperatorHostname:           viper.GetString("next_public_operator_hostname"),
		VAPIDPublicKey:             viper.GetString("vapid_public_key"),
		VAPIDPrivateKey:            viper.GetString("vapid_private_key"),
		VAPIDSubscriber:            viper.GetString("vapid_subscriber"),
		StudentDailyCheckoutTime:   os.Getenv("STUDENT_DAILY_CHECKOUT_TIME"),
	}
}

type AuditAppendObserver func(eventType string, duration time.Duration, rows int, err error)

type DeliveryObserver func(transport, template, caller string, duration time.Duration, err error)

// DeviceFleetObserver records one Device Fleet operation. The composition
// root supplies it so this package keeps no metrics dependency.
type DeviceFleetObserver func(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, code string, err error)

// IdentityAccessObserver records one Identity & Access operation. The
// composition root supplies it so this package keeps no metrics dependency.
type IdentityAccessObserver func(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, code string, err error)

// IdentityRoot is Identity & Access as the root composes it (#4006). The
// root binds every seam of the owner; this graph hands Compose the values it
// builds itself and consumes what comes back. The request reads come from
// the root's session token adapter.
type IdentityRoot struct {
	Compose     func(IdentityMaterial) (IdentityComposition, error)
	Permissions func(context.Context) []string
	ActorScope  func(context.Context) (accountID int64, scope string)
	AuditActor  func(context.Context) (accountID int64, name string)
}

// IdentityMaterial is what this graph builds and the root's Identity &
// Access composition binds: the owners, the identity chain rows, the
// ledgers, the mail transport and the configuration it validated.
type IdentityMaterial struct {
	Config              ServiceConfig
	Logger              *slog.Logger
	Observe             IdentityAccessObserver
	Organizations       organizationtenancy.Capability
	Persons             peopledirectory.Capability
	Membership          schoolmembership.Capability
	Settings            settingsCompose.SettingsService
	Audit               auditModels.Command
	AuthEvents          authEventLedger
	OperatorAuditLog    operatorAuditEntries
	PushSubscriptions   pushSubscriptionRows
	Dispatcher          *email.Dispatcher
	DefaultFrom         email.Email
	MailIdentity        email.ReplyToResolver
	FrontendURL         string
	ParentsURL          string
	SchoolURL           string
	InvitationExpiry    time.Duration
	PasswordResetExpiry time.Duration
	// Outbox and EmailStatus are read at call time: Delivery is composed
	// after Identity & Access.
	Outbox      func() MailResultOutbox
	EmailStatus func(ctx context.Context, tenantID, id int64) (deliveryModule.State, bool, error)
	// The identity chain rows the account flows read and write.
	PersonRows       personRows
	StaffRows        staffRows
	TeacherRows      teacherRows
	StudentRows      studentRows
	GuardianProfiles guardianProfileRows
	StudentGuardians studentGuardianRows
	// GuardianEnrollments are the enrollment requests an accepted guardian
	// invitation claims.
	GuardianEnrollments interface {
		BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error)
	}
	// The rows the caller context's retained consumers read.
	Groups         EducationGroupRepository
	StaffGroups    schoolstructure.StaffGroupQuery
	ActivityGroups timetablePorts.CallerActivityGroups
	Sessions       *presenceCompose.SessionRecords
	Presence       *studentpresence.Module
}

// IdentityComposition is what this graph consumes of Identity & Access: the
// module, the caller context with the rows its retained consumers read, and
// the bindings of the owners that may not name the module.
type IdentityComposition struct {
	Access               *identityaccess.Module
	Caller               CallerRowReads
	GroupLiveCaller      GroupLiveCallerContext
	SupervisionCaller    SupervisionCallerContext
	GuardianInvitations  GuardianInvitationReads
	ParentGuardianAccess parentportal.GuardianAccess
	// MailRenderers render the Identity & Access mail kinds of the outbox.
	MailRenderers map[string]deliveryCompose.MailRenderer
}

// GuardianInvitationReads serves the People Directory guardian list and the
// parents portal over the guardian invitations.
type GuardianInvitationReads interface {
	peoplePorts.GuardianInvitations
	ListByProfile(ctx context.Context, guardianProfileID int64) ([]parentportal.GuardianInvitationRecord, error)
}

// CallerRowReads is the caller context with the retained rows its consumers
// in this graph read.
type CallerRowReads interface {
	Caller() identityaccess.CallerContext
	CareRequestStaff
	ClassDaySlotListStaff
	securityruntime.StudentAccessUserContext
	classdayCompose.CallerReader
	HasCurrentStaff(context.Context) (bool, error)
}
type DurableDeliveryObserver func(transport, template, operation string, duration time.Duration, count int, err error)

// ProductEventTracker captures the product analytics events of the services
// this graph composes. The composition root owns the tracker and its lifetime.
type ProductEventTracker interface {
	Capture(distinctID, event string, props map[string]any)
}

func enforceInstanceTimePolicy(appEnv string) bool {
	appEnv = strings.ToLower(strings.TrimSpace(appEnv))
	return appEnv == "production" || appEnv == "staging"
}

// staffAdministrationPermitted is the Security Runtime's staff:manage check
// the staff directory asks before a create adopts an existing staff record.
func staffAdministrationPermitted(actorPermissions []string) bool {
	return securityruntime.HasPermission(securityruntime.PermissionStaffManage, actorPermissions)
}

// feedbackCounterOrUnconfigured keeps the reduced test graph constructible:
// the composition requires a Feedback owner, and a graph built without one
// fails at the first deletion preview instead of at startup, exactly as the
// retired provider did.
func feedbackCounterOrUnconfigured(counter studentdeletioncompose.Feedback) studentdeletioncompose.Feedback {
	if counter != nil {
		return counter
	}
	return unconfiguredFeedbackCounter{}
}

type unconfiguredFeedbackCounter struct{}

func (unconfiguredFeedbackCounter) CountForStudent(context.Context, int64) (int, error) {
	return 0, errors.New("student deletion: feedback counter is not configured")
}

// MailOutbox is Delivery's mail outbox as the Identity & Access and
// Enrollment mails the root enqueues use it.
type MailOutbox interface {
	EnqueueOutbox(context.Context, deliveryCompose.MailEnqueueRequest) error
}

// MailResultOutbox also reports the stored intent, for a mail that a later
// mail must wait for.
type MailResultOutbox interface {
	MailOutbox
	EnqueueOutboxWithResult(context.Context, deliveryCompose.MailEnqueueRequest) (deliveryCompose.MailEnqueued, error)
}

// OutboxEnqueuer serves MailOutbox and MailResultOutbox over the Delivery
// enqueue facade.
type OutboxEnqueuer struct{ Outbox *deliveryCompose.EmailOutbox }

func (o OutboxEnqueuer) EnqueueOutbox(ctx context.Context, req deliveryCompose.MailEnqueueRequest) error {
	_, err := o.EnqueueOutboxWithResult(ctx, req)
	return err
}

func (o OutboxEnqueuer) EnqueueOutboxWithResult(ctx context.Context, req deliveryCompose.MailEnqueueRequest) (deliveryCompose.MailEnqueued, error) {
	stored, err := o.Outbox.Enqueue(ctx, req)
	if err != nil {
		return deliveryCompose.MailEnqueued{}, err
	}
	return deliveryCompose.MailEnqueued{ID: stored.ID}, nil
}

// CareRequestStaff resolves the staff member deciding a care request. found
// is false, without an error, for a caller who is no staff member.
type CareRequestStaff interface {
	CurrentStaffID(context.Context) (staffID int64, found bool, err error)
}
