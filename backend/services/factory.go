// Package services provides service layer implementations
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	"github.com/moto-nrw/project-phoenix/analytics"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/pwa"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	devicefleetModule "github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	devicefleetLegacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/legacy"
	documentCompose "github.com/moto-nrw/project-phoenix/modules/documentrendering/compose"
	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
	emergencysnapshotlegacy "github.com/moto-nrw/project-phoenix/modules/emergencysnapshot/legacy"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	grouplivelegacy "github.com/moto-nrw/project-phoenix/modules/grouplive/legacy"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	planexportlegacy "github.com/moto-nrw/project-phoenix/modules/planexport/legacy"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	supervisiondashboardlegacy "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/legacy"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/activities"
	auditService "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	_ "github.com/moto-nrw/project-phoenix/services/config/defaults"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
	"github.com/moto-nrw/project-phoenix/services/database"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/services/iot"
	staffclock "github.com/moto-nrw/project-phoenix/services/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/services/users"
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

type substitutionActorResolver struct {
	identity identityaccess.CallerIdentities
}

func (r substitutionActorResolver) ResolveActor(ctx context.Context) (*education.Actor, error) {
	staffID, err := r.identity.StaffID(ctx)
	if err != nil {
		if expectedMissingSubstitutionIdentity(err) {
			return nil, nil
		}
		return nil, err
	}
	teacherID, err := r.identity.TeacherID(ctx)
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
	// Auth is the composed Identity & Access module: the sessions, the
	// account lifecycle, the role administration, the invitations and the
	// operator flows the surfaces consume (#3364).
	Auth  *identityaccess.Module
	Audit auditModels.Command
	// MFA and Passkey are the Identity & Access second factor and the
	// school-portal WebAuthn ceremonies (#3331).
	MFA                  identityaccess.AccountMFA
	Passkey              identityaccess.AccountPasskeyFlows
	Active               studentpresence.Presence
	ActiveCleanup        studentpresence.PresenceCleanup
	WorkSession          timetracking.WorkSessionService
	WorkTimeMonth        timetracking.WorkTimeMonthService
	StaffAbsence         timetracking.StaffAbsenceService
	StaffBalanceAdjust   timetracking.StaffBalanceAdjustmentService
	StaffMonthClose      timetracking.StaffMonthCloseService
	StaffOverview        timetracking.StaffOverviewService
	TimeTrackingAuditLog timetracking.TimeTrackingAuditLogService
	StaffTimeExport      timetracking.StaffTimeExportService
	Activities           activities.ActivityService
	Education            education.Service
	Substitution         education.SubstitutionModule
	// GradeTransition is the owner workflow behind the school-year rollover
	// (#2711): the admin HTTP surface calls exactly its public commands.
	GradeTransition    *gradetransition.Workflow
	Facilities         facilities.Service
	Schulhof           facilities.SchulhofService
	WC                 facilities.WCService
	Invitation         InvitationCapability
	GuardianInvitation GuardianInvitationCapability
	IoT                iot.Service
	StaffClock         *staffclock.Service
	Settings           config.SettingsService
	TenantSettings     *config.TenantOperations
	PayrollStatus      config.PayrollStatusGetter
	// The Dienstplan side of Workforce (#3418): the planning capability the
	// staff-shift route mounts, the self-service assignments and the week
	// overview, and the shift-type administration, all public contracts.
	StaffShifts               workforceModule.StaffShiftPlanning
	StaffAssignments          workforceModule.StaffAssignmentQuery
	StaffScheduleOverview     workforceModule.StaffScheduleOverviewQuery
	ShiftTypes                workforceModule.ShiftTypeAdministration
	PlanningTracks            timetableplanning.PlanningTrackService
	PickupSchedule            careplan.PickupScheduleService
	PartialAbsence            careplan.PartialAbsenceService
	ArrivalSchedule           careplan.ArrivalScheduleService
	CareDay                   careplan.CareDayQuery
	TimetableBridge           *timetableplanning.TimetableBridgeService
	Materialization           timetableplanning.MaterializationService
	TemplateSplit             *timetableplanning.TemplateSplitService
	TimetableCleanup          timetableplanning.TimetableCleanupService
	TimeTrackingCleanup       timetracking.TimeTrackingCleanupService
	StudentChangeLogCleanup   users.StudentChangeLogCleanupService
	Instance                  timetableplanning.InstanceService
	AutoStart                 timetableplanning.AutoStartService
	AutoEnd                   timetableplanning.AutoEndService
	TimetableOperations       timetableplanning.TimetableOperationsService
	Users                     users.PersonService
	Birthdays                 users.BirthdayService
	StaffDocuments            users.StaffDocumentService
	StudentDocuments          careplan.StudentDocuments
	FileStore                 *filestorageModule.Module
	CaregiverCapability       users.CaregiverCapabilityService
	Guardian                  *users.GuardianService
	PeopleDirectory           peopledirectory.Capability
	GuardianProfileLoader     *users.GuardianProfileLoader
	UserContext               *repositories.CallerRows
	Database                  database.DatabaseService
	DatabaseStatsCapabilities func(context.Context) database.StatsCapabilities
	Import                    *importService.ImportService[importModels.StudentImportRow]        // Student import service
	StaffImport               *importService.ImportService[importModels.StaffImportRow]          // Staff (Mitarbeiter) import service
	ClassListImport           *importService.ImportService[importModels.ClassListEntryImportRow] // Class-list entry import (#2382)
	OpeningBalanceImport      importService.OpeningBalanceImportFactory                          // Opening balance import (#2132), request-scoped
	ListExport                *listexport.RendererService
	Emergency                 emergencysnapshot.Query
	SlotLists                 classday.SlotLists
	PlanExport                planexport.Service
	Reminders                 reminder.Capability
	Notifications             notifications.Notifier
	PushSubscriptions         notifications.PushSubscriptionService
	PWAUsage                  pwa.UsageService
	NotificationPreferences   notifications.PreferenceService
	AbsenceNotifier           notifications.AbsenceNotifier
	RealtimeHub               *realtime.Hub     // SSE event hub (shared by services and API)
	Tracker                   analytics.Tracker // Product analytics (PostHog; no-op without POSTHOG_API_KEY)
	Mailer                    email.Mailer
	DefaultFrom               email.Email
	FrontendURL               string
	InvitationTokenExpiry     time.Duration
	PasswordResetTokenExpiry  time.Duration

	// Platform domain (operator dashboard)
	// OperatorAuth and OperatorInvitation are the Identity & Access
	// operator directory, token mint and provisioning flows the operator
	// dashboard consumes (#3364).
	OperatorAuth         *identityaccess.Module
	OperatorInvitation   *identityaccess.Module
	OperatorProvisioning organizationtenancy.Provisioning
	Announcement         communication.Capability
	Schools              organizationtenancy.Capability
	Students             StudentServices
	StudentDeletion      *studentdeletion.Workflow
	CareLifecycle        careplan.CareLifecycle
	StudentAudit         users.StudentAuditService
	MasterDataReview     users.MasterDataReviewService
	CareRequests         carerequests.Service
	// OfferingChanges is the post-enrollment offering change-request lifecycle
	// (#1665), shared by the parents portal and the staff review queue.
	OfferingChanges   enrollment.OfferingChangeRequestService
	PickupAdjustments enrollment.PickupAdjustmentService
	ExcusedRequests   careplan.ExcusedAbsenceRequests
	ParentRequests    *users.ParentRequestCoordinator
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
	TimetableData           *timetableplanning.TimetableDataService
	InstanceSeriesConverter timetableplanning.InstanceSeriesConverter
	OperatorMFA             identityaccess.OperatorMFAFlows
	OperatorPasskey         identityaccess.OperatorPasskeyFlows
	UnregisteredTagScans    auditService.UnregisteredTagScanService

	// Delivery owns the leased email and push outboxes; EmailOutboxWorker
	// drains both transports.
	EmailOutboxWorker *deliveryModule.Worker
	Delivery          *deliveryModule.Module

	// Enrollment domain (parent-enrollment PR 5+).
	EnrollmentFormSchema      enrollment.FormSchemaService
	EnrollmentCareOffering    enrollment.CareOfferingService
	EnrollmentCaptcha         *enrollment.CaptchaService
	EnrollmentRequest         enrollment.RequestService
	EnrollmentPhase           enrollment.PhaseService
	EnrollmentPhaseExpiry     enrollment.PhaseExpiryService
	EnrollmentDecision        enrollment.DecisionService
	EnrollmentReport          enrollment.ReportService
	ClassDayArrivalExceptions enrollment.ClassDayArrivalExceptionService
	EnrollmentRollover        enrollment.RolloverService
	EnrollmentChangeRequest   enrollment.ChangeRequestService
	EnrollmentDeletion        enrollment.EnrollmentDeletionService
	EnrollmentRejectedCleanup enrollment.RejectedEnrollmentCleaner

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
	ParentEventEmitter *parentmessaging.Emitter

	// Calendar (staff and parent personal calendars)
	Calendar            calendarCompose.Application
	CalendarFeedCleanup calendarService.FeedCleanupService

	// ParentAnnouncement (staff-side parent broadcast news authoring, #1669)
	ParentAnnouncement communication.ParentAnnouncementCapability

	// SettingsSideEffects is the per-key handler registry the API binds to
	// SettingsResource.OnValueSet. Domain packages register handlers here
	// (facilities at startup, students via EnableStudentPhotos). API never
	// owns the registry - its only job is to dispatch.
	SettingsSideEffects *sideeffects.Registry
	// StudentPhotos is set by EnableStudentPhotos. nil until the API layer
	// supplies a PhotoUnlinker (file IO is an api-layer concern, not a
	// service-layer one).
	StudentPhotos   users.StudentPhotoService
	StudentConsents *repositories.StudentConsents
}

