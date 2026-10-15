// Package services provides service layer implementations
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"time"

	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetableblockdisplay"

	"github.com/moto-nrw/project-phoenix/database/repositories/pwausage"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	"github.com/moto-nrw/project-phoenix/modules/appointments"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	importService "github.com/moto-nrw/project-phoenix/modules/dataimport/compose/retained"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/pwa"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	devicefleetModule "github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	devicefleetLegacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
	emergencysnapshotlegacy "github.com/moto-nrw/project-phoenix/modules/emergencysnapshot/legacy"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	grouplivelegacy "github.com/moto-nrw/project-phoenix/modules/grouplive/legacy"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	platformModels "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	planexportCompose "github.com/moto-nrw/project-phoenix/modules/planexport/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	supervisiondashboardlegacy "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/legacy"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	workforceRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/workforce/compose/repositoryadapter"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	gradetransitioncompose "github.com/moto-nrw/project-phoenix/workflows/gradetransition/compose"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	reminderPorts "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"
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

// Factory provides access to all services
type Factory struct {
	settingsRuntimeDB *bun.DB
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
	Facilities      facilitiesCompose.RoomService
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
	PlanningTracks          timetable.PlanningTrackAdministration
	PickupSchedule          careplan.PickupScheduleService
	PartialAbsence          careplan.PartialAbsenceService
	ArrivalSchedule         careplan.ArrivalScheduleService
	CareDay                 careplan.CareDayQuery
	TimetableBridge         timetable.EndedSessionCompletion
	Materialization         timetable.MaterializationCapability
	TimetableCleanup        timetable.TimetableCleanup
	TimeTrackingCleanup     workforceCompose.TimeTrackingCleanupService
	StudentChangeLogCleanup peopleCompose.StudentChangeLogCleanupService
	Instance                timetable.InstanceLifecycleCapability
	AutoStart               timetable.InstanceAutoStart
	AutoEnd                 timetable.InstanceAutoEnd
	TimetableOperations     timetable.OperationCapability
	Users                   *peopleCompose.PersonDirectory
	// StaffDocuments is Workforce's personnel-record administration: the
	// Dokumente tab, the Stammdaten sections and the payroll number (#3752).
	StaffDocuments            *workforceModule.StaffAdmin
	StudentDocuments          careplan.StudentDocuments
	FileStore                 *filestorageModule.Module
	CaregiverCapability       peopleCompose.CaregiverCapabilityService
	Guardian                  *peopleCompose.GuardianService
	PeopleDirectory           peopledirectory.Capability
	Database                  transactionruntime.DatabaseStats
	DatabaseStatsCapabilities func(context.Context) transactionruntime.DatabaseStatsCapabilities
	Import                    *importService.ImportService[importModels.StudentImportRow]        // Student import service
	StaffImport               *importService.ImportService[importModels.StaffImportRow]          // Staff (Mitarbeiter) import service
	ClassListImport           *importService.ImportService[importModels.ClassListEntryImportRow] // Class-list entry import (#2382)
	OpeningBalanceImport      importService.OpeningBalanceImportFactory                          // Opening balance import (#2132), request-scoped
	ListExport                *lists.RendererService
	Emergency                 emergencysnapshot.Query
	SlotLists                 classday.SlotLists
	PlanExport                planexport.Service
	Reminders                 reminder.Capability
	Notifications             notifications.Notifier
	PushSubscriptions         notifications.PushSubscriptionService
	PWAUsage                  pwa.UsageService
	NotificationPreferences   notifications.PreferenceService
	AbsenceNotifier           notifications.AbsenceNotifier
	RealtimeHub               *realtime.Hub // SSE event hub (shared by services and API)
	Mailer                    email.Mailer
	DefaultFrom               email.Email
	FrontendURL               string
	InvitationTokenExpiry     time.Duration
	PasswordResetTokenExpiry  time.Duration

	// Platform domain (operator dashboard)
	OperatorProvisioning organizationtenancy.Provisioning
	Announcement         communication.Capability
	Schools              organizationtenancy.Capability
	Students             StudentServices
	StudentDeletion      *studentdeletion.Workflow
	CareLifecycle        careplan.CareLifecycle
	StudentAudit         peopleCompose.StudentAuditService
	MasterDataReview     masterdatarequests.Decisions
	CareRequests         carerequests.Service
	ExcusedRequests      careplan.ExcusedAbsenceRequests
	ParentRequests       parentrequests.Coordinator
	// RequestReviewPolicy is the one cross-domain decision about WHO may see
	// and decide parent requests. The API layer reads it to explain an empty
	// queue; the four request services enforce it per child.
	RequestReviewPolicy *ParentRequestReviewPolicy
	StudentStatusDays   studentpresence.StatusDays
	AbsenceOverview     studentpresence.StatusDayOverviews
	StudentHistory      studentpresence.StudentHistory
	// Statistics is the Statistik report (#2606).
	Statistics              studentpresence.StatisticsReports
	OGSGroupLive            grouplive.Query
	SupervisionDashboard    supervisiondashboard.Query
	TimetableData           TimetablePlanning
	InstanceSeriesConverter timetable.InstanceSeriesConversion
	UnregisteredTagScans    auditCompose.UnregisteredTagScanService

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

	// SettingsSideEffects is the per-key handler registry the API binds to
	// SettingsResource.OnValueSet. Domain packages register handlers here
	// (facilities at startup, students via EnableStudentPhotos). API never
	// owns the registry - its only job is to dispatch.
	SettingsSideEffects *settingsCompose.SideEffectRegistry
	// StudentPhotos is set by EnableStudentPhotos. nil until the API layer
	// supplies a PhotoUnlinker (file IO is an api-layer concern, not a
	// service-layer one).
	StudentPhotos   peopleCompose.StudentPhotoService
	StudentConsents StudentConsentRecords
}

// SetSettingsObservers wires delivery-owned metrics without coupling the
// settings application layer to a metrics implementation.
func (f *Factory) SetSettingsObservers(
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

// FactoryConfig is the process configuration snapshot consumed by one service
// graph. Capturing it at the composition root keeps service construction pure
// and lets independent graphs coexist in the same process.
type FactoryConfig struct {
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

func CurrentFactoryConfig() FactoryConfig {
	return FactoryConfig{
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
	Config              FactoryConfig
	Logger              *slog.Logger
	Observe             IdentityAccessObserver
	Organizations       organizationtenancy.Capability
	Persons             peopledirectory.Capability
	Membership          schoolmembership.Capability
	Settings            settingsCompose.SettingsService
	Audit               auditModels.Command
	AuthEvents          auditModels.AuthEventRepository
	OperatorAuditLog    platformModels.OperatorAuditLogRepository
	PushSubscriptions   deliveryModels.PushSubscriptionRepository
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
	Outbox      func() platformModels.OutboxResultEnqueuer
	EmailStatus func(ctx context.Context, tenantID, id int64) (deliveryModule.State, bool, error)
	// The identity chain rows the account flows read and write.
	PersonRows       userModels.PersonRepository
	StaffRows        userModels.StaffRepository
	TeacherRows      userModels.TeacherRepository
	StudentRows      userModels.StudentRepository
	GuardianProfiles userModels.GuardianProfileRepository
	StudentGuardians userModels.StudentGuardianRepository
	// GuardianEnrollments are the enrollment requests an accepted guardian
	// invitation claims.
	GuardianEnrollments interface {
		BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error)
	}
	// The rows the caller context's retained consumers read.
	Groups         EducationGroupRepository
	StaffGroups    schoolstructure.StaffGroupQuery
	ActivityGroups arrivalTimetable.CallerActivityGroups
	Sessions       *presenceCompose.SessionRecords
	Presence       *studentpresence.Module
}

// IdentityComposition is what this graph consumes of Identity & Access: the
// module, the caller context with the rows its retained consumers read, and
// the bindings of the owners that may not name the module.
type IdentityComposition struct {
	Access               *identityaccess.Module
	Caller               CallerRowReads
	GroupLiveCaller      grouplivelegacy.CallerContext
	SupervisionCaller    supervisiondashboardlegacy.CallerContext
	GuardianInvitations  GuardianInvitationReads
	ParentGuardianAccess parentportal.GuardianAccess
	// MailRenderers render the Identity & Access mail kinds of the outbox.
	MailRenderers map[string]emailoutbox.Renderer
}

// GuardianInvitationReads serves the People Directory guardian list and the
// parents portal over the guardian invitations.
type GuardianInvitationReads interface {
	peopleCompose.GuardianInvitations
	ListByProfile(ctx context.Context, guardianProfileID int64) ([]parentportal.GuardianInvitationRecord, error)
}

// CallerRowReads is the caller context with the retained rows its consumers
// in this graph read.
type CallerRowReads interface {
	Caller() identityaccess.CallerContext
	CareRequestStaff
	classdayCompose.StaffReader
	securityruntime.StudentAccessUserContext
	classdayCompose.CallerReader
	HasCurrentStaff(context.Context) (bool, error)
}
type DurableDeliveryObserver func(transport, template, operation string, duration time.Duration, count int, err error)

// DataImportObserver records one Data Import run (#2708): rows parsed,
// accepted, rejected, created and updated plus the run duration. The
// composition root supplies it so this package keeps no metrics dependency.
type DataImportObservation = importService.ImportObservation
type DataImportObserver func(DataImportObservation)

// ProductEventTracker captures the product analytics events of the services
// this graph composes. The composition root owns the tracker and its lifetime.
type ProductEventTracker interface {
	Capture(distinctID, event string, props map[string]any)
}

func newAuditCommand(store auditModels.AppendStore, logger *slog.Logger, observe AuditAppendObserver) (*auditCompose.Command, error) {
	if store == nil || logger == nil || observe == nil {
		return nil, errors.New("audit command store, logger, and observer are required")
	}
	command, err := auditCompose.NewCommand(store, func(observation auditCompose.AppendObservation) {
		observe(observation.EventType, observation.Duration, observation.Rows, observation.Err)
		log := logger.Debug
		if observation.Err != nil {
			log = logger.Error
		}
		log("audit append",
			"event_type", observation.EventType,
			"duration_ms", observation.Duration.Milliseconds(),
			"rows", observation.Rows,
			"failed", observation.Err != nil,
		)
	})
	if err != nil {
		return nil, err
	}
	return command, nil
}

// NewFactoryWithModules builds the legacy service graph around the migrated
// module capabilities it still consumes.
func NewFactoryWithModules(
	carePlan careplan.Capability,
	db *bun.DB,
	logger *slog.Logger,
	publicAPIURL string,
	tenantRuntime tenant.UnitOfWork,
	organizations organizationtenancy.Capability,
	persons peopledirectory.Capability,
	groups schoolstructure.Capability,
	rooms facilitiesModule.Capability,
	membership schoolmembership.Capability,
	calendar schoolcalendar.Calendar,
	timetableCapability timetable.Capability,
	appointmentCapability appointments.Capability,
	communicationCapability communication.Capability,
	observeCommunication func(communicationCompose.Observation),
	observeCarePlan CarePlanObserver,
	mealPlan parentportalcompose.MealPlanProvider,
	bindMealPlanSettings MealPlanSettingsBinder,
	feedbackCounter studentdeletioncompose.Feedback,
	bindFeedbackSettings FeedbackSettingsBinder,
	observeAuditAppend AuditAppendObserver,
	observeDelivery DeliveryObserver,
	observeDurableDelivery DurableDeliveryObserver,
	observeDeviceFleet DeviceFleetObserver,
	observeIdentityAccess IdentityAccessObserver,
	workTime workforceModule.Capability,
	workforce WorkforceWiring,
	observeDataImport DataImportObserver,
	productEvents ProductEventTracker,
	fileStorage FileStorageWiring,
	identity IdentityRoot,
	people PeopleWiring,
	enrollment EnrollmentWiring,
	clocks ...func() time.Time,
) (*Factory, error) {
	if err := people.validate(); err != nil {
		return nil, err
	}
	if identity.Compose == nil || identity.Permissions == nil || identity.ActorScope == nil || identity.AuditActor == nil {
		return nil, errors.New("the Identity & Access composition and the request reads are required")
	}
	if carePlan == nil || organizations == nil || persons == nil || groups == nil || rooms == nil || membership == nil || calendar == nil || timetableCapability == nil || appointmentCapability == nil || communicationCapability == nil || observeCommunication == nil || observeCarePlan == nil || mealPlan == nil || bindMealPlanSettings == nil || feedbackCounter == nil || bindFeedbackSettings == nil || observeAuditAppend == nil || observeDelivery == nil || observeDurableDelivery == nil || observeDeviceFleet == nil || observeIdentityAccess == nil || workTime == nil || observeDataImport == nil || fileStorage.Objects == nil || fileStorage.Observe == nil {
		return nil, errors.New("organization tenancy, people directory, school structure, facilities, school membership, school calendar, timetable, appointments, communication, care plan, meal plan, feedback, Audit, Delivery, Identity & Access, Workforce, and Data Import capabilities with their binders and observers are required")
	}
	communicationCompose.InstallMessageQueryInstrumentation(db)
	cfg := CurrentFactoryConfig()
	cfg.PublicAPIURL = publicAPIURL
	return NewFactoryWithConfig(carePlan, db, logger, cfg, tenantRuntime, organizations, persons, groups, rooms, membership, calendar, timetableCapability, appointmentCapability, communicationCapability, observeCommunication, observeCarePlan, mealPlan, bindMealPlanSettings, feedbackCounter, bindFeedbackSettings, observeAuditAppend, observeDelivery, observeDurableDelivery, observeDeviceFleet, observeIdentityAccess, workTime, workforce, observeDataImport, productEvents, fileStorage, identity, people, enrollment, false, clocks...)
}

// WorkforceWiring is what the root binds of Workforce into the service graph
// (#4008, ADR 0053 point 8). The adapters over the owners the root composes
// come built. The bindings over the staff rows, audit trails, settings and
// mail only this graph builds come as functions the graph calls with them.
type WorkforceWiring struct {
	// StaffAbsences is the Workforce staff absence repository with the
	// request rows translated through School Membership and the People
	// Directory.
	StaffAbsences workforceCompose.StaffAbsenceRepository
	// ShiftRows and ShiftTypeRows serve the retained Dienstplan rows of the
	// Timetable composition from Workforce.
	ShiftRows     *arrivalTimetable.WorkforceShiftRows
	ShiftTypeRows *arrivalTimetable.WorkforceShiftTypeRows
	// StaffAdmin composes the personnel-record administration.
	StaffAdmin func(WorkforceStaffAdminSources) (*workforceModule.StaffAdmin, error)
	// TimeTracking composes the time-tracking side of Workforce.
	TimeTracking func(WorkforceTimeTrackingSources) (*workforceCompose.TimeTracking, error)
	// PlanExportStaff serves the plan export's staff names.
	PlanExportStaff planexport.StaffNameReader
}

// WorkforceStaffAdminSources are the retained staff and person rows and the
// audit trails the personnel-record administration is bound over.
type WorkforceStaffAdminSources struct {
	Staff   userModels.StaffRepository
	Persons userModels.PersonRepository
	Audits  *auditCompose.Persistence
	Logger  *slog.Logger
	Now     func() time.Time
}

// WorkforceTimeTrackingSources are the retained rows, audit trails,
// settings, broadcaster and mail the time-tracking ports are bound over.
type WorkforceTimeTrackingSources struct {
	Audits         *auditCompose.Persistence
	Supervisions   workforceCompose.GroupSupervisorRepository
	SessionRecords workforceCompose.GroupRepository
	Staff          userModels.StaffRepository
	People         *peopleCompose.PersonDirectory
	Holidays       workforceCompose.HolidayDatesReader
	Settings       settingsCompose.SettingsService
	Payroll        settingsCompose.PayrollStatusGetter
	Broadcaster    realtime.Broadcaster
	Mail           WorkforceMail
	ShiftPlanSync  workforceCompose.ShiftPlanSyncer
	Logger         *slog.Logger
}

// WorkforceMail is the tenant mail the absence requests go out through.
type WorkforceMail struct {
	Dispatcher interface {
		Dispatch(context.Context, email.DeliveryRequest)
	}
	From        email.Email
	ReplyTo     email.ReplyToResolver
	FrontendURL string
}

func (w WorkforceWiring) complete() bool {
	return w.StaffAbsences != nil && w.ShiftRows != nil && w.ShiftTypeRows != nil &&
		w.StaffAdmin != nil && w.TimeTracking != nil && w.PlanExportStaff != nil
}

// PeopleWiring is what the root builds for the People Directory, School
// Membership and School Structure and hands to this graph in place of
// building it here (#4007, ADR 0053 point 8): the retained repositories over
// those owners and the adapters that bind the owners to the consumers this
// graph composes. An adapter over a value this graph composes itself (an
// Audit trail, the Identity & Access module, the session records, the person
// service, the realtime hub) comes as a constructor the graph calls with that
// value.
type PeopleWiring struct {
	// Identity is the Identity & Access module the retained repositories read
	// accounts and memberships through.
	Identity         *identityaccess.Module
	Persons          userModels.PersonRepository
	Students         userModels.StudentRepository
	StudentGuardians userModels.StudentGuardianRepository
	GuardianProfiles userModels.GuardianProfileRepository
	Staff            userModels.StaffRepository
	Teachers         userModels.TeacherRepository
	Groups           EducationGroupRepository
	GroupTeachers    GroupTeacherRepository
	ClassTeachers    ClassTeacherRepository
	// CaregiverBindingLock serializes the caregiver capability re-check over
	// the owners whose tables can retain the capability.
	CaregiverBindingLock userModels.CaregiverBindingLocker

	// The directories of the School Structure group and substitution
	// services.
	GroupRooms            education.RoomDirectory
	GroupTeacherDirectory education.TeacherDirectory
	StaffChecks           interface {
		education.StaffDirectory
		education.StaffLockStore
	}
	SubstitutionPersons education.PersonQuery
	Caregivers          education.CaregiverDirectory
	ExternalCaregivers  education.ExternalCaregiverDirectory

	// Student Presence's reads of the students, the visitors and the schools.
	StudentDisplay      presenceservice.StudentDisplayReader
	CrossTenantStudents presenceservice.CrossTenantRepo
	PresenceSchools     presenceservice.SchoolQuery
	PresenceStudents    presenceservice.PresenceStudents

	// The Organisation & Tenancy school reads of the mail identity, the
	// enrollment flows, the device displays and the parent-message branding.
	SchoolContacts    emailoutbox.SchoolContacts
	EnrollmentSchools enrollmentOwner.SchoolDirectory
	DisplaySchools    devicefleetLegacy.SchoolDirectory
	SchoolNames       interface {
		FindSchoolName(ctx context.Context, id int64) (string, bool, error)
	}

	// StudentAudit records the per-child change history through the People
	// Directory.
	StudentAudit peopleCompose.StudentAuditRecorder

	// Care Plan's schedule rows in Timetable's retained vocabulary.
	PickupSchedules   arrivalTimetable.CarePickupScheduleRows
	ArrivalSchedules  arrivalTimetable.CareArrivalScheduleRows
	PickupExceptions  arrivalTimetable.CarePickupExceptionRows
	ArrivalExceptions arrivalTimetable.CareArrivalExceptionRows

	// Constructors over the values this graph composes.
	SupervisionRecords       func(studentpresence.SupervisionRecords) studentpresence.SupervisionRecords
	SupervisionBlockers      func(studentpresence.SupervisionRecords) peopleCompose.SupervisionBlockers
	CaregiverIdentity        func(*identityaccess.Module) peopleCompose.CaregiverIdentity
	CaregiverCapabilityAudit func(AuthEventAppender) peopleCompose.CaregiverCapabilityAudit
	GuardianFinancialAudit   func(auditModels.GuardianFinancialChangeCreator) peopleCompose.GuardianFinancialAudit
	GuardianDataAccessLog    func(DataAccessAppender) peopleCompose.GuardianDataAccessLog
	ClassAssignmentAudit     func(auditModels.StaffMasterDataChangeCreator) education.ClassAssignmentAudit
	SubstitutionAudit        func(auditModels.SubstitutionChangeCreator) education.SubstitutionAudit
	StudentConsents          func(StudentConsentAppender) StudentConsentRecords
	StudentChangeLogCleanup  func(edits peopleCompose.StudentChangeLogStore, deletions DataDeletionAppender, settings RetentionSettings, logger *slog.Logger) peopleCompose.StudentChangeLogCleanupService
	StatusDayPeople          func(*peopleCompose.PersonDirectory) presenceservice.StatusDayOverviewPeople
	GroupRosterPeople        func(*peopleCompose.PersonDirectory) GroupRoster
	MasterDataDecisions      func(MasterDataDecisionWiring) (*careplanCompose.MasterDataDecisions, error)
	ParentRequestCoordinator func(ParentRequestCoordinatorWiring) (parentrequests.Coordinator, error)
	ParentRequestWriteScope  func(policy ParentRequestScopePolicy, permissions func(context.Context) []string) careplanCompose.ReviewScopeResolver
}

// The Audit Platform trails the root's adapters append to, as the narrow
// ports they write through.
type (
	AuthEventAppender interface {
		Create(ctx context.Context, event *auditModels.AuthEvent) error
	}
	DataAccessAppender interface {
		Create(ctx context.Context, entry *auditModels.DataAccessLog) error
	}
	DataDeletionAppender interface {
		Create(ctx context.Context, deletion *auditModels.DataDeletion) error
	}
	StudentConsentAppender interface {
		Create(ctx context.Context, entry *auditModels.StudentConsentChange) error
	}
)

// RetentionSettings resolves a tenant's retention period setting.
type RetentionSettings interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}

// validate refuses a wiring with a missing value or constructor: the graph
// binds every one of them while it composes.
func (w PeopleWiring) validate() error {
	required := []any{
		w.Identity, w.Persons, w.Students, w.StudentGuardians, w.GuardianProfiles, w.Staff, w.Teachers,
		w.Groups, w.GroupTeachers, w.ClassTeachers, w.CaregiverBindingLock,
		w.GroupRooms, w.GroupTeacherDirectory, w.StaffChecks, w.SubstitutionPersons, w.Caregivers, w.ExternalCaregivers,
		w.StudentDisplay, w.CrossTenantStudents, w.PresenceSchools, w.PresenceStudents,
		w.SchoolContacts, w.EnrollmentSchools, w.DisplaySchools, w.SchoolNames, w.StudentAudit,
		w.PickupSchedules, w.ArrivalSchedules, w.PickupExceptions, w.ArrivalExceptions,
		w.SupervisionRecords, w.SupervisionBlockers, w.CaregiverIdentity, w.CaregiverCapabilityAudit,
		w.GuardianFinancialAudit, w.GuardianDataAccessLog, w.ClassAssignmentAudit, w.SubstitutionAudit,
		w.StudentConsents, w.StudentChangeLogCleanup, w.StatusDayPeople, w.GroupRosterPeople,
		w.MasterDataDecisions, w.ParentRequestCoordinator, w.ParentRequestWriteScope,
	}
	for _, value := range required {
		if value == nil || reflect.ValueOf(value).IsZero() {
			return errors.New("the People Directory, School Membership and School Structure wiring is incomplete")
		}
	}
	return nil
}

// EnrollmentWiring is what the root binds of Enrollment into this graph
// (#4012, ADR 0053 point 8): the booking projection over Care Plan's
// bookings and the adapters over the owners the root composes come built.
// The flows over the settings, audit trails, outbox, realtime hub and the
// other values only this graph builds come as functions the graph calls with
// them.
type EnrollmentWiring struct {
	// Bookings is the Enrollment owner with Care Plan's bookings bound in.
	Bookings *enrollmentOwner.BookingProjection
	// ApprovedOfferings resolves approved bookings to the People Directory's
	// students.
	ApprovedOfferings *enrollmentCompose.ApprovedOfferingProjection
	// CareBookings records and replaces a child's bookings in Care Plan.
	CareBookings EnrollmentCareBookings
	// Guardians is the People Directory guardian read of the deletion preview.
	Guardians enrollmentCompose.GuardianDirectory
	// The parents portal's reads of a child's care periods and bookings.
	ParentCarePeriods     EnrollmentCarePeriods
	ParentOfferingHistory EnrollmentOfferingHistory
	// OfferingPlanning serves the offering-change review's manual planning
	// and course groups from Enrollment and Timetable.
	OfferingPlanning careplanCompose.OfferingChangePlanning

	// BindSettingsGuards registers Enrollment's active phase restrictions as
	// the Settings Platform's collection guards (#1663).
	BindSettingsGuards func(settingsCompose.BatchSettingsService)
	// MailRenderers render Enrollment's outbox kinds from the default sender.
	MailRenderers       func(defaultFrom email.Email) map[string]emailoutbox.Renderer
	Notifications       func(EnrollmentSettings, platformModels.OutboxEnqueuer) enrollmentOwner.Notifications
	Deletions           func(EnrollmentDeletionSources) enrollmentCompose.DeletionModule
	Phases              func(EnrollmentPhaseSources) *enrollmentCompose.Phases
	Decisions           func(EnrollmentDecisionSources) *enrollmentCompose.Decisions
	Capacity            func(EnrollmentSettings) enrollmentOwner.OfferingCapacity
	AdminSubscribers    func(notifications.EmailSubscribers, notifications.EmailAccounts) enrollmentCompose.AdminMailSubscribers
	Intake              func(EnrollmentIntakeSources) *enrollmentCompose.Intake
	Reports             func(EnrollmentReportSources) enrollmentOwner.Reports
	GuardianInvitations func(EnrollmentGuardianInvitations) enrollmentCompose.GuardianInvitationAvailability
	ChangeRequests      func(EnrollmentChangeRequestSources) enrollmentOwner.ChangeRequests
	Rollovers           func(EnrollmentRolloverSources) enrollmentOwner.Rollovers
	OfferingChanges     func(OfferingChangeInputs) (careplan.OfferingChangeCapability, error)
	OfferingConflicts   func(careplan.OfferingChangeConflicts) careplanCompose.ParentRequestConflicts
}

// validate refuses a wiring with a missing value or constructor.
func (w EnrollmentWiring) validate() error {
	required := []any{
		w.Bookings, w.ApprovedOfferings, w.CareBookings, w.Guardians, w.ParentCarePeriods, w.ParentOfferingHistory,
		w.OfferingPlanning, w.BindSettingsGuards, w.MailRenderers, w.Notifications, w.Deletions, w.Phases,
		w.Decisions, w.Capacity, w.AdminSubscribers, w.Intake, w.Reports, w.GuardianInvitations,
		w.ChangeRequests, w.Rollovers, w.OfferingChanges, w.OfferingConflicts,
	}
	for _, value := range required {
		if value == nil || reflect.ValueOf(value).IsZero() {
			return errors.New("the Enrollment wiring is incomplete")
		}
	}
	return nil
}

// EnrollmentCareBookings records and replaces the bookings of an approved
// child in Care Plan.
type EnrollmentCareBookings interface {
	RecordCareBookings(ctx context.Context, childID int64, inputs []enrollmentOwner.CareBookingInput) error
	ReplaceCareBookings(ctx context.Context, childID int64, inputs []enrollmentOwner.CareBookingInput) error
}

// EnrollmentCarePeriods answers the parents portal's care periods.
type EnrollmentCarePeriods interface {
	CarePeriods(ctx context.Context, studentID int64) ([]*parentportalcompose.CarePeriod, error)
}

// EnrollmentOfferingHistory answers the parents portal's booked offerings.
type EnrollmentOfferingHistory interface {
	OfferingHistory(ctx context.Context, requestChildID int64) ([]*parentportalcompose.OfferingBooking, error)
}

// EnrollmentSettings is the slice of the settings service the Enrollment
// bindings resolve through.
type EnrollmentSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveString(ctx context.Context, key string) (string, error)
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	LockClassCollectionPair(ctx context.Context) error
}

