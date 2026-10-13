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
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetableblockdisplay"

	"github.com/moto-nrw/project-phoenix/database/repositories/pwausage"
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
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
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
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
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
	staffclock "github.com/moto-nrw/project-phoenix/workflows/staffclock/compose"
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
	Activities           arrivalTimetable.ActivityService
	Education            schoolstructure.GroupManagement
	Substitution         education.SubstitutionModule
	// GradeTransition is the owner workflow behind the school-year rollover
	// (#2711): the admin HTTP surface calls exactly its public commands.
	GradeTransition    *gradetransition.Workflow
	Facilities         facilitiesCompose.RoomService
	Schulhof           facilitiesCompose.SchulhofService
	WC                 facilitiesCompose.WCService
	Invitation         InvitationCapability
	GuardianInvitation GuardianInvitationCapability
	IoT                devicefleetCompose.IoTService
	StaffClock         *staffclock.Service
	Settings           settingsCompose.SettingsService
	TenantSettings     *settingsCompose.TenantOperations
	PayrollStatus      settingsCompose.PayrollStatusGetter
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
	TimeTrackingCleanup     timetracking.TimeTrackingCleanupService
	StudentChangeLogCleanup peopleCompose.StudentChangeLogCleanupService
	Instance                timetable.InstanceLifecycleCapability
	AutoStart               timetable.InstanceAutoStart
	AutoEnd                 timetable.InstanceAutoEnd
	TimetableOperations     timetable.OperationCapability
	Users                   *peopleCompose.PersonDirectory
	Birthdays               peopledirectory.Birthdays
	// StaffDocuments is Workforce's personnel-record administration: the
	// Dokumente tab, the Stammdaten sections and the payroll number (#3752).
	StaffDocuments            *workforceModule.StaffAdmin
	StudentDocuments          careplan.StudentDocuments
	FileStore                 *filestorageModule.Module
	CaregiverCapability       peopleCompose.CaregiverCapabilityService
	Guardian                  *peopleCompose.GuardianService
	PeopleDirectory           peopledirectory.Capability
	GuardianProfileLoader     *peopleCompose.GuardianProfileLoader
	UserContext               *CallerRows
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
	OperatorMFA             identityaccess.OperatorMFAFlows
	OperatorPasskey         identityaccess.OperatorPasskeyFlows
	UnregisteredTagScans    auditCompose.UnregisteredTagScanService

	// Delivery owns the leased email and push outboxes; EmailOutboxWorker
	// drains both transports.
	EmailOutboxWorker *deliveryModule.Worker
	Delivery          *deliveryModule.Module

	// Enrollment domain (parent-enrollment PR 5+).
	EnrollmentFormSchema   enrollmentOwner.FormSchemaAdministration
	EnrollmentCareOffering careplan.CareOfferingCapability
	EnrollmentCaptcha      enrollmentOwner.CaptchaVerifier
	EnrollmentRequest      *enrollmentCompose.Intake
	EnrollmentPhase        enrollmentOwner.PhaseAdministration
	EnrollmentPhaseExpiry  enrollmentOwner.PhaseExpiryWarnings
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
	EnrollmentRejectedCleanup EnrollmentRejectedCleanup

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
	StudentConsents *StudentConsents
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

// ProductEventTracker captures the product analytics events of the services
// this graph composes. The composition root owns the tracker and its lifetime.
type ProductEventTracker interface {
	Capture(distinctID, event string, props map[string]any)
}