// SetSettingsObservers wires delivery-owned metrics without coupling the
// settings application layer to a metrics implementation.
func (f *Factory) SetSettingsObservers(
	lookup config.SettingsLookupObserver,
	sideEffectFailure sideeffects.FailureObserver,
) {
	if observable, ok := f.Settings.(interface {
		SetLookupObserver(config.SettingsLookupObserver)
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
	EnrollmentRequireCaptcha   bool
	EnrollmentCaptchaSecretKey string
	EnrollmentCaptchaSiteKey   string
}

func currentFactoryConfig() FactoryConfig {
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
		EnrollmentRequireCaptcha:   strings.TrimSpace(os.Getenv("ENROLLMENT_REQUIRE_CAPTCHA")) == "true",
		EnrollmentCaptchaSecretKey: os.Getenv("ENROLLMENT_CAPTCHA_SECRET_KEY"),
		EnrollmentCaptchaSiteKey:   os.Getenv("ENROLLMENT_CAPTCHA_SITE_KEY"),
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
type DurableDeliveryObserver func(transport, template, operation string, duration time.Duration, count int, err error)

// DataImportObserver records one Data Import run (#2708): rows parsed,
// accepted, rejected, created and updated plus the run duration. The
// composition root supplies it so this package keeps no metrics dependency.
type DataImportObservation = importService.ImportObservation
type DataImportObserver func(DataImportObservation)

func newAuditCommand(store auditModels.AppendStore, logger *slog.Logger, observe AuditAppendObserver) (auditModels.Command, error) {
	if store == nil || logger == nil || observe == nil {
		return nil, errors.New("audit command store, logger, and observer are required")
	}
	command, err := auditService.NewCommand(store, func(observation auditService.AppendObservation) {
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
	repos *repositories.Factory,
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
	feedbackCounter users.FeedbackEntryCounter,
	bindFeedbackSettings FeedbackSettingsBinder,
	observeAuditAppend AuditAppendObserver,
	observeDelivery DeliveryObserver,
	observeDurableDelivery DurableDeliveryObserver,
	observeDeviceFleet DeviceFleetObserver,
	observeIdentityAccess IdentityAccessObserver,
	workTime workforceModule.Capability,
	observeDataImport DataImportObserver,
	fileStorage FileStorageWiring,
	clocks ...func() time.Time,
) (*Factory, error) {
	if organizations == nil || persons == nil || groups == nil || rooms == nil || membership == nil || calendar == nil || timetableCapability == nil || appointmentCapability == nil || communicationCapability == nil || observeCommunication == nil || observeCarePlan == nil || mealPlan == nil || bindMealPlanSettings == nil || feedbackCounter == nil || bindFeedbackSettings == nil || observeAuditAppend == nil || observeDelivery == nil || observeDurableDelivery == nil || observeDeviceFleet == nil || observeIdentityAccess == nil || workTime == nil || observeDataImport == nil || fileStorage.Objects == nil || fileStorage.Observe == nil {
		return nil, errors.New("organization tenancy, people directory, school structure, facilities, school membership, school calendar, timetable, appointments, communication, care plan, meal plan, feedback, Audit, Delivery, Identity & Access, Workforce, and Data Import capabilities with their binders and observers are required")
	}
	communicationCompose.InstallMessageQueryInstrumentation(db)
	repos.BindAppointments(appointmentCapability)
	cfg := currentFactoryConfig()
	cfg.PublicAPIURL = publicAPIURL
	return newFactory(repos, db, logger, cfg, tenantRuntime, organizations, persons, groups, rooms, membership, calendar, timetableCapability, communicationCapability, observeCommunication, observeCarePlan, mealPlan, bindMealPlanSettings, feedbackCounter, bindFeedbackSettings, observeAuditAppend, observeDelivery, observeDurableDelivery, observeDeviceFleet, observeIdentityAccess, workTime, observeDataImport, fileStorage, false, clocks...)
}

// FileStorageWiring carries what the File Storage module needs from the root
// and cannot compose itself: the uploads object store and the metrics sink.
// Test factories that leave Objects nil get no file storage and no attachment
// purger; production composition requires both.
type FileStorageWiring struct {
	Objects filestorageCompose.ObjectBackend
	Observe func(filestorageCompose.Observation)
}

func newFactory(
	repos *repositories.Factory,
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
	communicationCapability communication.Capability,
	observeCommunication func(communicationCompose.Observation),
	observeCarePlan CarePlanObserver,
	mealPlan parentportalcompose.MealPlanProvider,
	bindMealPlanSettings MealPlanSettingsBinder,
	feedbackCounter users.FeedbackEntryCounter,
	bindFeedbackSettings FeedbackSettingsBinder,
	observeAuditAppend AuditAppendObserver,
	observeDelivery DeliveryObserver,
	observeDurableDelivery DurableDeliveryObserver,
	observeDeviceFleet DeviceFleetObserver,
	observeIdentityAccess IdentityAccessObserver,
	workTime workforceModule.Capability,
	observeDataImport DataImportObserver,
	fileStorage FileStorageWiring,
	allowAuditRootWrites bool,
	clocks ...func() time.Time,
) (*Factory, error) {
	now := optionalClock(clocks)
	today := timezone.CalendarDateClock(now)
	// Persons first: the school projections sort by the names this binds.
	repos.BindPeopleDirectory(persons)
	repos.BindSchoolMembership(membership)
	repos.BindSchoolCalendar(calendar)
	repos.BindOrganizationTenancy(organizations)
	repos.BindSchoolStructure(groups)
	repos.BindFacilities(rooms)
	repos.Student = overlappingRosterGroupNames{StudentRepository: repos.Student, groups: groups}
	settingsRuntime := newSettingsRuntime(db, nil)
	repos.SetConfigRuntime(settingsRuntime)

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
	usercontextLogger := logger.With("service", "usercontext")
	authLogger := logger.With("service", "auth")
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
	repos.ConfigureAuditRuntime(auditReadRuntime)
	auditAppender := repos.NewAuditStore(auditTransactionRuntime)
	auditCommand, err := newAuditCommand(auditAppender, auditLogger, observeAuditAppend)
	if err != nil {
		return nil, err
	}
	repos.RouteAuditWrites(auditCommand)
	if err := bindClassListEntryAdministration(membership, persons, auditCommand); err != nil {
		return nil, err
	}
	studentConsentService := repositories.NewStudentConsentsFor(persons, repos.StudentConsentChange)

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

	// Product analytics (PostHog) — no-op when POSTHOG_API_KEY is unset
	tracker, err := analytics.New(
		cfg.PostHogAPIKey,
		cfg.PostHogHost,
		analyticsDeployment(cfg.AppEnv, cfg.TenantDomain),
		logger.With("component", "analytics"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize education service first (needed for active service)
	educationService := education.NewService(
		repos.Group,
		repos.GroupTeacher,
		repos.ClassTeacher,
		repos.Room,
		repos.Teacher,
		repos.Staff,
		repos.Student,
		repos.GroupSubstitution,
		db,
	)
	// Announces group_access_changed after a group-leader change (#2084).
	if broadcastAware, ok := educationService.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}
	// Class assignment rewrites scope the Lehrkraft student day view (#1772)
	// and land in the Stammdaten audit trail.
	if auditAware, ok := educationService.(interface {
		SetMasterDataAudit(auditModels.StaffMasterDataChangeCreator)
	}); ok {
		auditAware.SetMasterDataAudit(repos.StaffMasterDataChange)
	}

	// Reconciles already-materialized future timetable rosters when a grade
	// transition graduates or restores students (#405).
	rosterReconciler := timetableplanning.NewRosterReconciler(
		repos.ActivityInstance,
		repos.InstanceStudent,
		repos.StudentEnrollment,
		logger,
		now,
	)

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
	homeLayoutService := config.NewHomeLayoutService(
		repositories.NewHomeLayoutRepository(settingsRuntime),
		settingsRuntime,
		logger,
		settingsChanged,
	)

	// Initialize settings service (new schema-driven settings system)
	settingsService := config.NewSettingsServiceWithHomeLayouts(
		repos.SettingValue,
		repos.SettingAudit,
		newSchoolSettingsStore(organizations),
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
	// Wire the enrollment class-restriction probe so the settings service can
	// refuse disabling concrete-class collection while an active phase
	// restricts eligibility to specific classes (#1663). Runs inside the
	// caller's tenant tx, so the phase lookup is RLS-scoped.
	if guarded, ok := settingsService.(interface {
		SetClassRestrictionGuard(func(context.Context) (bool, error))
	}); ok {
		guarded.SetClassRestrictionGuard(func(ctx context.Context) (bool, error) {
			return repos.Enrollment().HasActiveClassRestrictedPhase(ctx)
		})
	}
	// Same for the grade-level restriction, which survives concrete-class
	// collection being off but not grade-level collection (#1663).
	if guarded, ok := settingsService.(interface {
		SetGradeRestrictionGuard(func(context.Context) (bool, error))
	}); ok {
		guarded.SetGradeRestrictionGuard(func(ctx context.Context) (bool, error) {
			return repos.Enrollment().HasActiveGradeRestrictedPhase(ctx)
		})
	}
	// And the cap probe, so enrollment.grade_level_max cannot be lowered below
	// a grade an active phase already restricts itself to — which would leave
	// that phase with no selectable eligible grade at all (#1663).
	if guarded, ok := settingsService.(interface {
		SetGradeCapGuard(func(context.Context) (int, error))
	}); ok {
		guarded.SetGradeCapGuard(func(ctx context.Context) (int, error) {
			return repos.Enrollment().MaxActivePhaseGrade(ctx)
		})
	}

	// Identity & Access is composed with the auth service below; the person
	// service reads its role administration at call time (#3314).
	var identityAccess *identityaccess.Module
	identityRoles := roleAdministration{current: func() *identityaccess.Module { return identityAccess }}

	// Initialize users service first (needed for active service)
	usersService := users.NewPersonService(users.PersonServiceDependencies{
		PersonDirectory:      repositories.NewPersonDirectory(persons),
		StudentDirectory:     repositories.NewStudentDirectory(persons),
		PersonRepo:           repos.Person,
		RFIDRepo:             repos.RFIDCard,
		AccountExists:        repositories.AccountExists(repos.Profile),
		StudentRepo:          repos.Student,
		StaffRepo:            repos.Staff,
		TeacherRepo:          repos.Teacher,
		LehrkraftRoles:       identityRoles,
		PersonnelNumberAudit: repos.PersonnelNumberChange,

		// Staff Stammdaten (#1423)
		StaffMasterDataRepo:    repos.StaffMasterData,
		StaffQualificationRepo: repos.StaffQualification,
		StaffFinancialRepo:     repos.StaffFinancialData,
		StammdatenAudit:        repos.StaffMasterDataChange,
		DataAccessLog:          repos.DataAccessLog,

		DB:              db,
		SettingsService: settingsService,
		Logger:          logger.With("service", "users"),
	})

	// Birthday display (#1542): who is celebrating today, plus the school
	// settings and personal opt-out that decide who may be shown.
	birthdayService := users.NewBirthdayService(users.BirthdayServiceDependencies{
		StudentRepo:     repos.Student,
		StaffRepo:       repos.Staff,
		PersonRepo:      repos.Person,
		SettingsService: settingsService,
		Logger:          logger.With("service", "birthdays"),
		Now:             now,
	})

	// Staff documents (#1424): metadata + per-category authority for the
	// Dokumente tab. Shares the Stammdaten audit trail and access log.
	staffDocumentService := users.NewStaffDocumentService(
		db,
		repos.StaffDocument,
		repos.Staff,
		repos.StaffMasterData,
		repos.StaffMasterDataChange,
		repos.DataAccessLog,
		logger.With("service", "staff_documents"),
	)

	// Initialize guardian service
	// Replies to tenant-bound mail belong to the OGS, not to moto (#1936).
	// Built once here and shared: the outbox worker covers every queued kind,
	// the guardian service covers its own synchronous invitation send.
	tenantMailIdentity := emailoutbox.NewTenantMailIdentity(schoolContactDirectory{schools: organizations}, func(ctx context.Context, tenantID int64) (string, error) {
		return settingsService.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, logger)

	guardianService := users.NewGuardianService(users.GuardianServiceDependencies{
		GuardianProfileRepo:     repos.GuardianProfile,
		GuardianPhoneNumberRepo: repos.GuardianPhoneNumber,
		StudentGuardianRepo:     repos.StudentGuardian,
		GuardianInvitations:     newGuardianInvitationReads(func() identityaccess.GuardianInvitations { return identityAccess }),
		StudentRepo:             repos.Student,
		PersonRepo:              repos.Person,
		GuardianFinancialRepo:   repos.GuardianFinancialData,
		GuardianFinancialAudit:  repos.GuardianFinancialChange,
		DataAccessLog:           repos.DataAccessLog,
		Mailer:                  mailer,
		Dispatcher:              dispatcher,
		FrontendURL:             frontendURL,
		DefaultFrom:             defaultFrom,
		InvitationExpiry:        invitationTokenExpiry,
		MailIdentity:            tenantMailIdentity,
		DB:                      db,
	})

	// Thin loader that backs the public + parent enrollment me/profile
	// endpoints. Pulls the multi-schema join out of the handlers (Rule 1)
	// and into the existing GuardianProfileRepository.LoadProfileWithChildren.
	guardianProfileLoader := users.NewGuardianProfileLoader(repos.GuardianProfile, db, logger.With("service", "guardian-profile-loader"))

	// Public holidays per Bundesland (#1418 3a) and tenant closing days
	// (#1418 3b) share the Soll=0 semantics. The Soll consumers get the UNION
	// of both from the School Calendar; the statistics reports and the
	// holidays endpoint keep reading only the statutory holidays.
	nonWorkingDayService := nonWorkingDays{calendar: calendar}
	// School-defined Abwesenheitsarten (#2403): the time-tracking services
	// resolve custom names on both the write path (which base type an art
	// inherits) and the read paths.
	staffAbsenceTypeService := AbsenceTypes(repos.StaffAbsenceType)
	// Time-account changes fan out tenant-wide after commit.
	timeTrackingEvents := TimeTrackingEvents(realtimeHub)
	// The #1843 sick cascade is the shift-plan-sync workflow, which needs the
	// planning capability and the timetable services built long after the
	// absence service; the deferred binding resolves it on every call and the
	// assignment below happens once those services exist.
	var shiftPlanSyncer shiftplansync.SickCascade
	// The weekly history summaries re-price Sonderarbeitszeit weeks (#3259)
	// through the month service built below.
	var overrideMonths timetracking.WorkTimeMonthService

	// Initialize work session service (before active service - needed for NFC auto-check-in)
	workSessionService := timetracking.NewWorkSessionService(repos.WorkSession, repos.WorkSessionBreak, NewWorkSessionAudit(repos.WorkSessionEdit), repos.StaffAbsence, repos.GroupSupervisor, repos.ActiveGroup, WorkSessionStaff(repos.Staff, repositories.MustNewStaffEmployment(db)), NewWorkSessionSchedules(repos.StaffWorkSchedule), NewWorkSessionTimeModels(repos.WorkTimeModel), PresenceSettings(settingsService), activeLogger, db, RenderTimeTrackingPDF, RenderTimeTrackingWorkbook,
		// Planned-shift lookups for the auto-checkout job (#1798).
		timetracking.WithWorkSessionShifts(NewTimeTrackingShifts(workTime)),
		timetracking.WithWorkSessionEvents(timeTrackingEvents),
		// The session service's weekly summaries reduce their Soll by holidays too.
		timetracking.WithWorkSessionHolidays(nonWorkingDayService),
		timetracking.WithWorkSessionAbsenceTypes(staffAbsenceTypeService),
		timetracking.WithWorkSessionTargetOverrides(staffTargetOverrideWeeks{overrides: workTime, months: func() timetracking.WorkTimeMonthService { return overrideMonths }}),
	)
	staffClockService := newStaffClockService(usersService, repos.RFIDCard, workSessionService)

	// Monatskarte read model (#1842) — everything computed on read, the
	// Übertrag is live.
	workTimeMonthService := timetracking.NewWorkTimeMonthService(
		repos.WorkSession,
		repos.WorkSessionBreak,
		repos.StaffAbsence,
		StaffScheduleAssignments(repositories.MustNewStaffEmployment(db)),
		NewWorkScheduleTargets(repos.StaffWorkSchedule),
		NewWorkTimeTargetModels(repos.WorkTimeModel),
		NewTimeTrackingShifts(workTime),
		PresenceSettings(settingsService),
		activeLogger,
		timetracking.WithMonthHolidays(nonWorkingDayService),
		// Sonderarbeitszeiten (#3259) win over closing days and the schedule.
		timetracking.WithMonthTargetOverrides(workTime),
		// Stundenkonto transactions (#1420) enter the carry chain by effective date.
		timetracking.WithMonthAdjustments(repos.StaffBalanceAdjust),
		// Frozen months (#1417) short-circuit the carry chain so a retroactive
		// correction can no longer rewrite a closed month's Übertrag.
		timetracking.WithMonthSnapshots(MonthSnapshotCapability(repos.StaffMonthSnapshot)),
	)
	overrideMonths = workTimeMonthService

	// Initialize staff absence service
	staffAbsenceService := timetracking.NewStaffAbsenceService(repos.StaffAbsence, repos.WorkSession, repos.StaffVacationQuota, repos.StaffAbsenceAudit, PresenceSettings(settingsService), workTimeMonthService,
		timetracking.WithAbsenceTypes(staffAbsenceTypeService),
		timetracking.WithAbsenceEvents(timeTrackingEvents),
		timetracking.WithAbsenceLogger(activeLogger),
		// Deletes leave an append-only tombstone in the cross-staff audit log
		// (#1417). The delete paths fail without this wiring.
		timetracking.WithAbsenceDeletionAudit(NewTimeTrackingDeletionAudit(repos.TimeTrackingDeletion)),
		// Vacation takeover at the moto introduction (#2132): the summary
		// subtracts pre-introduction days, the write paths book/delete them.
		timetracking.WithVacationOpenings(repos.StaffVacationOpening),
		// Absence email notifications (#1419 4d).
		timetracking.WithAbsenceEmail(timetracking.AbsenceEmailDeps{
			Settings:    PresenceSettings(settingsService),
			Dispatcher:  absenceEmailDispatcher{dispatcher: dispatcher, from: defaultFrom, identity: tenantMailIdentity, logger: activeLogger},
			StaffRepo:   absenceEmailStaffDirectory{source: repos.Staff},
			SchoolRepo:  absenceEmailSchoolDirectory{schools: organizations},
			FrontendURL: frontendURL,
			Logger:      activeLogger,
		}),
		timetracking.WithAbsenceShiftPlanSyncer(shiftplansyncCompose.DeferredSickCascade(func() shiftplansync.SickCascade { return shiftPlanSyncer })),
		// A rebooking inside a closed month (#3258) could not move its frozen
		// closing balance, so the service rejects it.
		timetracking.WithAbsenceMonthSnapshots(MonthSnapshotCapability(repos.StaffMonthSnapshot)),
	)

	// Stundenkonto lifecycle transactions (#1420): payout, comp-time grants,
	// school-year reset. Reads the live balance through the month service.
	staffBalanceAdjustService := timetracking.NewStaffBalanceAdjustmentService(repos.StaffBalanceAdjust, workTimeMonthService, PresenceSettings(settingsService), activeLogger,
		timetracking.WithAdjustmentEvents(timeTrackingEvents),
		// A booking inside a closed month (#1417) could not move its frozen
		// closing balance, so the ledger rejects it.
		timetracking.WithAdjustmentSnapshots(MonthSnapshotCapability(repos.StaffMonthSnapshot)),
		// Deletes leave an append-only tombstone in the cross-staff audit log
		// (#1417). The delete path fails without this wiring.
		timetracking.WithAdjustmentDeletionAudit(NewTimeTrackingDeletionAudit(repos.TimeTrackingDeletion)),
	)

	// Monatsabschluss (#1417): freezes a month's closing balance so a
	// retroactive correction can no longer rewrite every later Übertrag.
	staffMonthCloseService := timetracking.NewStaffMonthCloseService(
		MonthSnapshotCapability(repos.StaffMonthSnapshot),
		workTimeMonthService,
		MonthCloseStaff(repos.Staff),
		PresenceSettings(settingsService),
		activeLogger,
		timetracking.WithMonthCloseEvents(timeTrackingEvents),
	)

	// Tenant-wide time-tracking views (#1417 2a). Prefetches all inputs once
	// and runs the SAME per-staff month math over in-memory readers, so the
	// list can never drift from the /staff/{id} detail view.
	staffOverviewService := timetracking.NewStaffOverviewService(
		OverviewStaff(repos.Staff),
		repos.WorkSession,
		repos.WorkSessionBreak,
		repos.StaffAbsence,
		repos.StaffBalanceAdjust,
		repos.StaffVacationQuota,
		MonthSnapshotCapability(repos.StaffMonthSnapshot),
		NewWorkScheduleTargets(repos.StaffWorkSchedule),
		NewWorkTimeTargetModels(repos.WorkTimeModel),
		NewTimeTrackingShifts(workTime),
		PresenceSettings(settingsService),
		activeLogger,
		timetracking.WithOverviewHolidays(nonWorkingDayService),
		timetracking.WithOverviewTargetOverrides(workTime),
		// Vacation takeover (#2132): the Resturlaub column subtracts
		// pre-introduction days exactly like the /staff/{id} detail view.
		timetracking.WithOverviewVacationOpenings(repos.StaffVacationOpening),
	)

	// Cross-staff payroll/evidence export (#1417 2b): rows via the overview's
	// prefetch (month) and the single-staff export cells (day); every download
	// writes an audit.data_access_logs row or fails.
	// One payroll-status instance: the /payroll page and the DATEV writers
	// must judge completeness identically.
	payrollStatusService := config.NewPayrollStatusService(settingsService, func(ctx context.Context) (int, int, error) {
		staff, err := repos.Staff.List(ctx, nil)
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

	staffTimeExportService := timetracking.NewStaffTimeExportService(
		staffOverviewService,
		workSessionService,
		TimeExportStaff(repos.Staff),
		NewTimeTrackingDataAccessAudit(repos.DataAccessLog),
		PayrollExportSettings{Source: payrollStatusService},
		activeLogger,
		RenderTimeTrackingWorkbook,
	)

	// Cross-staff audit feed (#1417): merges the four change trails into one
	// keyset-paginated view. Read-only; permission gating at the route.
	timeTrackingAuditLogService := timetracking.NewTimeTrackingAuditLogService(
		NewTimeTrackingAuditReader(repos.TimeTrackingAuditLog),
		StaffDisplayNames(repos.Staff),
		PresenceSettings(settingsService),
	)

	// Initialize attendance sync service (WP-B10). Implements
	// studentpresence.AttendanceSyncer - called from CreateVisit / EndVisit to mirror
	// into schedule.instance_students and enrich SSE events. No circular
	// dependency because it only depends on repos, not on studentpresence.Presence.
	attendanceSyncService := timetableplanning.NewAttendanceSyncService(
		repos.ActivityInstance,
		repos.InstanceStudent,
		logger.With("service", "attendance-sync"),
	)
	approvedOfferingProjection := enrollment.NewApprovedOfferingProjection(repos.Enrollment(), offeringStudents{query: persons})
	pickupBaselines, err := careplanCompose.NewPickupBaselines(repos.CarePlan(), approvedOfferingProjection, func(ctx context.Context) (bool, error) {
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
	arrivalBaselines, err := NewArrivalBaselines(repos.CarePlan(), persons, classArrivalQueries, approvedOfferingProjection, func(ctx context.Context) (bool, error) {
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
		Records:          repos.CarePlan(),

		PickupBaselines: pickupBaselines,
	})

	// Single entry point for completing instances whose active.group somebody
	// else ended (force-start here, the nightly session-end job in the
	// scheduler). Finalizes attendance first, so a completed instance never
	// keeps genuinely expected rows — readers take those as the "not booked
	// into care that day" marker (#1747). Repos only, so no cycle with the
	// active service it is injected into.
	timetableBridgeService := timetableplanning.NewTimetableBridgeService(timetableplanning.TimetableBridgeDependencies{
		Instances:        repos.ActivityInstance,
		InstanceStudents: repos.InstanceStudent,
		CareDays:         careDayService,
	})

	// Initialize active service with SSE broadcaster
	sessionGroups, sessionSupervisors := presenceCompose.SessionRepositories(repos.ActiveGroup)
	activeServiceDeps := presenceservice.PresenceDependencies{
		PrincipalReader:          AttendancePrincipal,
		SchoolPresence:           newStudentPresence(db, logger),
		StudentDisplay:           studentDisplayProjection{students: persons, groups: groups},
		GroupRepo:                sessionGroups,
		SessionStartLock:         repos.SessionStartLock,
		SupervisorRepo:           sessionSupervisors,
		StudentStatusRepo:        repos.StudentStatusDay,
		CrossTenantRepo:          repos.CrossTenant,
		Schools:                  newActiveSchoolQuery(organizations),
		StudentRepo:              PresenceStudents(db, repos.Student),
		StaffRepo:                NewAttendanceStaffDirectory(repos.Staff),
		RoomRepo:                 NewAttendanceRooms(repos.Room),
		YardRoomColor:            yardRoomColorQuery(rooms),
		ActivityGroupRepo:        repositories.NewSessionActivities(repos.ActivityGroup),
		ActivityCatRepo:          NewAttendanceActivityCategories(repos.ActivityCategory),
		EducationGroupRepo:       NewAttendanceEducationGroups(repos.Group, repos.Student),
		DeviceRepo:               NewSessionDeviceDirectory(repos.Device, settingsService, activeLogger),
		StaffNames:               NewAttendanceStaffNames(repos.Staff, usersService),
		DB:                       db,
		Broadcaster:              realtimeHub,           // Pass SSE broadcaster
		Tracker:                  tracker,               // Product analytics (PostHog)
		WorkSessionService:       workSessionService,    // NFC auto-check-in
		AttendanceSyncer:         attendanceSyncService, // WP-B10 mirror + SSE enrichment
		TimetableBridgeCompleter: timetableBridgeService,
		Logger:                   activeLogger,
		Now:                      now,
	}
	// Chat-pill emitter (#1803): also provides guardian-only invalidations for
	// enrollment writes that change a child's live care data.
	pillEmitter := communicationCompose.NewParentEventEmitter(communicationCompose.ParentEventEmitterConfig{
		DB:          db,
		Runtime:     tenantRuntime,
		ThreadRepo:  repos.ParentMessageThread,
		MessageRepo: repos.ParentMessage,
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
	activitiesService, err := activities.NewService(
		timetableCapability,
		repos.ActivityGroup,
		repos.ActivitySchedule,
		repos.ActivitySupervisor,
		repos.StudentEnrollment,
		repos.ActiveGroup,
		repos.Staff,
		repos.Student,
	)
	if err != nil {
		return nil, err
	}

	// Construct care-offering validation before services that can mutate its
	// recurrence resources. Room/timeframe FKs use ON DELETE SET NULL, so those
	// delete services must preflight the same materializability invariant as
	// template and calendar-period mutations.
	enrollmentCareOfferingService := enrollment.NewCareOfferingService(enrollment.CareOfferingServiceConfig{
		Repo:                  enrollment.NewCareOfferingRepository(repos.CarePlan()),
		Bookings:              repos.Enrollment(),
		ActivityGroupRepo:     repos.ActivityGroup,
		ActivityScheduleRepo:  repos.ActivitySchedule,
		CalendarPeriodRepo:    repos.CalendarPeriod,
		TimeframeRepo:         repos.Timeframe,
		ActivityExceptionRepo: repos.ActivityException,
		Phases:                repos.Enrollment(),
		Settings:              settingsService,
		Today:                 today,
		LockTemplateRecurrence: func(ctx context.Context) error {
			return timetableplanning.LockTenantRecurrenceWrites(ctx, db)
		},
		Logger: logger.With("service", "enrollment-care-offering"),
	})
	careOfferingSeriesValidator, ok := enrollmentCareOfferingService.(enrollment.CareOfferingSeriesValidator)
	if !ok {
		return nil, fmt.Errorf("enrollment care offering service does not implement series validation")
	}
	if _, ok := enrollmentCareOfferingService.(enrollment.CareOfferingCalendarPeriodValidator); !ok {
		return nil, fmt.Errorf("enrollment care offering service does not implement calendar period validation")
	}
	careOfferingResourceValidator, ok := enrollmentCareOfferingService.(enrollment.CareOfferingMaterializationResourceValidator)
	if !ok {
		return nil, fmt.Errorf("enrollment care offering service does not implement materialization resource validation")
	}
	careOfferingPhaseValidator, ok := enrollmentCareOfferingService.(enrollment.CareOfferingPhaseValidator)
	if !ok {
		return nil, fmt.Errorf("enrollment care offering service does not implement phase validation")
	}

	// Initialize facilities service
	facilitiesService := facilities.NewServiceWithConfig(facilities.ServiceConfig{
		Rooms:     rooms,
		Occupancy: facilitiesLegacy.OccupancyProjection(roomOccupancyPresence{newStudentPresence(db, logger)}, repos.ActivityGroup, membership, persons),
		History:   facilitiesLegacy.HistoryProjection(roomHistoryPresence{newStudentPresence(db, logger)}, repos.ActivityGroup, membership, persons),
		ValidateDeletion: func(ctx context.Context, roomID int64) error {
			activeGroups, err := repos.ActiveGroup.FindActiveByRoomID(ctx, roomID)
			if err != nil {
				return err
			}
			if len(activeGroups) > 0 {
				return facilitiesModule.ErrRoomInUse
			}
			if err := careOfferingResourceValidator.ValidateRoomDeletion(ctx, roomID); err != nil {
				if errors.Is(err, enrollment.ErrCareOfferingInvalid) {
					return facilitiesModule.ErrRoomRequiredByOffering
				}
				return err
			}
			return nil
		},
	})

	// Initialize Schulhof service (depends on facilities, activities, and active services)
	schulhofService := facilities.NewSchulhofService(
		facilitiesService,
		facilitiesLegacy.ActivityCatalog(activitiesService),
		facilitiesLegacy.OpenGroupCatalog(facilitiesGroupSupervisions(newStudentPresence(db, logger)), facilitiesRoomSessions(newStudentPresence(db, logger)), facilitiesGroupVisits(newStudentPresence(db, logger))),
		facilitiesLogger,
	)

	// Initialize WC service (depends on facilities and activities services)
	wcService := facilities.NewWCService(
		facilitiesService,
		facilitiesLegacy.ActivityCatalog(activitiesService),
		facilitiesLogger,
	)

	planningTrackService := timetableplanning.NewPlanningTrackService(repos.PlanningTrack, db)

	// The Dienstplan side of Workforce (#3418): shifts, series, shift types,
	// the self-service assignments ("Mein Tag", #1844) and the week overview,
	// composed by the owner over its native rows. The timetable, room, staff
	// and work-time facts arrive through the retained owner repositories; the
	// shift_moved Änderungsprotokoll entry (#1884) and the SSE invalidation
	// are bound here as well.
	shiftPlanning, err := workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce:       workTime,
		Staff:           repos.Staff,
		CalendarPeriods: repos.CalendarPeriod,
		DeviationEvents: repos.DeviationEvent,
		Instances:       repos.ActivityInstance,
		InstanceStaff:   repos.InstanceStaff,
		Rooms:           repos.Room,
		ActivityGroups:  repos.ActivityGroup,
		WorkSchedules:   repos.StaffWorkSchedule,
		WorkModels:      repos.WorkTimeModel,
		Holidays:        nonWorkingDayService,
		CategoryLinker:  activitiesService.SetCategoryShiftTypeLinks,
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
	// facade (#3418).
	shiftRows := workforceCompose.NewShiftRows(workTime)
	shiftTypeRows := workforceCompose.NewShiftTypeRows(workTime)

	// Couples pulled-forward day pickup times with the per-block partial
	// absences (#2360). Shared by the staff pickup-exception writers and the
	// parent care-exception writers so both derive the same state.
	// Later pickups open a block decision for the Leitung (#3261).
	pickupAutoExcusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: db, Records: repos.CarePlan(), Baselines: pickupBaselines,
		Blocks: newStudentPresence(db, logger), Preview: newPickupExcusalTimetable(timetableCapability, db), Extensions: newPickupExcusalTimetable(timetableCapability, db),
	})
	if err != nil {
		return nil, err
	}

	// Initialize pickup schedule service
	pickupScheduleService, err := NewPickupSchedules(db, repos.CarePlan(), persons, pickupBaselines,
		pickupAutoExcusal, logger.With("service", "pickup-schedule"))
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
		Presence: newStudentPresence(db, logger),
		Dashboard: devicefleetLegacy.NewDashboardSources(devicefleetLegacy.DashboardDependencies{
			ActiveGroups:   repos.ActiveGroup,
			Templates:      repos.ActivityGroup,
			Instances:      repos.ActivityInstance,
			PickupSchedule: pickupScheduleService,
		}),
		Tenants: devicefleetLegacy.NewTenantFacts(devicefleetLegacy.TenantFactDependencies{
			Schools:  displaySchoolDirectory{schools: organizations},
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
	iotService := iot.NewService(deviceFleet)

	partialAbsenceService, err := careplanCompose.NewPartialAbsences(db, repos.CarePlan(), newStudentPresence(db, logger), pickupAutoExcusal)
	if err != nil {
		return nil, err
	}

	// Initialize materialization service (WP-B8). Turns activity templates into
	// concrete schedule.activity_instances + instance_staff/instance_students
	// for a date window. Consumed by the scheduler task (gated on the
	// timetable.materialization_enabled setting) and the manual admin endpoint.
	materializationService := timetableplanning.NewMaterializationService(
		repos.ActivityGroup,
		repos.ActivitySchedule,
		repos.StudentEnrollment,
		repos.ActivitySupervisor,
		repos.CalendarPeriod,
		repos.ActivityInstance,
		repos.InstanceStaff,
		repos.InstanceStudent,
		repos.ActivityException,
		repos.Timeframe,
		db,
		realtimeHub,
		logger.With("service", "materialization"),
		// Per-date care filter (#2487): a child stays on the rosters the
		// materializer builds up to and including their last care day, and
		// drops out of every day after it.
		timetableplanning.WithCareBoundReader(repos.Student),
		// Holidays and closing days carry no series occurrences unless the
		// series opts into closing days (#3594); the School Calendar answers.
		timetableplanning.WithNonWorkingDays(calendar),
	)

	// Initialize instance lifecycle before template split: the split reuses its
	// deviation snapshot/reapply machinery when replacing future occurrences.
	recoveryRepo := repositories.NewActivityRecoveryRepository(db, repos.InstanceStudent)
	// The cancellation notice (#2601) rides on the announcement service, which
	// is built after the instance service; the lifecycle reaches it through
	// this late-bound publisher.
	var guardianNoticePublisher communication.CareCancellationPublisher
	// Conflict detection and staffing are the Timetable owner's (#3550): the
	// start check of the instance lifecycle and the auto-start tick, and the
	// planning reads api/timetable serves.
	timetableConflicts, err := NewTimetableConflictDetection(TimetableConflictReaders{
		Instances:         repos.ActivityInstance,
		InstanceStaff:     repos.InstanceStaff,
		InstanceStudents:  repos.InstanceStudent,
		Exceptions:        repos.ActivityException,
		Schedules:         repos.ActivitySchedule,
		Staff:             repos.Staff,
		CalendarPeriods:   repos.CalendarPeriod,
		ArrivalExceptions: repos.StudentArrivalException,
		Sessions:          repos.ActiveGroup,
		Shifts:            shiftRows,
		Presence:          newStudentPresence(db, logger),
		ArrivalBaselines:  arrivalBaselines,
		Logger:            logger.With("service", "timetable-conflicts"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable conflict detection: %w", err)
	}
	instanceService := timetableplanning.NewInstanceService(timetableplanning.InstanceServiceDependencies{
		StartConflicts:     timetableConflicts,
		Presence:           newStudentPresence(db, logger),
		GuardianNotices:    lateCareCancellationPublisher{resolve: func() communication.CareCancellationPublisher { return guardianNoticePublisher }},
		CareDayService:     timetableplanning.NewInstanceCareDays(careDayService, repos.CarePlan()),
		InstanceRepo:       repos.ActivityInstance,
		IdempotencyRepo:    repos.InstanceIdempotency,
		InstanceStaffRepo:  repos.InstanceStaff,
		InstanceStudents:   repos.InstanceStudent,
		ExceptionRepo:      repos.ActivityException,
		ActiveGroupRepo:    repos.ActiveGroup,
		SupervisorRepo:     repos.GroupSupervisor,
		RoomRepo:           repos.Room,
		ActivityGroupRepo:  repos.ActivityGroup,
		StaffRepo:          repos.Staff,
		StudentRepo:        repos.Student,
		CalendarPeriodRepo: repos.CalendarPeriod,
		ActiveService:      activeService,
		Materialization:    materializationService,
		DeviationEventRepo: repos.DeviationEvent,
		Broadcaster:        realtimeHub,
		DB:                 db,
		Logger:             logger.With("service", "instance-lifecycle"),
		Settings:           settingsService,
		RecoveryRepo:       recoveryRepo,
		Now:                now,
		// Development and test seed profiles create completed instances at the
		// current clock time. Lifecycle policy remains covered by dedicated tests.
		EnforceTimePolicy: enforceInstanceTimePolicy(cfg.AppEnv),
	})

	// Initialize template split service (WP-B3). "Dieser und alle folgenden":
	// caps the old template's schedules + rosters at an effective date,
	// creates a successor template and re-plans the affected window via the
	// materialization service. A split that moves the Zielgruppe away from
	// 'angebot' drops the successor's source rule; the carried roster must then
	// shed its source-derived rows (#2147 review). The enrollment decision
	// service that performs that resync is constructed later, so the split
	// reaches it through this late-bound hook.
	var resyncOfferingRoster func(context.Context, timetableplanning.OfferingRosterResyncInput) error
	templateSplitService := timetableplanning.NewTemplateSplitService(timetableplanning.TemplateSplitDependencies{
		GroupRepo:                  repos.ActivityGroup,
		CategoryRepo:               repos.ActivityCategory,
		PlanningTrackRepo:          repos.PlanningTrack,
		ScheduleRepo:               repos.ActivitySchedule,
		EnrollmentRepo:             repos.StudentEnrollment,
		SupervisorRepo:             repos.ActivitySupervisor,
		InstanceRepo:               repos.ActivityInstance,
		TimeframeRepo:              repos.Timeframe,
		Materialization:            materializationService,
		InstanceService:            instanceService,
		ValidateCareOfferingSeries: careOfferingSeriesValidator.ValidateTemplateSeries,
		ValidateOfferingSource:     careOfferingSeriesValidator.ValidateTemplateOfferingSource,
		Broadcaster:                realtimeHub,
		Logger:                     logger.With("service", "template-split"),
		DB:                         db,
	}, timetableplanning.WithOfferingRosterResync(func(ctx context.Context, in timetableplanning.OfferingRosterResyncInput) error {
		return resyncOfferingRoster(ctx, in)
	}))

	// Initialize timetable GDPR cleanup service (WP-B14). Deletes
	// schedule.activity_instances (CASCADE → instance_staff + instance_students)
	// and schedule.activity_exceptions older than the tenant's retention window.
	// Per-student audit rows via DataDeletion; exceptions slog-only.
	timetableCleanupService := timetableplanning.NewTimetableCleanupService(
		repos.ActivityInstance,
		repos.ActivityException,
		repos.InstanceStudent,
		repos.DataDeletion,
		repos.DeviationEvent,
		settingsService,
		logger.With("service", "timetable-cleanup"),
		now,
	)

	// Initialize time-tracking GDPR cleanup service (Tranche 0b). Deletes
	// active.work_sessions (CASCADE → work_session_breaks +
	// audit.work_session_edits) and active.staff_absences older than the
	// tenant's retention window. Per-staff audit rows via DataDeletion
	// (staff_id subject, added in migration 1.15.58).
	timeTrackingCleanupService := timetracking.NewTimeTrackingCleanupService(
		repos.WorkSession,
		repos.StaffAbsence,
		NewTimeTrackingRetentionAudit(repos.DataDeletion),
		PresenceSettings(settingsService),
		logger.With("service", "time-tracking-cleanup"),
	)

	// Per-child change-history retention cleanup (issue #1455). Deletes
	// audit.student_field_edits older than the tenant's retention window
	// (gdpr.student_change_log_retention_days, default 90). One per-student
	// DataDeletion audit row per run; shares the nightly cleanup window.
	studentChangeLogCleanupService := users.NewStudentChangeLogCleanupService(
		repos.StudentFieldEdit,
		repos.DataDeletion,
		settingsService,
		logger.With("service", "student-change-log-cleanup"),
	)

	autoStartService := timetableplanning.NewAutoStartService(timetableplanning.AutoStartDependencies{
		InstanceRepo:      repos.ActivityInstance,
		InstanceStaffRepo: repos.InstanceStaff,
		InstanceService:   instanceService,
		RoomRepo:          repos.Room,
		Conflicts:         timetableConflicts,
		Logger:            logger.With("service", "timetable-auto-start"),
	})
	autoEndService := timetableplanning.NewAutoEndService(repos.ActivityInstance, instanceService)

	classExceptions, err := NewClassArrivalExceptions(classArrivalQueries, persons)
	if err != nil {
		return nil, err
	}
	arrivalScheduleService, err := NewArrivalSchedules(db, repos.CarePlan(), persons, arrivalBaselines,
		ClassArrivalPlans(classArrivalQueries), classExceptions, logger.With("service", "arrival-schedule"))
	if err != nil {
		return nil, err
	}

	timetableOperationsService := timetableplanning.NewTimetableOperationsService(timetableplanning.TimetableOperationsDependencies{
		InstanceRepo:       repos.ActivityInstance,
		InstanceStaffRepo:  repos.InstanceStaff,
		InstanceStudents:   repos.InstanceStudent,
		InstanceService:    instanceService,
		ActiveGroupRepo:    repos.ActiveGroup,
		ActivityGroupRepo:  repos.ActivityGroup,
		ActiveService:      activeService,
		ArrivalService:     arrivalScheduleService,
		PickupService:      pickupScheduleService,
		CareDayService:     careDayService,
		SupervisorRepo:     repos.GroupSupervisor,
		Presence:           newStudentPresence(db, logger),
		StudentRepo:        repos.Student,
		EducationGroupRepo: repos.Group,
		RoomRepo:           repos.Room,
		PersonService:      timetableOperationPeople{OperationPersonService: usersService, membership: membership},
		PlanningTrackRepo:  repos.PlanningTrack,
		Settings:           settingsService,
		Broadcaster:        realtimeHub,
		DB:                 db,
		Logger:             logger.With("service", "timetable-operations"),
		Now:                now,
		RecoveryRepo:       recoveryRepo,
	})

	tenantDomain := strings.TrimSpace(cfg.TenantDomain)
	if tenantDomain == "" {
		return nil, fmt.Errorf("TENANT_DOMAIN is required")
	}

	// Operator frontend URL for invitation emails. The operator subdomain is separate
	// from FRONTEND_URL, so we link directly to the operator host to avoid a
	// cross-origin redirect hop that email content scanners treat as a phishing
	// signal. Constructed conditionally - only required when actually sending
	// invitations. InviteOperator and ResendOperatorInvitation guard on empty
	// operatorFrontendURL.
	var operatorFrontendURL string
	if operatorHostname := cfg.OperatorHostname; operatorHostname != "" {
		protocol := "http"
		if strings.HasPrefix(frontendURL, "https://") {
			protocol = "https"
		}
		operatorFrontendURL = fmt.Sprintf("%s://%s", protocol, strings.TrimRight(operatorHostname, "/"))
	}
	if operatorFrontendURL == "" {
		return nil, fmt.Errorf("NEXT_PUBLIC_OPERATOR_HOSTNAME is required")
	}

	sessionTokenAuth, err := authjwt.NewTokenAuthWithDurations(cfg.JWTSecret, cfg.JWTExpiry, cfg.JWTRefreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("invalid auth JWT configuration: %w", err)
	}
	sessionCodec, err := signedIdentityTokensOf(sessionTokenAuth)
	if err != nil {
		return nil, err
	}

	// Identity & Access serves tenant, parent and school login, refresh,
	// switching, logout, session validation, cleanup and revocation (#3251),
	// the account lifecycle (#3225), the second factor and both portals'
	// passkey ceremonies (#3331) and the operator flows (#3252, #3332). The
	// module reads the tenant runtime back at call time, so SetTenantRuntime
	// keeps its meaning after composition.
	operatorDependencies, err := newOperatorDependencies(operatorAuthenticationWiring{
		repos:         operatorRepositoriesOf(repos),
		organizations: organizations,
		persons:       persons,
		membership:    membership,
		logger:        platformLogger,
	})
	if err != nil {
		return nil, err
	}
	// Email change tokens deliberately reuse PASSWORD_RESET_TOKEN_EXPIRY_MINUTES
	// because both serve the same purpose (one-time verification links with the same
	// delivery constraints and security profile). If the two ever need to diverge,
	// introduce EMAIL_CHANGE_TOKEN_EXPIRY_MINUTES and fall back to passwordResetTokenExpiry.
	// The 15-minute floor accounts for email delivery latency + user interaction time.
	emailChangeExpiry := passwordResetTokenExpiry
	if emailChangeExpiry < 15*time.Minute {
		logger.Warn("email change token expiry bumped to minimum 15 minutes",
			slog.Int("configured_minutes", int(passwordResetTokenExpiry.Minutes())),
			slog.Int("effective_minutes", 15),
		)
		emailChangeExpiry = 15 * time.Minute
	}

	// The delivery module is composed after the identity module, so the
	// guardian mail reads its outbox at call time.
	var emailOutboxService *emailoutbox.Service
	identityAccess, err = newIdentityAccessWithSessions(db, accountAuthenticationWiring{
		repos:    sessionRepositoriesOf(repos, organizations),
		codec:    sessionCodec,
		settings: settingsService,
		audit:    auditCommand,
		logger:   authLogger,
		observe:  observeIdentityAccess,
		mfa: &mfaWiring{
			repos: repos, settings: settingsService,
			dispatcher: dispatcher, defaultFrom: defaultFrom, frontendURL: frontendURL,
			jwtSecret: cfg.JWTSecret, logger: authLogger,
			passkeys: &identityaccessCompose.PasskeyDependencies{
				RPID: tenantDomain, RPName: "moto",
				TenantDomain: tenantDomain, OperatorFrontendURL: operatorFrontendURL,
			},
		},
		operators:          operatorDependencies,
		demoAccess:         demoAccessWiringFor(appEnv, dispatcher, defaultFrom, frontendURL, viper.GetInt("demo_max_active_schools"), authLogger),
		demoStandingSchool: standingDemoSchool(viper.GetBool("demo_standing_school")),
		operatorLinks: &operatorLinkWiring{
			dispatcher: dispatcher, defaultFrom: defaultFrom,
			frontendURL: frontendURL, operatorFrontendURL: operatorFrontendURL,
			invitationExpiry: invitationTokenExpiry, emailChangeExpiry: emailChangeExpiry,
			logger: platformLogger,
		},
		resets: &passwordResetWiring{
			dispatcher: dispatcher, defaultFrom: defaultFrom,
			staffURL: frontendURL, parentsURL: parentsURL, schoolURL: schoolURL,
			expiry: passwordResetTokenExpiry, rateLimitEnabled: cfg.RateLimitEnabled,
		},
		invitations: &invitationWiring{
			dispatcher: dispatcher, defaultFrom: defaultFrom, staffURL: frontendURL, schoolURL: schoolURL,
			mailIdentity: tenantMailIdentity, expiry: invitationTokenExpiry,
		},
		// The lifecycle flows (#3225) read the retained role management back
		// at call time; it is composed below.
		lifecycle: &lifecycleWiring{
			settings: settingsService, audit: auditCommand,
			caregivers: caregiverProfiles{persons: persons, membership: membership},
			guardianMail: &guardianInvitationWiring{
				settings: settingsService, schools: organizations,
				outbox:      func() platformModels.OutboxEnqueuer { return outboxEnqueuer{outbox: emailOutboxService} },
				enrollments: repos.ParentEnrollmentRequest,
				// The accept and login links go to the parents portal, never
				// to the staff frontend.
				parentsURL: parentsURL, fallbackExpiry: invitationTokenExpiry,
				logger: authLogger.With("flow", "guardian_invitation"),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	invitationService := InvitationCapability(identityAccess)

	// Delivery composition is declared here so legacy email producers and the
	// guardian invitation service share the same durable capability.
	// Every outbox kind's renderer is known at startup, so the registry is
	// complete before the worker can claim anything. Enrollment keeps one
	// renderer per kind so subjects + templates stay independent; the
	// rollover pair reuses the submission template as a placeholder until
	// proper branded copy lands. Calendar appointments share one renderer
	// for all four kinds.
	enrollmentRendererCfg := enrollment.EmailRendererConfig{DefaultFrom: defaultFrom}
	appointmentRenderer := emailoutbox.RendererFunc(NewCalendarAppointmentRenderer(CalendarEmailDependencies{
		DefaultFrom: defaultFrom,
		DB:          db,
		Guardians:   repos.StudentGuardian,
	}))
	emailTemplateRegistry := emailoutbox.NewTemplateRegistry(map[string]emailoutbox.Renderer{
		platformModels.EmailKindGuardianInvitation: guardianInvitationRenderer(NewGuardianInvitationRenderer(GuardianInvitationRendererConfig{
			DefaultFrom: defaultFrom,
		})),
		platformModels.EmailKindParentAnnouncement: emailoutbox.RendererFunc(communicationCompose.NewParentAnnouncementRenderer(communicationCompose.ParentAnnouncementEmailConfig{
			DefaultFrom: defaultFrom,
		})),
		platformModels.EmailKindParentMessage:                      emailoutbox.RendererFunc(communicationCompose.NewParentMessageRenderer(communicationCompose.ParentMessageRendererConfig{DefaultFrom: defaultFrom})),
		platformModels.EmailKindAppointmentPublished:               appointmentRenderer,
		platformModels.EmailKindAppointmentUpdated:                 appointmentRenderer,
		platformModels.EmailKindAppointmentCancelled:               appointmentRenderer,
		platformModels.EmailKindAppointmentReminder:                appointmentRenderer,
		platformModels.EmailKindEnrollmentSubmitted:                emailoutbox.RendererFunc(enrollment.NewEnrollmentSubmittedRenderer(enrollmentRendererCfg)),
		platformModels.EmailKindEnrollmentAdminNotify:              emailoutbox.RendererFunc(enrollment.NewEnrollmentAdminNotificationRenderer(enrollmentRendererCfg)),
		platformModels.EmailKindEnrollmentApproved:                 emailoutbox.RendererFunc(enrollment.NewEnrollmentApprovedRenderer(enrollmentRendererCfg)),
		platformModels.EmailKindEnrollmentWaitlisted:               emailoutbox.RendererFunc(enrollment.NewEnrollmentWaitlistedRenderer(enrollmentRendererCfg)),
		platformModels.EmailKindEnrollmentRejected:                 emailoutbox.RendererFunc(enrollment.NewEnrollmentRejectedRenderer(enrollmentRendererCfg)),
		platformModels.EmailKindEnrollmentDecisionDigest:           emailoutbox.RendererFunc(enrollment.NewEnrollmentDecisionDigestRenderer(enrollmentRendererCfg)),
		platformModels.EmailKindEnrollmentChangeRequestSubmitted:   emailoutbox.RendererFunc(enrollment.NewEnrollmentChangeRequestSubmittedRenderer(enrollmentRendererCfg)),
		platformModels.EmailKindEnrollmentChangeRequestQuestion:    emailoutbox.RendererFunc(enrollment.NewEnrollmentChangeRequestQuestionRenderer(enrollmentRendererCfg)),
		platformModels.EmailKindEnrollmentChangeRequestParentReply: emailoutbox.RendererFunc(enrollment.NewEnrollmentChangeRequestParentReplyRenderer(enrollmentRendererCfg)),
		platformModels.EmailKindEnrollmentChangeRequestApproved:    emailoutbox.RendererFunc(enrollment.NewEnrollmentChangeRequestApprovedRenderer(enrollmentRendererCfg)),
		platformModels.EmailKindEnrollmentChangeRequestRejected:    emailoutbox.RendererFunc(enrollment.NewEnrollmentChangeRequestRejectedRenderer(enrollmentRendererCfg)),
		platformModels.EmailKindEnrollmentRolloverOptIn:            emailoutbox.RendererFunc(enrollment.NewEnrollmentRolloverOptInRenderer(enrollmentRendererCfg)),
		platformModels.EmailKindEnrollmentRolloverOptOut:           emailoutbox.RendererFunc(enrollment.NewEnrollmentRolloverOptOutRenderer(enrollmentRendererCfg)),
	})
	vapidConfig := notifications.VAPIDConfig{
		PublicKey: strings.TrimSpace(cfg.VAPIDPublicKey), PrivateKey: strings.TrimSpace(cfg.VAPIDPrivateKey),
		Subscriber: strings.TrimSpace(cfg.VAPIDSubscriber),
	}
	if err := vapidConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid VAPID configuration: %w", err)
	}
	deliveryRuntime, err := deliveryCompose.New(deliveryCompose.Dependencies{
		DB:     db,
		People: guardianDisplayResolver{query: guardianService},
		Provider: &deliveryProvider{
			registry: emailTemplateRegistry, mailer: mailer, mailIdentity: tenantMailIdentity,
			push: deliveryCompose.NewWebPushSender(deliveryModule.WebPushConfig{
				Subscriber: vapidConfig.Subscriber, PublicKey: vapidConfig.PublicKey, PrivateKey: vapidConfig.PrivateKey,
			}, newExpiredPushSubscriptionCleaner(db, repos.PushSubscription)),
			logger: logger.With("service", "delivery"), db: db,
			pushAuthorized: newPushAuthorizationChecker(db, repos.PushSubscription),
		},
		Observe: func(observation deliveryModule.Observation) {
			observeDurableDelivery(string(observation.Transport), observation.Template, observation.Operation, observation.Duration, observation.Count, observation.Err)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("initialize delivery module: %w", err)
	}
	emailOutboxWorker := deliveryRuntime.Worker
	emailOutboxService = emailoutbox.NewService(durableEmailAdapter{module: deliveryRuntime.Module})

	guardianInvitationService := GuardianInvitationCapability(identityAccess)

	caregiverCapabilityService := users.NewCaregiverCapabilityService(users.CaregiverCapabilityServiceDependencies{
		RequestActorScope:      requestActorScope,
		Identity:               caregiverIdentity{identityAccess},
		AuthEventRepo:          repos.AuthEvent,
		PersonRepo:             repos.Person,
		StaffRepo:              repos.Staff,
		CaregiverBindingLock:   repos.CaregiverBindingLock,
		TeacherRepo:            repos.Teacher,
		GroupTeacherRepo:       repos.GroupTeacher,
		GroupSubstitutionRepo:  repos.GroupSubstitution,
		GroupSupervisorRepo:    repos.GroupSupervisor,
		ActivitySupervisorRepo: repos.ActivitySupervisor,
		RoleAssignments:        identityRoles,
		DB:                     db,
	})

	// The Identity & Access caller context (#3501) resolves the caller's
	// chain through the owners of each link; the rows serve the consumers
	// that still speak the retained records.
	staffGroups, err := repositories.NewUserContextStaffGroups(groups, membership, workTime)
	if err != nil {
		return nil, err
	}
	userContextService, err := newCallerRows(callerContextWiring{
		Accounts:             identityAccess,
		Persons:              repos.Person,
		Membership:           membership,
		StaffGroups:          staffGroups,
		SupervisedActivities: supervisedActivityGroupIDs(repos.ActivityGroup),
		Presence:             newStudentPresence(db, logger),
		Settings:             settingsService,
		Logger:               usercontextLogger,
	}, repositories.CallerRowSources{
		Groups: repos.Group, Staff: repos.Staff, Teachers: repos.Teacher, Students: repos.Student,
		Activities: repos.ActivityGroup, Sessions: repos.ActiveGroup, Logger: usercontextLogger,
	})
	if err != nil {
		return nil, err
	}
	callerContext := userContextService.Caller()
	// Terminvertretungen move Betreuungsplan staffing across the Workforce and
	// Timetable line: the shift-plan-sync workflow serves the substitution
	// module's schedule port (#3418).
	scheduleSubstitution, err := shiftplansyncCompose.NewSubstitution(shiftplansyncCompose.SubstitutionDependencies{
		Instances: instanceService, ActivityInstances: repos.ActivityInstance, InstanceStaff: repos.InstanceStaff,
		Staff: repos.Staff, Broadcaster: realtimeHub, Logger: logger.With("service", "schedule-substitution"),
	})
	if err != nil {
		return nil, err
	}
	substitutionService := education.NewSubstitutionModule(education.SubstitutionDependencies{
		Groups: repos.Group, Substitutions: repos.GroupSubstitution, Persons: newEducationPersonQuery(persons),
		Teachers: repos.Teacher, Staff: repos.Staff, Actors: substitutionActorResolver{identity: callerContext},
		ActiveGroups: repos.ActiveGroup, ActiveSupervisors: repos.GroupSupervisor,
		ActiveSupervisorCreator: activeService,
		Audit:                   repos.SubstitutionChange, DB: db, Broadcaster: realtimeHub,
		Logger:   logger.With("service", "substitution"),
		Schedule: scheduleSubstitution,
		CanSeeAll: func(ctx context.Context, assignmentBound, admin, hasStaff bool) (bool, error) {
			if assignmentBound {
				return false, nil
			}
			scope, err := settingsService.ResolveString(ctx, configModels.KeyOperationalOverviewScope)
			if err != nil {
				return false, fmt.Errorf("resolve operational overview scope: %w", err)
			}
			return admin || (scope == configModels.OverviewScopeAllStaff && hasStaff), nil
		},
		Now: now,
	})

	// Initialize database stats service
	databaseService := database.NewService(database.StatsDependencies{
		Students: func(ctx context.Context) (int, error) {
			return repos.Student.CountEnrolled(ctx)
		},
		Teachers: func(ctx context.Context) (int, error) { rows, err := repos.Staff.List(ctx, nil); return len(rows), err },
		Rooms:    func(ctx context.Context) (int, error) { rows, err := repos.Room.List(ctx, nil); return len(rows), err },
		Activities: func(ctx context.Context) (int, error) {
			rows, err := repos.ActivityGroup.List(ctx, nil)
			return len(rows), err
		},
		Groups: func(ctx context.Context) (int, error) { rows, err := repos.Group.List(ctx, nil); return len(rows), err },
		Roles: func(ctx context.Context) (int, error) {
			rows, err := identityAccess.ListRoles(ctx, identityaccess.RoleFilter{})
			return len(rows), err
		},
		Devices: func(ctx context.Context) (int, error) {
			rows, err := repos.Device.List(ctx, nil)
			return len(rows), err
		},
		PermissionCount: func(ctx context.Context) (int, error) {
			rows, err := identityAccess.ListPermissions(ctx, identityaccess.PermissionFilter{})
			return len(rows), err
		},
	}, databaseLogger)

	// Initialize cleanup service
	activeCleanupService := presenceservice.NewPresenceCleanup(
		newStudentPresence(db, logger),
		sessionSupervisors,
		NewDeletionAudit(repos.DataDeletion),
		today,
	)
	unregisteredTagScanService, err := auditService.NewUnregisteredTagScanService(
		repos.UnregisteredTagScan,
		auditService.UnregisteredTagScanRuntime{TenantID: tenant.FromContext},
	)
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
	dataImports := newImports(importWiring{
		Identity:      guardianAccess,
		Organizations: organizations,
		Groups:        groups, Rooms: rooms,
		Persons: persons, Membership: membership, Workforce: workTime,
		CarePlan: repos.CarePlan(), Presence: newStudentPresence(db, logger),
		InvitationService: invitationService,
		OpeningBalance: importService.OpeningBalanceImportDeps{
			BalanceAdjustService: OpeningBalanceBookingCapability(staffBalanceAdjustService),
			StaffAbsenceService:  VacationTakeoverCapability(staffAbsenceService),
		},
		ConsentHistory: auditService.NewConsentRecorder(auditCommand),
		Audit:          auditCommand,
		Observe: func(observation importService.ImportObservation) {
			observeDataImport(observation)
		},
	})

	enrollmentFormSchemaService := enrollment.NewFormSchemaService(enrollment.FormSchemaServiceConfig{
		Owner:    repos.Enrollment(),
		Settings: settingsService,
		Logger:   logger.With("service", "enrollment-form-schema"),
	})

	enrollmentCaptchaService := enrollment.NewCaptchaService(enrollment.CaptchaServiceConfig{
		Settings:       settingsService,
		Logger:         logger.With("service", "enrollment-captcha"),
		RequireCaptcha: cfg.EnrollmentRequireCaptcha,
		SecretKey:      cfg.EnrollmentCaptchaSecretKey,
		SiteKey:        cfg.EnrollmentCaptchaSiteKey,
	})

	enrollmentDeletionPreview := enrollment.NewDeletionPreview(repos.Enrollment(), enrollmentGuardianDirectory{persons}, repos.EnrollmentOfferingAdjustment.CountForDeletion, repos.CarePlan().CountCareOfferingBookings)
	enrollmentDeletionService := enrollment.NewEnrollmentDeletionService(
		repos.Enrollment(),
		repos.Enrollment(),
		enrollmentDeletionPreview,
		repos.EnrollmentDeletionAudit,
		db,
		logger.With("service", "enrollment-deletion"),
		enrollmentDeliveryAdapter{module: deliveryRuntime.Module, tenantID: func(ctx context.Context) (int64, error) {
			id, err := tenant.TenantFromContext(ctx)
			return id.Int64(), err
		}},
	)
	enrollmentRejectedCleanupService := enrollment.NewRejectedEnrollmentCleanupService(
		repos.Enrollment(),
		repos.Enrollment(),
		repos.Enrollment(),
		enrollmentDeliveryAdapter{module: deliveryRuntime.Module, tenantID: func(ctx context.Context) (int64, error) {
			id, err := tenant.TenantFromContext(ctx)
			return id.Int64(), err
		}},
		settingsService,
		db,
		logger.With("service", "enrollment-rejected-cleanup"),
		enrollment.RejectedEnrollmentCleanupAuditDependencies{
			Deletion: enrollmentDeletionPreview,
			Audit:    repos.EnrollmentDeletionAudit,
		},
	)

	enrollmentPhaseService := enrollment.NewPhaseService(enrollment.PhaseServiceConfig{
		Owner:            repos.Enrollment(),
		CareOfferingRepo: enrollment.NewCareOfferingRepository(repos.CarePlan()),
		CalendarPeriods:  calendar,
		LockTemplateRecurrence: func(ctx context.Context) error {
			return timetableplanning.LockTenantRecurrenceWrites(ctx, db)
		},
		ValidateCareOfferingPhaseChange: careOfferingPhaseValidator.ValidatePhaseChange,
		Settings:                        settingsService,
		Responses:                       newPhaseResponseSources(repos.Enrollment(), persons, repos.CarePlan()),
		DB:                              db,
		Logger:                          logger.With("service", "enrollment-phase"),
	})
	enrollmentPhaseExpiryService := enrollment.NewPhaseExpiryService(enrollment.NewPhaseExpiryProjection(
		repos.Enrollment(), phaseExpiryStudents{query: persons}, phaseExpiryCarePlanDirectory{query: repos.CarePlan()},
		repos.Enrollment(),
	))

	studentAuditService := users.NewStudentAuditService(requestAuditActor, repositories.NewStudentAuditFor(persons))
	careLifecycleService, err := careplanCompose.NewCareLifecycle(careplanCompose.CareLifecycleDependencies{
		DB: db, Records: repos.CarePlan(),
		Owners: repositories.NewCareLifecycleOwners(db, repositories.CareLifecycleOwnerSources{
			Students: repos.Student, Persons: repos.Person, Membership: membership,
			People: persons, Timetable: timetableCapability, Calendar: calendar,
		}, repositories.NewCareEndRecorder(studentAuditService)),
		LockCareBookingWrites: func(ctx context.Context) error {
			return timetableplanning.LockTenantRecurrenceWrites(ctx, db)
		},
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return settingsService.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
		},
		Fingerprint: securityruntime.Fingerprint,
		Logger:      logger.With("service", "care_lifecycle"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose care lifecycle: %w", err)
	}
	users.WirePersonCareParticipation(usersService, careParticipationResolver(careLifecycleService))
	careplanCompose.WireCareParticipation(careDayService, careLifecycleService)
	enrollmentDecisionService := enrollment.NewDecisionService(enrollment.DecisionServiceConfig{
		Bookings:                  enrollmentCareBookingCommands{owner: repos.CarePlan()},
		Requests:                  repos.Enrollment(),
		Children:                  repos.Enrollment(),
		Guardians:                 repos.Enrollment(),
		LateInviteRepo:            repos.Enrollment(),
		ApprovedOfferings:         enrollment.NewApprovedOfferingProjection(repos.Enrollment(), offeringStudents{query: persons}),
		CareOfferingRepo:          enrollment.NewCareOfferingRepository(repos.CarePlan()),
		Phases:                    repos.Enrollment(),
		Schemas:                   repos.Enrollment(),
		DataAccessLogRepo:         repos.DataAccessLog,
		OfferingAdjustmentRepo:    repos.EnrollmentOfferingAdjustment,
		RestorationAuditRepo:      repos.EnrollmentRestorationAudit,
		SchoolRepo:                enrollmentSchoolDirectory{schools: organizations},
		PersonRepo:                repos.Person,
		StaffRepo:                 repos.Staff,
		StudentRepo:               repos.Student,
		StudentGuardianRepo:       repos.StudentGuardian,
		GuardianFinancialAudit:    repos.GuardianFinancialChange,
		StudentEnrollment:         persons,
		DepartureCompanions:       repositories.NewStudentCompanionRepository(repos.CarePlan()),
		DeleteDepartureCompanions: repos.CarePlan().DeleteCompanionEdges,
		GuardianProfileRepo:       repos.GuardianProfile,
		GuardianPhoneRepo:         repos.GuardianPhoneNumber,
		PickupScheduleRepo:        repos.StudentPickupSchedule,
		PickupBaselines:           pickupBaselines,
		ArrivalScheduleRepo:       repos.StudentArrivalSchedule,
		StudentEnrollmentRepo:     repos.StudentEnrollment,
		ActivityGroupRepo:         repos.ActivityGroup,
		ActivityScheduleRepo:      repos.ActivitySchedule,
		CalendarPeriodRepo:        repos.CalendarPeriod,
		TimeframeRepo:             repos.Timeframe,
		ActivityExceptionRepo:     repos.ActivityException,
		GuardianAccess:            guardianAccess,
		OutboxEnqueuer:            outboxEnqueuer{outbox: emailOutboxService},
		StudentAudit:              studentAuditService,
		StudentConsents:           studentConsentService,
		CareWithdrawal:            careLifecycleService,
		Broadcaster:               realtimeHub,
		PickupGuardianNotifier:    pillEmitter,
		FrontendURL:               frontendURL,
		ParentsURL:                parentsURL,
		Settings:                  settingsService,
		LockTemplateRecurrence: func(ctx context.Context) error {
			return timetableplanning.LockTenantRecurrenceWrites(ctx, db)
		},
		// Sourced-roster resyncs must also refresh already-materialized future
		// occurrences (#2147 review) — the materializer never revisits them.
		InstanceRosters: rosterReconciler,
		// Offering-sourced weekly Gehzeit changes move the same baseline a
		// staff weekly edit does, so they re-derive the auto excusals of the
		// students' future day exceptions too (#2360). The wrapper reuses an
		// ambient tenant transaction and opens one otherwise — the resync's
		// care-day locks are transaction-scoped.
		ResyncPickupAutoExcusals: func(ctx context.Context, studentIDs []int64) error {
			return tenant.WithTenantTx(ctx, db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
				for _, studentID := range studentIDs {
					if err := pickupAutoExcusal.ResyncFutureExceptions(txCtx, studentID); err != nil {
						return err
					}
				}
				return nil
			})
		},
		SnapshotPickupWeekdayChanges: func(ctx context.Context, studentID int64, date timezone.Date) (map[int]string, error) {
			return pickupAutoExcusal.SnapshotWeeklyPickups(ctx, studentID, date)
		},
		RecordPickupWeekdayChanges: func(ctx context.Context, studentID int64, date timezone.Date, before map[int]string) error {
			return pickupAutoExcusal.RecordWeeklyPickupChanges(ctx, studentID, date, careplan.WeeklyPickupSnapshot(before))
		},
		ClearPickupWeekdayExtension: timetableCapability.ClearPickupWeekdayExtension,
		// The reconciler takes these BEFORE writing weekly rows — the same
		// student → schedule-row → care-day lock order the staff weekly
		// editors use, so the two weekly writers cannot deadlock against
		// each other (#2360 review). Uses the ambient tenant transaction;
		// missing students (concurrent offboarding) are skipped, matching
		// the resync's tolerance.
		LockPickupStudents: func(ctx context.Context, studentIDs []int64) error {
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
		Logger: logger.With("service", "enrollment-decision"),
		Today:  today,
	})
	offeringRosterResyncer, ok := enrollmentDecisionService.(enrollment.OfferingRosterResyncer)
	if !ok {
		return nil, fmt.Errorf("enrollment decision service does not implement offering roster resync")
	}
	// Bind the split service's offering-roster hook now that the decision
	// service exists.
	resyncOfferingRoster = offeringRosterResyncer.ResyncTemplateOfferingRoster
	// Grade transitions rewrite school classes, so they must re-reconcile the
	// offering-sourced templates' Jahrgang-filtered rosters (#2137). The
	// workflow is composed here because the decision service that provides
	// the resync is constructed late; it binds the People Directory, the
	// School Membership, the Timetable roster reconciliation and the
	// recurrence gate, and builds School Structure and Student Presence over
	// the shared database itself (#2711).
	gradeTransitionResyncer, ok := enrollmentDecisionService.(education.OfferingSourceResyncer)
	if !ok {
		return nil, fmt.Errorf("enrollment decision service does not implement the grade-transition offering resync")
	}
	gradeTransitionWorkflow, err := gradetransitioncompose.New(gradetransitioncompose.Dependencies{
		DB: db, Directory: persons, Membership: membership, Rosters: rosterReconciler,
		LockRecurrenceWrites:  func(ctx context.Context) error { return timetableplanning.LockTenantRecurrenceWrites(ctx, db) },
		ResyncOfferingRosters: gradeTransitionResyncer.ResyncOfferingSourcedTemplates,
		Logger:                logger.With("workflow", "grade_transition"), Audit: auditCommand, Clock: now,
	})
	if err != nil {
		return nil, fmt.Errorf("compose grade transition workflow: %w", err)
	}
	// A care-offering edit changes the wanted roster of every template sourcing
	// it (#2147 review). Wired late because the decision service is constructed
	// after the care-offering service.
	careOfferingSourceBinder, ok := enrollmentCareOfferingService.(enrollment.CareOfferingSourceResyncBinder)
	if !ok {
		return nil, fmt.Errorf("enrollment care offering service does not accept the sourced-template resyncer")
	}
	careOfferingSourcedResyncer, ok := enrollmentDecisionService.(enrollment.CareOfferingSourcedTemplateResyncer)
	if !ok {
		return nil, fmt.Errorf("enrollment decision service does not implement the offering-update resync")
	}
	careOfferingSourceBinder.SetSourcedTemplateResyncer(careOfferingSourcedResyncer)
	pickupResyncBinder, ok := enrollmentCareOfferingService.(enrollment.CareOfferingPickupResyncBinder)
	if !ok {
		return nil, fmt.Errorf("enrollment care offering service does not accept the pickup resyncer")
	}
	pickupResyncer, ok := enrollmentDecisionService.(enrollment.CareOfferingPickupResyncer)
	if !ok {
		return nil, fmt.Errorf("enrollment decision service does not implement pickup resync")
	}
	pickupResyncBinder.SetPickupResyncer(pickupResyncer)
	// A phase service-window change re-bounds every roster row derived from
	// the phase's offerings, so the templates sourcing them must resync too
	// (#2147 review). Same late binding as above.
	phaseSourceBinder, ok := enrollmentPhaseService.(enrollment.CareOfferingSourceResyncBinder)
	if !ok {
		return nil, fmt.Errorf("enrollment phase service does not accept the sourced-template resyncer")
	}
	phaseSourceBinder.SetSourcedTemplateResyncer(careOfferingSourcedResyncer)

	enrollmentRequestService := enrollment.NewRequestService(enrollment.RequestServiceConfig{
		Requests:           repos.Enrollment(),
		Children:           repos.Enrollment(),
		Bookings:           enrollmentCareBookingCommands{owner: repos.CarePlan()},
		Guardians:          repos.Enrollment(),
		LateInviteRepo:     repos.Enrollment(),
		CareOfferingRepo:   enrollment.NewCareOfferingRepository(repos.CarePlan()),
		Catalog:            repos.Enrollment(),
		SchoolRepo:         enrollmentSchoolDirectory{schools: organizations},
		StudentRepo:        repos.Student,
		GuardianAuthorizer: repos.StudentGuardian,
		RateLimitRepo:      repos.Enrollment(),
		OutboxEnqueuer:     outboxEnqueuer{outbox: emailOutboxService},
		Settings:           settingsService,
		ManualDecider:      enrollmentDecisionService,
		FrontendURL:        frontendURL, // admin notification email
		ParentsURL:         parentsURL,  // parent confirmation/status emails
		DB:                 db,
		Logger:             logger.With("service", "enrollment-request"),
	})

	enrollmentReportService := enrollment.NewReportService(enrollment.ReportServiceConfig{
		Requests:               repos.Enrollment(),
		Children:               repos.Enrollment(),
		Guardians:              repos.Enrollment(),
		CareOfferingRepo:       enrollment.NewCareOfferingRepository(repos.CarePlan()),
		Schemas:                repos.Enrollment(),
		Phases:                 repos.Enrollment(),
		DataAccessLogRepo:      repos.DataAccessLog,
		StudentRepo:            repos.Student,
		StudentGuardianRepo:    repos.StudentGuardian,
		StudentCompanionRepo:   repositories.NewStudentCompanionRepository(repos.CarePlan()),
		PersonRepo:             repos.Person,
		EducationGroupRepo:     repos.Group,
		StudentStatusDayRepo:   repos.StudentStatusDay,
		ClassListEntries:       NewClassListEntryRosterReader(membership),
		PickupScheduleSvc:      pickupScheduleService,
		ArrivalScheduleSvc:     arrivalScheduleService,
		ClassArrivalExceptions: arrivalScheduleService,
		CareDaySvc:             careDayService,
		Settings:               settingsService,
		CareParticipation:      careLifecycleService,
	})
	// The class-day view's one write seam (#2970): a Lehrkraft sets the
	// class-wide arrival day exception through moto schule.
	classDayArrivalExceptionService := enrollment.NewClassDayArrivalExceptionService(enrollment.ClassDayArrivalExceptionConfig{
		ArrivalSchedule: arrivalScheduleService,
		Settings:        settingsService,
		BlockStarts:     timetableOperationsService,
		Broadcaster:     realtimeHub,
		Logger:          logger.With("service", "class-day-arrival-exceptions"),
	})
	enrollmentDecisionApplier, _ := enrollmentDecisionService.(enrollment.ChangeRequestDecisionApplier)

	studentService := users.NewStudentService(
		repositories.NewStudentDirectory(persons),
		persons,
		repos.Student,
	)
	// Created before the change-request service: its multi-child approval takes
	// the companion lock order through this service.
	companionService, err := careplanCompose.NewCompanions(repos.CarePlan(), repositories.NewCompanionStudents(repos.Student, persons, studentAuditService))
	if err != nil {
		return nil, fmt.Errorf("compose care plan companions: %w", err)
	}

	// Child documents (#777): metadata, per-category authority and the
	// per-child access gate for the Dokumente tab. Needs the user context to
	// answer "is this caller verified staff", so it is wired after it.
	studentDocumentService, err := newStudentDocuments(db, repos.CarePlan(), repos.Student, userContextService, repos.StudentFieldEdit, repos.DataAccessLog)
	if err != nil {
		return nil, fmt.Errorf("compose care plan documents: %w", err)
	}

	enrollmentChangeRequestService := enrollment.NewChangeRequestService(enrollment.ChangeRequestServiceConfig{
		Bookings:             enrollmentCareBookingCommands{owner: repos.CarePlan()},
		Requests:             repos.Enrollment(),
		Children:             repos.Enrollment(),
		Guardians:            repos.Enrollment(),
		LateInviteRepo:       repos.Enrollment(),
		CareOfferingRepo:     enrollment.NewCareOfferingRepository(repos.CarePlan()),
		Catalog:              repos.Enrollment(),
		SchoolRepo:           enrollmentSchoolDirectory{schools: organizations},
		GuardianProfileRepo:  repos.GuardianProfile,
		GuardianPhoneRepo:    repos.GuardianPhoneNumber,
		PersonRepo:           repos.Person,
		StudentRepo:          repos.Student,
		GuardianAuthorizer:   repos.StudentGuardian,
		DecisionService:      enrollmentDecisionApplier,
		CompanionGraphLocker: companionGraphCoordinator{CompanionLocks: companionService, strandings: repos.Student},
		Settings:             settingsService,
		OutboxEnqueuer:       outboxEnqueuer{outbox: emailOutboxService},
		FrontendURL:          frontendURL,
		ParentsURL:           parentsURL,
		DB:                   db,
		Logger:               logger.With("service", "enrollment-change-request"),
	})

	// Rollover service depends on DecisionService for the
	// rollover_auto_approve=true deadline path.
	enrollmentRolloverCatalogCloner, ok := enrollmentCareOfferingService.(enrollment.RolloverOfferingCatalogCloner)
	if !ok {
		return nil, fmt.Errorf("enrollment care offering service does not implement rollover catalog cloning")
	}
	enrollmentRolloverService := enrollment.NewRolloverService(enrollment.RolloverServiceConfig{
		Bookings:              enrollmentCareBookingCommands{owner: repos.CarePlan()},
		Phases:                repos.Enrollment(),
		Requests:              repos.Enrollment(),
		Children:              repos.Enrollment(),
		OfferingCatalogCloner: enrollmentRolloverCatalogCloner,
		SchoolRepo:            enrollmentSchoolDirectory{schools: organizations},
		OutboxEnqueuer:        outboxEnqueuer{outbox: emailOutboxService},
		Settings:              settingsService,
		DecisionService:       enrollmentDecisionService,
		ParentsURL:            parentsURL,
		DB:                    db,
		Logger:                logger.With("service", "enrollment-rollover"),
	})
	requestReviewPolicy := NewParentRequestReviewPolicy(callerContext.ParentRequestReviews)

	// One append-only ledger for every parent request, shared by all four
	// domains so a request's history survives edits, decisions and corrections
	// the request rows themselves overwrite (#2267).
	parentRequestEvents := users.NewParentRequestEventRecorder(repos.ParentRequestEvent)

	// The sharing rules live in the parents domain, which is composed after the
	// request services it serves, so the sharing port resolves that service
	// lazily through requestShareVisibility (#2267, story 47).
	var requestShareVisibility parentmessaging.ShareVisibilityResolver
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
		repos.CarePlan(),
		persons,
		arrivalScheduleService,
		pickupScheduleService,
		newStudentPresence(db, logger),
		pickupAutoExcusal,
		repos.CarePlan(),
		userContextService,
		pillEmitter,
		realtimeHub,
		requestReviewPolicy,
		parentRequestEvents,
		logger.With("service", "care-requests"),
		studentAuditService,
		WithRequestShareVisibility(requestShares),
		WithCareRequestToday(today),
	)

	// Post-enrollment offering changes (#1665): the parents portal submits them,
	// staff decide them on the same review page, and an approval applies the
	// switch through the decision service's dated adjustment path.
	directOfferingApplier, ok := enrollmentDecisionService.(enrollment.DirectOfferingAdjustmentApplier)
	if !ok {
		return nil, fmt.Errorf("enrollment decision service does not implement direct offering adjustment")
	}
	offeringChangeRequestService := enrollment.NewOfferingChangeRequestServiceWithPolicy(enrollment.OfferingChangeRequestServiceConfig{
		ChangeRepo:             enrollment.NewOfferingChangeRepository(repos.CarePlan(), offeringChangeStudentSearch{people: persons}),
		Children:               repos.Enrollment(),
		Requests:               repos.Enrollment(),
		Phases:                 repos.Enrollment(),
		CareOfferingRepo:       enrollment.NewCareOfferingRepository(repos.CarePlan()),
		ImpactRepo:             manualPlanningReader{db: db, courseGroups: timetableCapability},
		StudentRepo:            repos.Student,
		PersonRepo:             repos.Person,
		CareWithdrawalRepo:     repos.CareWithdrawal,
		OfferingAdjustmentRepo: repos.EnrollmentOfferingAdjustment,
		UserContext:            userContextService.Caller(),
		Applier:                enrollmentDecisionApplier,
		DirectApplier:          directOfferingApplier,
		Settings:               settingsService,
		Emitter:                pillEmitter,
		Logger:                 logger.With("service", "offering-change-requests"),
		Today:                  today,
		EventRecorder:          parentRequestEvents,
	}, requestReviewPolicy)
	pickupOfferingCoordinator, ok := offeringChangeRequestService.(enrollment.DirectOfferingAdjustmentCoordinator)
	if !ok {
		return nil, fmt.Errorf("offering change service does not implement direct pickup adjustment coordination")
	}
	pickupAdjustmentService := enrollment.NewPickupAdjustmentService(enrollment.PickupAdjustmentServiceConfig{
		PickupSchedules:     pickupScheduleService,
		ArrivalSchedules:    arrivalScheduleService,
		PickupScheduleRepo:  repos.StudentPickupSchedule,
		ArrivalScheduleRepo: repos.StudentArrivalSchedule,
		PickupBaselines:     pickupBaselines,
		Offerings:           pickupOfferingCoordinator,
		Settings:            settingsService,
		Audit:               studentAuditService,
		Students:            repos.Student,
		DB:                  db,
		Today:               today,
	})

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
			db, repos.StudentGuardian, logger.With("channel", "sse"))),
		notifications.NewDurableWebPushChannel(
			db, repos.PushSubscription, vapidConfig,
			durablePushAdapter{module: deliveryRuntime.Module}, logger.With("channel", "web_push"),
		),
	)
	notificationConsent, err := communicationCompose.NewNotificationConsent(communicationCompose.NotificationConsentConfig{
		DB:      db,
		Observe: observeCommunication,
	})
	if err != nil {
		return nil, err
	}
	notificationPreferencesService := notifications.NewPreferenceService(
		notificationConsent,
		settingsService,
		db,
		identityAccess,
	)
	staffNotificationRecipients := notifications.NewStaffRecipientResolver(
		notificationPreferencesService,
		repos.Student,
		repos.Group,
		repos.Staff,
		identityAccess,
		settingsService,
		repos.WorkSession,
	)
	staffParentMessageNotifier := notifications.NewStaffParentMessageNotifier(
		notificationsService,
		staffNotificationRecipients,
		repos.ParentMessageThread,
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
	excusedRequestService, err := newExcusedAbsenceRequests(excusedRequestWiring{
		carePlan: repos.CarePlan(), students: repos.Student, persons: repos.Person,
		scope:   parentRequestReviewScope(requestReviewPolicy),
		emitter: pillEmitter, broadcaster: realtimeHub, events: parentRequestEvents,
		notifier: absenceNotifier, observe: observeCarePlan,
		shares: requestShares,
		logger: logger.With("service", "excused-requests"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose excused absence requests: %w", err)
	}

	messagingService := communicationCompose.NewParentMessaging(communicationCompose.ParentMessagingConfig{
		ThreadRepo:  repos.ParentMessageThread,
		MessageRepo: repos.ParentMessage,
		ReadRepo:    repos.ParentMessageRead,
		Persons:     usersService,
		UserContext: userContextService,
		Settings:    settingsService,
		Broadcaster: realtimeHub,
		DB:          db,
		Logger:      logger.With("service", "messaging"),
		Notifier:    notificationsService,
		Preferences: notificationPreferencesService,
		// E-Mail an den Sorgeberechtigten bei neuer OGS-Nachricht (#2307): der
		// Rueckfall fuer alle, die Push nicht eingerichtet haben.
		Outbox:           outboxEnqueuer{outbox: emailOutboxService},
		GuardianProfiles: repos.GuardianProfile,
		Schools:          schoolNameDirectory{schools: organizations},
		LoginImages:      settingsService,
		ParentsURL:       parentsURL,
		Observe:          observeCommunication,
	})

	// OGS-internal colleague chat (#2598). Shares the transport with the
	// parent-OGS messenger (SSE hub + push) but none of its authorization:
	// access here is thread membership, nothing else.
	staffMessagingService := communicationCompose.NewStaffMessaging(communicationCompose.StaffMessagingConfig{
		ThreadRepo:  repos.StaffMessageThread,
		MessageRepo: repos.StaffMessage,
		ReadRepo:    repos.StaffMessageRead,
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
		CalendarFacts: repositories.CalendarFacts{
			StaffRepo:            repos.Staff,
			StudentRepo:          repos.Student,
			GuardianProfileRepo:  repositories.NewCalendarGuardianDirectory(repos.GuardianProfile, persons),
			StudentGuardianRepo:  repos.StudentGuardian,
			ChildRepo:            repos.ParentChild,
			GroupRepo:            repos.Group,
			InstanceStaffRepo:    repos.InstanceStaff,
			ActivityInstanceRepo: repos.ActivityInstance,
			RoomRepo:             repos.Room,
			StaffShiftRepo:       shiftRows,
			ShiftTypeRepo:        shiftTypeRows,
			SchoolRepo:           organizations,
			AccountRepo:          identityAccess,
			StaffFeedRepo:        identityAccess,
			PersonRepo:           repos.Person,
		},
		Appointments:           repos.Appointments(),
		UserContext:            userContextService.Caller(),
		DB:                     db,
		CalendarRenderer:       schoolCalendarRendererAdapter{renderer: repos.SchoolCalendar()},
		Outbox:                 emailOutboxService,
		PushOutbox:             durablePushAdapter{module: deliveryRuntime.Module},
		Settings:               settingsService,
		CalDAVPolicy:           calendarCalDAVPolicy{settings: settingsService},
		StaffFeedTombstoneRepo: repos.CalendarStaffFeedTombstone,
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
		ChildRepo:                 repos.ParentChild,
		EnrollablePhaseRepo:       repos.ParentEnrollablePhase,
		EnrollmentSettings:        settingsService,
		EnrollmentRequestRepo:     repos.ParentEnrollmentRequest,
		GuardianProfileRepo:       repos.GuardianProfile,
		Attendance:                newStudentPresence(db, logger),
		StatusDayRepo:             repos.StudentStatusDay,
		MealPlan:                  mealPlan,
		StudentRepo:               repos.Student,
		CarePlan:                  repos.CarePlan(),
		People:                    persons,
		PickupAutoExcusal:         pickupAutoExcusal,
		Settings:                  settingsService,
		Broadcaster:               realtimeHub,
		PersonRepo:                repos.Person,
		ChangeRequestRepo:         repos.StudentDataChangeRequest,
		CareRequestRepo:           repos.CareScheduleChangeRequest,
		ExcusedRequestRepo:        repos.ExcusedAbsenceRequest,
		OfferingChangeRequestRepo: enrollment.NewOfferingChangeRepository(repos.CarePlan(), offeringChangeStudentSearch{people: persons}),
		FamilyProtectionEvents:    repos.FamilyProtection,
		ParentRequestShares:       repos.ParentRequestShare,
		ParentRequestEvents:       parentRequestEvents,
		StudentAudit:              studentAuditService,
		MessageThreadRepo:         repos.ParentMessageThread,
		MessageRepo:               repos.ParentMessage,
		MessageReadRepo:           repos.ParentMessageRead,
		Conversations: communicationCompose.NewParentConversationCore(communicationCompose.ParentConversationConfig{
			ThreadRepo: repos.ParentMessageThread, MessageRepo: repos.ParentMessage, ReadRepo: repos.ParentMessageRead,
			Broadcaster: realtimeHub, Logger: logger.With("service", "parent"),
		}),
		ParentMessageNotifier: staffParentMessageNotifier,
		ArrivalSchedules:      arrivalScheduleService,
		PickupSchedules:       pickupScheduleService,
		CareRequests:          careRequestService,
		ExcusedRequests:       excusedRequestService,
		Emitter:               pillEmitter,
		AnnouncementRepo:      repos.ParentAnnouncement,
		GuardianInvites:       NewParentGuardianAccess(guardianInvitationService),
		GuardianInvitations:   newGuardianInvitationReads(func() identityaccess.GuardianInvitations { return identityAccess }),
		StudentGuardianRepo:   repos.StudentGuardian,
		GuardianPhoneRepo:     repos.GuardianPhoneNumber,
		StudentConsents:       studentConsentService,
		StudentPhotos: func() parentportal.StudentPhotoUnlinker {
			if factory == nil || factory.StudentPhotos == nil {
				return nil
			}
			return factory.StudentPhotos
		},
		AbsenceNotifier:  absenceNotifier,
		CarePeriods:      repos.Enrollment(),
		OfferingHistory:  repos.Enrollment(),
		CareOfferingRepo: enrollment.NewCareOfferingRepository(repos.CarePlan()),
		OfferingChanges:  offeringChangeRequestService,
		Logger:           logger.With("service", "parent"),
		Now:              now,
	})

	parentAnnouncementService := communicationCompose.NewParentAnnouncements(communicationCompose.ParentAnnouncementConfig{
		Repo:             repos.ParentAnnouncement,
		Settings:         settingsService,
		Outbox:           emailOutboxService,
		PushOutbox:       durablePushAdapter{module: deliveryRuntime.Module},
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
			Settings:         fileStorageSettings{service: settingsService},
			Events:           fileStorageEvents{repo: repos.FileEvent},
			FileCleanups:     documentCompose.NewFileCleanupStore(db),
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
	provisioningAdapters, err := repositories.NewOperatorProvisioningAdapters(repositories.OperatorProvisioningDependencies{
		DB:           db,
		Devices:      deviceFleet,
		Persons:      persons,
		Membership:   membership,
		PersonRepo:   repos.Person,
		StaffRepo:    repos.Staff,
		Accounts:     identityAccess,
		ActiveGroups: repos.ActiveGroup,
		Supervisors:  repos.GroupSupervisor,
		Categories:   repos.ActivityCategory,
		AuditLog:     repos.OperatorAuditLog,
	})
	if err != nil {
		return nil, fmt.Errorf("compose operator provisioning adapters: %w", err)
	}
	operatorProvisioningService, err := newOperatorProvisioning(operatorProvisioningSources{
		accounts:       repositories.NewOperatorAccountDirectory(identityAccess, persons, membership, organizations),
		organizations:  organizations,
		adapters:       provisioningAdapters,
		sessions:       identityAccess,
		invitations:    invitationService,
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

	listExportService := listexport.NewService()
	// The Notfallliste is the emergency snapshot read projection (#2704): the
	// owner facades are its tenant-safe reads, the retained student and
	// guardian repositories, the presence-mode read and the renderer its
	// compatibility bindings.
	emergencyService, err := emergencysnapshotlegacy.New(emergencysnapshotlegacy.Sources{
		Presence:     newStudentPresence(db, logger),
		PresenceMode: activeService,
		Students:     repos.Student,
		Persons:      persons,
		Contacts:     repos.StudentGuardian,
		Rooms:        rooms,
		Settings:     settingsService,
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
	slotListsService := classdayCompose.NewSlotLists(classdayCompose.SlotListDependencies{
		Timetable:       repositories.NewClassDayTimetableRows(db),
		Presence:        newStudentPresence(db, logger),
		CarePlan:        repos.CarePlan(),
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
	// planning screens use — it renders, it never writes. The retained
	// schedule services and repositories are its compatibility bindings.
	planExportService := planexportlegacy.New(planexportlegacy.Sources{
		Overview:       shiftPlanning.Overview,
		ShiftTypes:     shiftTypeRows,
		Instances:      repos.ActivityInstance,
		InstanceStaff:  repos.InstanceStaff,
		Students:       repos.InstanceStudent,
		Rooms:          repos.Room,
		Staff:          planExportStaffNames{staff: repos.Staff},
		ActivityGroups: repos.ActivityGroup,
		PlanningTracks: repos.PlanningTrack,
		ClosingDays:    planExportClosingDays{calendar: calendar},
		Holidays:       planExportHolidays{calendar: calendar},
		Renderer:       listExportService,
		Logger:         logger.With("service", "plan_export"),
	})

	pushSubscriptionsService := notifications.NewPushSubscriptionService(
		db,
		repos.PushSubscription,
		identityAccess,
		vapidConfig,
		logger.With("service", "push_subscriptions"),
	)

	pwaUsageService := pwa.NewUsageService(
		db,
		repos.PWAStandaloneUsage,
		pwaUsageCounts{provisioning: operatorProvisioningService},
		identityAccess,
		settingsService,
		logger.With("service", "pwa_usage"),
	)

	remindersService := reminderCompose.NewQuery(reminderPorts.QueryDependencies{
		Clock:        reminderClock(),
		CurrentStaff: reminderStaffIdentity(userContextService),
		Settings:     reminderSettings{settingsService},
		Attendance:   newStudentPresence(db, logger),
		Pickup:       reminderPickupReader{source: pickupScheduleService},
		Instance:     reminderTimetableReader{source: timetableCapability},
		Room:         reminderRoomReader{source: rooms},
		Student:      reminderStudentReader{source: repos.Student},
		Person:       reminderPersonReader{source: repos.Person},
		Supervision:  reminderSupervisionReader{source: activeService, presence: newStudentPresence(db, logger)},
		Visits:       reminderVisitReader{source: newStudentPresence(db, logger)},
		Logger:       logger.With("service", "reminders"),

		// Bulk readers for ComputeBatch. They answer the three genuinely
		// per-person facts for the whole tenant in one query each, which is what
		// keeps the per-minute cost flat in the number of staff.
		BulkSupervision:   reminderBulkSupervisionReader{source: repos.GroupSupervisor},
		BulkInstanceStaff: reminderTimetableReader{source: timetableCapability},
	})

	studentStatusDayService := presenceservice.NewStatusDays(
		repos.StudentStatusDay,
		NewManualPartialAbsenceDates(repos.CarePlan()),
		db,
		repos.CarePlan().LockExceptionDay,
		now,
	)
	studentStatusDayOverviewService := presenceservice.NewStatusDayOverviews(repos.StudentStatusDay, StatusDayOverviewPeople(usersService))
	ogsGroupLiveService, err := grouplivelegacy.New(grouplivelegacy.Sources{
		Presence:          newStudentPresence(db, logger),
		People:            usersService,
		Education:         educationService,
		Substitutions:     substitutionService,
		UserContext:       groupLiveCaller{userContextService},
		Active:            activeService,
		Settings:          settingsService,
		Pickups:           pickupScheduleService,
		Arrivals:          arrivalScheduleService,
		Instances:         instanceService,
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
		ActiveGroups: openRoomSessionPresence{newStudentPresence(db, logger), timetableCapability},
		OpenVisits:   activeService,
		Rooms:        openRoomDirectory{rooms: rooms},
		UserContext:  supervisionCaller{userContextService},
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

	timetableDataService := timetableplanning.NewTimetableDataService(timetableplanning.TimetableDataDependencies{
		InstanceStudentRepo:        repos.InstanceStudent,
		ActivityInstanceRepo:       repos.ActivityInstance,
		ActivityExceptionRepo:      repos.ActivityException,
		ActivityScheduleRepo:       repos.ActivitySchedule,
		InstanceStaffRepo:          repos.InstanceStaff,
		ActiveGroupRepo:            repos.ActiveGroup,
		SupervisorRepo:             repos.GroupSupervisor,
		ArrivalBaselines:           arrivalBaselines,
		ArrivalExceptionRepo:       repos.StudentArrivalException,
		PickupScheduleRepo:         repos.StudentPickupSchedule,
		PickupBaselines:            pickupBaselines,
		PickupExceptionRepo:        repos.StudentPickupException,
		Presence:                   newStudentPresence(db, logger),
		RoomRepo:                   repos.Room,
		ActivityCategoryRepo:       repos.ActivityCategory,
		PlanningTrackRepo:          repos.PlanningTrack,
		ActivityGroupRepo:          repos.ActivityGroup,
		ActivitySupervisorRepo:     repos.ActivitySupervisor,
		StudentEnrollmentRepo:      repos.StudentEnrollment,
		TimeframeRepo:              repos.Timeframe,
		EducationGroupRepo:         repos.Group,
		ValidateCareOfferingSeries: careOfferingSeriesValidator.ValidateTemplateSeries,
		ResyncOfferingRoster:       offeringRosterResyncer.ResyncTemplateOfferingRoster,
		ValidateOfferingSource:     careOfferingSeriesValidator.ValidateTemplateOfferingSource,
		DeviationEventRepo:         repos.DeviationEvent,
		AttendanceCorrectionRepo:   repositories.NewAttendanceCorrectionRepository(auditReadRuntime),
		PersonRepo:                 repos.Person,
		ConflictAcks:               timetableCapability,
		ConflictDetection:          timetableConflicts,
		RecoveryRepo:               recoveryRepo,
		Broadcaster:                realtimeHub,
		Logger:                     logger.With("service", "timetable-data"),
		DB:                         db,
		Today:                      today,
	})
	instanceSeriesConverter := timetableplanning.NewInstanceSeriesConversionService(timetableplanning.InstanceSeriesConversionDependencies{
		DB:              db,
		InstanceRepo:    repos.ActivityInstance,
		InstanceService: instanceService,
		TimetableData:   timetableDataService,
	})
	masterDataReviewService := users.NewMasterDataReviewServiceWithAuditAndPolicy(authjwt.PermissionsFromCtx,
		repos.StudentDataChangeRequest,
		repos.Student,
		repos.Person,
		userContextService,
		pillEmitter,
		studentAuditService,
		requestReviewPolicy,
		parentRequestEvents,
		logger.With("service", "master-data-review"),
		realtimeHub,
	)
	// Co-guardian notices (#2267, story 47). A staff decision on one parent.s
	// request tells the OTHER guardians that the child.s care changed. Whoever
	// the parent explicitly shared the request with gets the full pill; every
	// other guardian gets a neutral line with no reason and no author.
	//
	// Bound deliberately AFTER the request services and parentService exist:
	// the sharing rules live in the parents domain, and the request domains
	// must not import it just to ask who a request was shared with. The
	// offering and master-data services take it by setter; the care and
	// excused requests read it lazily through requestShares.
	var _ parentmessaging.ShareVisibilityResolver = parentService
	if resolver, ok := any(parentService).(parentmessaging.ShareVisibilityResolver); ok {
		requestShareVisibility = resolver
		for _, service := range []any{
			offeringChangeRequestService,
			masterDataReviewService,
		} {
			if sink, ok := service.(interface {
				SetRequestShareVisibility(parentmessaging.ShareVisibilityResolver)
			}); ok {
				sink.SetRequestShareVisibility(resolver)
			}
		}
	}

	excusedCoordinatorPort := excusedRequestCoordinatorPort{requests: excusedRequestService}
	parentRequestCoordinator := users.NewParentRequestCoordinator(authjwt.PermissionsFromCtx,
		masterDataReviewService.(users.MasterDataBulkReviewPort),
		excusedCoordinatorPort,
	)
	// Conflict-resolution ports (#2267, stories 6-10) — injected by setter, so
	// adding a domain to the resolver never rewrites the bulk-approval
	// constructor above. All five request kinds are wired here or the resolve
	// route answers conflict_kind_unsupported for the missing one.
	parentRequestCoordinator.SetMasterDataConflictPort(masterDataReviewService.(users.ParentRequestConflictPort))
	parentRequestCoordinator.SetExcusedConflictPort(excusedCoordinatorPort)
	parentRequestCoordinator.SetCareConflictPort(careRequestService.(users.ParentRequestConflictPort))
	parentRequestCoordinator.SetOfferingConflictPort(offeringChangeRequestService.(users.ParentRequestConflictPort))
	// The resolver records ONLY the staff-entered result. Every verdict it
	// takes goes through a domain Decide, which writes its own decided event.
	parentRequestCoordinator.SetEventRecorder(parentRequestEvents)

	factory = &Factory{
		settingsRuntimeDB:       db,
		Auth:                    identityAccess,
		Audit:                   auditCommand,
		MFA:                     identityAccess,
		Passkey:                 identityAccess,
		Active:                  activeService,
		ActiveCleanup:           activeCleanupService,
		WorkSession:             workSessionService,
		WorkTimeMonth:           workTimeMonthService,
		StaffAbsence:            staffAbsenceService,
		StaffBalanceAdjust:      staffBalanceAdjustService,
		StaffMonthClose:         staffMonthCloseService,
		StaffOverview:           staffOverviewService,
		TimeTrackingAuditLog:    timeTrackingAuditLogService,
		StaffTimeExport:         staffTimeExportService,
		Activities:              activitiesService,
		Education:               educationService,
		Substitution:            substitutionService,
		GradeTransition:         gradeTransitionWorkflow,
		Facilities:              facilitiesService,
		Schulhof:                schulhofService,
		WC:                      wcService,
		IoT:                     iotService,
		StaffClock:              staffClockService,
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
		TemplateSplit:           templateSplitService,
		TimetableCleanup:        timetableCleanupService,
		TimeTrackingCleanup:     timeTrackingCleanupService,
		StudentChangeLogCleanup: studentChangeLogCleanupService,
		Instance:                instanceService,
		AutoStart:               autoStartService,
		AutoEnd:                 autoEndService,
		TimetableOperations:     timetableOperationsService,
		Users:                   usersService,
		Birthdays:               birthdayService,
		StaffDocuments:          staffDocumentService,
		StudentDocuments:        studentDocumentService,
		FileStore:               fileStoreService,
		CaregiverCapability:     caregiverCapabilityService,
		Guardian:                guardianService,
		PeopleDirectory:         persons,
		GuardianProfileLoader:   guardianProfileLoader,
		UserContext:             userContextService,
		Database:                databaseService,
		DatabaseStatsCapabilities: func(ctx context.Context) database.StatsCapabilities {
			return securityruntime.DatabaseStatsCapabilities(authjwt.PermissionsFromCtx(ctx))
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
			Command:                   NewCalendarReminderCommand(db, calendarSvc),
			ParentAnnouncementCommand: parentAnnouncementService,
		},
		Notifications:            notificationsService,
		PushSubscriptions:        pushSubscriptionsService,
		PWAUsage:                 pwaUsageService,
		NotificationPreferences:  notificationPreferencesService,
		AbsenceNotifier:          absenceNotifier,
		RealtimeHub:              realtimeHub, // Expose SSE hub for API layer
		Tracker:                  tracker,     // Product analytics (PostHog)
		Invitation:               invitationService,
		GuardianInvitation:       guardianInvitationService,
		Mailer:                   mailer,
		DefaultFrom:              defaultFrom,
		FrontendURL:              frontendURL,
		InvitationTokenExpiry:    invitationTokenExpiry,
		PasswordResetTokenExpiry: passwordResetTokenExpiry,

		// The operator dashboard reads the directory and drives the
		// invitation and e-mail change flows through the same composed
		// Identity & Access module; the two fields name the two roles the
		// operator routes depend on (#3364).
		OperatorAuth:         identityAccess,
		OperatorInvitation:   identityAccess,
		OperatorProvisioning: operatorProvisioningService,
		Announcement:         communicationCapability,
		Schools:              organizations,
		Students:             StudentServices{Directory: studentService, Companions: companionService},
		CareLifecycle:        careLifecycleService,
		StudentAudit:         studentAuditService,
		StudentConsents:      studentConsentService,
		MasterDataReview:     masterDataReviewService,
		CareRequests:         careRequestService,
		OfferingChanges:      offeringChangeRequestService,
		PickupAdjustments:    pickupAdjustmentService,
		ExcusedRequests:      excusedRequestService,
		ParentRequests:       parentRequestCoordinator,
		RequestReviewPolicy:  requestReviewPolicy,
		StudentStatusDays:    studentStatusDayService,
		AbsenceOverview:      studentStatusDayOverviewService,
		StudentHistory:       presenceservice.NewStudentHistory(newStudentPresence(db, logger), historyRoomNames(rooms), NewDataAccessAudit(repos.DataAccessLog), NewHistorySlots(repos.InstanceStudent)),
		Statistics: newStatistics(db, logger, presenceCompose.StatisticsDependencies{
			StatusDays:  statisticsStatusDays{repos.CarePlan()},
			Courses:     newCourseStatistics(db),
			Holidays:    tenantHolidays{calendar: calendar},
			ClosingDays: tenantClosingDays{calendar: calendar},
			Periods:     statisticsReportPeriods{calendar},
			Students:    statisticsReportStudents{repos.Student},
			Rooms:       statisticsReportRooms{rooms},
			AccessLog:   statisticsAuditLog{repos.DataAccessLog},
			Retention:   statisticsRetention{settingsService},
			Logger:      logger.With("service", "statistics"),
			Now:         now,
		}),
		OGSGroupLive:            ogsGroupLiveService,
		SupervisionDashboard:    supervisionDashboardService,
		TimetableData:           timetableDataService,
		InstanceSeriesConverter: instanceSeriesConverter,
		OperatorMFA:             identityAccess,
		OperatorPasskey:         identityAccess,
		UnregisteredTagScans:    unregisteredTagScanService,

		EmailOutboxWorker: emailOutboxWorker,
		Delivery:          deliveryRuntime.Module,

		EnrollmentFormSchema:      enrollmentFormSchemaService,
		EnrollmentCareOffering:    enrollmentCareOfferingService,
		EnrollmentCaptcha:         enrollmentCaptchaService,
		EnrollmentRequest:         enrollmentRequestService,
		EnrollmentPhase:           enrollmentPhaseService,
		EnrollmentPhaseExpiry:     enrollmentPhaseExpiryService,
		EnrollmentDecision:        enrollmentDecisionService,
		EnrollmentReport:          enrollmentReportService,
		ClassDayArrivalExceptions: classDayArrivalExceptionService,
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

	factory.SettingsSideEffects = sideeffects.NewRegistry()
	facilitiesLegacy.RegisterSettingsSideEffects(factory.SettingsSideEffects, schulhofService, wcService)
	registerCareWithdrawalSettingsSideEffects(factory.SettingsSideEffects, careLifecycleService)
	tenantSettings := config.NewTenantOperations(
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
		Instances:       instanceService,
		TimetableData:   factory.TimetableData,
		InstanceStaff:   repos.InstanceStaff,
		Broadcaster:     factory.RealtimeHub,
		Logger:          logger.With("service", "shift_plan_sync"),
		Today:           today,
	})
	if err != nil {
		return nil, err
	}
	// The People Directory serves guardians through the owner's legacy
	// guardian service (#2663); bind it now that the service exists.
	factory.bindGuardianDirectory(persons, db)
	// Permanent child deletion is the owner workflow of #2710. It runs over
	// the bound People Directory, Care Plan and Timetable capabilities; the
	// photo unlink resolves lazily because EnableStudentPhotos runs later in
	// the API bootstrap.
	studentDeletion, err := studentdeletioncompose.New(studentdeletioncompose.Dependencies{
		DB: db, Directory: persons, CarePlan: repos.CarePlan(), Timetable: timetableCapability,
		Feedback: feedbackCounterOrUnconfigured(feedbackCounter), IsVerifiedStaff: userContextService.HasCurrentStaff,
		LockCareBookingWrites: func(ctx context.Context) error { return timetableplanning.LockTenantRecurrenceWrites(ctx, db) },
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

func optionalClock(clocks []func() time.Time) func() time.Time {
	if len(clocks) == 0 {
		return nil
	}
	if len(clocks) > 1 {
		panic("services.NewFactory accepts at most one clock")
	}
	return clocks[0]
}

// StudentPhotoBootstrap aggregates what api/base.go must provide to wire the
// photo lifecycle: the file IO is an api-layer concern (shared with the
// login-image and avatar upload helpers), and the owner's runtime slot is
// filled here because the factory holds the rest.
type StudentPhotoBootstrap struct {
	Unlinker users.PhotoUnlinker
	// PhotoRuntime is the slot the People Directory owner resolves its photo
	// runtime from; EnableStudentPhotos fills it with the surfaces this
	// factory holds.
	PhotoRuntime *peopleCompose.StudentPhotoRuntime
	Logger       *slog.Logger
}

// EnableStudentPhotos stores the photo lifecycle and registers its settings
// handler on f.SettingsSideEffects. Idempotent: repeated calls overwrite the
// prior service. Call once at API bootstrap.
func (f *Factory) EnableStudentPhotos(deps StudentPhotoBootstrap) {
	f.StudentPhotos = NewStudentPhotos(f.PeopleDirectory, deps.PhotoRuntime, StudentPhotoRuntimeDependencies{
		Settings:    f.Settings,
		Broadcaster: f.RealtimeHub,
		Unlinker:    deps.Unlinker,
		Consents:    f.StudentConsents,
		Logger:      deps.Logger,
	})
	users.RegisterStudentPhotoSettingsSideEffects(f.SettingsSideEffects, f.StudentPhotos)
}

// feedbackCounterOrUnconfigured keeps the reduced test graph constructible:
// the composition requires a Feedback owner, and a graph built without one
// fails at the first deletion preview instead of at startup, exactly as the
// retired provider did.
func feedbackCounterOrUnconfigured(counter users.FeedbackEntryCounter) users.FeedbackEntryCounter {
	if counter != nil {
		return counter
	}
	return unconfiguredFeedbackCounter{}
}

type unconfiguredFeedbackCounter struct{}

func (unconfiguredFeedbackCounter) CountForStudent(context.Context, int64) (int, error) {
	return 0, errors.New("student deletion: feedback counter is not configured")
}

// analyticsDeployment is the deployment property of every analytics event:
// "demo" for the public demo, otherwise the tenant domain of this instance.
// The frontend sends the same value (frontend/src/lib/analytics-deployment.ts).
func analyticsDeployment(appEnv, tenantDomain string) string {
	if IsDemoEnvironment(appEnv) {
		return "demo"
	}
	return strings.TrimSpace(tenantDomain)
}