// EnrollmentGuardianInvitations lists the open invitations of guardian
// profiles.
type EnrollmentGuardianInvitations interface {
	ListOpenGuardianInvitations(context.Context, []int64) ([]identityaccess.GuardianInvitation, error)
}

// EnrollmentPhaseSources are the values of this graph the phase
// administration is bound over.
type EnrollmentPhaseSources struct {
	LockTemplateRecurrence          func(context.Context) error
	ValidateCareOfferingPhaseChange func(context.Context, int64, *enrollmentOwner.Phase) error
	// SourcedTemplates resolves the booking materialization on every edit.
	SourcedTemplates func() enrollmentOwner.SourcedTemplateResyncer
	Settings         EnrollmentSettings
	Logger           *slog.Logger
}

// EnrollmentDecisionSettings resolves the tenant settings of the decision
// flow.
type EnrollmentDecisionSettings interface {
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// EnrollmentStudentAuditor records tracked profile changes made while
// approving or synchronizing enrollment data for an existing student.
type EnrollmentStudentAuditor interface {
	RecordChangesForActor(ctx context.Context, before, after *userModels.Student, editedBy int64) error
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after userModels.StudentStatus) error
}

// EnrollmentConsentAuditor records effective consent transitions made while an
// enrollment submission is applied to a student.
type EnrollmentConsentAuditor interface {
	RecordTransitions(ctx context.Context, before, after *userModels.Student, source string, actorAccountID *int64, changedAt time.Time) error
}

// EnrollmentDepartureCompanions reads Care Plan's "läuft mit" edges of a
// student.
type EnrollmentDepartureCompanions interface {
	ListForStudent(context.Context, int64) ([]*userModels.StudentCompanion, error)
	CompanionDaysCoveredExcluding(context.Context, []int64, int64) (map[int64]map[string]bool, error)
}

// EnrollmentDecisionSources are the owners and retained repositories the root
// binds the decision flow to. A nil source skips the work that needs it, the
// way the decision flow always treated an unwired repository.
type EnrollmentDecisionSources struct {
	Requests               enrollmentCompose.DecisionRequests
	Children               enrollmentCompose.DecisionChildren
	Guardians              enrollmentCompose.DecisionGuardians
	LateInvites            enrollmentCompose.DecisionLateInvites
	Phases                 enrollmentCompose.DecisionPhases
	Schemas                enrollmentCompose.DecisionSchemas
	CareOfferings          enrollmentCompose.CareOfferingCatalog
	DataAccessLog          auditModels.DataAccessLogRepository
	OfferingAdjustments    auditModels.EnrollmentOfferingAdjustmentRepository
	Restorations           auditModels.EnrollmentRestorationRepository
	Notifications          enrollmentOwner.Notifications
	Persons                userModels.PersonRepository
	Staff                  userModels.StaffRepository
	Students               userModels.StudentRepository
	StudentEnrollment      enrollmentCompose.DecisionStudentEnrollment
	Companions             EnrollmentDepartureCompanions
	DeleteCompanions       func(context.Context, []int64) error
	StudentGuardians       userModels.StudentGuardianRepository
	GuardianFinancialAudit auditModels.GuardianFinancialChangeCreator
	GuardianProfiles       userModels.GuardianProfileRepository
	GuardianPhones         userModels.GuardianPhoneNumberRepository
	PickupSchedules        enrollmentCompose.WeeklyPickupSchedules
	ArrivalSchedules       enrollmentCompose.WeeklyArrivalSchedules
	// CareBookings is Care Plan's booking materialization (#3560). Nil skips
	// the roster materialization of an approval (focused tests).
	CareBookings enrollmentCompose.DecisionBookings
	// GuardianAccess is the Identity & Access capability an approval uses to
	// recognise a parent's existing portal account. Required: an approval
	// without it fails instead of silently skipping the account attach.
	GuardianAccess  enrollmentCompose.DecisionGuardianAccess
	StudentAudit    EnrollmentStudentAuditor
	StudentConsents EnrollmentConsentAuditor
	CareWithdrawal  enrollmentCompose.CareWithdrawalReconciler
	// Broadcaster announces student_updated + student_companions_changed
	// after an approved enrollment sync trimmed a child's "läuft mit" links.
	Broadcaster realtime.Broadcaster
	FrontendURL string
	// ParentsURL is the base of the status links in the parent mails. It
	// falls back to FrontendURL when empty.
	ParentsURL             string
	Settings               EnrollmentDecisionSettings
	LockTemplateRecurrence func(context.Context) error
	Pickups                enrollmentCompose.WeeklyPickupHooks
	Logger                 *slog.Logger
	Today                  func() timezone.Date
}

// EnrollmentIntakeSettings resolves the tenant settings of the intake by key.
// A resolver that also answers ResolveMany is read through one snapshot.
type EnrollmentIntakeSettings interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// EnrollmentIntakeSources are the owners and retained repositories the root
// binds the intake to. A nil source skips the work that needs it, the way
// the intake always treated an unwired repository.
type EnrollmentIntakeSources struct {
	Requests           enrollmentCompose.IntakeRequests
	Children           enrollmentCompose.IntakeChildren
	Bookings           enrollmentOwner.CareBookingCommands
	Guardians          enrollmentCompose.IntakeGuardians
	LateInviteRepo     enrollmentCompose.IntakeLateInvites
	CareOfferingRepo   enrollmentCompose.IntakeOfferings
	Capacity           enrollmentOwner.OfferingCapacity
	Catalog            enrollmentCompose.IntakeCatalog
	SchoolRepo         enrollmentOwner.SchoolDirectory
	Notifications      enrollmentOwner.Notifications
	StudentRepo        enrollmentCompose.StudentMatches
	GuardianAuthorizer enrollmentCompose.GuardianStudentAuthorizer
	RateLimitRepo      enrollmentCompose.SubmissionRateLimiter
	OutboxEnqueuer     platformModels.OutboxEnqueuer
	Settings           EnrollmentIntakeSettings
	// AdminSubscribers adds the staff who switched on the "Neue Anmeldung"
	// mail (#3780); nil keeps the configured address list only.
	AdminSubscribers enrollmentCompose.AdminMailSubscribers
	ManualDecider    enrollmentCompose.ManualEnrollmentDecider
	// ParentChanges makes a request unread again after a parent edit (#3778).
	ParentChanges enrollmentCompose.RequestParentChanges
	// FrontendURL is the base of the staff links, ParentsURL of the
	// parent-facing ones; it falls back to FrontendURL.
	FrontendURL string
	ParentsURL  string
	Logger      *slog.Logger
}

// EnrollmentChangeRequestSources are the owners and retained repositories the
// root binds the change requests to.
type EnrollmentChangeRequestSources struct {
	Bookings            enrollmentCompose.CareBookingChanges
	Requests            enrollmentCompose.ChangeRequestRecords
	Children            enrollmentCompose.IntakeChildren
	Guardians           enrollmentCompose.IntakeGuardians
	LateInviteRepo      enrollmentCompose.DecisionLateInvites
	CareOfferingRepo    enrollmentCompose.IntakeOfferings
	Capacity            enrollmentOwner.OfferingCapacity
	Catalog             enrollmentCompose.IntakeCatalog
	Notifications       enrollmentOwner.Notifications
	GuardianProfileRepo userModels.GuardianProfileRepository
	GuardianPhoneRepo   userModels.GuardianPhoneNumberRepository
	GuardianInvitations enrollmentCompose.GuardianInvitationAvailability
	PersonRepo          EnrollmentReviewerPersons
	StudentRepo         enrollmentCompose.StudentMatches
	GuardianAuthorizer  enrollmentCompose.GuardianStudentAuthorizer
	// Decisions applies an approved change to a child the decision flow
	// already approved; BookingGates wrap those writes. A nil Decisions
	// leaves every child to the booking replacement.
	Decisions            enrollmentOwner.ApprovedChildChanges
	BookingGates         enrollmentCompose.CareBookingGates
	CompanionGraphLocker enrollmentCompose.CompanionGraphCoordinator
	Settings             EnrollmentIntakeSettings
	OutboxEnqueuer       platformModels.OutboxEnqueuer
	FrontendURL          string
	ParentsURL           string
	Logger               *slog.Logger
}

// EnrollmentReviewerPersons resolves the persons behind reviewer accounts.
type EnrollmentReviewerPersons interface {
	FindByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*userModels.Person, error)
}

// EnrollmentReportOwnerReads are the Enrollment owner reads of the reports.
type EnrollmentReportOwnerReads interface {
	enrollmentCompose.ReportRequests
	enrollmentCompose.ReportChildren
	enrollmentCompose.ReportGuardians
	enrollmentCompose.ReportSchemas
	enrollmentCompose.ReportPhases
}

// EnrollmentReportSources are the reads the root binds the reports to.
type EnrollmentReportSources struct {
	Owner             EnrollmentReportOwnerReads
	Offerings         enrollmentCompose.ReportOfferings
	AccessLog         auditModels.DataAccessLogRepository
	Students          userModels.StudentRepository
	Persons           userModels.PersonRepository
	Groups            enrollmentCompose.RosterGroups
	StudentGuardians  userModels.StudentGuardianRepository
	Companions        enrollmentCompose.RosterCompanions
	ClassListEntries  enrollmentCompose.ClassListEntries
	PickupSchedules   enrollmentCompose.PickupSchedules
	CareParticipation enrollmentCompose.CareParticipation
	Settings          interface {
		ResolveBool(ctx context.Context, key string) (bool, error)
	}
}

// EnrollmentRolloverSettings resolves the grade collection settings the
// rollover reads.
type EnrollmentRolloverSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// EnrollmentRolloverSources are the owners the root binds the rollover to.
type EnrollmentRolloverSources struct {
	Bookings enrollmentOwner.CareBookingCommands
	Phases   enrollmentCompose.RolloverPhases
	Requests enrollmentCompose.RolloverRequests
	Children enrollmentCompose.RolloverChildren
	// Catalog clones the source phase's care-offering catalog (#2249).
	Catalog          enrollmentCompose.RolloverCatalogCloner
	Notifications    enrollmentOwner.Notifications
	PhaseEligibility enrollmentCompose.PhaseEligibilityGuard
	Outbox           platformModels.OutboxEnqueuer
	Settings         EnrollmentRolloverSettings
	// Decisions approves auto-renewed rows on an auto-approve phase.
	Decisions  enrollmentCompose.RolloverDecider
	ParentsURL string
	Logger     *slog.Logger
}

// EnrollmentDeletionOwner is the Enrollment owner slice the admin deletion
// and the retention cleanup read, lock and delete through.
type EnrollmentDeletionOwner = enrollmentCompose.DeletionOwner

// EnrollmentDeletionSources are the owners the root binds the admin deletion
// and the retention cleanup to.
type EnrollmentDeletionSources struct {
	Owner     EnrollmentDeletionOwner
	Guardians enrollmentCompose.GuardianDirectory
	// CountAuditAdjustments counts the audited offering adjustments of a
	// request or one of its children.
	CountAuditAdjustments func(context.Context, int64, *int64) (int, error)
	// CountBookings counts Care Plan's bookings of the request children.
	CountBookings func(context.Context, []int64) (int, error)
	Audit         auditModels.EnrollmentDeletionRepository
	Delivery      enrollmentCompose.EnrollmentDeletionDelivery
	Settings      interface {
		ResolveInt(ctx context.Context, key string) (int, error)
	}
	Logger *slog.Logger
}