func newAuditCommand(store auditModels.AppendStore, logger *slog.Logger, observe AuditAppendObserver) (auditModels.Command, error) {
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
	observeDataImport DataImportObserver,
	productEvents ProductEventTracker,
	fileStorage FileStorageWiring,
	clocks ...func() time.Time,
) (*Factory, error) {
	if carePlan == nil || organizations == nil || persons == nil || groups == nil || rooms == nil || membership == nil || calendar == nil || timetableCapability == nil || appointmentCapability == nil || communicationCapability == nil || observeCommunication == nil || observeCarePlan == nil || mealPlan == nil || bindMealPlanSettings == nil || feedbackCounter == nil || bindFeedbackSettings == nil || observeAuditAppend == nil || observeDelivery == nil || observeDurableDelivery == nil || observeDeviceFleet == nil || observeIdentityAccess == nil || workTime == nil || observeDataImport == nil || fileStorage.Objects == nil || fileStorage.Observe == nil {
		return nil, errors.New("organization tenancy, people directory, school structure, facilities, school membership, school calendar, timetable, appointments, communication, care plan, meal plan, feedback, Audit, Delivery, Identity & Access, Workforce, and Data Import capabilities with their binders and observers are required")
	}
	communicationCompose.InstallMessageQueryInstrumentation(db)
	cfg := currentFactoryConfig()
	cfg.PublicAPIURL = publicAPIURL
	return newFactory(carePlan, db, logger, cfg, tenantRuntime, organizations, persons, groups, rooms, membership, calendar, timetableCapability, appointmentCapability, communicationCapability, observeCommunication, observeCarePlan, mealPlan, bindMealPlanSettings, feedbackCounter, bindFeedbackSettings, observeAuditAppend, observeDelivery, observeDurableDelivery, observeDeviceFleet, observeIdentityAccess, workTime, observeDataImport, productEvents, fileStorage, false, clocks...)
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
	observeDataImport DataImportObserver,
	productEvents ProductEventTracker,
	fileStorage FileStorageWiring,
	allowAuditRootWrites bool,
	clocks ...func() time.Time,
) (*Factory, error) {
	now := optionalClock(clocks)
	today := timezone.CalendarDateClock(now)
	roomRepository := NewRoomRepository(rooms)
	// The retained activities and schedule repositories belong to Timetable;
	// they are built over the capabilities this graph binds (#2743).
	timetableRepos := NewTimetableRepositories(db, TimetableDependencies{
		Capability: timetableCapability, Students: persons, Groups: groups, Rooms: rooms,
		Calendar: calendar, Membership: membership, Workforce: workTime,
	})
	// The People Directory and School Membership repositories are built
	// where they are used (#2743), over the capabilities this graph binds.
	peopleIdentity := newPeopleIdentityAccess(db, observeIdentityAccess)
	personRepository := NewPersonRepository(db)
	studentRepository := NewStudentRepository(db, membership, groups)
	studentGuardianRepository := newStudentGuardianRepository(db, peopleIdentity, observeIdentityAccess)
	guardianProfileRepository := NewGuardianProfileRepository(db)
	guardianPhoneRepository := peopleCompose.NewGuardianPhoneNumberRepository(db)
	guardianFinancialRepository := peopleCompose.NewGuardianFinancialDataRepository(db)
	familyProtectionRepository := peopleCompose.NewFamilyProtectionEventRepository(db)
	parentRequestShareRepository := peopleCompose.NewParentRequestShareEventRepository(db)
	parentRequestEventRepository := peopleCompose.NewParentRequestEventRepository(db)
	members := newMembershipRepositories(db, personRepository, peopleIdentity, membership, rooms, workTime)
	staffRepository, teacherRepository, guestRepository := members.Staff, members.Teacher, members.Guest
	groupRepository, groupTeacherRepository, classTeacherRepository := members.Group, members.GroupTeacher, members.ClassTeacher
	groupSubstitutionRepository := newGroupSubstitutionRepository(groupRepository, membership, workTime)
	identityChain := lifecycleRepositories{
		persons: personRepository, staff: staffRepository, teachers: teacherRepository, students: studentRepository,
		guardianProfiles: guardianProfileRepository, studentGuardians: studentGuardianRepository,
	}
	// The repositories the retained services read are built here, over the
	// owners this graph binds (#2743).
	enrollmentModule := enrollmentCompose.New()
	enrollmentRecords := NewEnrollmentBookingProjection(enrollmentModule)
	sessionDevices, err := NewDeviceFleet(db)
	if err != nil {
		return nil, fmt.Errorf("compose session device fleet: %w", err)
	}
	sessionRecords := NewSessionRecords(db, SessionRecordOwners{
		Now: now, Devices: sessionDevices, Rooms: rooms, Timetable: timetableCapability, Membership: membership, Persons: persons,
	})
	groupSupervisions := NewGroupSupervisionRecords(sessionRecords, groups)
	staffAbsences := NewStaffAbsenceRepository(workTime, membership, persons)
	calendarPeriods := arrivalTimetable.NewCalendarPeriodRepository(calendar, arrivalTimetable.NewPlanningCalendarPeriodUsage(enrollmentModule, timetableCapability))
	statusDays := NewStudentStatusDayRepository(carePlan)
	parentPortal := NewParentPortalRepositories(db, ParentPortalOwners{
		People: persons, Groups: groups, Membership: membership,
		Identity: peopleIdentity, Enrollment: enrollmentModule, Now: now,
	}).WithSchools(organizations)
	caregiverBindingLock := NewCaregiverBindingLocker(membership, workTime, newStudentPresence(db, logger), timetableCapability)
	settingsRuntime := newSettingsRuntime(db, nil)

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
	if err := bindClassListEntryAdministration(membership, persons, auditCommand); err != nil {
		return nil, err
	}
	studentConsentService := NewStudentConsentsFor(persons, audits.StudentConsentChange)

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
		NewEducationRooms(rooms),
		NewEducationTeachers(teacherRepository),
		NewEducationStaff(staffRepository),
		studentRepository,
		groupSubstitutionRepository,
		education.NewLegacyRepositoryRuntime(db),
		education.GroupServiceOptions{Broadcaster: realtimeHub, Audit: NewEducationClassAssignmentAudit(audits.StaffMasterDataChange)},
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
			return enrollmentRecords.HasActiveClassRestrictedPhase(ctx)
		})
	}
	// Same for the grade-level restriction, which survives concrete-class
	// collection being off but not grade-level collection (#1663).
	if guarded, ok := settingsService.(interface {
		SetGradeRestrictionGuard(func(context.Context) (bool, error))
	}); ok {
		guarded.SetGradeRestrictionGuard(func(ctx context.Context) (bool, error) {
			return enrollmentRecords.HasActiveGradeRestrictedPhase(ctx)
		})
	}
	// And the cap probe, so enrollment.grade_level_max cannot be lowered below
	// a grade an active phase already restricts itself to — which would leave
	// that phase with no selectable eligible grade at all (#1663).
	if guarded, ok := settingsService.(interface {
		SetGradeCapGuard(func(context.Context) (int, error))
	}); ok {
		guarded.SetGradeCapGuard(func(ctx context.Context) (int, error) {
			return enrollmentRecords.MaxActivePhaseGrade(ctx)
		})
	}

	// Identity & Access is composed with the auth service below; the person
	// service reads its role administration at call time (#3314).
	var identityAccess *identityaccess.Module
	identityRoles := roleAdministration{current: func() *identityaccess.Module { return identityAccess }}

	// The staff and teacher lookups and the staff writes are School Membership
	// and Workforce territory (#3752); the person service takes them through
	// the StaffDirectory port.
	staffDirectory := NewStaffDirectory(StaffDirectoryDependencies{
		DB: db, Persons: personRepository, Staff: staffRepository, Teachers: teacherRepository, LehrkraftRoles: identityRoles,
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
		AccountExists:    AccountExists(peopleIdentity),
		StudentRepo:      studentRepository,
		TeacherRepo:      teacherRepository,
		StaffDirectory:   staffDirectory,
		CareParticipation: careParticipationResolver(func() careplan.CareParticipation {
			return careParticipation
		}),
	})

	// Birthday display (#1542): who is celebrating today, plus the school
	// settings and personal opt-out that decide who may be shown.
	birthdayService := NewBirthdays(
		BirthdayRepositories{Students: studentRepository, Staff: staffRepository, Persons: personRepository},
		settingsService, logger.With("service", "birthdays"), now)

	// Personnel record (#1417, #1423, #1424): the payroll number, the Stammdaten
	// sections and the Dokumente tab are Workforce's own administration; the
	// staff and person rows behind it and the audit trail are bound here.
	staffAdmin, err := NewStaffAdmin(StaffAdminDependencies{
		DB: db, Staff: staffRepository, Persons: personRepository, Membership: membership,
		MasterDataAudit: audits.StaffMasterDataChange, PersonnelNumber: audits.PersonnelNumberChange, DataAccessLog: audits.DataAccessLog,
		Logger: logger.With("service", "staff_documents"), Now: now,
	})
	if err != nil {
		return nil, fmt.Errorf("staff admin: %w", err)
	}

	// Replies to tenant-bound mail belong to the OGS, not to moto (#1936).
	// Built once here and shared by every tenant-bound sender.
	tenantMailIdentity := emailoutbox.NewTenantMailIdentity(schoolContactDirectory{schools: organizations}, func(ctx context.Context, tenantID int64) (string, error) {
		return settingsService.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, logger)

	// Initialize guardian service
	guardianService := peopleCompose.NewGuardianService(peopleCompose.GuardianServiceDependencies{
		GuardianProfileRepo:     guardianProfileRepository,
		GuardianPhoneNumberRepo: guardianPhoneRepository,
		StudentGuardianRepo:     studentGuardianRepository,
		GuardianInvitations:     newGuardianInvitationReads(func() identityaccess.GuardianInvitations { return identityAccess }),
		StudentRepo:             studentRepository,
		PersonRepo:              personRepository,
		GuardianFinancialRepo:   guardianFinancialRepository,
		GuardianFinancialAudit:  newGuardianFinancialAudit(audits.GuardianFinancialChange),
		DataAccessLog:           newGuardianDataAccessLog(audits.DataAccessLog),
		RolePresets:             guardianRolePresets{},
		DB:                      db,
	})

	// Thin loader that backs the public + parent enrollment me/profile
	// endpoints. Pulls the multi-schema join out of the handlers (Rule 1)
	// and into the existing GuardianProfileRepository.LoadProfileWithChildren.
	guardianProfileLoader := peopleCompose.NewGuardianProfileLoader(guardianProfileRepository, db, logger.With("service", "guardian-profile-loader"))

	// Public holidays per Bundesland (#1418 3a) and tenant closing days
	// (#1418 3b) share the Soll=0 semantics. The Soll consumers get the UNION
	// of both from the School Calendar; the statistics reports and the
	// holidays endpoint keep reading only the statutory holidays.
	nonWorkingDayService := nonWorkingDays{calendar: calendar}
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
	// Push subscriptions and PWA standalone usage belong to Delivery.
	pushSubscriptions := deliveryCompose.NewPushSubscriptionRepository(db)
	// Parent and staff messages belong to Communication; the stores are
	// built where the message services use them.
	parentMessages := parentStore.NewParentMessageRepository(db)
	// Work sessions, breaks, balances, vacation rows, absence audits and the
	// work-time templates belong to Workforce (#2690); the retained contracts
	// are served by adapters over the one facade, built where they are used.
	workSessions := workforceLegacy.NewWorkSessionRepository(workTime)
	workSessionBreaks := workforceLegacy.NewWorkSessionBreakRepository(workTime)
	staffAbsenceAudits := workforceLegacy.NewStaffAbsenceAuditRepository(workTime)
	staffVacationQuotas := workforceLegacy.NewStaffVacationQuotaRepository(workTime)
	staffVacationOpenings := workforceLegacy.NewStaffVacationOpeningRepository(workTime)
	staffBalanceAdjustments := workforceLegacy.NewStaffBalanceAdjustmentRepository(workTime)
	staffWorkSchedules := workforceRepositoryAdapter.NewStaffWorkScheduleRepository(workTime)
	workTimeModels := workforceRepositoryAdapter.NewWorkTimeModelRepository(workTime)
	staffAbsenceTypeService := AbsenceTypes(workTime)
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
	workSessionService := timetracking.NewWorkSessionService(workSessions, workSessionBreaks, NewWorkSessionAudit(audits.WorkSessionEdit), staffAbsences, groupSupervisions, sessionRecords, WorkSessionStaff(staffRepository, MustNewStaffEmployment(db)), NewWorkSessionSchedules(staffWorkSchedules), NewWorkSessionTimeModels(workTimeModels), PresenceSettings(settingsService), activeLogger, db, RenderTimeTrackingPDF, RenderTimeTrackingWorkbook,
		// Planned-shift lookups for the auto-checkout job (#1798).
		timetracking.WithWorkSessionShifts(NewTimeTrackingShifts(workTime)),
		timetracking.WithWorkSessionEvents(timeTrackingEvents),
		// The session service's weekly summaries reduce their Soll by holidays too.
		timetracking.WithWorkSessionHolidays(nonWorkingDayService),
		timetracking.WithWorkSessionAbsenceTypes(staffAbsenceTypeService),
		timetracking.WithWorkSessionTargetOverrides(staffTargetOverrideWeeks{overrides: workTime, months: func() timetracking.WorkTimeMonthService { return overrideMonths }}),
	)
	staffClockService := newStaffClockService(usersService, peopleIdentity, workSessionService)

	// Monatskarte read model (#1842) — everything computed on read, the
	// Übertrag is live.
	workTimeMonthService := timetracking.NewWorkTimeMonthService(
		workSessions,
		workSessionBreaks,
		staffAbsences,
		StaffScheduleAssignments(MustNewStaffEmployment(db)),
		NewWorkScheduleTargets(staffWorkSchedules),
		NewWorkTimeTargetModels(workTimeModels),
		NewTimeTrackingShifts(workTime),
		PresenceSettings(settingsService),
		activeLogger,
		timetracking.WithMonthHolidays(nonWorkingDayService),
		// Sonderarbeitszeiten (#3259) win over closing days and the schedule.
		timetracking.WithMonthTargetOverrides(workTime),
		// Stundenkonto transactions (#1420) enter the carry chain by effective date.
		timetracking.WithMonthAdjustments(staffBalanceAdjustments),
		// Frozen months (#1417) short-circuit the carry chain so a retroactive
		// correction can no longer rewrite a closed month's Übertrag.
		timetracking.WithMonthSnapshots(MonthSnapshotCapability(workTime)),
	)
	overrideMonths = workTimeMonthService

	// Initialize staff absence service
	staffAbsenceService := timetracking.NewStaffAbsenceService(staffAbsences, workSessions, staffVacationQuotas, staffAbsenceAudits, PresenceSettings(settingsService), workTimeMonthService,
		timetracking.WithAbsenceTypes(staffAbsenceTypeService),
		timetracking.WithAbsenceEvents(timeTrackingEvents),
		timetracking.WithAbsenceLogger(activeLogger),
		// Deletes leave an append-only tombstone in the cross-staff audit log
		// (#1417). The delete paths fail without this wiring.
		timetracking.WithAbsenceDeletionAudit(NewTimeTrackingDeletionAudit(audits.TimeTrackingDeletion)),
		// Vacation takeover at the moto introduction (#2132): the summary
		// subtracts pre-introduction days, the write paths book/delete them.
		timetracking.WithVacationOpenings(staffVacationOpenings),
		// Absence email notifications (#1419 4d).
		timetracking.WithAbsenceEmail(timetracking.AbsenceEmailDeps{
			Settings:    PresenceSettings(settingsService),
			Dispatcher:  absenceEmailDispatcher{dispatcher: dispatcher, from: defaultFrom, identity: tenantMailIdentity, logger: activeLogger},
			StaffRepo:   absenceEmailStaffDirectory{source: staffRepository},
			SchoolRepo:  absenceEmailSchoolDirectory{schools: organizations},
			FrontendURL: frontendURL,
			Logger:      activeLogger,
		}),
		timetracking.WithAbsenceShiftPlanSyncer(shiftplansyncCompose.DeferredSickCascade(func() shiftplansync.SickCascade { return shiftPlanSyncer })),
		// A rebooking inside a closed month (#3258) could not move its frozen
		// closing balance, so the service rejects it.
		timetracking.WithAbsenceMonthSnapshots(MonthSnapshotCapability(workTime)),
	)

	// Stundenkonto lifecycle transactions (#1420): payout, comp-time grants,
	// school-year reset. Reads the live balance through the month service.
	staffBalanceAdjustService := timetracking.NewStaffBalanceAdjustmentService(staffBalanceAdjustments, workTimeMonthService, PresenceSettings(settingsService), activeLogger,
		timetracking.WithAdjustmentEvents(timeTrackingEvents),
		// A booking inside a closed month (#1417) could not move its frozen
		// closing balance, so the ledger rejects it.
		timetracking.WithAdjustmentSnapshots(MonthSnapshotCapability(workTime)),
		// Deletes leave an append-only tombstone in the cross-staff audit log
		// (#1417). The delete path fails without this wiring.
		timetracking.WithAdjustmentDeletionAudit(NewTimeTrackingDeletionAudit(audits.TimeTrackingDeletion)),
	)

	// Monatsabschluss (#1417): freezes a month's closing balance so a
	// retroactive correction can no longer rewrite every later Übertrag.
	staffMonthCloseService := timetracking.NewStaffMonthCloseService(
		MonthSnapshotCapability(workTime),
		workTimeMonthService,
		MonthCloseStaff(staffRepository),
		PresenceSettings(settingsService),
		activeLogger,
		timetracking.WithMonthCloseEvents(timeTrackingEvents),
	)

	// Tenant-wide time-tracking views (#1417 2a). Prefetches all inputs once
	// and runs the SAME per-staff month math over in-memory readers, so the
	// list can never drift from the /staff/{id} detail view.
	staffOverviewService := timetracking.NewStaffOverviewService(
		OverviewStaff(staffRepository),
		workSessions,
		workSessionBreaks,
		staffAbsences,
		staffBalanceAdjustments,
		staffVacationQuotas,
		MonthSnapshotCapability(workTime),
		NewWorkScheduleTargets(staffWorkSchedules),
		NewWorkTimeTargetModels(workTimeModels),
		NewTimeTrackingShifts(workTime),
		PresenceSettings(settingsService),
		activeLogger,
		timetracking.WithOverviewHolidays(nonWorkingDayService),
		timetracking.WithOverviewTargetOverrides(workTime),
		// Vacation takeover (#2132): the Resturlaub column subtracts
		// pre-introduction days exactly like the /staff/{id} detail view.
		timetracking.WithOverviewVacationOpenings(staffVacationOpenings),
	)

	// Cross-staff payroll/evidence export (#1417 2b): rows via the overview's
	// prefetch (month) and the single-staff export cells (day); every download
	// writes an audit.data_access_logs row or fails.
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

	staffTimeExportService := timetracking.NewStaffTimeExportService(
		staffOverviewService,
		workSessionService,
		TimeExportStaff(staffRepository),
		NewTimeTrackingDataAccessAudit(audits.DataAccessLog),
		PayrollExportSettings{Source: payrollStatusService},
		activeLogger,
		RenderTimeTrackingWorkbook,
	)

	// Cross-staff audit feed (#1417): merges the four change trails into one
	// keyset-paginated view. Read-only; permission gating at the route.
	timeTrackingAuditLogService := timetracking.NewTimeTrackingAuditLogService(
		NewTimeTrackingAuditReader(audits.TimeTrackingAuditLog),
		StaffDisplayNames(staffRepository),
		PresenceSettings(settingsService),
	)

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
	approvedOfferingProjection := enrollmentCompose.NewApprovedOfferingProjection(enrollmentRecords, offeringStudents{query: persons})
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
		PickupExceptions:  NewPickupExceptionRepository(carePlan),
		ArrivalExceptions: NewArrivalExceptionRepository(carePlan),
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
		SchoolPresence:           newStudentPresence(db, logger),
		StudentDisplay:           studentDisplayProjection{students: persons, groups: groups},
		GroupRepo:                sessionGroups,
		SessionStartLock:         newStudentPresence(db, logger),
		SupervisorRepo:           sessionSupervisors,
		StudentStatusRepo:        statusDays,
		CrossTenantRepo:          NewCrossTenantQuery(newStudentPresence(db, logger), persons, groups),
		Schools:                  newActiveSchoolQuery(organizations),
		StudentRepo:              PresenceStudents(db, studentRepository),
		StaffRepo:                NewAttendanceStaffDirectory(staffRepository),
		RoomRepo:                 NewAttendanceRooms(roomRepository),
		YardRoomColor:            yardRoomColorQuery(rooms),
		ActivityGroupRepo:        NewSessionActivities(timetableRepos.GroupReads()),
		ActivityCatRepo:          NewAttendanceActivityCategories(timetableRepos.ActivityCategory),
		EducationGroupRepo:       NewAttendanceEducationGroups(groupRepository, studentRepository),
		DeviceRepo:               NewSessionDeviceDirectory(legacyDevices, settingsService, activeLogger),
		StaffNames:               NewAttendanceStaffNames(staffRepository, usersService),
		DB:                       db,
		Broadcaster:              realtimeHub,           // Pass SSE broadcaster
		Tracker:                  productEvents,         // Product analytics (PostHog)
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
	enrollmentCareOfferingService, err := newCareOfferingCatalog(careOfferingCatalogInputs{
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
		Occupancy: facilitiesLegacy.OccupancyProjection(roomOccupancyPresence{newStudentPresence(db, logger)}, timetableRepos.ActivityGroup, membership, persons),
		History:   facilitiesLegacy.HistoryProjection(roomHistoryPresence{newStudentPresence(db, logger)}, timetableRepos.ActivityGroup, membership, persons),
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
		facilitiesLegacy.OpenGroupCatalog(facilitiesGroupSupervisions(newStudentPresence(db, logger)), facilitiesRoomSessions(newStudentPresence(db, logger)), facilitiesGroupVisits(newStudentPresence(db, logger))),
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
	shiftRows := NewWorkforceShiftRows(workTime)
	shiftTypeRows := NewWorkforceShiftTypeRows(workTime)

	// Couples pulled-forward day pickup times with the per-block partial
	// absences (#2360). Shared by the staff pickup-exception writers and the
	// parent care-exception writers so both derive the same state.
	// Later pickups open a block decision for the Leitung (#3261).
	pickupAutoExcusal, err := careplanCompose.NewPickupAutoExcusal(careplanCompose.PickupExcusalDependencies{
		DB: db, Records: carePlan, Baselines: pickupBaselines,
		Blocks: newStudentPresence(db, logger), Preview: newPickupExcusalTimetable(timetableCapability, NewPartialAbsencePreview(db)), Extensions: newPickupExcusalTimetable(timetableCapability, NewPartialAbsencePreview(db)),
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
		Presence: newStudentPresence(db, logger),
		Dashboard: devicefleetLegacy.NewDashboardSources(devicefleetLegacy.DashboardDependencies{
			ActiveGroups:   sessionRecords,
			Templates:      timetableRepos.ActivityGroup,
			Instances:      timetableRepos.ActivityInstance,
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
	iotService := devicefleetCompose.NewIoTService(deviceFleet)

	partialAbsenceService, err := careplanCompose.NewPartialAbsences(db, carePlan, newStudentPresence(db, logger), pickupAutoExcusal)
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
	materializationDeps := timetableMaterializationDependencies(timetableMaterializationInputs{
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
		ArrivalExceptions: NewArrivalExceptionRepository(carePlan),
		Sessions:          sessionRecords,
		Shifts:            shiftRows,
		Presence:          newStudentPresence(db, logger),
		ArrivalBaselines:  arrivalBaselines,
		Logger:            logger.With("service", "timetable-conflicts"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable conflict detection: %w", err)
	}
	substituteConflicts, err := newTimetableSubstituteConflicts(timetableRows)
	if err != nil {
		return nil, fmt.Errorf("compose timetable substitute conflicts: %w", err)
	}
	instanceService, err := newTimetableLifecycle(timetableLifecycleInputs{
		Rows: timetableRows,
		Planning: arrivalTimetable.InstanceLifecycleDependencies{
			IdempotencyRepo: timetableRepos.InstanceIdempotency, ExceptionRepo: timetableRepos.ActivityException, CalendarPeriodRepo: calendarPeriods,
			DB: db,
		},
		Staff:               staffRepository,
		Sessions:            sessionRecords,
		Supervisions:        groupSupervisions,
		Presence:            newStudentPresence(db, logger),
		SessionEnder:        activeService,
		CareDays:            careDayService,
		CareDayLocks:        carePlan,
		GuardianNotices:     lateCareCancellationPublisher{resolve: func() communication.CareCancellationPublisher { return guardianNoticePublisher }},
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
	staffDeviationDeps, err := timetableStaffDeviationDependencies(timetableDeviationInputs{
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
	timetableCleanupService, err := newTimetableCleanup(timetableRetentionInputs{
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
	timeTrackingCleanupService := timetracking.NewTimeTrackingCleanupService(
		workSessions,
		staffAbsences,
		NewTimeTrackingRetentionAudit(audits.DataDeletion),
		PresenceSettings(settingsService),
		logger.With("service", "time-tracking-cleanup"),
	)

	// Per-child change-history retention cleanup (issue #1455). Deletes
	// audit.student_field_edits older than the tenant's retention window
	// (gdpr.student_change_log_retention_days, default 90). One per-student
	// DataDeletion audit row per run; shares the nightly cleanup window.
	studentChangeLogCleanupService := NewStudentChangeLogCleanup(
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

	timetableOperationsService, err := newTimetableOperations(timetableOperationInputs{
		Rows:           timetableRows,
		PlanningTracks: timetableCapability,
		Sessions:       sessionRecords,
		Supervisions:   groupSupervisions,
		Visits:         newStudentPresence(db, logger),
		Presence:       activeService,
		People:         timetableOperationPeople{OperationPeople: usersService, membership: membership},
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
		repos:         operatorRepositoriesOf(identityChain, operatorAuditLog),
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
		repos:    sessionRepositoriesOf(identityChain, organizations, audits.AuthEvent, pushSubscriptions),
		codec:    sessionCodec,
		settings: settingsService,
		audit:    auditCommand,
		logger:   authLogger,
		observe:  observeIdentityAccess,
		mfa: &mfaWiring{
			operatorAuditLog: operatorAuditLog, settings: settingsService,
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
			settings:     settingsService,
			outbox:       func() platformModels.OutboxEnqueuer { return outboxEnqueuer{outbox: emailOutboxService} },
			welcomeDelay: staffWelcomeDelay,
		},
		// The lifecycle flows (#3225) read the retained role management back
		// at call time; it is composed below.
		lifecycle: &lifecycleWiring{
			settings: settingsService, audit: auditCommand,
			caregivers: caregiverProfiles{persons: persons, membership: membership},
			guardianMail: &guardianInvitationWiring{
				settings: settingsService, schools: organizations,
				outbox:      func() platformModels.OutboxResultEnqueuer { return outboxEnqueuer{outbox: emailOutboxService} },
				enrollments: parentPortal.ParentEnrollmentRequest,
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
	appointmentRenderer := emailoutbox.RendererFunc(NewCalendarAppointmentRenderer(CalendarEmailDependencies{
		DefaultFrom: defaultFrom,
		DB:          db,
		Guardians:   studentGuardianRepository,
	}))
	var deliveryForGuardianWelcome *deliveryCompose.Runtime
	guardianPredecessorStatus := func(ctx context.Context, outboxID int64) (sent bool, terminal bool, err error) {
		if deliveryForGuardianWelcome == nil {
			return false, false, errors.New("delivery module is not initialized")
		}
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return false, false, fmt.Errorf("guardian welcome predecessor: tenant is required: %w", err)
		}
		state, found, err := deliveryForGuardianWelcome.EmailStatus(ctx, tenantID.Int64(), outboxID)
		if err != nil {
			return false, false, err
		}
		if !found {
			return false, true, nil
		}
		switch state {
		case deliveryModule.StateSent:
			return true, false, nil
		case deliveryModule.StateCancelled, deliveryModule.StateDeadLetter:
			return false, true, nil
		default:
			return false, false, nil
		}
	}
	renderers := map[string]emailoutbox.Renderer{
		platformModels.EmailKindGuardianInvitation: guardianInvitationRenderer(NewGuardianInvitationRenderer(GuardianInvitationRendererConfig{
			DefaultFrom: defaultFrom,
		})),
		platformModels.EmailKindGuardianWelcome: guardianInvitationRenderer(NewGuardianWelcomeRenderer(GuardianInvitationRendererConfig{
			DefaultFrom: defaultFrom, PrecedingEmailStatus: guardianPredecessorStatus,
		})),
		platformModels.EmailKindStaffWelcome: NewStaffWelcomeRenderer(StaffWelcomeRendererConfig{
			DefaultFrom: defaultFrom, MailIdentity: tenantMailIdentity, Logger: authLogger,
			InvitationDeliverySent: identityAccess.SchoolInvitationDeliverySent,
		}),
		platformModels.EmailKindParentAnnouncement: emailoutbox.RendererFunc(communicationCompose.NewParentAnnouncementRenderer(communicationCompose.ParentAnnouncementEmailConfig{
			DefaultFrom: defaultFrom,
		})),
		platformModels.EmailKindParentMessage:        emailoutbox.RendererFunc(communicationCompose.NewParentMessageRenderer(communicationCompose.ParentMessageRendererConfig{DefaultFrom: defaultFrom})),
		platformModels.EmailKindAppointmentPublished: appointmentRenderer,
		platformModels.EmailKindAppointmentUpdated:   appointmentRenderer,
		platformModels.EmailKindAppointmentCancelled: appointmentRenderer,
		platformModels.EmailKindAppointmentReminder:  appointmentRenderer,
	}
	for kind, renderer := range enrollmentMailRenderers(enrollmentCompose.NewMailRenderers(), defaultFrom) {
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
		People: guardianDisplayResolver{query: guardianService},
		Provider: &deliveryProvider{
			registry: emailTemplateRegistry, mailer: mailer, mailIdentity: tenantMailIdentity,
			push: deliveryCompose.NewWebPushSender(deliveryModule.WebPushConfig{
				Subscriber: vapidConfig.Subscriber, PublicKey: vapidConfig.PublicKey, PrivateKey: vapidConfig.PrivateKey,
			}, newExpiredPushSubscriptionCleaner(db, pushSubscriptions)),
			logger: logger.With("service", "delivery"), db: db,
			pushAuthorized: newPushAuthorizationChecker(db, pushSubscriptions),
		},
		Observe: func(observation deliveryModule.Observation) {
			observeDurableDelivery(string(observation.Transport), observation.Template, observation.Operation, observation.Duration, observation.Count, observation.Err)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("initialize delivery module: %w", err)
	}
	deliveryForGuardianWelcome = deliveryRuntime
	emailOutboxWorker := deliveryRuntime.Worker
	emailOutboxService = emailoutbox.NewService(durableEmailAdapter{module: deliveryRuntime.Module})

	guardianInvitationService := GuardianInvitationCapability(identityAccess)

	caregiverCapabilityService := peopleCompose.NewCaregiverCapabilityService(peopleCompose.CaregiverCapabilityServiceDependencies{
		RequestActorScope:      requestActorScope,
		Identity:               caregiverIdentity{identityAccess},
		Audit:                  newCaregiverCapabilityAudit(audits.AuthEvent),
		PersonRepo:             personRepository,
		StaffRepo:              staffRepository,
		CaregiverBindingLock:   caregiverBindingLock,
		TeacherRepo:            teacherRepository,
		GroupTeacherRepo:       groupTeacherRepository,
		GroupSubstitutionRepo:  groupSubstitutionRepository,
		GroupSupervisorRepo:    CaregiverSupervisionBlockers{Records: groupSupervisions},
		ActivitySupervisorRepo: arrivalTimetable.CaregiverPlannedSupervisionBlockers{Supervisors: timetableRepos.ActivitySupervisor},
		RoleAssignments:        identityRoles,
	})

	// The Identity & Access caller context (#3501) resolves the caller's
	// chain through the owners of each link; the rows serve the consumers
	// that still speak the retained records.
	staffGroups, err := NewUserContextStaffGroups(groups, membership, workTime)
	if err != nil {
		return nil, err
	}
	userContextService, err := newCallerRows(callerContextWiring{
		Accounts:             identityAccess,
		Persons:              personRepository,
		Membership:           membership,
		StaffGroups:          staffGroups,
		SupervisedActivities: arrivalTimetable.NewCallerActivityGroups(timetableRepos.ActivityGroup).SupervisedActivityGroupIDs,
		Presence:             newStudentPresence(db, logger),
		Settings:             settingsService,
		Logger:               usercontextLogger,
	}, CallerRowSources{
		Groups: groupRepository, Staff: staffRepository, Teachers: teacherRepository, Students: studentRepository,
		Activities: arrivalTimetable.NewCallerActivityGroups(timetableRepos.ActivityGroup), Sessions: sessionRecords, Logger: usercontextLogger,
	})
	if err != nil {
		return nil, err
	}
	callerContext := userContextService.Caller()
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
		Persons: newEducationPersonQuery(persons), Teachers: NewEducationCaregivers(teacherRepository),
		Staff: NewEducationStaff(staffRepository), ExternalCaregivers: NewEducationExternalCaregivers(staffRepository, guestRepository),
		Actors:       substitutionActorResolver{identity: callerContext},
		ActiveGroups: sessionRecords, ActiveSupervisors: groupSupervisions,
		ActiveSupervisorCreator: activeService,
		Audit:                   NewEducationSubstitutionAudit(audits.SubstitutionChange),
		Runtime:                 education.NewLegacyRepositoryRuntime(db), Broadcaster: realtimeHub,
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
		newStudentPresence(db, logger),
		sessionSupervisors,
		NewDeletionAudit(audits.DataDeletion),
		PresenceSettings(settingsService),
		today,
	)
	unregisteredTagScanService, err := auditCompose.NewUnregisteredTagScanService(
		newUnregisteredTagScanRepository(legacyDeviceFleet),
		auditCompose.UnregisteredTagScanRuntime{TenantID: tenant.FromContext},
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
		CarePlan: carePlan, Presence: newStudentPresence(db, logger),
		InvitationService: invitationService,
		OpeningBalance: importService.OpeningBalanceImportDeps{
			BalanceAdjustService: OpeningBalanceBookingCapability(staffBalanceAdjustService),
			StaffAbsenceService:  VacationTakeoverCapability(staffAbsenceService),
		},
		ConsentHistory: auditCompose.NewConsentRecorder(auditCommand),
		Audit:          auditCommand,
		Observe: func(observation importService.ImportObservation) {
			observeDataImport(observation)
		},
	})

	enrollmentCollection := enrollmentCollectionSettings{settings: settingsService}
	enrollmentFormSchemaService := enrollmentCompose.NewFormSchemas(enrollmentRecords, enrollmentCollection, logger.With("service", "enrollment-form-schema"))

	enrollmentCaptchaService := enrollmentCompose.NewCaptcha(enrollmentCaptchaSettings{settings: settingsService}, "", logger.With("service", "enrollment-captcha"))

	// Enrollment brands its mails and notifies decisions through the Delivery
	// outbox; the retained request, decision, change-request and rollover
	// services share this capability.
	enrollmentNotifications := newEnrollmentNotifications(enrollmentRecords, settingsService,
		outboxEnqueuer{outbox: emailOutboxService}, enrollmentSchoolDirectory{schools: organizations})

	// The admin deletion and the retention cleanup of rejected enrollments
	// share one impact preview (#3564).
	enrollmentDeletions := NewEnrollmentDeletionModule(EnrollmentDeletionSources{
		Owner: enrollmentRecords, Guardians: enrollmentGuardianDirectory{persons},
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
	enrollmentPhaseService := enrollmentCompose.NewPhases(enrollmentCompose.PhaseDependencies{
		Records:                         enrollmentRecords,
		Offerings:                       enrollmentPhaseOfferings{carePlan: carePlan},
		Calendar:                        calendar,
		LockTemplateRecurrence:          recurrenceLock.LockRecurrenceWrites,
		ValidateCareOfferingPhaseChange: careOfferingPhaseGuard(enrollmentCareOfferingService),
		SourcedTemplates:                func() enrollmentOwner.SourcedTemplateResyncer { return phaseSourcedTemplates },
		Settings:                        enrollmentCollection,
		Roster:                          phaseResponseRoster{query: persons},
		CareExits:                       phaseResponseCareExits{query: carePlan},
		PortalAccounts:                  persons,
		Logger:                          logger.With("service", "enrollment-phase"),
	})
	enrollmentPhaseExpiryService := enrollmentCompose.NewPhaseExpiryWarnings(enrollmentCompose.NewPhaseExpirySnapshots(
		enrollmentRecords, phaseExpiryStudents{query: persons}, phaseExpiryCarePlanDirectory{query: carePlan},
		enrollmentRecords,
	))

	studentAuditService := peopleCompose.NewStudentAuditService(requestAuditActor, NewStudentAuditFor(persons))
	careLifecycleService, err := careplanCompose.NewCareLifecycle(careplanCompose.CareLifecycleDependencies{
		DB: db, Records: carePlan,
		Owners: NewCareLifecycleOwners(CareLifecycleOwnerSources{
			Students: studentRepository, Persons: personRepository, Membership: membership,
			People: persons, Timetable: timetableCapability, Calendar: calendar,
			Presence: newUnloggedStudentPresence(db), PlanReads: mustPresenceReads(db),
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
	careBookings, err := newBookingMaterialization(bookingMaterializationInputs{
		Catalog: enrollmentCareOfferingService, Timetable: timetableCapability, Rosters: rosterReconciler,
		Students: persons, Periods: calendar, Enrollment: enrollmentRecords,
		Approved: enrollmentCompose.NewApprovedOfferingProjection(enrollmentRecords, offeringStudents{query: persons}),
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
	enrollmentDecisions := NewEnrollmentDecisions(EnrollmentDecisionSources{
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
		PickupSchedules:        arrivalTimetable.NewEnrollmentPickupSchedules(NewPickupScheduleRepository(carePlan)),
		ArrivalSchedules:       arrivalTimetable.NewEnrollmentArrivalSchedules(NewArrivalScheduleRepository(carePlan)),
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
	enrollmentOfferingCapacity := NewEnrollmentOfferingCapacity(enrollmentCareOfferings, enrollmentRecords, settingsService)
	// Consent rows back both the notification preferences below and the
	// opt-in "Neue Anmeldung" mail of the intake (#3780).
	notificationConsent, err := communicationCompose.NewNotificationConsent(communicationCompose.NotificationConsentConfig{
		DB:      db,
		Observe: observeCommunication,
	})
	if err != nil {
		return nil, err
	}
	enrollmentRequestService := NewEnrollmentIntake(EnrollmentIntakeSources{
		Requests:           enrollmentRecords,
		ParentChanges:      enrollmentRecords,
		Children:           enrollmentRecords,
		Bookings:           enrollmentCareBookingCommands{owner: carePlan},
		Guardians:          enrollmentRecords,
		LateInviteRepo:     enrollmentRecords,
		CareOfferingRepo:   enrollmentCareOfferings,
		Capacity:           enrollmentOfferingCapacity,
		Catalog:            enrollmentRecords,
		SchoolRepo:         enrollmentSchoolDirectory{schools: organizations},
		Notifications:      enrollmentNotifications,
		StudentRepo:        studentRepository,
		GuardianAuthorizer: studentGuardianRepository,
		RateLimitRepo:      enrollmentRecords,
		OutboxEnqueuer:     outboxEnqueuer{outbox: emailOutboxService},
		Settings:           settingsService,
		AdminSubscribers:   NewEnrollmentAdminSubscribers(notificationConsent, identityAccess),
		ManualDecider:      enrollmentDecisions,
		FrontendURL:        frontendURL, // admin notification email
		ParentsURL:         parentsURL,  // parent confirmation/status emails
		Logger:             logger.With("service", "enrollment-request"),
	})

	enrollmentReports := newEnrollmentReports(enrollmentReportSources{
		Owner:             enrollmentRecords,
		Offerings:         enrollmentCareOfferings,
		AccessLog:         audits.DataAccessLog,
		Students:          studentRepository,
		Persons:           personRepository,
		Groups:            NewGroupNames(groupRepository),
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
	classDayService := newClassDay(classDaySources{
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
	studentDocumentDeps, err := studentDocumentDependencies(carePlan, studentRepository, userContextService, audits.StudentFieldEdit, audits.DataAccessLog)
	if err != nil {
		return nil, fmt.Errorf("compose care plan documents: %w", err)
	}
	studentDocumentDeps.DB = db
	studentDocumentService, err := careplanCompose.NewStudentDocuments(studentDocumentDeps)
	if err != nil {
		return nil, fmt.Errorf("compose care plan documents: %w", err)
	}

	enrollmentChangeRequestService := NewEnrollmentChangeRequests(EnrollmentChangeRequestSources{
		Bookings:             enrollmentCareBookingCommands{owner: carePlan},
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
		GuardianInvitations:  enrollmentGuardianInvitationAvailability{invitations: guardianInvitationService},
		PersonRepo:           personRepository,
		StudentRepo:          studentRepository,
		GuardianAuthorizer:   studentGuardianRepository,
		Decisions:            enrollmentDecisions,
		BookingGates:         careBookings,
		CompanionGraphLocker: companionGraphCoordinator{CompanionLocks: companionService, strandings: studentRepository},
		Settings:             settingsService,
		OutboxEnqueuer:       outboxEnqueuer{outbox: emailOutboxService},
		FrontendURL:          frontendURL,
		ParentsURL:           parentsURL,
		Logger:               logger.With("service", "enrollment-change-request"),
	})

	// The rollover approves auto-renewed rows through the decision flow on
	// the rollover_auto_approve=true deadline path.
	enrollmentRolloverService := NewEnrollmentRollovers(EnrollmentRolloverSources{
		Bookings:         enrollmentCareBookingCommands{owner: carePlan},
		Phases:           enrollmentRecords,
		Requests:         enrollmentRecords,
		Children:         enrollmentRecords,
		Catalog:          enrollmentCareOfferingService,
		Notifications:    enrollmentNotifications,
		PhaseEligibility: enrollmentPhaseService,
		Outbox:           outboxEnqueuer{outbox: emailOutboxService},
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
		newStudentPresence(db, logger),
		pickupAutoExcusal,
		carePlan,
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

	// Post-enrollment offering changes (#1665, #3561): the parents portal
	// submits them, staff decide them on the same review page, and an
	// approval applies the switch through Care Plan's dated offering
	// adjustment. Permanent pickup-time changes switch to a matching offering
	// through the same review.
	offeringChanges, err := newOfferingChanges(offeringChangeInputs{
		CarePlan: carePlan, Enrollment: enrollmentRecords, Students: studentRepository,
		Withdrawals: NewCareWithdrawalCompletionRepository(carePlan), Settings: settingsService,
		Planning: manualPlanningReader{query: enrollmentCompose.NewManualPlanningQuery(db), courseGroups: timetableCapability},
		Bookings: careBookings, Reviews: requestReviewPolicy, Emitter: pillEmitter,
		Events: parentRequestEvents, Shares: requestShares, Today: today,
		Logger: logger.With("service", "offering-change-requests"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose care plan offering changes: %w", err)
	}
	pickupAdjustments, err := newPickupAdjustments(pickupAdjustmentInputs{
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
			durablePushAdapter{module: deliveryRuntime.Module}, logger.With("channel", "web_push"),
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
		workSessions,
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
	excusedRequestService, err := newExcusedAbsenceRequests(excusedRequestWiring{
		carePlan: carePlan, students: studentRepository, persons: personRepository,
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
		Outbox:           outboxEnqueuer{outbox: emailOutboxService},
		GuardianProfiles: guardianProfileRepository,
		Schools:          schoolNameDirectory{schools: organizations},
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
		DB:                     db,
		CalendarRenderer:       schoolCalendarRendererAdapter{renderer: calendar},
		Outbox:                 emailOutboxService,
		PushOutbox:             durablePushAdapter{module: deliveryRuntime.Module},
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
		Attendance:                newStudentPresence(db, logger),
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
		GuardianInvites:       NewParentGuardianAccess(guardianInvitationService),
		GuardianInvitations:   newGuardianInvitationReads(func() identityaccess.GuardianInvitations { return identityAccess }),
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
		CarePeriods:      parentCarePeriods{owner: enrollmentRecords},
		OfferingHistory:  parentOfferingHistory{owner: enrollmentRecords},
		CareOfferingRepo: enrollmentCareOfferings,
		OfferingChanges:  offeringChanges,
		Logger:           logger.With("service", "parent"),
		Now:              now,
	})

	parentAnnouncementService := communicationCompose.NewParentAnnouncements(communicationCompose.ParentAnnouncementConfig{
		Repo:             parentPortal.ParentAnnouncement,
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
			Events:           fileStorageEvents{repo: audits.FileEvent},
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
		DB:           db,
		Devices:      deviceFleet,
		Persons:      persons,
		Membership:   membership,
		PersonRepo:   personRepository,
		StaffRepo:    staffRepository,
		Accounts:     identityAccess,
		ActiveGroups: sessionRecords,
		Supervisors:  groupSupervisions,
		Categories:   arrivalTimetable.NewActivityCategorySeeder(timetableRepos.ActivityCategory),
		AuditLog:     operatorAuditLog,
	})
	if err != nil {
		return nil, fmt.Errorf("compose operator provisioning adapters: %w", err)
	}
	operatorProvisioningService, err := newOperatorProvisioning(operatorProvisioningSources{
		accounts:       NewOperatorAccountDirectory(identityAccess, persons, membership, organizations),
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

	listExportService := lists.NewRendererService()
	// The Notfallliste is the emergency snapshot read projection (#2704): the
	// owner facades are its tenant-safe reads, the retained student and
	// guardian repositories, the presence-mode read and the renderer its
	// compatibility bindings.
	emergencyService, err := emergencysnapshotlegacy.New(emergencysnapshotlegacy.Sources{
		Presence:     newStudentPresence(db, logger),
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
		Presence:        newStudentPresence(db, logger),
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
		Staff:          planExportStaffNames{staff: staffRepository},
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
		Clock:        reminderClock(),
		CurrentStaff: reminderStaffIdentity(userContextService),
		Settings:     reminderSettings{settingsService},
		Attendance:   newStudentPresence(db, logger),
		Pickup:       reminderPickupReader{source: pickupScheduleService},
		Instance:     reminderTimetableReader{source: timetableCapability},
		Room:         reminderRoomReader{source: rooms},
		Student:      reminderStudentReader{source: studentRepository},
		Person:       reminderPersonReader{source: personRepository},
		Supervision:  reminderSupervisionReader{source: activeService, presence: newStudentPresence(db, logger)},
		Visits:       reminderVisitReader{source: newStudentPresence(db, logger)},
		Logger:       logger.With("service", "reminders"),

		// Bulk readers for ComputeBatch. They answer the three genuinely
		// per-person facts for the whole tenant in one query each, which is what
		// keeps the per-minute cost flat in the number of staff.
		BulkSupervision:   reminderBulkSupervisionReader{source: groupSupervisions},
		BulkInstanceStaff: reminderTimetableReader{source: timetableCapability},
	})

	studentStatusDayService := presenceservice.NewStatusDays(
		statusDays,
		NewManualPartialAbsenceDates(carePlan),
		db,
		carePlan.LockExceptionDay,
		now,
	)
	studentStatusDayOverviewService := presenceservice.NewStatusDayOverviews(statusDays, StatusDayOverviewPeople(usersService))
	ogsGroupLiveService, err := grouplivelegacy.New(grouplivelegacy.Sources{
		Presence:          newStudentPresence(db, logger),
		People:            GroupRosterPeople(usersService),
		Education:         educationService,
		Substitutions:     substitutionService,
		UserContext:       groupLiveCaller{userContextService},
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

	timetableData, err := newTimetableData(timetableDataInputs{
		Rows:             timetableRows,
		ArrivalBaselines: arrivalBaselines,
		PickupBaselines:  pickupBaselines,
		Visits:           newStudentPresence(db, logger),
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
	timetableTemplateDeps, err := timetableTemplateDependencies(timetableTemplateInputs{
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
	attendanceCorrections, err := newTimetableAttendanceCorrections(timetableCorrectionInputs{
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
	masterDataDecisions, err := newMasterDataDecisions(masterDataDecisionWiring{
		carePlan: carePlan, fields: persons, students: studentRepository, persons: personRepository,
		audit: studentAuditService, scope: parentRequestWriteScope(requestReviewPolicy),
		emitter: pillEmitter, broadcaster: realtimeHub, events: parentRequestEvents, shares: requestShares,
		logger: logger.With("service", "master-data-review"), today: today,
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
	parentRequestCoordinator, err := newParentRequestCoordinator(parentRequestCoordinatorWiring{
		masterData: masterDataDecisions, excused: excusedRequestService, care: careRequestService,
		offering: offeringChangeConflictPort{changes: offeringChanges}, events: parentRequestEvents,
	})
	if err != nil {
		return nil, fmt.Errorf("compose parent request coordinator: %w", err)
	}

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
		TimetableCleanup:        timetableCleanupService,
		TimeTrackingCleanup:     timeTrackingCleanupService,
		StudentChangeLogCleanup: studentChangeLogCleanupService,
		Instance:                instanceService,
		AutoStart:               autoStartService,
		AutoEnd:                 autoEndService,
		TimetableOperations:     timetableOperationsService,
		Users:                   usersService,
		Birthdays:               birthdayService,
		StaffDocuments:          staffAdmin,
		StudentDocuments:        studentDocumentService,
		FileStore:               fileStoreService,
		CaregiverCapability:     caregiverCapabilityService,
		Guardian:                guardianService,
		PeopleDirectory:         persons,
		GuardianProfileLoader:   guardianProfileLoader,
		UserContext:             userContextService,
		Database:                databaseService,
		DatabaseStatsCapabilities: func(ctx context.Context) transactionruntime.DatabaseStatsCapabilities {
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
		MasterDataReview:     masterDataDecisions,
		CareRequests:         careRequestService,
		ExcusedRequests:      excusedRequestService,
		ParentRequests:       parentRequestCoordinator,
		RequestReviewPolicy:  requestReviewPolicy,
		StudentStatusDays:    studentStatusDayService,
		AbsenceOverview:      studentStatusDayOverviewService,
		StudentHistory:       presenceservice.NewStudentHistory(newStudentPresence(db, logger), historyRoomNames(rooms), NewDataAccessAudit(audits.DataAccessLog), NewHistorySlots(timetableRepos.InstanceStudent)),
		Statistics: newStatistics(db, logger, presenceCompose.StatisticsDependencies{
			StatusDays:  statisticsStatusDays{carePlan},
			Courses:     newCourseStatistics(mustPresenceReads(db)),
			Holidays:    tenantHolidays{calendar: calendar},
			ClosingDays: tenantClosingDays{calendar: calendar},
			Periods:     statisticsReportPeriods{calendar},
			Students:    statisticsReportStudents{studentRepository},
			Rooms:       statisticsReportRooms{rooms},
			AccessLog:   statisticsAuditLog{audits.DataAccessLog},
			Retention:   statisticsRetention{settingsService},
			Logger:      logger.With("service", "statistics"),
			Now:         now,
		}),
		OGSGroupLive:            ogsGroupLiveService,
		SupervisionDashboard:    supervisionDashboardService,
		TimetableData:           timetablePlanning,
		InstanceSeriesConverter: instanceSeriesConverter,
		OperatorMFA:             identityAccess,
		OperatorPasskey:         identityAccess,
		UnregisteredTagScans:    unregisteredTagScanService,

		EmailOutboxWorker: emailOutboxWorker,
		Delivery:          deliveryRuntime.Module,

		EnrollmentFormSchema:      enrollmentFormSchemaService,
		EnrollmentCareOffering:    carePlanCareOfferings{enrollmentCareOfferingService, careBookings, offeringChanges, pickupAdjustments},
		EnrollmentCaptcha:         enrollmentCaptchaService,
		EnrollmentRequest:         enrollmentRequestService,
		EnrollmentPhase:           enrollmentPhaseService,
		EnrollmentPhaseExpiry:     enrollmentPhaseExpiryService,
		EnrollmentDecision:        enrollmentDecisionService,
		EnrollmentReport:          enrollmentReports,
		ClassDayArrivalExceptions: classDayService,
		EnrollmentRollover:        enrollmentRolloverService,
		EnrollmentChangeRequest:   enrollmentChangeRequestService,
		EnrollmentDeletion:        enrollmentDeletionService,
		EnrollmentRejectedCleanup: EnrollmentRejectedCleanup{cleaner: enrollmentRejectedCleanupService},

		Parent:              parentService,
		Messaging:           messagingService,
		StaffMessaging:      staffMessagingService,
		Calendar:            calendarSvc,
		CalendarFeedCleanup: calendarSvc,
		ParentAnnouncement:  parentAnnouncementService,
		ParentEventEmitter:  pillEmitter,
	}

	factory.SettingsSideEffects = NewSettingsSideEffects()
	facilitiesLegacy.RegisterSettingsSideEffects(factory.SettingsSideEffects, schulhofService, wcService)
	registerCareWithdrawalSettingsSideEffects(factory.SettingsSideEffects, careLifecycleService)
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
	factory.bindGuardianDirectory(persons, db)
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
	Unlinker peopleCompose.PhotoUnlinker
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
	RegisterStudentPhotoSettingsSideEffects(f.SettingsSideEffects, f.StudentPhotos)
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