// OfferingChangeInputs are the owners the offering-change review is composed
// over.
type OfferingChangeInputs struct {
	CarePlan careplan.Capability
	// Catalog reads the care offerings; nil reads them from CarePlan.
	Catalog     careplanCompose.OfferingChangeCatalog
	Enrollment  OfferingChangeEnrollmentReads
	Students    userModels.StudentRepository
	Withdrawals userModels.CareWithdrawalCompletionRepository
	Settings    OfferingChangeSettingsReads
	Planning    careplanCompose.OfferingChangePlanning
	Bookings    careplan.BookingMaterializationCapability
	Reviews     OfferingChangeReviewPolicy
	// Permissions reads the permissions of the request claims.
	Permissions func(context.Context) []string
	Emitter     *communicationCompose.ParentEventEmitter
	Events      userModels.ParentRequestEventRepository
	Shares      careplanCompose.ShareVisibility
	Today       func() timezone.Date
	Logger      *slog.Logger
}

// OfferingChangeEnrollmentReads is the part of the Enrollment owner an
// offering change reads.
type OfferingChangeEnrollmentReads interface {
	StudentCarePeriods(context.Context, int64) ([]*enrollmentOwner.StudentCarePeriod, error)
	RequestByID(context.Context, int64, bool) (*enrollmentOwner.Request, error)
	ChildByID(context.Context, int64) (*enrollmentOwner.RequestChild, error)
	ChildrenByID(context.Context, []int64) ([]*enrollmentOwner.RequestChild, error)
	Phase(context.Context, int64) (*enrollmentOwner.Phase, error)
	RequestChildOfferingsAtDate(context.Context, int64, enrollmentOwner.Date) ([]*enrollmentOwner.RequestChildOffering, error)
	EffectiveOfferingSelectionsAtDates(context.Context, map[int64]enrollmentOwner.Date) ([]*enrollmentOwner.RequestChildOffering, error)
	OfferingCatalogState(context.Context, int64, int64, enrollmentOwner.Date, enrollmentOwner.Date) (*enrollmentOwner.OfferingCatalogState, error)
	OfferingCapacityPeak(context.Context, int64, []int64, enrollmentOwner.Date, enrollmentOwner.Date) (int, error)
}

// OfferingChangeReviewPolicy is the parent-request review policy's scope of
// the caller.
type OfferingChangeReviewPolicy interface {
	Scope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error)
}

// EducationGroupRepository is the retained group store this graph and its
// consumers read: School Structure's contract plus the group supervisors the
// staff notification recipients resolve.
type EducationGroupRepository interface {
	education.LegacyGroupRepository
	// ListGroupSupervisors answers who supervises the groups on the day, as
	// the notification recipients' own pair type (#3556).
	ListGroupSupervisors(ctx context.Context, groupIDs []int64, on timezone.Date) ([]notifications.StaffGroupPair, error)
}

// GroupTeacherRepository is the retained contract of education.group_teacher,
// served over School Membership.
type GroupTeacherRepository interface {
	Create(ctx context.Context, relation *schoolmembership.GroupAssignment) error
	FindByID(ctx context.Context, id any) (*schoolmembership.GroupAssignment, error)
	Update(ctx context.Context, relation *schoolmembership.GroupAssignment) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*schoolmembership.GroupAssignment, error)
	FindByGroup(ctx context.Context, groupID int64) ([]*schoolmembership.GroupAssignment, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*schoolmembership.GroupAssignment, error)
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*schoolmembership.GroupAssignment, error)
	// The group service's teacher assignment store (#3556).
	education.GroupTeacherStore
	// ListGroupTeacherBlockers returns group assignments as
	// caregiver-capability blocker rows.
	ListGroupTeacherBlockers(ctx context.Context, teacherID, tenantID int64) ([]userModels.BlockerGroup, error)
}

// ClassTeacherRepository is the retained contract of the staff-to-school-class
// assignments (#1772), served over School Membership.
type ClassTeacherRepository interface {
	Create(ctx context.Context, assignment *schoolmembership.ClassAssignment) error
	FindByID(ctx context.Context, id any) (*schoolmembership.ClassAssignment, error)
	Update(ctx context.Context, assignment *schoolmembership.ClassAssignment) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*schoolmembership.ClassAssignment, error)
	// The class assignment service's store (#3556).
	education.ClassTeacherStore
	// FindByStaff returns the class assignments of one staff member.
	FindByStaff(ctx context.Context, staffID int64) ([]*schoolmembership.ClassAssignment, error)
}

// GroupRoster is the live group roster's People port: the participation
// candidates of a group with their names.
type GroupRoster interface {
	GroupMembers(ctx context.Context, groupID int64) ([]grouplive.RosterStudent, error)
}

// StudentConsentRecords resolves and records a child's consent states over the
// People Directory and the Audit Platform trail.
type StudentConsentRecords interface {
	CurrentStates(ctx context.Context, student *userModels.Student, canManagePhoto bool) ([]userModels.StudentConsentState, error)
	RecordTransitions(ctx context.Context, before, after *userModels.Student, source string, actorAccountID *int64, changedAt time.Time) error
	CurrentStudentConsents(ctx context.Context, snapshot peopledirectory.StudentConsentSnapshot, canManagePhoto bool) ([]peopledirectory.StudentConsentState, error)
	RecordStudentConsentTransitions(ctx context.Context, before, after peopledirectory.StudentConsentSnapshot, source string, actorAccountID *int64, changedAt time.Time) error
}

// ParentRequestScopePolicy is the identity-owned review reach the Stammdaten
// decision writes with.
type ParentRequestScopePolicy interface {
	Scope(ctx context.Context, permissions []string) (bool, []int64, error)
}

// MasterDataDecisionWiring is the graph's view of the Stammdaten decision's
// dependencies. Nil effects are skipped.
type MasterDataDecisionWiring struct {
	CarePlan    careplan.Capability
	Fields      peopledirectory.StudentFieldReviewQuery
	Students    userModels.StudentRepository
	Persons     userModels.PersonRepository
	Audit       peopleCompose.StudentChangeRecorder
	Scope       careplanCompose.ReviewScopeResolver
	Emitter     *communicationCompose.ParentEventEmitter
	Broadcaster realtime.Broadcaster
	Events      userModels.ParentRequestEventRepository
	Shares      careplanCompose.ShareVisibility
	Logger      *slog.Logger
	Today       func() timezone.Date
}

// ParentRequestCoordinatorWiring names the four request queues the
// cross-kind commands are composed over.
type ParentRequestCoordinatorWiring struct {
	MasterData *careplanCompose.MasterDataDecisions
	Excused    careplan.ExcusedAbsenceRequests
	Care       any
	Offering   careplanCompose.ParentRequestConflicts
	Events     userModels.ParentRequestEventRepository
	// Permissions reads the permissions of the request claims.
	Permissions func(context.Context) []string
}

// FileStorageWiring carries what the File Storage module needs from the root
// and cannot compose itself: the uploads object store and the metrics sink.
// Test factories that leave Objects nil get no file storage and no attachment
// purger; production composition requires both.
type FileStorageWiring struct {
	Objects filestorageCompose.ObjectBackend
	Observe func(filestorageCompose.Observation)
}

func NewFactoryWithConfig(
	carePlan careplan.Capability,
	db *bun.DB,
	logger *slog.Logger,
	cfg FactoryConfig,
	tenantRuntime tenant.UnitOfWork,
	organizations organizationtenancy.Capability,
	persons peopledirectory.Capability,
	groups schoolstructure.Capability,
	rooms facilitiesModule.Capability,
	membership schoolmembership.Capability,
	calendar schoolcalendar.Calendar,
	timetableCapability timetable.Capability,
	appointmentCapability appointments.Capability,
	communicationCapability communication.Capability,
	observeCommunication func(communicationCompose.Observation),
	observeCarePlan CarePlanObserver,
	mealPlan parentportalcompose.MealPlanProvider,
	bindMealPlanSettings MealPlanSettingsBinder,
	feedbackCounter studentdeletioncompose.Feedback,
	bindFeedbackSettings FeedbackSettingsBinder,
	observeAuditAppend AuditAppendObserver,
	observeDelivery DeliveryObserver,
	observeDurableDelivery DurableDeliveryObserver,
	observeDeviceFleet DeviceFleetObserver,
	observeIdentityAccess IdentityAccessObserver,
	workTime workforceModule.Capability,
	workforce WorkforceWiring,
	observeDataImport DataImportObserver,
	productEvents ProductEventTracker,
	fileStorage FileStorageWiring,
	identityRoot IdentityRoot,
	people PeopleWiring,
	enrollmentWiring EnrollmentWiring,
	allowAuditRootWrites bool,
	clocks ...func() time.Time,
) (*Factory, error) {
	if !workforce.complete() {
		return nil, errors.New("the root's Workforce wiring is required")
	}
	if err := enrollmentWiring.validate(); err != nil {
		return nil, err
	}
	now := OptionalClock(clocks)
	today := timezone.CalendarDateClock(now)
	roomRepository := NewRoomRepository(rooms)
	// The retained activities and schedule repositories belong to Timetable;
	// they are built over the capabilities this graph binds (#2743).
	timetableRepos := NewTimetableRepositories(db, TimetableDependencies{
		Capability: timetableCapability, Students: persons, Groups: groups, Rooms: rooms,
		Calendar: calendar, Membership: membership, Workforce: workTime,
	})
	// The People Directory, School Membership and School Structure
	// repositories come from the root, which builds them over the same owners
	// (#4007).
	peopleIdentity := people.Identity
	personRepository := people.Persons
	studentRepository := people.Students
	studentGuardianRepository := people.StudentGuardians
	guardianProfileRepository := people.GuardianProfiles
	guardianPhoneRepository := peopleCompose.NewGuardianPhoneNumberRepository(db)
	guardianFinancialRepository := peopleCompose.NewGuardianFinancialDataRepository(db)
	familyProtectionRepository := peopleCompose.NewFamilyProtectionEventRepository(db)
	parentRequestShareRepository := peopleCompose.NewParentRequestShareEventRepository(db)
	parentRequestEventRepository := peopleCompose.NewParentRequestEventRepository(db)
	staffRepository, teacherRepository := people.Staff, people.Teachers
	groupRepository, groupTeacherRepository, classTeacherRepository := people.Groups, people.GroupTeachers, people.ClassTeachers
	groupSubstitutionRepository := NewGroupSubstitutionRepository(groupRepository, membership, workTime)
	// The repositories the retained services read are built here, over the
	// owners this graph binds (#2743).
	enrollmentModule := enrollmentWiring.Bookings.Module
	enrollmentRecords := NewEnrollmentBookingProjection(enrollmentWiring.Bookings)
	sessionDevices, err := NewDeviceFleet(db)
	if err != nil {
		return nil, fmt.Errorf("compose session device fleet: %w", err)
	}
	sessionRecords := NewSessionRecords(db, SessionRecordOwners{
		Now: now, Devices: sessionDevices, Rooms: rooms, Timetable: timetableCapability, Membership: membership, Persons: persons,
	})
	groupSupervisions := people.SupervisionRecords(sessionRecords)
	staffAbsences := workforce.StaffAbsences
	calendarPeriods := arrivalTimetable.NewCalendarPeriodRepository(calendar, arrivalTimetable.NewPlanningCalendarPeriodUsage(enrollmentModule, timetableCapability))
	statusDays := NewStudentStatusDayRepository(carePlan)
	parentPortal := NewParentPortalRepositories(db, ParentPortalOwners{
		People: persons, Groups: groups, Membership: membership,
		Identity: peopleIdentity, Enrollment: enrollmentModule, Now: now,
	}).WithSchools(organizations)
	caregiverBindingLock := people.CaregiverBindingLock
	settingsRuntime := settingsCompose.NewTenantSettingsRuntime(db, nil)

	mailer, err := email.NewMailer(email.MailerConfig{
		Host:        cfg.EmailSMTPHost,
		Port:        cfg.EmailSMTPPort,
		User:        cfg.EmailSMTPUser,
		Password:    cfg.EmailSMTPPassword,
		DefaultFrom: email.NewEmail(cfg.EmailFromName, cfg.EmailFromAddress),
		TemplateDir: "./templates",
		Logger:      logger.With("service", "email"),
		AppEnv:      cfg.AppEnv,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize email transport: %w", err)
	}
	if _, ok := mailer.(*email.MockMailer); ok {
		logger.Warn("SMTP mailer not configured; using mock mailer (tokens will not be sent via SMTP)")
	}

	// Create scoped loggers for services that need them
	activeLogger := logger.With("service", "active")
	facilitiesLogger := logger.With("service", "facilities")
	databaseLogger := logger.With("service", "database")
	platformLogger := logger.With("service", "platform")
	emailLogger := logger.With("component", "email")
	auditLogger := logger.With("component", "audit-command")
	auditTransactionRuntime := func(ctx context.Context) (bun.IDB, int64) {
		auditTenantID := tenant.FromContext(ctx)
		raw, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			if allowAuditRootWrites {
				return db, auditTenantID
			}
			return nil, auditTenantID
		}
		switch tx := raw.(type) {
		case bun.Tx:
			return tx, auditTenantID
		case *bun.Tx:
			if tx != nil {
				return tx, auditTenantID
			}
		}
		panic(fmt.Sprintf("audit command: unsupported transaction %T", raw))
	}
	auditReadRuntime := func(ctx context.Context) (bun.IDB, int64) {
		transaction, tenantID := auditTransactionRuntime(ctx)
		if transaction == nil {
			return db, tenantID
		}
		return transaction, tenantID
	}
	// The Audit composition binds every retained Audit query adapter to the
	// caller's transaction, tenant and read-only root DB fallback, and the
	// append store to the authoritative transaction (ADR 0049). Writes are
	// routed through the fail-closed command before services capture them.
	audits, err := auditCompose.NewPersistence(auditCompose.PersistenceDependencies{
		Reads: auditReadRuntime, Appends: auditTransactionRuntime,
	})
	if err != nil {
		return nil, err
	}
	auditCommand, err := newAuditCommand(audits.AppendStore, auditLogger, observeAuditAppend)
	if err != nil {
		return nil, err
	}
	audits = audits.WithRoutedWrites(auditCommand)
	operatorAuditLog := NewOperatorAuditLog(db)
	if err := BindClassListEntryAdministration(membership, persons, auditCommand); err != nil {
		return nil, err
	}
	studentConsentService := people.StudentConsents(audits.StudentConsentChange)

	dispatcher := email.NewDispatcher(mailer, emailLogger, email.DeliveryObserver(observeDelivery))

	defaultFrom := email.NewEmail(cfg.EmailFromName, cfg.EmailFromAddress)
	if defaultFrom.Address == "" {
		defaultFrom = email.NewEmail("moto", "no-reply@moto.local")
	}

	rawFrontendURL := cfg.FrontendURL
	frontendURL := strings.TrimRight(rawFrontendURL, "/")
	if frontendURL == "" {
		return nil, fmt.Errorf("FRONTEND_URL is required")
	}

	appEnv := strings.ToLower(cfg.AppEnv)
	if appEnv == "production" && !strings.HasPrefix(frontendURL, "https://") {
		return nil, fmt.Errorf("FRONTEND_URL must use https:// in production (received %q)", rawFrontendURL)
	}

	rawPublicAPIURL := cfg.PublicAPIURL
	publicAPIURL := strings.TrimRight(rawPublicAPIURL, "/")
	if publicAPIURL == "" {
		return nil, fmt.Errorf("NEXT_PUBLIC_API_URL is required")
	}
	if appEnv == "production" && !strings.HasPrefix(publicAPIURL, "https://") {
		return nil, fmt.Errorf("NEXT_PUBLIC_API_URL must use https:// in production (received %q)", rawPublicAPIURL)
	}

	// Parents-portal URL - used for every parent-facing email link
	// (status, decision emails, guardian invitation accept).
	rawParentsURL := cfg.ParentsURL
	parentsURL := strings.TrimRight(rawParentsURL, "/")
	if parentsURL == "" {
		return nil, fmt.Errorf("PARENTS_URL is required")
	}
	if appEnv == "production" && !strings.HasPrefix(parentsURL, "https://") {
		return nil, fmt.Errorf("PARENTS_URL must use https:// in production (received %q)", rawParentsURL)
	}

	// School-portal URL (#2207) - used for Lehrkraft invitation links, which
	// must land on the school portal where the accept flow lives.
	rawSchoolURL := cfg.SchoolURL
	schoolURL := strings.TrimRight(rawSchoolURL, "/")
	if schoolURL == "" {
		return nil, fmt.Errorf("SCHOOL_URL is required")
	}
	if appEnv == "production" && !strings.HasPrefix(schoolURL, "https://") {
		return nil, fmt.Errorf("SCHOOL_URL must use https:// in production (received %q)", rawSchoolURL)
	}

	invitationExpiryHours := cfg.InvitationTokenExpiryHours
	if invitationExpiryHours <= 0 {
		invitationExpiryHours = 48
	} else if invitationExpiryHours > 168 {
		invitationExpiryHours = 168
	}
	invitationTokenExpiry := time.Duration(invitationExpiryHours) * time.Hour

	passwordResetExpiryMinutes := cfg.PasswordResetExpiryMinutes
	if passwordResetExpiryMinutes <= 0 {
		passwordResetExpiryMinutes = 30
	} else if passwordResetExpiryMinutes > 1440 {
		passwordResetExpiryMinutes = 1440
	}
	passwordResetTokenExpiry := time.Duration(passwordResetExpiryMinutes) * time.Minute

	// Create realtime hub for SSE broadcasting (single shared instance)
	realtimeHub := deliveryCompose.NewRealtimeHub(logger.With("component", "sse-hub"))

	// Initialize education service first (needed for active service)
	educationService := education.NewGroupManagement(
		groupRepository,
		groupTeacherRepository,
		classTeacherRepository,
		people.GroupRooms,
		people.GroupTeacherDirectory,
		people.StaffChecks,
		studentRepository,
		groupSubstitutionRepository,
		education.NewLegacyRepositoryRuntime(db),
		education.GroupServiceOptions{Broadcaster: realtimeHub, Audit: people.ClassAssignmentAudit(audits.StaffMasterDataChange)},
	)

	// Reconciles already-materialized future timetable rosters when a grade
	// transition graduates or restores students (#405).
	rosterReconciler := arrivalTimetable.NewRosterReconciler(
		timetableRepos.ActivityInstance,
		timetableRepos.InstanceStudent,
		timetableRepos.StudentEnrollment,
		logger,
		now,
	)
	// The Timetable owner's tenant recurrence gate (#3424 slice S2) orders
	// every recurrence writer the factory composes.
	recurrenceLock, err := arrivalTimetable.NewRecurrenceWriteLock(arrivalTimetable.RecurrenceLockDependencies{
		DB: db, GradeTransitionLockKey: schoolstructure.TransitionsLockKey,
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable recurrence lock: %w", err)
	}

	// The grade transition workflow (#2711) is composed after the enrollment
	// decision service exists: it needs the offering-roster resync that
	// service provides. rosterReconciler is its Timetable port.

	// Start page composition (#2875) shares the settings tenant runtime: the
	// personal layout and the school's prescription are read inside the same
	// tenant transaction as every other tenant-scoped setting. The repository
	// is built here rather than on the repository factory so the start page
	// adds no field to a composition root.
	settingsChanged := func(_ context.Context, tenantID int64, key string) {
		event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key})
		_ = realtimeHub.BroadcastToTenant(tenantID, event)
	}
	homeLayoutService := settingsCompose.NewHomeLayoutService(
		settingsCompose.NewHomeLayoutRepository(settingsRuntime),
		settingsRuntime,
		logger,
		settingsChanged,
	)

	// Initialize settings service (new schema-driven settings system)
	settingsService := settingsCompose.NewSettingsServiceWithHomeLayouts(
		settingsCompose.NewSettingValueRepository(settingsRuntime),
		settingsCompose.NewSettingAuditRepository(settingsRuntime),
		NewSchoolSettingsStore(organizations),
		settingsRuntime,
		logger,
		homeLayoutService,
	)
	if mealPlan != nil {
		bindMealPlanSettings(
			func(ctx context.Context) (bool, error) {
				return settingsService.ResolveBool(ctx, configModels.KeyMealPlanEnabled)
			},
			func(ctx context.Context) (bool, error) {
				return settingsService.ResolveBool(ctx, configModels.KeyMealRegistrationEnabled)
			},
			func(ctx context.Context) (string, error) {
				return settingsService.ResolveString(ctx, configModels.KeyMealRegistrationCutoffTime)
			},
		)
	}
	if bindFeedbackSettings != nil {
		bindFeedbackSettings(
			func(ctx context.Context) (bool, error) {
				return settingsService.ResolveBool(ctx, configModels.KeyFeedbackEnabled)
			},
			func(ctx context.Context) (int, error) {
				return settingsService.ResolveInt(ctx, configModels.KeyFeedbackDataRetentionDays)
			},
		)
	}
	// Enrollment's active phase restrictions guard the collection settings
	// (#1663); the root binds them.
	enrollmentWiring.BindSettingsGuards(settingsService)

	// Replies to tenant-bound mail belong to the OGS, not to moto (#1936).
	// Built once here and shared by every tenant-bound sender.
	tenantMailIdentity := emailoutbox.NewTenantMailIdentity(people.SchoolContacts, func(ctx context.Context, tenantID int64) (string, error) {
		return settingsService.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, logger)
	// Push subscriptions and PWA standalone usage belong to Delivery.
	pushSubscriptions := deliveryCompose.NewPushSubscriptionRepository(db)
	staffGroups, err := education.NewStaffGroups(education.StaffGroupsDependencies{
		Groups: groups, Assignments: membership, Substitutions: workTime,
	})
	if err != nil {
		return nil, err
	}
	callerActivityGroups := arrivalTimetable.NewCallerActivityGroups(timetableRepos.ActivityGroup)
	// The delivery module is composed after the identity module, so the
	// guardian and staff mails read its outbox and its delivery state at
	// call time.
	var emailOutboxService *emailoutbox.Service
	var deliveryForIdentity *deliveryCompose.Runtime
	// Identity & Access is composed by the root over the values this graph
	// built (#4006): sessions, the account lifecycle, the second factor, the
	// operator flows and the caller context.
	identity, err := identityRoot.Compose(IdentityMaterial{
		Config: cfg, Logger: logger, Observe: observeIdentityAccess,
		Organizations: organizations, Persons: persons, Membership: membership,
		Settings: settingsService, Audit: auditCommand, AuthEvents: audits.AuthEvent,
		OperatorAuditLog: operatorAuditLog, PushSubscriptions: pushSubscriptions,
		Dispatcher: dispatcher, DefaultFrom: defaultFrom, MailIdentity: tenantMailIdentity,
		FrontendURL: frontendURL, ParentsURL: parentsURL, SchoolURL: schoolURL,
		InvitationExpiry: invitationTokenExpiry, PasswordResetExpiry: passwordResetTokenExpiry,
		Outbox: func() platformModels.OutboxResultEnqueuer { return OutboxEnqueuer{Outbox: emailOutboxService} },
		EmailStatus: func(ctx context.Context, tenantID, id int64) (deliveryModule.State, bool, error) {
			if deliveryForIdentity == nil {
				return "", false, errors.New("delivery module is not initialized")
			}
			return deliveryForIdentity.EmailStatus(ctx, tenantID, id)
		},
		PersonRows: personRepository, StaffRows: staffRepository, TeacherRows: teacherRepository, StudentRows: studentRepository,
		GuardianProfiles: guardianProfileRepository, StudentGuardians: studentGuardianRepository,
		GuardianEnrollments: parentPortal.ParentEnrollmentRequest,
		Groups:              groupRepository, StaffGroups: staffGroups, ActivityGroups: callerActivityGroups,
		Sessions: sessionRecords, Presence: NewLoggedStudentPresence(db, logger),
	})
	if err != nil {
		return nil, err
	}
	identityAccess := identity.Access
	userContextService := identity.Caller
	callerContext := userContextService.Caller()

	// The staff and teacher lookups and the staff writes are School Membership
	// and Workforce territory (#3752); the person service takes them through
	// the StaffDirectory port.
	staffDirectory := peopleCompose.NewStaffDirectory(peopleCompose.StaffDirectoryDependencies{
		Runtime: peopleCompose.NewLegacyRepositoryRuntime(db), Persons: personRepository, Staff: staffRepository, Teachers: teacherRepository, LehrkraftRoles: identityAccess,
		MayManageStaff: staffAdministrationPermitted,
	})

	// Care Plan is composed further down and reads the person directory, so
	// the directory reaches its participation decision at call time.
	var careParticipation careplan.CareParticipation

	// Initialize users service first (needed for active service)
	usersService := peopleCompose.NewPersonDirectory(peopleCompose.PersonDirectoryDependencies{
		PersonDirectory:  peopleCompose.NewPersonWrites(persons),
		StudentDirectory: peopleCompose.NewStudentDirectory(persons),
		PersonRepo:       personRepository,
		RFIDRepo:         peopleIdentity,
		AccountExists:    identityaccessCompose.AccountExists(peopleIdentity),
		StudentRepo:      studentRepository,
		TeacherRepo:      teacherRepository,
		StaffDirectory:   staffDirectory,
		CareParticipation: CareParticipationResolver(func() careplan.CareParticipation {
			return careParticipation
		}),
	})

	// Personnel record (#1417, #1423, #1424): the payroll number, the Stammdaten
	// sections and the Dokumente tab are Workforce's own administration; the
	// staff and person rows behind it and the audit trail are bound here.
	staffAdmin, err := workforce.StaffAdmin(WorkforceStaffAdminSources{
		Staff: staffRepository, Persons: personRepository, Audits: audits,
		Logger: logger.With("service", "staff_documents"), Now: now,
	})
	if err != nil {
		return nil, fmt.Errorf("staff admin: %w", err)
	}

	// Initialize guardian service
	guardianService := peopleCompose.NewGuardianService(peopleCompose.GuardianServiceDependencies{
		GuardianProfileRepo:     guardianProfileRepository,
		GuardianPhoneNumberRepo: guardianPhoneRepository,
		StudentGuardianRepo:     studentGuardianRepository,
		GuardianInvitations:     identity.GuardianInvitations,
		StudentRepo:             studentRepository,
		PersonRepo:              personRepository,
		GuardianFinancialRepo:   guardianFinancialRepository,
		GuardianFinancialAudit:  people.GuardianFinancialAudit(audits.GuardianFinancialChange),
		DataAccessLog:           people.GuardianDataAccessLog(audits.DataAccessLog),
		RolePresets: peopleCompose.GuardianRolePresetFuncs{
			Default: securityruntime.DefaultStudentGuardianRole, Presets: securityruntime.StudentGuardianRolePreset,
		},
		DB: db,
	})

	// Public holidays per Bundesland (#1418 3a) and tenant closing days
	// (#1418 3b) share the Soll=0 semantics. The Soll consumers get the UNION
	// of both from the School Calendar; the statistics reports and the
	// holidays endpoint keep reading only the statutory holidays.
	nonWorkingDayService := NonWorkingDays{Calendar: calendar}
	// School-defined Abwesenheitsarten (#2403): the time-tracking services
	// resolve custom names on both the write path (which base type an art
	// inherits) and the read paths.
	// The retained device and unregistered-tag-scan contracts read the
	// unobserved Device Fleet composition the repository factory built
	// (#2676); the observed module composed below serves the IoT routes.
	legacyDeviceFleet, err := NewDeviceFleet(db)
	if err != nil {
		return nil, err
	}
	legacyDevices := devicefleetRepositoryAdapter.NewDeviceRepository(legacyDeviceFleet)
	// Parent and staff messages belong to Communication; the stores are
	// built where the message services use them.
	parentMessages := parentStore.NewParentMessageRepository(db)
	// Work sessions, breaks, balances, vacation rows, absence audits and the
	// work-time templates belong to Workforce (#2690); the retained contracts
	// are served by adapters over the one facade, built where they are used.
	staffWorkSchedules := workforceRepositoryAdapter.NewStaffWorkScheduleRepository(workTime)
	workTimeModels := workforceRepositoryAdapter.NewWorkTimeModelRepository(workTime)
	// The #1843 sick cascade is the shift-plan-sync workflow, which needs the
	// planning capability and the timetable services built long after the
	// absence service; the deferred binding resolves it on every call and the
	// assignment below happens once those services exist.
	var shiftPlanSyncer shiftplansync.SickCascade
	// One payroll-status instance: the /payroll page and the DATEV writers
	// must judge completeness identically.
	payrollStatusService := settingsCompose.NewPayrollStatusService(settingsService, func(ctx context.Context) (int, int, error) {
		staff, err := staffRepository.List(ctx, nil)
		if err != nil {
			return 0, 0, err
		}
		withoutPersonnelNumber := 0
		for _, member := range staff {
			if member.PersonnelNumber == nil || *member.PersonnelNumber == "" {
				withoutPersonnelNumber++
			}
		}
		return len(staff), withoutPersonnelNumber, nil
	})

	// The time-tracking side of Workforce (#3413): stamps, history and edits,
	// absences, month balances and their close, overview, exports and the
	// audit feed, over the Workforce rows and the ports bound here.
	timeTracking, err := workforce.TimeTracking(WorkforceTimeTrackingSources{
		Audits: audits, Supervisions: groupSupervisions, SessionRecords: sessionRecords,
		Staff: staffRepository, People: usersService, Holidays: nonWorkingDayService,
		Settings: settingsService, Payroll: payrollStatusService, Broadcaster: realtimeHub,
		Mail:          WorkforceMail{Dispatcher: dispatcher, From: defaultFrom, ReplyTo: tenantMailIdentity, FrontendURL: frontendURL},
		ShiftPlanSync: shiftplansyncCompose.DeferredSickCascade(func() shiftplansync.SickCascade { return shiftPlanSyncer }),
		Logger:        activeLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("compose time tracking: %w", err)
	}

	// The Timetable owner's attendance mirror (WP-B10, #3424 slice S3) serves
	// Student Presence's attendance syncer port: CreateVisit / EndVisit trigger
	// it in their own tenant transaction to mirror into
	// schedule.instance_students and enrich SSE events. No circular
	// dependency because it only depends on repos, not on studentpresence.Presence.
	attendanceSyncService, err := NewTimetableAttendanceMirror(
		arrivalTimetable.OwnerRows{Instances: timetableRepos.ActivityInstance, Participants: timetableRepos.InstanceStudent},
		logger.With("service", "attendance-sync"),
	)
	if err != nil {
		return nil, fmt.Errorf("compose timetable attendance mirror: %w", err)
	}
	approvedOfferingProjection := enrollmentWiring.ApprovedOfferings
	pickupBaselines, err := careplanCompose.NewPickupBaselines(carePlan, approvedOfferingProjection, func(ctx context.Context) (bool, error) {
		return settingsService.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return nil, err
	}
	// The arrival mirror: the class timetable supplies the regular time and,
	// with enrollment.bookings_authoritative on, the approved bookings supply
	// the care days (#2414, ADR 0005). The care-day resolver reads through it
	// so a stale row on an unbooked weekday stops marking a child expected.
	classArrivalQueries, err := arrivalTimetable.NewClassArrivals(db, func(observation arrivalTimetable.Observation) {
		logger.Debug("class arrival query",
			"operation", observation.Operation,
			"duration", observation.Duration,
			"queries", observation.Stats.Queries,
			"rows", observation.Stats.Rows,
			"error", observation.Err)
	})
	if err != nil {
		return nil, err
	}
	arrivalBaselines, err := NewArrivalBaselines(carePlan, persons, classArrivalQueries, approvedOfferingProjection, func(ctx context.Context) (bool, error) {
		return settingsService.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
	})
	if err != nil {
		return nil, err
	}

	// Care-day derivation (#1747): intersects timetable assignments with the
	// children's care plans. Read-only, so it can be shared by every consumer
	// (instance lifecycle, operations roster, weekly planner, scheduler).
	// Built here, ahead of the active service, because the timetable bridge
	// below needs it and the active service needs the bridge.
	careDayService := careplanCompose.NewCareDays(careplanCompose.CareDayDependencies{
		ArrivalBaselines: arrivalBaselines,
		Records:          carePlan,

		PickupBaselines: pickupBaselines,
	})

	// Single entry point for completing instances whose active.group somebody
	// else ended (force-start here, the nightly session-end job in the
	// scheduler). Finalizes attendance first, so a completed instance never
	// keeps genuinely expected rows — readers take those as the "not booked
	// into care that day" marker (#1747). Repos only, so no cycle with the
	// active service it is injected into.
	timetableRows := arrivalTimetable.OwnerRows{
		Instances:         timetableRepos.ActivityInstance,
		InstanceStaff:     timetableRepos.InstanceStaff,
		Participants:      timetableRepos.InstanceStudent,
		Templates:         timetableRepos.ActivityGroup,
		Categories:        timetableRepos.ActivityCategory,
		Students:          studentRepository,
		EducationGroups:   TimetableEducationGroupNames(groupRepository),
		Rooms:             facilitiesCompose.NewRoomNameDirectory(roomRepository),
		PickupExceptions:  people.PickupExceptions,
		ArrivalExceptions: people.ArrivalExceptions,
	}
	timetableRows.DeviationEvents, timetableRows.Deviations = TimetableDeviationPorts(audits.DeviationEvent)
	timetableBridgeService, err := NewTimetableEndedSessionCompletion(timetableRows, careDayService)
	if err != nil {
		return nil, fmt.Errorf("compose ended session completion: %w", err)
	}

	// Initialize active service with SSE broadcaster
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(sessionRecords)
	activeServiceDeps := presenceservice.PresenceDependencies{
		PrincipalReader:          AttendancePrincipal,
		SchoolPresence:           NewLoggedStudentPresence(db, logger),
		StudentDisplay:           people.StudentDisplay,
		GroupRepo:                sessionGroups,
		SessionStartLock:         NewLoggedStudentPresence(db, logger),
		SupervisorRepo:           sessionSupervisors,
		StudentStatusRepo:        statusDays,
		CrossTenantRepo:          people.CrossTenantStudents,
		Schools:                  people.PresenceSchools,
		StudentRepo:              people.PresenceStudents,
		StaffRepo:                NewAttendanceStaffDirectory(staffRepository),
		RoomRepo:                 NewAttendanceRooms(roomRepository),
		YardRoomColor:            YardRoomColorQuery(rooms),
		ActivityGroupRepo:        NewSessionActivities(timetableRepos.GroupReads()),
		ActivityCatRepo:          NewAttendanceActivityCategories(timetableRepos.ActivityCategory),
		EducationGroupRepo:       NewAttendanceEducationGroups(groupRepository, studentRepository),
		DeviceRepo:               NewSessionDeviceDirectory(legacyDevices, settingsService, activeLogger),
		StaffNames:               NewAttendanceStaffNames(staffRepository, usersService),
		DB:                       db,
		Broadcaster:              realtimeHub,              // Pass SSE broadcaster
		Tracker:                  productEvents,            // Product analytics (PostHog)
		WorkSessionService:       timeTracking.AutoCheckIn, // NFC auto-check-in
		AttendanceSyncer:         attendanceSyncService,    // WP-B10 mirror + SSE enrichment
		TimetableBridgeCompleter: timetableBridgeService,
		Logger:                   activeLogger,
		Now:                      now,
	}
	// Chat-pill emitter (#1803): also provides guardian-only invalidations for
	// enrollment writes that change a child's live care data.
	pillEmitter := communicationCompose.NewParentEventEmitter(communicationCompose.ParentEventEmitterConfig{
		DB:          db,
		Runtime:     tenantRuntime,
		ThreadRepo:  parentPortal.ParentMessageThread,
		MessageRepo: parentMessages,
		Settings:    settingsService,
		Broadcaster: realtimeHub,
		Logger:      logger.With("service", "parent-events"),
	})
	activeService := presenceservice.NewPresence(activeServiceDeps,
		// The settings resolver lets auto-clear of sick / excused flags respect
		// the tenant's operations.sick_clear_mode and
		// operations.excused_clear_mode settings.
		presenceservice.WithPresenceSettings(PresenceSettings(settingsService)),
		// Session commands that run without a request transaction (scheduler
		// timeouts, daily session end) open their own tenant transaction.
		presenceservice.WithPresenceTenantRuntime(tenantRuntime),
		// Anwesenheitswechsel wecken die Sorgeberechtigten, damit der
		// Tagesstatus in der Eltern-App (#2252) live nachlaedt.
		presenceservice.WithGuardianWaker(pillEmitter),
	)

	// Initialize activities service
	activitiesService, err := arrivalTimetable.NewService(
		timetableCapability,
		timetableRepos.ActivityGroup,
		timetableRepos.ActivitySchedule,
		timetableRepos.ActivitySupervisor,
		timetableRepos.StudentEnrollment,
		sessionRecords,
		staffRepository,
		studentRepository,
	)
	if err != nil {
		return nil, err
	}

	// Construct care-offering validation before services that can mutate its
	// recurrence resources. Room/timeframe FKs use ON DELETE SET NULL, so those
	// delete services must preflight the same materializability invariant as
	// template and calendar-period mutations.
	// The decision service that keeps offering-sourced rosters and the pickup
	// projection in step is composed later; the catalog resolves it on every
	// edit (#2147 review).
	var careOfferingResyncer careOfferingResync
	resyncSourcedTemplates, resyncPickup := lateCareOfferingResync(&careOfferingResyncer)
	enrollmentCareOfferingService, err := NewCareOfferingCatalog(CareOfferingCatalogInputs{
		Records:          carePlan,
		Phases:           enrollmentRecords,
		Bookings:         enrollmentRecords,
		Timetable:        timetableCapability,
		Calendar:         calendar,
		Settings:         settingsService,
		SourcedTemplates: resyncSourcedTemplates,
		Pickup:           resyncPickup,
		LockRecurrence:   recurrenceLock.LockRecurrenceWrites,
		Today:            today,
		Logger:           logger.With("service", "enrollment-care-offering"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose care offering catalog: %w", err)
	}

	// Initialize facilities service
	facilitiesService := facilitiesCompose.NewRoomService(facilitiesCompose.RoomServiceConfig{
		Rooms:     rooms,
		Occupancy: facilitiesLegacy.OccupancyProjection(RoomOccupancyPresence{NewLoggedStudentPresence(db, logger)}, timetableRepos.ActivityGroup, membership, persons),
		History:   facilitiesLegacy.HistoryProjection(RoomHistoryPresence{NewLoggedStudentPresence(db, logger)}, timetableRepos.ActivityGroup, membership, persons),
		ValidateDeletion: func(ctx context.Context, roomID int64) error {
			activeGroups, err := sessionRecords.FindActiveByRoomID(ctx, roomID)
			if err != nil {
				return err
			}
			if len(activeGroups) > 0 {
				return facilitiesModule.ErrRoomInUse
			}
			if err := enrollmentCareOfferingService.ValidateRoomDeletion(ctx, roomID); err != nil {
				if errors.Is(err, careplan.ErrCareOfferingConfigInvalid) {
					return facilitiesModule.ErrRoomRequiredByOffering
				}
				return err
			}
			return nil
		},
	})

	// Initialize Schulhof service (depends on facilities, activities, and active services)
	schulhofService := facilitiesCompose.NewSchulhofService(
		facilitiesService,
		facilitiesLegacy.ActivityCatalog(activitiesService),
		facilitiesLegacy.OpenGroupCatalog(FacilitiesGroupSupervisions(NewLoggedStudentPresence(db, logger)), FacilitiesRoomSessions(NewLoggedStudentPresence(db, logger)), FacilitiesGroupVisits(NewLoggedStudentPresence(db, logger))),
		facilitiesLogger,
	)

	// Initialize WC service (depends on facilities and activities services)
	wcService := facilitiesCompose.NewWCService(
		facilitiesService,
		facilitiesLegacy.ActivityCatalog(activitiesService),
		facilitiesLogger,
	)

	planningTrackService := arrivalTimetable.NewPlanningTrackAdministration(timetableCapability, db)

	// The Dienstplan side of Workforce (#3418): shifts, series, shift types,
	// the self-service assignments ("Mein Tag", #1844) and the week overview,
	// composed by the owner over its native rows. The calendar period comes
	// from the School Calendar; the timetable blocks, their staffing and the
	// Angebote arrive in the Timetable owner's public vocabulary over the
	// retained repositories (#3424); the shift_moved Änderungsprotokoll entry
	// (#1884) and the SSE invalidation are bound here as well.
	shiftPlanning, err := workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce:       workTime,
		Staff:           staffRepository,
		CalendarPeriods: calendar,
		DeviationEvents: audits.DeviationEvent,
		Instances:       arrivalTimetable.NewTimetableInstanceReads(timetableRepos.ActivityInstance),
		InstanceStaff:   arrivalTimetable.NewTimetableInstanceStaffReads(timetableRepos.InstanceStaff),
		Rooms:           roomRepository,
		ActivityGroups:  arrivalTimetable.NewTimetableGroupReads(timetableRepos.ActivityGroup),
		WorkSchedules:   staffWorkSchedules,
		WorkModels:      workTimeModels,
		Holidays:        nonWorkingDayService,
		NonWorkingDays:  calendar,
		CategoryLinker:  arrivalTimetable.ShiftTypeCategoryLinker(activitiesService.SetCategoryShiftTypeLinks),
		DB:              db,
		Broadcaster:     realtimeHub,
		Logger:          logger.With("service", "shift_planning"),
		Today:           today,
	})
	if err != nil {
		return nil, err
	}
	// The retained-row readers the Timetable coverage probe, the staff
	// calendar feed and the plan export still speak, served over the same
	// facade by the legacy row adapters (#3418, #3424).
	shiftRows, shiftTypeRows := workforce.ShiftRows, workforce.ShiftTypeRows

	// Couples pulled-forward day pickup times with the per-block partial
	// absences (#2360). Shared by the staff pickup-exception writers and the
	// parent care-exception writers so both derive the same state.
	// Later pickups open a block decision for the Leitung (#3261).
	pickupAutoExcusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: db, Records: carePlan, Baselines: pickupBaselines,
		Blocks: NewLoggedStudentPresence(db, logger), Preview: NewPickupExcusalTimetable(timetableCapability, NewPartialAbsencePreview(db)), Extensions: NewPickupExcusalTimetable(timetableCapability, NewPartialAbsencePreview(db)),
	})
	if err != nil {
		return nil, err
	}

	// Initialize pickup schedule service
	pickupStudents, err := PickupPlanStudents(persons)
	if err != nil {
		return nil, err
	}
	pickupScheduleService, err := careplanCompose.NewPickupSchedules(db, carePlan, pickupBaselines,
		pickupAutoExcusal, pickupStudents, logger.With("service", "pickup-schedule"))
	if err != nil {
		return nil, err
	}
	// Compose the Device Fleet owner (#2676). It owns iot.devices and
	// display.displays; the info-point dashboard reads Facilities and Student
	// Presence through their public capabilities and the remaining
	// cross-owner facts through this owner's consumer-owned ports.
	deviceFleet, err := devicefleetCompose.New(devicefleetCompose.Dependencies{
		DB:       db,
		Rooms:    rooms,
		Presence: NewLoggedStudentPresence(db, logger),
		Dashboard: devicefleetLegacy.NewDashboardSources(devicefleetLegacy.DashboardDependencies{
			ActiveGroups:   sessionRecords,
			Templates:      timetableRepos.ActivityGroup,
			Instances:      timetableRepos.ActivityInstance,
			PickupSchedule: pickupScheduleService,
		}),
		Tenants: devicefleetLegacy.NewTenantFacts(devicefleetLegacy.TenantFactDependencies{
			Schools:  people.DisplaySchools,
			Settings: settingsService,
		}),
		Now:          timezone.Now,
		OnlineWindow: devicefleetLegacy.NewOnlineWindowResolver(settingsService, logger.With("module", "device-fleet")),
		Observe: func(observation devicefleetCompose.Observation) {
			observeDeviceFleet(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, devicefleetModule.ErrorCode(observation.Err), observation.Err)
		},
	})
	if err != nil {
		return nil, err
	}
	iotService := devicefleetCompose.NewIoTService(deviceFleet)

	partialAbsenceService, err := careplanCompose.NewPartialAbsences(db, carePlan, NewLoggedStudentPresence(db, logger), pickupAutoExcusal)
	if err != nil {
		return nil, err
	}

	// Initialize materialization service (WP-B8). Turns activity templates into
	// concrete schedule.activity_instances + instance_staff/instance_students
	// for a date window. Consumed by the scheduler task (gated on the
	// timetable.materialization_enabled setting) and the manual admin endpoint.
	timetableTemplateRows := arrivalTimetable.TemplateRows{
		Groups: timetableRepos.ActivityGroup, Categories: timetableRepos.ActivityCategory, Schedules: timetableRepos.ActivitySchedule,
		Enrollments: timetableRepos.StudentEnrollment, Supervisors: timetableRepos.ActivitySupervisor, Instances: timetableRepos.ActivityInstance,
		InstanceStaff: timetableRepos.InstanceStaff, Participants: timetableRepos.InstanceStudent, Timeframes: timetableRepos.Timeframe,
		CalendarPeriods: calendarPeriods, Exceptions: timetableRepos.ActivityException, EducationGroups: groupRepository,
	}
	materializationDeps := TimetableMaterializationDependencies(TimetableMaterializationInputs{
		Rows:       timetableTemplateRows,
		CareBounds: studentRepository,
		// Holidays and closing days carry no series occurrences unless the
		// series opts into closing days (#3594); the School Calendar answers.
		NonWorkingDays: calendar,
		RecurrenceLock: recurrenceLock,
		Broadcaster:    realtimeHub,
		Logger:         logger.With("service", "materialization"),
	})
	materializationDeps.DB = db
	materializationService, err := arrivalTimetable.NewMaterialization(materializationDeps)
	if err != nil {
		return nil, fmt.Errorf("compose timetable materialization: %w", err)
	}

	// Initialize instance lifecycle before template split: the split reuses its
	// deviation snapshot/reapply machinery when replacing future occurrences.
	recoveryRepo := arrivalTimetable.NewRetainedActivityRecoveryRepository(db, timetableRepos.InstanceStudent)
	timetableRows.Locks = recoveryRepo
	// The cancellation notice (#2601) rides on the announcement service, which
	// is built after the instance service; the lifecycle reaches it through
	// this late-bound publisher.
	var guardianNoticePublisher communication.CareCancellationPublisher
	// Conflict detection and staffing are the Timetable owner's (#3550): the
	// start check of the instance lifecycle and the auto-start tick, and the
	// planning reads api/timetable serves.
	timetableConflicts, err := NewTimetableConflictDetection(TimetableConflictReaders{
		Instances:         timetableRepos.ActivityInstance,
		InstanceStaff:     timetableRepos.InstanceStaff,
		InstanceStudents:  timetableRepos.InstanceStudent,
		Exceptions:        timetableRepos.ActivityException,
		Schedules:         timetableRepos.ActivitySchedule,
		Staff:             staffRepository,
		CalendarPeriods:   calendarPeriods,
		ArrivalExceptions: people.ArrivalExceptions,
		Sessions:          sessionRecords,
		Shifts:            shiftRows,
		Presence:          NewLoggedStudentPresence(db, logger),
		ArrivalBaselines:  arrivalBaselines,
		Logger:            logger.With("service", "timetable-conflicts"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable conflict detection: %w", err)
	}
	substituteConflicts, err := NewTimetableSubstituteConflicts(timetableRows)
	if err != nil {
		return nil, fmt.Errorf("compose timetable substitute conflicts: %w", err)
	}
	instanceService, err := NewTimetableLifecycle(TimetableLifecycleInputs{
		Rows: timetableRows,
		Planning: arrivalTimetable.InstanceLifecycleDependencies{
			IdempotencyRepo: timetableRepos.InstanceIdempotency, ExceptionRepo: timetableRepos.ActivityException, CalendarPeriodRepo: calendarPeriods,
			DB: db,
		},
		Staff:               staffRepository,
		Sessions:            sessionRecords,
		Supervisions:        groupSupervisions,
		Presence:            NewLoggedStudentPresence(db, logger),
		SessionEnder:        activeService,
		CareDays:            careDayService,
		CareDayLocks:        carePlan,
		GuardianNotices:     communicationCompose.LateCareCancellationPublisher(func() communication.CareCancellationPublisher { return guardianNoticePublisher }),
		Settings:            settingsService,
		Materialization:     materializationService,
		RecurrenceLock:      recurrenceLock,
		StartConflicts:      timetableConflicts,
		SubstituteConflicts: substituteConflicts,
		Broadcaster:         realtimeHub,
		Logger:              logger.With("service", "instance-lifecycle"),
		Now:                 now,
		// Development and test seed profiles create completed instances at the
		// current clock time. Lifecycle policy remains covered by dedicated tests.
		EnforceTimePolicy: enforceInstanceTimePolicy(cfg.AppEnv),
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable instance lifecycle: %w", err)
	}
	// The Timetable owner's deviation writes (Vertretungsplan, #3424 slice S3)
	// hand the cancel branch and the understaffed acknowledgement to the
	// lifecycle.
	staffDeviationDeps, err := TimetableStaffDeviationDependencies(TimetableDeviationInputs{
		Rows: timetableRows, Staff: staffRepository, Supervisions: groupSupervisions, Lifecycle: instanceService.DeviationLifecycle(),
		Broadcaster: realtimeHub, Logger: logger.With("service", "timetable-deviations"), Now: now,
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable deviations: %w", err)
	}
	staffDeviationDeps.DB = db
	staffDeviations, err := arrivalTimetable.NewStaffDeviations(staffDeviationDeps)
	if err != nil {
		return nil, fmt.Errorf("compose timetable deviations: %w", err)
	}

	// The Timetable owner's GDPR retention (WP-B14, #3551). Deletes
	// schedule.activity_instances (CASCADE → instance_staff + instance_students)
	// and schedule.activity_exceptions older than the tenant's retention window.
	// Per-student audit rows via DataDeletion; exceptions slog-only.
	timetableCleanupService, err := NewTimetableCleanup(TimetableRetentionInputs{
		Owner:      timetableCapability,
		Deletions:  audits.DataDeletion,
		Deviations: audits.DeviationEvent,
		Settings:   settingsService,
		Logger:     logger.With("service", "timetable-cleanup"),
		Clock:      now,
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable cleanup: %w", err)
	}

	// Initialize time-tracking GDPR cleanup service (Tranche 0b). Deletes
	// active.work_sessions (CASCADE → work_session_breaks +
	// audit.work_session_edits) and active.staff_absences older than the
	// tenant's retention window. Per-staff audit rows via DataDeletion
	// (staff_id subject, added in migration 1.15.58).
	timeTrackingCleanupService, err := workforceCompose.NewTimeTrackingCleanup(workforceCompose.TimeTrackingCleanupDependencies{
		Workforce: workTime, Absences: staffAbsences, Audit: NewTimeTrackingRetentionAudit(audits.DataDeletion),
		Settings: PresenceSettings(settingsService), Logger: logger.With("service", "time-tracking-cleanup"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose time tracking cleanup: %w", err)
	}

	// Per-child change-history retention cleanup (issue #1455). Deletes
	// audit.student_field_edits older than the tenant's retention window
	// (gdpr.student_change_log_retention_days, default 90). One per-student
	// DataDeletion audit row per run; shares the nightly cleanup window.
	studentChangeLogCleanupService := people.StudentChangeLogCleanup(
		audits.StudentFieldEdit,
		audits.DataDeletion,
		settingsService,
		logger.With("service", "student-change-log-cleanup"),
	)

	autoStartService, autoEndService, err := newTimetableLifecycleSchedulers(
		timetableRows, instanceService, timetableConflicts, logger.With("service", "timetable-auto-start"),
	)
	if err != nil {
		return nil, fmt.Errorf("compose timetable auto start and end: %w", err)
	}

	classExceptions, err := NewClassArrivalExceptions(classArrivalQueries, persons)
	if err != nil {
		return nil, err
	}
	arrivalRules, arrivalDeps, err := ArrivalScheduleBinding(persons, ClassArrivalPlans(classArrivalQueries), classExceptions, logger.With("service", "arrival-schedule"))
	if err != nil {
		return nil, err
	}
	arrivalScheduleService, err := careplanCompose.NewArrivalSchedules(db, carePlan, arrivalBaselines, arrivalRules, arrivalDeps)
	if err != nil {
		return nil, err
	}

	timetableOperationsService, err := NewTimetableOperations(TimetableOperationInputs{
		Rows:           timetableRows,
		PlanningTracks: timetableCapability,
		Sessions:       sessionRecords,
		Supervisions:   groupSupervisions,
		Visits:         NewLoggedStudentPresence(db, logger),
		Presence:       activeService,
		People:         TimetableOperationPeople{OperationPeople: usersService, Membership: membership},
		Settings:       settingsService,
		CareDays:       careDayService,
		Arrivals:       arrivalScheduleService,
		Pickups:        pickupScheduleService,
		Lifecycle:      instanceService.OperationLifecycle(),
		Broadcaster:    realtimeHub,
		Logger:         logger.With("service", "timetable-operations"),
		Now:            now,
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable operations: %w", err)
	}

	// Delivery composition is declared here so legacy email producers and the
	// guardian invitation service share the same durable capability.
	// Every outbox kind's renderer is known at startup, so the registry is
	// complete before the worker can claim anything. Enrollment keeps one
	// renderer per kind so subjects + templates stay independent; the
	// rollover pair reuses the submission template as a placeholder until
	// proper branded copy lands. Calendar appointments share one renderer
	// for all four kinds.
	appointmentRenderer := emailoutbox.RendererFunc(NewCalendarAppointmentRenderer(CalendarEmailDependencies{
		DefaultFrom: defaultFrom,
		Runtime:     calendarCompose.NewTenantRuntime(db),
		Guardians:   studentGuardianRepository,
	}))
	renderers := map[string]emailoutbox.Renderer{
		platformModels.EmailKindParentAnnouncement: emailoutbox.RendererFunc(communicationCompose.NewParentAnnouncementRenderer(communicationCompose.ParentAnnouncementEmailConfig{
			DefaultFrom: defaultFrom,
		})),
		platformModels.EmailKindParentMessage:        emailoutbox.RendererFunc(communicationCompose.NewParentMessageRenderer(communicationCompose.ParentMessageRendererConfig{DefaultFrom: defaultFrom})),
		platformModels.EmailKindAppointmentPublished: appointmentRenderer,
		platformModels.EmailKindAppointmentUpdated:   appointmentRenderer,
		platformModels.EmailKindAppointmentCancelled: appointmentRenderer,
		platformModels.EmailKindAppointmentReminder:  appointmentRenderer,
	}
	for kind, renderer := range enrollmentWiring.MailRenderers(defaultFrom) {
		renderers[kind] = renderer
	}
	for kind, renderer := range identity.MailRenderers {
		renderers[kind] = renderer
	}
	emailTemplateRegistry := emailoutbox.NewTemplateRegistry(renderers)
	vapidConfig := notifications.VAPIDConfig{
		PublicKey: strings.TrimSpace(cfg.VAPIDPublicKey), PrivateKey: strings.TrimSpace(cfg.VAPIDPrivateKey),
		Subscriber: strings.TrimSpace(cfg.VAPIDSubscriber),
	}
	if err := vapidConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid VAPID configuration: %w", err)
	}
	deliveryRuntime, err := deliveryCompose.New(deliveryCompose.Dependencies{
		DB:     db,
		People: GuardianDisplayResolver{Query: guardianService},
		Provider: &DeliveryProvider{
			Registry: emailTemplateRegistry, Mailer: mailer, MailIdentity: tenantMailIdentity,
			Push: deliveryCompose.NewWebPushSender(deliveryModule.WebPushConfig{
				Subscriber: vapidConfig.Subscriber, PublicKey: vapidConfig.PublicKey, PrivateKey: vapidConfig.PrivateKey,
			}, NewExpiredPushSubscriptionCleaner(deliveryCompose.NewTenantTransactions(db), pushSubscriptions)),
			Logger: logger.With("service", "delivery"), Tenants: deliveryCompose.NewTenantTransactions(db),
			PushAuthorized: NewPushAuthorizationChecker(deliveryCompose.NewTenantTransactions(db), pushSubscriptions),
		},
		Observe: func(observation deliveryModule.Observation) {
			observeDurableDelivery(string(observation.Transport), observation.Template, observation.Operation, observation.Duration, observation.Count, observation.Err)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("initialize delivery module: %w", err)
	}
	deliveryForIdentity = deliveryRuntime
	emailOutboxWorker := deliveryRuntime.Worker
	emailOutboxService = emailoutbox.NewService(DurableEmailAdapter{Module: deliveryRuntime.Module})

	caregiverCapabilityService := peopleCompose.NewCaregiverCapabilityService(peopleCompose.CaregiverCapabilityServiceDependencies{
		RequestActorScope:      identityRoot.ActorScope,
		Identity:               people.CaregiverIdentity(identityAccess),
		Audit:                  people.CaregiverCapabilityAudit(audits.AuthEvent),
		PersonRepo:             personRepository,
		StaffRepo:              staffRepository,
		CaregiverBindingLock:   caregiverBindingLock,
		TeacherRepo:            teacherRepository,
		GroupTeacherRepo:       groupTeacherRepository,
		GroupSubstitutionRepo:  groupSubstitutionRepository,
		GroupSupervisorRepo:    people.SupervisionBlockers(groupSupervisions),
		ActivitySupervisorRepo: arrivalTimetable.CaregiverPlannedSupervisionBlockers{Supervisors: timetableRepos.ActivitySupervisor},
		RoleAssignments:        identityAccess,
	})

	// Terminvertretungen move Betreuungsplan staffing across the Workforce and
	// Timetable line: the shift-plan-sync workflow serves the substitution
	// module's schedule port (#3418).
	scheduleSubstitution, err := shiftplansyncCompose.NewSubstitution(shiftplansyncCompose.SubstitutionDependencies{
		Deviations: staffDeviations, Staff: staffRepository, Broadcaster: realtimeHub, Logger: logger.With("service", "schedule-substitution"),
		ActivityInstances: arrivalTimetable.NewTimetableInstanceReads(timetableRepos.ActivityInstance),
		InstanceStaff:     arrivalTimetable.NewTimetableInstanceStaffReads(timetableRepos.InstanceStaff),
	})
	if err != nil {
		return nil, err
	}
	substitutionService := education.NewSubstitutionModule(education.SubstitutionDependencies{
		Groups: groupRepository, Substitutions: NewEducationHandovers(groupSubstitutionRepository),
		Persons: people.SubstitutionPersons, Teachers: people.Caregivers,
		Staff: people.StaffChecks, ExternalCaregivers: people.ExternalCaregivers,
		Actors:       SubstitutionActorResolver{Identity: callerContext},
		ActiveGroups: sessionRecords, ActiveSupervisors: groupSupervisions,
		ActiveSupervisorCreator: activeService,
		Audit:                   people.SubstitutionAudit(audits.SubstitutionChange),
		Runtime:                 education.NewLegacyRepositoryRuntime(db), Broadcaster: realtimeHub,
		Logger:   logger.With("service", "substitution"),
		Schedule: scheduleSubstitution,
		// Workforce owns education.group_substitution and decides who sees all
		// of them; the Settings Platform answers its overview-scope port.
		CanSeeAll: func(ctx context.Context, assignmentBound, admin, hasStaff bool) (bool, error) {
			return workforceModule.CanSeeAllSubstitutions(ctx, func(ctx context.Context) (bool, error) {
				scope, err := settingsService.ResolveString(ctx, configModels.KeyOperationalOverviewScope)
				if err != nil {
					return false, fmt.Errorf("resolve operational overview scope: %w", err)
				}
				return scope == configModels.OverviewScopeAllStaff, nil
			}, assignmentBound, admin, hasStaff)
		},
		Now: now,
	})

	// Initialize database stats service
	databaseService := transactionruntime.NewDatabaseStats(transactionruntime.DatabaseStatsDependencies{
		Students: func(ctx context.Context) (int, error) {
			return studentRepository.CountEnrolled(ctx)
		},
		Teachers: func(ctx context.Context) (int, error) {
			rows, err := staffRepository.List(ctx, nil)
			return len(rows), err
		},
		Rooms: func(ctx context.Context) (int, error) {
			rows, err := roomRepository.List(ctx, nil)
			return len(rows), err
		},
		Activities: func(ctx context.Context) (int, error) {
			rows, err := timetableRepos.ActivityGroup.List(ctx, nil)
			return len(rows), err
		},
		Groups: func(ctx context.Context) (int, error) {
			rows, err := groupRepository.List(ctx, nil)
			return len(rows), err
		},
		Roles: func(ctx context.Context) (int, error) {
			rows, err := identityAccess.ListRoles(ctx, identityaccess.RoleFilter{})
			return len(rows), err
		},
		Devices: func(ctx context.Context) (int, error) {
			rows, err := legacyDevices.List(ctx, nil)
			return len(rows), err
		},
		PermissionCount: func(ctx context.Context) (int, error) {
			rows, err := identityAccess.ListPermissions(ctx, identityaccess.PermissionFilter{})
			return len(rows), err
		},
	}, databaseLogger)

	// Initialize cleanup service
	activeCleanupService := presenceservice.NewPresenceCleanup(
		NewLoggedStudentPresence(db, logger),
		sessionSupervisors,
		NewDeletionAudit(audits.DataDeletion),
		PresenceSettings(settingsService),
		today,
	)
	unregisteredTagScanService, err := NewUnregisteredTagScans(legacyDeviceFleet)
	if err != nil {
		return nil, err
	}

	// Enrollment acceptance grants parents portal access through the public
	// Identity & Access capability instead of the account, mapping and role
	// repositories (#2699); it is the same module instance the auth service
	// delegates its session flows to.
	guardianAccess := identityAccess
	// Data Import (#2708): every accepted row is committed through the owner
	// commands the composer binds. The observer records rows
	// parsed/accepted/rejected per run without personal data.
	dataImports := NewImports(ImportWiring{
		Identity:      guardianAccess,
		Organizations: organizations,
		Groups:        groups, Rooms: rooms,
		Persons: persons, Membership: membership, Workforce: workTime,
		CarePlan: carePlan, Presence: NewLoggedStudentPresence(db, logger),
		InvitationService: identityAccess,
		OpeningBalance: importService.OpeningBalanceImportDeps{
			BalanceAdjustService: timeTracking.OpeningBalances,
			StaffAbsenceService:  timeTracking.VacationTakeovers,
		},
		ConsentHistory: auditCompose.NewConsentRecorder(auditCommand),
		Audit:          auditCommand,
		Observe: func(observation importService.ImportObservation) {
			observeDataImport(observation)
		},
	})

	// Enrollment brands its mails and notifies decisions through the Delivery
	// outbox; the retained request, decision, change-request and rollover
	// services share this capability.
	enrollmentNotifications := enrollmentWiring.Notifications(settingsService, OutboxEnqueuer{Outbox: emailOutboxService})

	// The admin deletion and the retention cleanup of rejected enrollments
	// share one impact preview (#3564).
	enrollmentDeletions := enrollmentWiring.Deletions(EnrollmentDeletionSources{
		Owner: enrollmentRecords, Guardians: enrollmentWiring.Guardians,
		CountAuditAdjustments: audits.EnrollmentOfferingAdjustment.CountForDeletion,
		CountBookings:         carePlan.CountCareOfferingBookings,
		Audit:                 audits.EnrollmentDeletionAudit,
		Delivery: enrollmentDeliveryAdapter{module: deliveryRuntime.Module, tenantID: func(ctx context.Context) (int64, error) {
			id, err := tenant.TenantFromContext(ctx)
			return id.Int64(), err
		}},
		Settings: settingsService,
		Logger:   logger,
	})
	enrollmentDeletionService := enrollmentDeletions.Deletions
	enrollmentRejectedCleanupService := enrollmentDeletions.Cleanup

	// A phase service-window change re-bounds every roster row derived from
	// the phase's offerings, so the templates sourcing them must resync too
	// (#2147 review). The booking materialization composed below provides
	// the resync; the phase administration resolves it on every edit.
	var phaseSourcedTemplates enrollmentOwner.SourcedTemplateResyncer
	enrollmentPhaseService := enrollmentWiring.Phases(EnrollmentPhaseSources{
		LockTemplateRecurrence:          recurrenceLock.LockRecurrenceWrites,
		ValidateCareOfferingPhaseChange: careOfferingPhaseGuard(enrollmentCareOfferingService),
		SourcedTemplates:                func() enrollmentOwner.SourcedTemplateResyncer { return phaseSourcedTemplates },
		Settings:                        settingsService,
		Logger:                          logger.With("service", "enrollment-phase"),
	})

	studentAuditService := peopleCompose.NewStudentAuditService(identityRoot.AuditActor, people.StudentAudit)
	careLifecycleService, err := careplanCompose.NewCareLifecycle(careplanCompose.CareLifecycleDependencies{
		DB: db, Records: carePlan,
		Owners: NewCareLifecycleOwners(CareLifecycleOwnerSources{
			Students: studentRepository, Persons: personRepository, Membership: membership,
			People: persons, Timetable: timetableCapability, Calendar: calendar,
			Presence: NewUnloggedStudentPresence(db), PlanReads: MustPresenceReads(db),
			Enrollment: enrollmentRecords,
		}, NewCareEndRecorder(studentAuditService)),
		LockCareBookingWrites: recurrenceLock.LockRecurrenceWrites,
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return settingsService.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
		},
		Fingerprint: securityruntime.Fingerprint,
		Logger:      logger.With("service", "care_lifecycle"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose care lifecycle: %w", err)
	}
	careParticipation = careLifecycleService
	careplanCompose.WireCareParticipation(careDayService, careLifecycleService)
	// Offering-sourced weekly Gehzeit changes move the same baseline a staff
	// weekly edit does, so they re-derive the auto excusals of the students'
	// future day exceptions too (#2360). The wrapper reuses an ambient tenant
	// transaction and opens one otherwise — the resync's care-day locks are
	// transaction-scoped.
	resyncPickupAutoExcusals := func(ctx context.Context, studentIDs []int64) error {
		return tenant.WithTenantTx(ctx, db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
			for _, studentID := range studentIDs {
				if err := pickupAutoExcusal.ResyncFutureExceptions(txCtx, studentID); err != nil {
					return err
				}
			}
			return nil
		})
	}
	// Care Plan's booking materialization (#3560): the rosters an approval
	// derives, the offering-sourced Regeltermine, the dated offering
	// adjustments and the offering pickup times. Sourced-roster resyncs also
	// refresh already-materialized future occurrences (#2147 review) through
	// the Timetable roster reconciliation.
	careBookings, err := NewBookingMaterialization(BookingMaterializationInputs{
		Catalog: enrollmentCareOfferingService, Timetable: timetableCapability, Rosters: rosterReconciler,
		Students: persons, Periods: calendar, Enrollment: enrollmentRecords,
		Approved: approvedOfferingProjection,
		Settings: settingsService, Bookings: carePlan, Withdrawals: careLifecycleService,
		Adjustments: audits.EnrollmentOfferingAdjustment, Persons: personRepository, Accounts: guardianAccess,
		Pickup: pickupBaselines, PickupRows: carePlan,
		LockRecurrence:              recurrenceLock.LockRecurrenceWrites,
		ResyncPickupAutoExcusals:    resyncPickupAutoExcusals,
		ClearPickupWeekdayExtension: timetableCapability.ClearPickupWeekdayExtension,
		Broadcaster:                 realtimeHub,
		GuardianNotifier:            pillEmitter,
		Today:                       today,
		Logger:                      logger.With("service", "care-plan-bookings"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose care plan booking materialization: %w", err)
	}
	// Care Plan's offerings in the enrollment rows the decision flow, the
	// intake, the change requests, the capacity gate and the reports read.
	enrollmentCareOfferings := enrollmentCompose.NewCareOfferingRecords(carePlan)
	enrollmentDecisions := enrollmentWiring.Decisions(EnrollmentDecisionSources{
		Requests:               enrollmentRecords,
		Children:               enrollmentRecords,
		Guardians:              enrollmentRecords,
		LateInvites:            enrollmentRecords,
		Phases:                 enrollmentRecords,
		Schemas:                enrollmentRecords,
		CareOfferings:          enrollmentCareOfferings,
		DataAccessLog:          audits.DataAccessLog,
		OfferingAdjustments:    audits.EnrollmentOfferingAdjustment,
		Restorations:           audits.EnrollmentRestorationAudit,
		Notifications:          enrollmentNotifications,
		Persons:                personRepository,
		Staff:                  staffRepository,
		Students:               studentRepository,
		StudentEnrollment:      persons,
		Companions:             NewStudentCompanionRepository(carePlan),
		DeleteCompanions:       carePlan.DeleteCompanionEdges,
		StudentGuardians:       studentGuardianRepository,
		GuardianFinancialAudit: audits.GuardianFinancialChange,
		GuardianProfiles:       guardianProfileRepository,
		GuardianPhones:         guardianPhoneRepository,
		PickupSchedules:        arrivalTimetable.NewEnrollmentPickupSchedules(people.PickupSchedules),
		ArrivalSchedules:       arrivalTimetable.NewEnrollmentArrivalSchedules(people.ArrivalSchedules),
		CareBookings:           careBookings,
		GuardianAccess:         guardianAccess,
		StudentAudit:           studentAuditService,
		StudentConsents:        studentConsentService,
		CareWithdrawal:         careLifecycleService,
		Broadcaster:            realtimeHub,
		FrontendURL:            frontendURL,
		ParentsURL:             parentsURL,
		Settings:               settingsService,
		LockTemplateRecurrence: recurrenceLock.LockRecurrenceWrites,
		Pickups: enrollmentCompose.WeeklyPickupHooks{
			// An approved weekly Gehzeit plan takes the student lock BEFORE its
			// weekly rows — the same student → schedule-row → care-day lock
			// order the staff weekly editors use, so the writers cannot
			// deadlock against each other (#2360 review). Uses the ambient
			// tenant transaction; missing students (concurrent offboarding)
			// are skipped.
			LockStudents: func(ctx context.Context, studentIDs []int64) error {
				for _, studentID := range studentIDs {
					if err := persons.LockStudent(ctx, studentID); err != nil {
						if errors.Is(err, peopledirectory.ErrStudentNotFound) {
							continue
						}
						return err
					}
				}
				return nil
			},
			ResyncExcusal: resyncPickupAutoExcusals,
			Snapshot: func(ctx context.Context, studentID int64, date timezone.Date) (map[int]string, error) {
				return pickupAutoExcusal.SnapshotWeeklyPickups(ctx, studentID, date)
			},
			RecordChanges: func(ctx context.Context, studentID int64, date timezone.Date, before map[int]string) error {
				return pickupAutoExcusal.RecordWeeklyPickupChanges(ctx, studentID, date, careplan.WeeklyPickupSnapshot(before))
			},
		},
		Logger: logger.With("service", "enrollment-decision"),
		Today:  today,
	})
	enrollmentDecisionService := enrollmentCompose.PublicDecisions(enrollmentDecisions)
	// Grade transitions rewrite school classes, so they must re-reconcile the
	// offering-sourced templates' Jahrgang-filtered rosters (#2137). The
	// workflow is composed here because Care Plan's booking materialization
	// that provides the resync is composed late; it binds the People
	// Directory, the School Membership, the Timetable roster reconciliation
	// and the recurrence gate, and builds School Structure and Student
	// Presence over the shared database itself (#2711).
	gradeTransitionWorkflow, err := gradetransitioncompose.New(gradetransitioncompose.Dependencies{
		DB: db, Directory: persons, Membership: membership, Rosters: rosterReconciler,
		LockRecurrenceWrites:  recurrenceLock.LockRecurrenceWrites,
		ResyncOfferingRosters: careBookings.ResyncOfferingSourcedTemplates,
		Logger:                logger.With("workflow", "grade_transition"), Audit: auditCommand, Clock: now,
	})
	if err != nil {
		return nil, fmt.Errorf("compose grade transition workflow: %w", err)
	}
	// A care-offering edit changes the wanted roster of every template sourcing
	// it and the pickup projection (#2147 review); a phase service-window
	// change re-bounds the rosters derived from its offerings. The catalog and
	// the phase administration resolve the booking materialization composed
	// here on every edit.
	careOfferingResyncer = careBookings
	phaseSourcedTemplates = careBookings

	// One capacity gate for the submissions, their edits, the change-request
	// approvals and the restore of a withdrawn request.
	enrollmentOfferingCapacity := enrollmentWiring.Capacity(settingsService)
	// Consent rows back both the notification preferences below and the
	// opt-in "Neue Anmeldung" mail of the intake (#3780).
	notificationConsent, err := communicationCompose.NewNotificationConsent(communicationCompose.NotificationConsentConfig{
		DB:      db,
		Observe: observeCommunication,
	})
	if err != nil {
		return nil, err
	}
	enrollmentRequestService := enrollmentWiring.Intake(EnrollmentIntakeSources{
		Requests:           enrollmentRecords,
		ParentChanges:      enrollmentRecords,
		Children:           enrollmentRecords,
		Bookings:           enrollmentWiring.CareBookings,
		Guardians:          enrollmentRecords,
		LateInviteRepo:     enrollmentRecords,
		CareOfferingRepo:   enrollmentCareOfferings,
		Capacity:           enrollmentOfferingCapacity,
		Catalog:            enrollmentRecords,
		SchoolRepo:         people.EnrollmentSchools,
		Notifications:      enrollmentNotifications,
		StudentRepo:        studentRepository,
		GuardianAuthorizer: studentGuardianRepository,
		RateLimitRepo:      enrollmentRecords,
		OutboxEnqueuer:     OutboxEnqueuer{Outbox: emailOutboxService},
		Settings:           settingsService,
		AdminSubscribers:   enrollmentWiring.AdminSubscribers(notificationConsent, identityAccess),
		ManualDecider:      enrollmentDecisions,
		FrontendURL:        frontendURL, // admin notification email
		ParentsURL:         parentsURL,  // parent confirmation/status emails
		Logger:             logger.With("service", "enrollment-request"),
	})

	enrollmentReports := enrollmentWiring.Reports(EnrollmentReportSources{
		Owner:             enrollmentRecords,
		Offerings:         enrollmentCareOfferings,
		AccessLog:         audits.DataAccessLog,
		Students:          studentRepository,
		Persons:           personRepository,
		Groups:            education.NewGroupNames(groupRepository),
		StudentGuardians:  studentGuardianRepository,
		Companions:        NewStudentCompanionRepository(carePlan),
		ClassListEntries:  NewClassListEntryRosterReader(membership),
		PickupSchedules:   pickupScheduleService,
		CareParticipation: careLifecycleService,
		Settings:          settingsService,
	})
	// The school portal's class-day capability (#2701): the day report over
	// Enrollment's day roster, the supervision sheet (#2527) and the one
	// write seam of moto schule (#2970).
	classDayService := NewClassDay(ClassDaySources{
		Caller:                 userContextService,
		Reports:                enrollmentReports,
		StatusDays:             statusDays,
		PickupTimes:            pickupScheduleService,
		ArrivalTimes:           arrivalScheduleService,
		CareDays:               careDayService,
		Companions:             NewStudentCompanionRepository(carePlan),
		Students:               studentRepository,
		Persons:                personRepository,
		StudentGuardians:       studentGuardianRepository,
		AccessLog:              audits.DataAccessLog,
		ClassArrivalExceptions: arrivalScheduleService,
		Settings:               settingsService,
		BlockStarts:            timetableOperationsService,
		Broadcaster:            realtimeHub,
		Logger:                 logger.With("service", "class-day-arrival-exceptions"),
	})

	studentService := peopleCompose.NewStudentService(
		peopleCompose.NewStudentDirectory(persons),
		persons,
		studentRepository,
	)
	// Created before the change-request service: its multi-child approval takes
	// the companion lock order through this service.
	companionService, err := careplanCompose.NewCompanions(carePlan, NewCompanionStudents(studentRepository, persons, studentAuditService))
	if err != nil {
		return nil, fmt.Errorf("compose care plan companions: %w", err)
	}

	// Child documents (#777): metadata, per-category authority and the
	// per-child access gate for the Dokumente tab. Needs the user context to
	// answer "is this caller verified staff", so it is wired after it.
	studentDocumentDeps, err := StudentDocumentDependencies(carePlan, studentRepository, userContextService, audits.StudentFieldEdit, audits.DataAccessLog)
	if err != nil {
		return nil, fmt.Errorf("compose care plan documents: %w", err)
	}
	studentDocumentDeps.DB = db
	studentDocumentService, err := careplanCompose.NewStudentDocuments(studentDocumentDeps)
	if err != nil {
		return nil, fmt.Errorf("compose care plan documents: %w", err)
	}

	enrollmentChangeRequestService := enrollmentWiring.ChangeRequests(EnrollmentChangeRequestSources{
		Bookings:             enrollmentWiring.CareBookings,
		Requests:             enrollmentRecords,
		Children:             enrollmentRecords,
		Guardians:            enrollmentRecords,
		LateInviteRepo:       enrollmentRecords,
		CareOfferingRepo:     enrollmentCareOfferings,
		Capacity:             enrollmentOfferingCapacity,
		Catalog:              enrollmentRecords,
		Notifications:        enrollmentNotifications,
		GuardianProfileRepo:  guardianProfileRepository,
		GuardianPhoneRepo:    guardianPhoneRepository,
		GuardianInvitations:  enrollmentWiring.GuardianInvitations(identityAccess),
		PersonRepo:           personRepository,
		StudentRepo:          studentRepository,
		GuardianAuthorizer:   studentGuardianRepository,
		Decisions:            enrollmentDecisions,
		BookingGates:         careBookings,
		CompanionGraphLocker: companionGraphCoordinator{CompanionLocks: companionService, strandings: studentRepository},
		Settings:             settingsService,
		OutboxEnqueuer:       OutboxEnqueuer{Outbox: emailOutboxService},
		FrontendURL:          frontendURL,
		ParentsURL:           parentsURL,
		Logger:               logger.With("service", "enrollment-change-request"),
	})

	// The rollover approves auto-renewed rows through the decision flow on
	// the rollover_auto_approve=true deadline path.
	enrollmentRolloverService := enrollmentWiring.Rollovers(EnrollmentRolloverSources{
		Bookings:         enrollmentWiring.CareBookings,
		Phases:           enrollmentRecords,
		Requests:         enrollmentRecords,
		Children:         enrollmentRecords,
		Catalog:          enrollmentCareOfferingService,
		Notifications:    enrollmentNotifications,
		PhaseEligibility: enrollmentPhaseService,
		Outbox:           OutboxEnqueuer{Outbox: emailOutboxService},
		Settings:         settingsService,
		Decisions:        enrollmentDecisions,
		ParentsURL:       parentsURL,
		Logger:           logger.With("service", "enrollment-rollover"),
	})
	requestReviewPolicy := NewParentRequestReviewPolicy(callerContext.ParentRequestReviews)

	// One append-only ledger for every parent request, shared by all four
	// domains so a request's history survives edits, decisions and corrections
	// the request rows themselves overwrite (#2267).
	parentRequestEvents := parentRequestEventRepository

	// The sharing rules live in the parents domain, which is composed after the
	// request services it serves, so the sharing port resolves that service
	// lazily through requestShareVisibility (#2267, story 47).
	var requestShareVisibility communicationCompose.ParentShareVisibilityResolver
	requestShares := shareVisibilityFunc(func(ctx context.Context, studentID int64, requestType string, requestID int64) ([]int64, error) {
		if requestShareVisibility == nil {
			return nil, nil
		}
		return requestShareVisibility.SharedRecipientAccountIDs(ctx, studentID, requestType, requestID)
	})

	// Care-schedule change requests (#1803): the schedule-domain request
	// lifecycle (create / withdraw / staff decide + apply), decoupled from the
	// chat.
	careRequestService := NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
		carePlan,
		persons,
		arrivalScheduleService,
		pickupScheduleService,
		NewLoggedStudentPresence(db, logger),
		pickupAutoExcusal,
		carePlan,
		userContextService,
		pillEmitter,
		realtimeHub,
		requestReviewPolicy,
		identityRoot.Permissions,
		parentRequestEvents,
		logger.With("service", "care-requests"),
		studentAuditService,
		WithRequestShareVisibility(requestShares),
		WithCareRequestToday(today),
	)

	// Post-enrollment offering changes (#1665, #3561): the parents portal
	// submits them, staff decide them on the same review page, and an
	// approval applies the switch through Care Plan's dated offering
	// adjustment. Permanent pickup-time changes switch to a matching offering
	// through the same review.
	offeringChanges, err := enrollmentWiring.OfferingChanges(OfferingChangeInputs{
		CarePlan: carePlan, Enrollment: enrollmentRecords, Students: studentRepository,
		Withdrawals: NewCareWithdrawalCompletionRepository(carePlan), Settings: settingsService,
		Planning: enrollmentWiring.OfferingPlanning,
		Bookings: careBookings, Reviews: requestReviewPolicy, Permissions: identityRoot.Permissions, Emitter: pillEmitter,
		Events: parentRequestEvents, Shares: requestShares, Today: today,
		Logger: logger.With("service", "offering-change-requests"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose care plan offering changes: %w", err)
	}
	pickupAdjustments, err := NewPickupAdjustments(PickupAdjustmentInputs{
		CarePlan: carePlan, PickupSchedules: pickupScheduleService, ArrivalSchedules: arrivalScheduleService,
		Baselines: pickupBaselines, Offerings: offeringChanges, Settings: settingsService,
		Audit: studentAuditService, Students: studentRepository, Today: today,
	})
	if err != nil {
		return nil, fmt.Errorf("compose care plan pickup adjustments: %w", err)
	}

	// Review access is one cross-domain policy: admins remain school-wide;
	// group leaders are opt-in and limited to their current groups. Attach it
	// to every request service so the unified queue and the legacy per-type
	// decision routes cannot disagree about who may see or decide a request.
	// The notification router and the consent service are built here, ahead of
	// their consumers: messaging, the calendar (#1671) and the announcement
	// producer all need them, and messaging is constructed first.
	if !vapidConfig.Configured() {
		logger.Info("web push disabled: VAPID keys not configured (VAPID_PUBLIC_KEY / VAPID_PRIVATE_KEY / VAPID_SUBSCRIBER)")
	}
	notificationsService := notifications.NewServiceWithDeliveryObserver(
		settingsService,
		logger.With("service", "notifications"),
		notifications.DeliveryObserver(observeDelivery),
		notifications.NewSSEChannel(realtimeHub, notifications.WithGuardianChildAccess(
			db, studentGuardianRepository, logger.With("channel", "sse"))),
		notifications.NewDurableWebPushChannel(
			db, pushSubscriptions, vapidConfig,
			deliveryCompose.NewDurablePushOutbox(deliveryRuntime.Module), logger.With("channel", "web_push"),
		),
	)
	notificationPreferencesService := notifications.NewPreferenceService(
		notificationConsent,
		settingsService,
		db,
		identityAccess,
	)
	staffNotificationRecipients := notifications.NewStaffRecipientResolver(
		notificationPreferencesService,
		studentRepository,
		groupRepository,
		staffRepository,
		identityAccess,
		settingsService,
		workforceCompose.WorkPresence(workTime),
	)
	staffParentMessageNotifier := notifications.NewStaffParentMessageNotifier(
		notificationsService,
		staffNotificationRecipients,
		parentPortal.ParentMessageThread,
		db,
		logger.With("producer", "staff_parent_message_notifications"),
	)

	// Decided change requests reach the parent's devices through the pill
	// emitter, which is where all three request flows already converge (#1671).
	pillEmitter.WithDecisionNotifications(notificationsService, notificationPreferencesService)

	absenceNotifier := notifications.NewAbsenceNotifier(
		notificationsService,
		staffNotificationRecipients,
		db,
		logger.With("producer", "absence_notifications"),
	)

	// Excused-absence approval requests (#1845): the optional office-approval
	// gate for parent-submitted excused absences. Reuses the same review queue,
	// badge and pill machinery as the care-schedule requests; on approval it
	// writes the excused status days directly. The workflow is owned by Care
	// Plan (#3093); this root adapts the legacy directories, the review policy,
	// and the effect sinks. The sharing port is the lazy requestShares above.
	excusedRequestService, err := NewExcusedAbsenceRequestService(ExcusedRequestServiceWiring{
		CarePlan: carePlan, Students: studentRepository, Persons: personRepository,
		Scope:   ParentRequestReviewScope(requestReviewPolicy, identityRoot.Permissions),
		Emitter: pillEmitter, Broadcaster: realtimeHub, Events: parentRequestEvents,
		notifier: absenceNotifier, observe: observeCarePlan,
		shares: requestShares,
		Logger: logger.With("service", "excused-requests"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose excused absence requests: %w", err)
	}

	messagingService := communicationCompose.NewParentMessaging(communicationCompose.ParentMessagingConfig{
		ThreadRepo:  parentPortal.ParentMessageThread,
		MessageRepo: parentMessages,
		ReadRepo:    parentPortal.ParentMessageRead,
		Persons:     usersService,
		UserContext: callerContext,
		Settings:    settingsService,
		Broadcaster: realtimeHub,
		DB:          db,
		Logger:      logger.With("service", "messaging"),
		Notifier:    notificationsService,
		Preferences: notificationPreferencesService,
		// E-Mail an den Sorgeberechtigten bei neuer OGS-Nachricht (#2307): der
		// Rueckfall fuer alle, die Push nicht eingerichtet haben.
		Outbox:           OutboxEnqueuer{Outbox: emailOutboxService},
		GuardianProfiles: guardianProfileRepository,
		Schools:          people.SchoolNames,
		LoginImages:      settingsService,
		ParentsURL:       parentsURL,
		Observe:          observeCommunication,
	})

	// OGS-internal colleague chat (#2598). Shares the transport with the
	// parent-OGS messenger (SSE hub + push) but none of its authorization:
	// access here is thread membership, nothing else.
	staffMessagingService := communicationCompose.NewStaffMessaging(communicationCompose.StaffMessagingConfig{
		ThreadRepo:  staffStore.NewStaffMessageThreadRepository(db),
		MessageRepo: staffStore.NewStaffMessageRepository(db),
		ReadRepo:    parentPortal.StaffMessageRead,
		Persons:     usersService,
		Settings:    settingsService,
		Broadcaster: realtimeHub,
		DB:          db,
		Logger:      logger.With("service", "staffmessaging"),
		Notifier:    notificationsService,
		Preferences: notificationPreferencesService,
		Observe:     observeCommunication,
	})

	calendarSvc := NewCalendarPortal(CalendarDependencies{
		CalendarFacts: CalendarFacts{
			StaffRepo:            staffRepository,
			StudentRepo:          studentRepository,
			GuardianProfileRepo:  NewCalendarGuardianDirectory(guardianProfileRepository, persons),
			StudentGuardianRepo:  studentGuardianRepository,
			ChildRepo:            careplanCompose.NewParentChildLinks(parentPortal.ParentChild),
			GroupRepo:            groupRepository,
			InstanceStaffRepo:    timetableRepos.InstanceStaff,
			ActivityInstanceRepo: timetableRepos.ActivityInstance,
			RoomRepo:             roomRepository,
			StaffShiftRepo:       shiftRows,
			ShiftTypeRepo:        shiftTypeRows,
			SchoolRepo:           organizations,
			AccountRepo:          identityAccess,
			StaffFeedRepo:        identityAccess,
			PersonRepo:           personRepository,
		},
		Appointments:           appointmentCapability,
		UserContext:            userContextService.Caller(),
		Runtime:                calendarCompose.NewTenantRuntime(db),
		CalendarRenderer:       SchoolCalendarRendererAdapter{Renderer: calendar},
		Outbox:                 emailOutboxService,
		PushOutbox:             deliveryCompose.NewDurablePushOutbox(deliveryRuntime.Module),
		Settings:               settingsService,
		CalDAVPolicy:           calendarCalDAVPolicy{settings: settingsService},
		StaffFeedTombstoneRepo: schoolCalendarCompose.NewFeedHistory(db),
		ParentsURL:             parentsURL,
		FrontendURL:            frontendURL,
		CalDAVURL:              publicAPIURL,
		Notifier:               notificationsService,
		ReminderNotifier:       notificationsService,
		Preferences:            notificationPreferencesService,
		Logger:                 logger.With("service", "calendar"),
	})

	// The parent portal writes the child's care profile through Care Plan.
	parentCareProfiles, err := careplanCompose.NewStudentProfiles(db, func(careplanCompose.Observation) {})
	if err != nil {
		return nil, fmt.Errorf("compose parent portal care profiles: %w", err)
	}
	// The photo lifecycle exists only after EnableStudentPhotos runs in the
	// API bootstrap, so the parent service resolves it on use.
	var factory *Factory
	parentService := parentportalcompose.New(parentportalcompose.Dependencies{
		CareProfiles:              parentCareProfiles,
		ChildRepo:                 parentPortal.ParentChild,
		EnrollablePhaseRepo:       parentPortal.ParentEnrollablePhase,
		EnrollmentSettings:        settingsService,
		EnrollmentRequestRepo:     parentPortal.ParentEnrollmentRequest,
		GuardianProfileRepo:       guardianProfileRepository,
		Attendance:                NewLoggedStudentPresence(db, logger),
		StatusDayRepo:             statusDays,
		MealPlan:                  mealPlan,
		StudentRepo:               studentRepository,
		CarePlan:                  carePlan,
		People:                    persons,
		PickupAutoExcusal:         pickupAutoExcusal,
		Settings:                  settingsService,
		Broadcaster:               realtimeHub,
		PersonRepo:                personRepository,
		ChangeRequestRepo:         NewStudentDataChangeRequestRepository(carePlan),
		CareRequestRepo:           NewCareScheduleChangeRequestRepository(carePlan),
		ExcusedRequestRepo:        NewExcusedAbsenceRequestRepository(carePlan),
		OfferingChangeRequestRepo: enrollmentCompose.NewOfferingChangeRecords(carePlan),
		FamilyProtectionEvents:    familyProtectionRepository,
		ParentRequestShares:       parentRequestShareRepository,
		ParentRequestEvents:       parentRequestEvents,
		StudentAudit:              studentAuditService,
		MessageThreadRepo:         parentPortal.ParentMessageThread,
		MessageRepo:               parentMessages,
		MessageReadRepo:           parentPortal.ParentMessageRead,
		Conversations: communicationCompose.NewParentConversationCore(communicationCompose.ParentConversationConfig{
			ThreadRepo: parentPortal.ParentMessageThread, MessageRepo: parentMessages, ReadRepo: parentPortal.ParentMessageRead,
			Broadcaster: realtimeHub, Logger: logger.With("service", "parent"),
		}),
		ParentMessageNotifier: staffParentMessageNotifier,
		ArrivalSchedules:      arrivalScheduleService,
		PickupSchedules:       pickupScheduleService,
		CareRequests:          careRequestService,
		ExcusedRequests:       excusedRequestService,
		Emitter:               pillEmitter,
		AnnouncementRepo:      parentPortal.ParentAnnouncement,
		GuardianInvites:       identity.ParentGuardianAccess,
		GuardianInvitations:   identity.GuardianInvitations,
		StudentGuardianRepo:   studentGuardianRepository,
		GuardianPhoneRepo:     guardianPhoneRepository,
		StudentConsents:       studentConsentService,
		StudentPhotos: func() parentportal.StudentPhotoUnlinker {
			if factory == nil || factory.StudentPhotos == nil {
				return nil
			}
			return factory.StudentPhotos
		},
		AbsenceNotifier:  absenceNotifier,
		CarePeriods:      enrollmentWiring.ParentCarePeriods,
		OfferingHistory:  enrollmentWiring.ParentOfferingHistory,
		CareOfferingRepo: enrollmentCareOfferings,
		OfferingChanges:  offeringChanges,
		Logger:           logger.With("service", "parent"),
		Now:              now,
	})

	parentAnnouncementService := communicationCompose.NewParentAnnouncements(communicationCompose.ParentAnnouncementConfig{
		Repo:             parentPortal.ParentAnnouncement,
		Settings:         settingsService,
		Outbox:           emailOutboxService,
		PushOutbox:       deliveryCompose.NewDurablePushOutbox(deliveryRuntime.Module),
		Notifier:         notificationsService,
		ReminderNotifier: notificationsService,
		Preferences:      notificationPreferencesService,
		Deliveries:       announcementDeliveryAdapter{module: deliveryRuntime.Module},
		ParentsURL:       parentsURL,
		Logger:           logger.With("service", "announcement"),
	})

	// Bind the instance service's cancellation-notice publisher now that the
	// announcement service exists.
	guardianNoticePublisher = parentAnnouncementService

	// School file storage (#2596) and the attachments of Elternmitteilungen
	// (#2890) through the public File Storage capability (#2707). Die Datei
	// gehört der Dateiablage, der Empfängerkreis der Mitteilung: beide Seiten
	// zeigen aufeinander, also werden sie hier verbunden, wo alle drei Dienste
	// existieren. Ohne diese Verdrahtung verweigern die Anhang-Pfade den
	// Dienst, statt ohne Prüfung zu entscheiden.
	var fileStoreService *filestorageModule.Module
	if fileStorage.Objects != nil {
		fileStoreService, err = filestorageCompose.New(filestorageCompose.Dependencies{
			DB:               db,
			Objects:          fileStorage.Objects,
			Identity:         guardianAccess,
			People:           persons,
			Settings:         FileStorageSettings{Service: settingsService},
			Events:           FileStorageEvents{Repo: audits.FileEvent},
			HasPermission:    securityruntime.HasPermission,
			Announcements:    parentAnnouncementService,
			GuardianAudience: parentService,
			Observe:          fileStorage.Observe,
			Logger:           logger.With("service", "filestore"),
			Now:              now,
		})
		if err != nil {
			return nil, err
		}
		parentAnnouncementService.SetAttachmentPurger(fileStoreService)
	}

	// Operator provisioning belongs to Organisation & Tenancy (#3253); the
	// retained owners it touches are bound through its provisioning seams.
	provisioningAdapters, err := NewOperatorProvisioningAdapters(OperatorProvisioningDependencies{
		PersonStudents: peopleCompose.NewLegacyRepositoryRuntime(db),
		Devices:        deviceFleet,
		Persons:        persons,
		Membership:     membership,
		PersonRepo:     personRepository,
		StaffRepo:      staffRepository,
		Accounts:       identityAccess,
		ActiveGroups:   sessionRecords,
		Supervisors:    groupSupervisions,
		Categories:     arrivalTimetable.NewActivityCategorySeeder(timetableRepos.ActivityCategory),
		AuditLog:       operatorAuditLog,
	})
	if err != nil {
		return nil, fmt.Errorf("compose operator provisioning adapters: %w", err)
	}
	operatorProvisioningService, err := newOperatorProvisioning(operatorProvisioningSources{
		accounts:       NewOperatorAccountDirectory(identityAccess, persons, membership, organizations),
		organizations:  organizations,
		adapters:       provisioningAdapters,
		sessions:       identityAccess,
		invitations:    identityAccess,
		provisioning:   identityAccess,
		administration: identityAccess,
		schoolIdentity: identityAccess,
		roles:          identityAccess,
		settings:       settingsService,
		logger:         platformLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("compose operator provisioning: %w", err)
	}

	listExportService := lists.NewRendererService()
	// The Notfallliste is the emergency snapshot read projection (#2704): the
	// owner facades are its tenant-safe reads, the retained student and
	// guardian repositories, the presence-mode read and the renderer its
	// compatibility bindings.
	emergencyService, err := emergencysnapshotlegacy.New(emergencysnapshotlegacy.Sources{
		Presence:     NewLoggedStudentPresence(db, logger),
		PresenceMode: activeService,
		Students:     studentRepository,
		Persons:      persons,
		Contacts:     studentGuardianRepository,
		Rooms:        rooms,
		Renderer:     listExportService,
		Now:          now,
		Logger:       logger.With("module", "emergency-snapshot"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose emergency snapshot projection: %w", err)
	}
	// The slot lists are the class-day read projection (#2701): the owner
	// facades are its tenant-safe reads, the retained schedule services its
	// compatibility bindings.
	classDayReads, err := arrivalTimetable.NewPresenceReads(db)
	if err != nil {
		return nil, fmt.Errorf("compose class day timetable reads: %w", err)
	}
	slotListsService := classdayCompose.NewSlotLists(classdayCompose.SlotListDependencies{
		Timetable:       NewClassDayTimetableRows(classDayReads),
		Presence:        NewLoggedStudentPresence(db, logger),
		CarePlan:        carePlan,
		Students:        persons,
		Persons:         persons,
		Groups:          groups,
		Rooms:           rooms,
		CareDays:        careDayService,
		PickupTimes:     pickupScheduleService,
		ArrivalTimes:    arrivalScheduleService,
		PickupBaselines: pickupBaselines,
		Settings:        settingsService,
		UserContext:     userContextService,
		ListExport:      listExportService,
	})

	// Printable weekly plans (#2079) are the Document Rendering plan export
	// capability (#2706): a pure projection over the same reads the two
	// planning screens use — it renders, it never writes. The root binds its
	// ports to the owners' public reads.
	planExportService := planexportCompose.New(planexportCompose.Sources{
		Overview:       shiftPlanning.Overview,
		ShiftTypes:     workTime,
		Instances:      arrivalTimetable.NewTimetableInstanceReads(timetableRepos.ActivityInstance),
		InstanceStaff:  arrivalTimetable.NewTimetableInstanceStaffReads(timetableRepos.InstanceStaff),
		Students:       timetableRepos.InstanceStudent,
		Rooms:          rooms,
		Staff:          workforce.PlanExportStaff,
		ActivityGroups: arrivalTimetable.NewTimetableGroupReads(timetableRepos.ActivityGroup),
		PlanningTracks: timetableCapability,
		ClosingDays:    planExportClosingDays{calendar: calendar},
		Holidays:       planExportHolidays{calendar: calendar},
		Renderer:       listExportService,
		Logger:         logger.With("service", "plan_export"),
	})

	pushSubscriptionsService := notifications.NewPushSubscriptionService(
		db,
		pushSubscriptions,
		identityAccess,
		vapidConfig,
		logger.With("service", "push_subscriptions"),
	)

	pwaUsageService := pwa.NewUsageService(
		db,
		pwausage.NewPWAStandaloneUsageRepository(db),
		pwaUsageCounts{provisioning: operatorProvisioningService},
		identityAccess,
		settingsService,
		logger.With("service", "pwa_usage"),
	)

	remindersService := reminderCompose.NewQuery(reminderPorts.QueryDependencies{
		Clock:        ReminderClock(),
		CurrentStaff: ReminderStaffIdentity(userContextService),
		Settings:     ReminderSettings{settingsService},
		Attendance:   NewLoggedStudentPresence(db, logger),
		Pickup:       ReminderPickupReader{Source: pickupScheduleService},
		Instance:     ReminderTimetableReader{Source: timetableCapability},
		Room:         ReminderRoomReader{Source: rooms},
		Student:      ReminderStudentReader{Source: studentRepository},
		Person:       ReminderPersonReader{Source: personRepository},
		Supervision:  ReminderSupervisionReader{Source: activeService, Presence: NewLoggedStudentPresence(db, logger)},
		Visits:       ReminderVisitReader{Source: NewLoggedStudentPresence(db, logger)},
		Logger:       logger.With("service", "reminders"),

		// Bulk readers for ComputeBatch. They answer the three genuinely
		// per-person facts for the whole tenant in one query each, which is what
		// keeps the per-minute cost flat in the number of staff.
		BulkSupervision:   ReminderBulkSupervisionReader{Source: groupSupervisions},
		BulkInstanceStaff: ReminderTimetableReader{Source: timetableCapability},
	})

	studentStatusDayService := presenceservice.NewStatusDays(
		statusDays,
		NewManualPartialAbsenceDates(carePlan),
		db,
		carePlan.LockExceptionDay,
		now,
	)
	studentStatusDayOverviewService := presenceservice.NewStatusDayOverviews(statusDays, people.StatusDayPeople(usersService))
	ogsGroupLiveService, err := grouplivelegacy.New(grouplivelegacy.Sources{
		Presence:          NewLoggedStudentPresence(db, logger),
		People:            people.GroupRosterPeople(usersService),
		Education:         educationService,
		Substitutions:     substitutionService,
		UserContext:       identity.GroupLiveCaller,
		Active:            activeService,
		Settings:          settingsService,
		Pickups:           pickupScheduleService,
		Arrivals:          arrivalScheduleService,
		PlannedStudentIDs: instanceService.GetPlannedStudentIDsByDate,
		CareDays:          careDayService,
		CareParticipation: careLifecycleService,
		ExcusedRequests:   excusedRequestService,
		StatusDays:        studentStatusDayService,
		Logger:            logger.With("service", "ogs-group-live"),
		Now:               now,
	})
	if err != nil {
		return nil, fmt.Errorf("compose OGS group live projection: %w", err)
	}

	supervisionDashboardService, err := supervisiondashboardlegacy.New(supervisiondashboardlegacy.Sources{
		Active:       activeService,
		ActiveGroups: OpenRoomSessionPresence{NewLoggedStudentPresence(db, logger), timetableCapability},
		OpenVisits:   activeService,
		Rooms:        OpenRoomDirectory{Rooms: rooms},
		UserContext:  identity.SupervisionCaller,
		Education:    educationService,
		Schulhof:     schulhofService,
		Operations:   timetableOperationsService,
		Settings:     settingsService,
		Pickups:      pickupScheduleService,
		Arrivals:     arrivalScheduleService,
		Now:          now,
	})
	if err != nil {
		return nil, fmt.Errorf("compose supervision dashboard projection: %w", err)
	}

	timetableData, err := NewTimetableData(TimetableDataInputs{
		Rows:             timetableRows,
		ArrivalBaselines: arrivalBaselines,
		PickupBaselines:  pickupBaselines,
		Visits:           NewLoggedStudentPresence(db, logger),
		Groups:           timetableCapability,
		Sessions:         sessionRecords,
		ConflictAcks:     timetableCapability,
		BlockMetadata:    arrivalTimetable.NewBlockDisplayMetadata(db, timetableblockdisplay.List),
		Transactional:    true,
		Logger:           logger.With("service", "timetable-data"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable data: %w", err)
	}
	// The template writes of the Timetable owner (#3424 slice S2), including
	// "Dieser und alle folgenden": a split caps the old template, creates a
	// successor and re-plans the window through the materialization, keeping
	// the instance lifecycle's Vertretungsplan overrides. Enrollment's roster
	// resync reconciles offering-sourced rosters (#2137, #2147).
	timetableTemplateDeps, err := TimetableTemplateDependencies(TimetableTemplateInputs{
		Rows:                 timetableTemplateRows,
		PlanningTracks:       planningTrackService,
		Materialization:      materializationService,
		Deviations:           instanceService.SeriesDeviations(),
		CareOfferings:        enrollmentCareOfferingService,
		ResyncOfferingRoster: timetableOfferingRosterResync(careBookings),
		RecurrenceLock:       recurrenceLock,
		Broadcaster:          realtimeHub,
		Logger:               logger.With("service", "timetable-templates"),
		Today:                today,
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable templates: %w", err)
	}
	timetableTemplateDeps.DB = db
	timetableTemplates, err := arrivalTimetable.NewTemplateAdministration(timetableTemplateDeps)
	if err != nil {
		return nil, fmt.Errorf("compose timetable templates: %w", err)
	}
	instanceSeriesConverter, err := arrivalTimetable.NewInstanceSeriesConversion(arrivalTimetable.InstanceSeriesConversionDependencies{
		DB:             db,
		InstanceRepo:   timetableRepos.ActivityInstance,
		Lifecycle:      instanceService,
		Templates:      timetableTemplates,
		RecurrenceLock: recurrenceLock,
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable series conversion: %w", err)
	}
	attendanceCorrections, err := NewTimetableAttendanceCorrections(TimetableCorrectionInputs{
		Rows:   timetableRows,
		Trail:  audits.AttendanceCorrection,
		People: personRepository,
		Logger: logger.With("service", "attendance-correction"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable attendance corrections: %w", err)
	}
	timetablePlanning := TimetablePlanning{
		Templates:             timetableTemplates,
		RecurrenceLock:        recurrenceLock,
		Data:                  timetableData,
		ConflictDetection:     timetableConflicts,
		AttendanceCorrections: attendanceCorrections,
		Deviations:            staffDeviations,
	}
	// Stammdaten decisions (#3354) are Care Plan's: the request, the decision,
	// the correction and the co-guardian notice. People Directory keeps the
	// child's record and its change history behind the decision's records
	// port; the review scope is the one the Stammdaten queue is listed with.
	masterDataDecisions, err := people.MasterDataDecisions(MasterDataDecisionWiring{
		CarePlan: carePlan, Fields: persons, Students: studentRepository, Persons: personRepository,
		Audit: studentAuditService, Scope: people.ParentRequestWriteScope(requestReviewPolicy, identityRoot.Permissions),
		Emitter: pillEmitter, Broadcaster: realtimeHub, Events: parentRequestEvents, Shares: requestShares,
		Logger: logger.With("service", "master-data-review"), Today: today,
	})
	if err != nil {
		return nil, fmt.Errorf("compose master data decisions: %w", err)
	}
	// Co-guardian notices (#2267, story 47). A staff decision on one parent's
	// request tells the OTHER guardians that the child's care changed. Whoever
	// the parent explicitly shared the request with gets the full pill; every
	// other guardian gets a neutral line with no reason and no author.
	//
	// The sharing rules live in the parents domain, which is composed after
	// the request services, and the request domains must not import it just to
	// ask who a request was shared with. Every request domain reads it lazily
	// through requestShares, which resolves the parents service bound here.
	var _ communicationCompose.ParentShareVisibilityResolver = parentService
	if resolver, ok := any(parentService).(communicationCompose.ParentShareVisibilityResolver); ok {
		requestShareVisibility = resolver
	}

	// The cross-kind bulk approval and conflict resolution (#2267, stories
	// 6-10) coordinate the four request queues. Every queue is bound at
	// construction, or the resolve route would answer conflict_kind_unsupported
	// for the missing one. The coordinator records ONLY the staff-entered
	// result; every verdict it takes goes through a queue's own decision,
	// which writes its own decided event.
	parentRequestCoordinator, err := people.ParentRequestCoordinator(ParentRequestCoordinatorWiring{
		MasterData: masterDataDecisions, Excused: excusedRequestService, Care: careRequestService,
		Offering: enrollmentWiring.OfferingConflicts(offeringChanges), Events: parentRequestEvents,
		Permissions: identityRoot.Permissions,
	})
	if err != nil {
		return nil, fmt.Errorf("compose parent request coordinator: %w", err)
	}

	factory = &Factory{
		settingsRuntimeDB:       db,
		Audit:                   auditCommand,
		Active:                  activeService,
		ActiveCleanup:           activeCleanupService,
		WorkSession:             timeTracking,
		Activities:              activitiesService,
		Education:               educationService,
		Substitution:            substitutionService,
		GradeTransition:         gradeTransitionWorkflow,
		Facilities:              facilitiesService,
		Schulhof:                schulhofService,
		WC:                      wcService,
		IoT:                     iotService,
		Settings:                settingsService,
		PayrollStatus:           payrollStatusService,
		StaffShifts:             shiftPlanning.Planning(planExportService),
		StaffAssignments:        shiftPlanning.Assignments,
		StaffScheduleOverview:   shiftPlanning.Overview,
		ShiftTypes:              shiftPlanning.ShiftTypes,
		PlanningTracks:          planningTrackService,
		PickupSchedule:          pickupScheduleService,
		PartialAbsence:          partialAbsenceService,
		ArrivalSchedule:         arrivalScheduleService,
		CareDay:                 careDayService,
		TimetableBridge:         timetableBridgeService,
		Materialization:         materializationService,
		TimetableCleanup:        timetableCleanupService,
		TimeTrackingCleanup:     timeTrackingCleanupService,
		StudentChangeLogCleanup: studentChangeLogCleanupService,
		Instance:                instanceService,
		AutoStart:               autoStartService,
		AutoEnd:                 autoEndService,
		TimetableOperations:     timetableOperationsService,
		Users:                   usersService,
		StaffDocuments:          staffAdmin,
		StudentDocuments:        studentDocumentService,
		FileStore:               fileStoreService,
		CaregiverCapability:     caregiverCapabilityService,
		Guardian:                guardianService,
		PeopleDirectory:         persons,
		Database:                databaseService,
		DatabaseStatsCapabilities: func(ctx context.Context) transactionruntime.DatabaseStatsCapabilities {
			return securityruntime.DatabaseStatsCapabilities(identityRoot.Permissions(ctx))
		},
		Import:               dataImports.Student,        // Student import service
		StaffImport:          dataImports.Staff,          // Staff (Mitarbeiter) import service
		ClassListImport:      dataImports.ClassList,      // Class-list entry import (#2382)
		OpeningBalanceImport: dataImports.OpeningBalance, // Opening balance import (#2132)
		ListExport:           listExportService,
		PlanExport:           planExportService,
		Emergency:            emergencyService,
		SlotLists:            slotListsService,
		Reminders: reminder.Module{
			Query:                     remindersService,
			Command:                   reminderCompose.NewCommand(db, CalendarReminderDelivery(calendarSvc)),
			ParentAnnouncementCommand: parentAnnouncementService,
		},
		Notifications:            notificationsService,
		PushSubscriptions:        pushSubscriptionsService,
		PWAUsage:                 pwaUsageService,
		NotificationPreferences:  notificationPreferencesService,
		AbsenceNotifier:          absenceNotifier,
		RealtimeHub:              realtimeHub, // Expose SSE hub for API layer
		Mailer:                   mailer,
		DefaultFrom:              defaultFrom,
		FrontendURL:              frontendURL,
		InvitationTokenExpiry:    invitationTokenExpiry,
		PasswordResetTokenExpiry: passwordResetTokenExpiry,

		OperatorProvisioning: operatorProvisioningService,
		Announcement:         communicationCapability,
		Schools:              organizations,
		Students:             StudentServices{Directory: studentService, Companions: companionService},
		CareLifecycle:        careLifecycleService,
		StudentAudit:         studentAuditService,
		StudentConsents:      studentConsentService,
		MasterDataReview:     masterDataDecisions,
		CareRequests:         careRequestService,
		ExcusedRequests:      excusedRequestService,
		ParentRequests:       parentRequestCoordinator,
		RequestReviewPolicy:  requestReviewPolicy,
		StudentStatusDays:    studentStatusDayService,
		AbsenceOverview:      studentStatusDayOverviewService,
		StudentHistory:       presenceservice.NewStudentHistory(NewLoggedStudentPresence(db, logger), historyRoomNames(rooms), NewDataAccessAudit(audits.DataAccessLog), NewHistorySlots(timetableRepos.InstanceStudent)),
		Statistics: NewStatistics(logger, presenceCompose.StatisticsDependencies{
			DB:          db,
			StatusDays:  StatisticsStatusDays{carePlan},
			Courses:     NewCourseStatistics(MustPresenceReads(db)),
			Holidays:    TenantHolidays{Calendar: calendar},
			ClosingDays: TenantClosingDays{Calendar: calendar},
			Periods:     StatisticsReportPeriods{calendar},
			Students:    StatisticsReportStudents{studentRepository},
			Rooms:       StatisticsReportRooms{rooms},
			AccessLog:   StatisticsAuditLog{audits.DataAccessLog},
			Retention:   StatisticsRetention{settingsService},
			Logger:      logger.With("service", "statistics"),
			Now:         now,
		}),
		OGSGroupLive:            ogsGroupLiveService,
		SupervisionDashboard:    supervisionDashboardService,
		TimetableData:           timetablePlanning,
		InstanceSeriesConverter: instanceSeriesConverter,
		UnregisteredTagScans:    unregisteredTagScanService,

		EmailOutboxWorker: emailOutboxWorker,
		Delivery:          deliveryRuntime.Module,

		EnrollmentCareOffering:    carePlanCareOfferings{enrollmentCareOfferingService, careBookings, offeringChanges, pickupAdjustments},
		EnrollmentRequest:         enrollmentRequestService,
		EnrollmentPhase:           enrollmentPhaseService,
		EnrollmentDecision:        enrollmentDecisionService,
		EnrollmentReport:          enrollmentReports,
		ClassDayArrivalExceptions: classDayService,
		EnrollmentRollover:        enrollmentRolloverService,
		EnrollmentChangeRequest:   enrollmentChangeRequestService,
		EnrollmentDeletion:        enrollmentDeletionService,
		EnrollmentRejectedCleanup: enrollmentRejectedCleanupService,

		Parent:              parentService,
		Messaging:           messagingService,
		StaffMessaging:      staffMessagingService,
		Calendar:            calendarSvc,
		CalendarFeedCleanup: calendarSvc,
		ParentAnnouncement:  parentAnnouncementService,
		ParentEventEmitter:  pillEmitter,
	}

	factory.SettingsSideEffects = settingsCompose.NewSideEffectRegistry()
	facilitiesLegacy.RegisterSettingsSideEffects(factory.SettingsSideEffects, schulhofService, wcService)
	RegisterCareWithdrawalSettingsSideEffects(factory.SettingsSideEffects, careLifecycleService)
	tenantSettings := settingsCompose.NewTenantOperations(
		settingsService,
		payrollStatusService,
		settingsRuntime,
		factory.SettingsSideEffects.Dispatch,
		settingsChanged,
	)
	factory.TenantSettings = tenantSettings

	// #1843 sick cascade: the shift-plan-sync workflow is bound after
	// assembly because it needs the timetable data service while the absence
	// service is constructed long before it; the deferred binding above
	// resolves it per call.
	shiftPlanSyncer, err = shiftplansyncCompose.NewSickCascade(shiftplansyncCompose.SickCascadeDependencies{
		Planning:        factory.StaffShifts,
		Workforce:       workTime,
		LockStaffShifts: workforceCompose.NewStaffShiftLock(db),
		Deviations:      timetablePlanning.Deviations,
		TimetableData:   timetableRows.SickCascadeRows(arrivalTimetable.SubstituteDayLock(db)),
		InstanceStaff:   arrivalTimetable.NewTimetableInstanceStaffReads(timetableRepos.InstanceStaff),
		Broadcaster:     factory.RealtimeHub,
		Logger:          logger.With("service", "shift_plan_sync"),
		Today:           today,
	})
	if err != nil {
		return nil, err
	}
	// The People Directory serves guardians through the owner's legacy
	// guardian service (#2663); bind it now that the service exists.
	peopleCompose.BindGuardianDirectory(persons, &peopleCompose.GuardianDirectoryProvider{
		Guardians: guardianService, Runtime: peopleCompose.NewLegacyRepositoryRuntime(db),
		PermissionGranted: securityruntime.GuardianPermissionGranted,
	})
	// Permanent child deletion is the owner workflow of #2710. It runs over
	// the bound People Directory, Care Plan and Timetable capabilities; the
	// photo unlink resolves lazily because EnableStudentPhotos runs later in
	// the API bootstrap.
	studentDeletion, err := studentdeletioncompose.New(studentdeletioncompose.Dependencies{
		DB: db, Directory: persons, CarePlan: carePlan, Timetable: timetableCapability,
		Feedback: feedbackCounterOrUnconfigured(feedbackCounter), IsVerifiedStaff: userContextService.HasCurrentStaff,
		LockCareBookingWrites: recurrenceLock.LockRecurrenceWrites,
		UnlinkPhoto: func(ctx context.Context, path string) {
			if factory.StudentPhotos != nil {
				factory.StudentPhotos.ScheduleUnlinkAfterCommit(ctx, path)
			}
		},
		Broadcaster: realtimeHub, Logger: logger.With("workflow", "student_deletion"), Audit: auditCommand,
	})
	if err != nil {
		return nil, fmt.Errorf("compose student deletion workflow: %w", err)
	}
	factory.StudentDeletion = studentDeletion
	return factory, nil
}

func enforceInstanceTimePolicy(appEnv string) bool {
	appEnv = strings.ToLower(strings.TrimSpace(appEnv))
	return appEnv == "production" || appEnv == "staging"
}

func OptionalClock(clocks []func() time.Time) func() time.Time {
	if len(clocks) == 0 {
		return nil
	}
	if len(clocks) > 1 {
		panic("services.NewFactory accepts at most one clock")
	}
	return clocks[0]
}

// staffAdministrationPermitted is the Security Runtime's staff:manage check
// the staff directory asks before a create adopts an existing staff record.
func staffAdministrationPermitted(actorPermissions []string) bool {
	return securityruntime.HasPermission(securityruntime.PermissionStaffManage, actorPermissions)
}

// EnableStudentPhotos stores the photo lifecycle the root built over this
// graph's People Directory, settings, realtime hub and consents (#4007). The
// root registers its settings handler. Call once at API bootstrap.
func (f *Factory) EnableStudentPhotos(photos peopleCompose.StudentPhotoService) {
	f.StudentPhotos = photos
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
