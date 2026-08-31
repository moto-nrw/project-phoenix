// Package services provides service layer implementations
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/analytics"
	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/absence"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/moto-nrw/project-phoenix/services/announcement"
	auditService "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/auth"
	calendarService "github.com/moto-nrw/project-phoenix/services/calendar"
	"github.com/moto-nrw/project-phoenix/services/config"
	_ "github.com/moto-nrw/project-phoenix/services/config/defaults"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
	"github.com/moto-nrw/project-phoenix/services/database"
	"github.com/moto-nrw/project-phoenix/services/display"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/emergency"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/feedback"
	"github.com/moto-nrw/project-phoenix/services/filestore"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/services/iot"
	iotcheckin "github.com/moto-nrw/project-phoenix/services/iot/checkin"
	staffclock "github.com/moto-nrw/project-phoenix/services/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/services/mealplan"
	"github.com/moto-nrw/project-phoenix/services/messaging"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/services/ogsgrouplive"
	"github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/services/planexport"
	"github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/services/pwa"
	"github.com/moto-nrw/project-phoenix/services/reminders"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/slotlists"
	"github.com/moto-nrw/project-phoenix/services/staffmessaging"
	"github.com/moto-nrw/project-phoenix/services/statistics"
	"github.com/moto-nrw/project-phoenix/services/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type substitutionIdentity interface {
	GetCurrentStaff(context.Context) (*userModels.Staff, error)
	GetCurrentTeacher(context.Context) (*userModels.Teacher, error)
}

type substitutionActorResolver struct{ identity substitutionIdentity }

func (r substitutionActorResolver) ResolveActor(ctx context.Context) (*education.Actor, error) {
	staff, err := r.identity.GetCurrentStaff(ctx)
	if err != nil {
		if expectedMissingSubstitutionIdentity(err) {
			return nil, nil
		}
		return nil, err
	}
	teacher, err := r.identity.GetCurrentTeacher(ctx)
	if err != nil {
		if expectedMissingSubstitutionIdentity(err) {
			return nil, nil
		}
		return nil, err
	}
	return &education.Actor{StaffID: staff.ID, TeacherID: teacher.ID}, nil
}

func expectedMissingSubstitutionIdentity(err error) bool {
	return errors.Is(err, usercontext.ErrUserNotLinkedToPerson) ||
		errors.Is(err, usercontext.ErrUserNotLinkedToStaff) ||
		errors.Is(err, usercontext.ErrUserNotLinkedToTeacher)
}

// Factory provides access to all services
type Factory struct {
	settingsRuntimeDB        *bun.DB
	Auth                     auth.AuthService
	StaffPINAuth             auth.StaffPINAuthenticator
	MFA                      auth.MFAService
	Passkey                  auth.PasskeyService
	Active                   active.Service
	ActiveCleanup            active.CleanupService
	WorkSession              active.WorkSessionService
	WorkTimeMonth            active.WorkTimeMonthService
	Holidays                 schedule.HolidayService
	ClosingDays              schedule.ClosingDayService
	StaffAbsence             active.StaffAbsenceService
	StaffAbsenceType         active.StaffAbsenceTypeService
	StaffBalanceAdjust       active.StaffBalanceAdjustmentService
	StaffMonthClose          active.StaffMonthCloseService
	StaffOverview            active.StaffOverviewService
	TimeTrackingAuditLog     active.TimeTrackingAuditLogService
	StaffTimeExport          active.StaffTimeExportService
	Activities               activities.ActivityService
	Education                education.Service
	Substitution             education.SubstitutionModule
	GradeTransition          *education.GradeTransitionService
	Facilities               facilities.Service
	Schulhof                 facilities.SchulhofService
	WC                       facilities.WCService
	Invitation               auth.InvitationService
	GuardianInvitation       auth.GuardianInvitationService
	Feedback                 feedback.Service
	MealPlan                 mealplan.Service
	IoT                      iot.Service
	Checkin                  *iotcheckin.CheckinService
	StaffClock               *staffclock.Service
	Settings                 config.SettingsService
	TenantSettings           *config.TenantOperations
	PayrollStatus            config.PayrollStatusGetter
	Schedule                 schedule.Service
	StaffShifts              schedule.StaffShiftService
	StaffShiftSeries         schedule.StaffShiftSeriesService
	StaffAssignments         schedule.StaffAssignmentService
	StaffScheduleOverview    schedule.StaffScheduleOverviewGetter
	ShiftTypes               schedule.ShiftTypeService
	PlanningTracks           schedule.PlanningTrackService
	PickupSchedule           schedule.PickupScheduleService
	PartialAbsence           schedule.PartialAbsenceService
	ArrivalSchedule          schedule.ArrivalScheduleService
	CalendarPeriod           schedule.CalendarPeriodService
	CareDay                  schedule.CareDayService
	TimetableBridge          *schedule.TimetableBridgeService
	Materialization          schedule.MaterializationService
	TemplateSplit            *schedule.TemplateSplitService
	TimetableCleanup         schedule.TimetableCleanupService
	TimeTrackingCleanup      active.TimeTrackingCleanupService
	StudentChangeLogCleanup  users.StudentChangeLogCleanupService
	Instance                 schedule.InstanceService
	AutoStart                schedule.AutoStartService
	AutoEnd                  schedule.AutoEndService
	TimetableOperations      schedule.TimetableOperationsService
	Users                    users.PersonService
	Birthdays                users.BirthdayService
	StaffDocuments           users.StaffDocumentService
	StudentDocuments         users.StudentDocumentService
	FileStore                filestore.Service
	StaffOffboarding         users.StaffOffboardingService
	CaregiverCapability      users.CaregiverCapabilityService
	Guardian                 *users.GuardianService
	GuardianProfileLoader    *users.GuardianProfileLoader
	UserContext              usercontext.UserContextService
	Database                 database.DatabaseService
	Import                   *importService.ImportService[importModels.StudentImportRow]        // Student import service
	StaffImport              *importService.ImportService[importModels.StaffImportRow]          // Staff (Mitarbeiter) import service
	ClassListImport          *importService.ImportService[importModels.ClassListEntryImportRow] // Class-list entry import (#2382)
	OpeningBalanceImport     importService.OpeningBalanceImportFactory                          // Opening balance import (#2132), request-scoped
	ListExport               *listexport.RendererService
	Emergency                *emergency.Service
	SlotLists                slotlists.Service
	PlanExport               planexport.Service
	Reminders                reminders.Computer
	Notifications            notifications.Notifier
	PushSubscriptions        notifications.PushSubscriptionService
	PWAUsage                 pwa.UsageService
	NotificationPreferences  notifications.PreferenceService
	AbsenceNotifier          notifications.AbsenceNotifier
	RealtimeHub              *realtime.Hub     // SSE event hub (shared by services and API)
	Tracker                  analytics.Tracker // Product analytics (PostHog; no-op without POSTHOG_API_KEY)
	Mailer                   email.Mailer
	DefaultFrom              email.Email
	FrontendURL              string
	InvitationTokenExpiry    time.Duration
	PasswordResetTokenExpiry time.Duration

	// Platform domain (operator dashboard)
	OperatorAuth         platform.OperatorAuthService
	OperatorInvitation   platform.OperatorInvitationService
	OperatorProvisioning platform.OperatorProvisioningService
	Announcement         platform.AnnouncementService
	Schools              platform.SchoolService
	WorkTimeModels       *config.WorkTimeModelService
	Students             users.StudentService
	ClassListEntries     users.ClassListEntryService
	StudentDeletion      users.StudentDeletionService
	CareLifecycle        users.CareLifecycleService
	StudentAudit         users.StudentAuditService
	MasterDataReview     users.MasterDataReviewService
	CareRequests         schedule.CareScheduleRequestService
	// OfferingChanges is the post-enrollment offering change-request lifecycle
	// (#1665), shared by the parents portal and the staff review queue.
	OfferingChanges   enrollment.OfferingChangeRequestService
	PickupAdjustments enrollment.PickupAdjustmentService
	ExcusedRequests   absence.ExcusedAbsenceRequestService
	ParentRequests    *users.ParentRequestCoordinator
	FamilyProtection  *users.FamilyProtectionService
	// RequestReviewPolicy is the one cross-domain decision about WHO may see
	// and decide parent requests. The API layer reads it to explain an empty
	// queue; the four request services enforce it per child.
	RequestReviewPolicy *usercontext.ParentRequestReviewPolicy
	StudentStatusDays   *active.StudentStatusDayService
	AbsenceOverview     *active.StudentStatusDayOverviewService
	StudentHistory      active.StudentHistoryService
	// Statistics is the Statistik report (#2606).
	Statistics              statistics.Service
	OGSGroupLive            ogsgrouplive.Getter
	SupervisionDashboard    supervisiondashboard.Getter
	TimetableData           *schedule.TimetableDataService
	InstanceSeriesConverter schedule.InstanceSeriesConverter
	OperatorMFA             platform.OperatorMFAService
	OperatorPasskey         platform.OperatorPasskeyService
	UnregisteredTagScans    auditService.UnregisteredTagScanService

	// Email outbox (parent-enrollment PR 5) - shared across features.
	// EmailOutbox enqueues from feature code; EmailOutboxWorker drains
	// the table on a scheduler tick; EmailTemplateRegistry holds the
	// kind→Renderer mapping populated at startup.
	EmailOutbox           *platform.OutboxService
	EmailOutboxWorker     *platform.OutboxWorker
	EmailTemplateRegistry *platform.TemplateRegistry

	// Display domain (info-point dashboards, issue #1325)
	Display display.Service

	// Enrollment domain (parent-enrollment PR 5+).
	EnrollmentFormSchema      enrollment.FormSchemaService
	EnrollmentCareOffering    enrollment.CareOfferingService
	EnrollmentCaptcha         *enrollment.CaptchaService
	EnrollmentRequest         enrollment.RequestService
	EnrollmentPhase           enrollment.PhaseService
	EnrollmentPhaseExpiry     enrollment.PhaseExpiryService
	EnrollmentDecision        enrollment.DecisionService
	EnrollmentReport          enrollment.ReportService
	EnrollmentRollover        enrollment.RolloverService
	EnrollmentChangeRequest   enrollment.ChangeRequestService
	EnrollmentDeletion        enrollment.EnrollmentDeletionService
	EnrollmentRejectedCleanup enrollment.RejectedEnrollmentCleaner

	// Parent (cross-tenant guardian portal - PR 9)
	Parent parent.Service

	// Messaging (staff-side parent-OGS inbox / threads)
	Messaging *messaging.Service

	// StaffMessaging (OGS-internal colleague chat, #2598)
	StaffMessaging *staffmessaging.Service
	// StaffNotice (Tagesinformationen: interne Hinweise der Leitung, #2180)
	StaffNotice schedule.StaffNoticeService

	// ParentEventEmitter is the chat-pill + guardian-wake emitter (#1803/#1845).
	// Exposed so the API layer can wake a child's guardians (its message-
	// independent parent_child_updated SSE fan-out) after staff-side care writes,
	// so an open parents-app tab refetches the child's care state live (#1725).
	ParentEventEmitter *parentmessaging.Emitter

	// Calendar (staff and parent personal calendars)
	Calendar            calendarService.Service
	CalendarFeedCleanup calendarService.FeedCleanupService

	// ParentAnnouncement (staff-side parent broadcast news authoring, #1669)
	ParentAnnouncement announcement.Service

	// SettingsSideEffects is the per-key handler registry the API binds to
	// SettingsResource.OnValueSet. Domain packages register handlers here
	// (facilities at startup, students via EnableStudentPhotos). API never
	// owns the registry - its only job is to dispatch.
	SettingsSideEffects *sideeffects.Registry
	// StudentPhotos is set by EnableStudentPhotos. nil until the API layer
	// supplies a PhotoUnlinker (file IO is an api-layer concern, not a
	// service-layer one).
	StudentPhotos users.StudentPhotoService
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

// NewFactory creates a new services factory; tests may pass one application clock.
func NewFactory(repos *repositories.Factory, db *bun.DB, logger *slog.Logger, clocks ...func() time.Time) (*Factory, error) {
	now := optionalClock(clocks)
	today := timezone.CalendarDateClock(now)
	settingsRuntime := newSettingsRuntime(db, nil)
	repos.SetConfigRuntime(settingsRuntime)

	mailer, err := email.NewMailer()
	if err != nil {
		logger.Warn("SMTP mailer initialization failed, falling back to mock mailer", "error", err)
		mailer = email.NewMockMailer()
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

	dispatcher := email.NewDispatcher(mailer, emailLogger)

	defaultFrom := email.NewEmail(viper.GetString("email_from_name"), viper.GetString("email_from_address"))
	if defaultFrom.Address == "" {
		defaultFrom = email.NewEmail("moto", "no-reply@moto.local")
	}

	rawFrontendURL := viper.GetString("frontend_url")
	frontendURL := strings.TrimRight(rawFrontendURL, "/")
	if frontendURL == "" {
		return nil, fmt.Errorf("FRONTEND_URL is required")
	}

	appEnv := strings.ToLower(viper.GetString("app_env"))
	if appEnv == "production" && !strings.HasPrefix(frontendURL, "https://") {
		return nil, fmt.Errorf("FRONTEND_URL must use https:// in production (received %q)", rawFrontendURL)
	}

	// Parents-portal URL - used for every parent-facing email link
	// (status, decision emails, guardian invitation accept).
	rawParentsURL := viper.GetString("parents_url")
	parentsURL := strings.TrimRight(rawParentsURL, "/")
	if parentsURL == "" {
		return nil, fmt.Errorf("PARENTS_URL is required")
	}
	if appEnv == "production" && !strings.HasPrefix(parentsURL, "https://") {
		return nil, fmt.Errorf("PARENTS_URL must use https:// in production (received %q)", rawParentsURL)
	}

	// School-portal URL (#2207) - used for Lehrkraft invitation links, which
	// must land on the school portal where the accept flow lives.
	rawSchoolURL := viper.GetString("school_url")
	schoolURL := strings.TrimRight(rawSchoolURL, "/")
	if schoolURL == "" {
		return nil, fmt.Errorf("SCHOOL_URL is required")
	}
	if appEnv == "production" && !strings.HasPrefix(schoolURL, "https://") {
		return nil, fmt.Errorf("SCHOOL_URL must use https:// in production (received %q)", rawSchoolURL)
	}

	invitationExpiryHours := viper.GetInt("invitation_token_expiry_hours")
	if invitationExpiryHours <= 0 {
		invitationExpiryHours = 48
	} else if invitationExpiryHours > 168 {
		invitationExpiryHours = 168
	}
	invitationTokenExpiry := time.Duration(invitationExpiryHours) * time.Hour

	passwordResetExpiryMinutes := viper.GetInt("password_reset_token_expiry_minutes")
	if passwordResetExpiryMinutes <= 0 {
		passwordResetExpiryMinutes = 30
	} else if passwordResetExpiryMinutes > 1440 {
		passwordResetExpiryMinutes = 1440
	}
	passwordResetTokenExpiry := time.Duration(passwordResetExpiryMinutes) * time.Minute

	// Create realtime hub for SSE broadcasting (single shared instance)
	realtimeHub := realtime.NewHub(logger.With("component", "sse-hub"))

	// Product analytics (PostHog) — no-op when POSTHOG_API_KEY is unset
	tracker, err := analytics.New(
		viper.GetString("posthog_api_key"),
		viper.GetString("posthog_host"),
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
	rosterReconciler := schedule.NewRosterReconciler(
		repos.ActivityInstance,
		repos.InstanceStudent,
		repos.StudentEnrollment,
		logger,
		now,
	)

	// Initialize grade transition service
	gradeTransitionService := education.NewGradeTransitionService(education.GradeTransitionServiceDependencies{
		TransitionRepo:      repos.GradeTransition,
		StudentRepo:         repos.Student,
		PersonRepo:          repos.Person,
		VisitRepo:           repos.ActiveVisit,
		AttendanceRepo:      repos.Attendance,
		ClassTeacherRepo:    repos.ClassTeacher,
		StaffRepo:           repos.Staff,
		ClassListEntryRepo:  repos.ClassListEntry,
		ClassListEntryAudit: repos.ClassListEntryChange,
		RosterReconciler:    rosterReconciler,
		DB:                  db,
		Today:               today,
	})

	// Initialize settings service (new schema-driven settings system)
	settingsService := config.NewSettingsService(
		repos.SettingValue,
		repos.SettingAudit,
		newSchoolSettingsStore(repos.School),
		settingsRuntime,
		logger,
	)
	// Wire the enrollment class-restriction probe so the settings service can
	// refuse disabling concrete-class collection while an active phase
	// restricts eligibility to specific classes (#1663). Runs inside the
	// caller's tenant tx, so the phase lookup is RLS-scoped.
	if guarded, ok := settingsService.(interface {
		SetClassRestrictionGuard(func(context.Context) (bool, error))
	}); ok {
		guarded.SetClassRestrictionGuard(func(ctx context.Context) (bool, error) {
			return repos.Phase.ExistsActiveWithEligibleClasses(ctx)
		})
	}
	// Same for the grade-level restriction, which survives concrete-class
	// collection being off but not grade-level collection (#1663).
	if guarded, ok := settingsService.(interface {
		SetGradeRestrictionGuard(func(context.Context) (bool, error))
	}); ok {
		guarded.SetGradeRestrictionGuard(func(ctx context.Context) (bool, error) {
			return repos.Phase.ExistsActiveWithEligibleGradeLevels(ctx)
		})
	}
	// And the cap probe, so enrollment.grade_level_max cannot be lowered below
	// a grade an active phase already restricts itself to — which would leave
	// that phase with no selectable eligible grade at all (#1663).
	if guarded, ok := settingsService.(interface {
		SetGradeCapGuard(func(context.Context) (int, error))
	}); ok {
		guarded.SetGradeCapGuard(func(ctx context.Context) (int, error) {
			return repos.Phase.MaxActiveEligibleGradeLevel(ctx)
		})
	}

	// Initialize users service first (needed for active service)
	usersService := users.NewPersonService(users.PersonServiceDependencies{
		PersonRepo:           repos.Person,
		RFIDRepo:             repos.RFIDCard,
		AccountRepo:          repos.Account,
		StudentRepo:          repos.Student,
		StaffRepo:            repos.Staff,
		TeacherRepo:          repos.Teacher,
		RoleRepo:             repos.Role,
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
	tenantMailIdentity := platform.NewTenantMailIdentityService(repos.School, func(ctx context.Context, tenantID int64) (string, error) {
		return settingsService.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, logger)

	guardianService := users.NewGuardianService(users.GuardianServiceDependencies{
		GuardianProfileRepo:     repos.GuardianProfile,
		GuardianPhoneNumberRepo: repos.GuardianPhoneNumber,
		StudentGuardianRepo:     repos.StudentGuardian,
		GuardianInvitationRepo:  repos.GuardianInvitation,
		AccountRepo:             repos.Account,
		AccountParentRepo:       repos.AccountParent,
		AccountTenantRepo:       repos.AccountTenant,
		AccountRoleRepo:         repos.AccountRole,
		RoleRepo:                repos.Role,
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

	// Initialize work session service (before active service - needed for NFC auto-check-in)
	workSessionService := active.NewWorkSessionService(repos.WorkSession, repos.WorkSessionBreak, repos.WorkSessionEdit, repos.StaffAbsence, repos.GroupSupervisor, repos.ActiveGroup, repos.Staff, repos.StaffWorkSchedule, repos.WorkTimeModel, settingsService, activeLogger, db)
	// Planned-shift lookups for the auto-checkout job (#1798).
	workSessionService.SetStaffShiftRepo(repos.StaffShift)
	if broadcastAware, ok := workSessionService.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}
	staffClockService := staffclock.NewService(usersService, repos.RFIDCard, workSessionService)

	// Monatskarte read model (#1842) — everything computed on read, the
	// Übertrag is live.
	workTimeMonthService := active.NewWorkTimeMonthService(
		repos.WorkSession,
		repos.WorkSessionBreak,
		repos.StaffAbsence,
		repos.Staff,
		repos.StaffWorkSchedule,
		repos.WorkTimeModel,
		repos.StaffShift,
		settingsService,
		activeLogger,
	)

	// Public holidays per Bundesland (#1418 3a): computed from the
	// operations.federal_state setting, zero the Soll of their day.
	holidayService := schedule.NewHolidayService(settingsService, logger.With("service", "holidays"))
	// Tenant closing days (#1418 3b) share the Soll=0 semantics of public
	// holidays. The Soll consumers get the UNION of both via the composite
	// reader; Factory.Holidays stays the plain holiday service so the
	// /holidays endpoint keeps reporting only real public holidays.
	closingDayService := schedule.NewClosingDayService(repos.ClosingDay)
	nonWorkingDayService := schedule.NewNonWorkingDayResolver(holidayService, closingDayService)
	workTimeMonthService.SetHolidayReader(nonWorkingDayService)
	// Stundenkonto transactions (#1420) enter the carry chain by effective date.
	workTimeMonthService.SetAdjustmentReader(repos.StaffBalanceAdjust)
	// Frozen months (#1417) short-circuit the carry chain so a retroactive
	// correction can no longer rewrite a closed month's Übertrag.
	workTimeMonthService.SetSnapshotReader(repos.StaffMonthSnapshot)
	// The session service's weekly summaries reduce their Soll by holidays
	// too. The setter is not part of the WorkSessionService interface (it
	// would break external mocks), hence the assertion.
	if holidayAware, ok := workSessionService.(interface {
		SetHolidayReader(active.HolidayDatesReader)
	}); ok {
		holidayAware.SetHolidayReader(nonWorkingDayService)
	}

	// School-defined Abwesenheitsarten (#2403). Constructed before the absence
	// service so it can be injected there and resolve custom names on both the
	// write path (which base type an art inherits) and the read paths.
	staffAbsenceTypeService := active.NewStaffAbsenceTypeService(repos.StaffAbsenceType, activeLogger)
	if typeAware, ok := workSessionService.(interface {
		SetAbsenceTypeService(active.StaffAbsenceTypeService)
	}); ok {
		typeAware.SetAbsenceTypeService(staffAbsenceTypeService)
	}

	// Initialize staff absence service
	staffAbsenceService := active.NewStaffAbsenceService(repos.StaffAbsence, repos.WorkSession, repos.StaffVacationQuota, repos.StaffAbsenceAudit, settingsService, workTimeMonthService)
	if typeAware, ok := staffAbsenceService.(interface {
		SetAbsenceTypeService(active.StaffAbsenceTypeService)
	}); ok {
		typeAware.SetAbsenceTypeService(staffAbsenceTypeService)
	}
	if broadcastAware, ok := staffAbsenceService.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}

	// Stundenkonto lifecycle transactions (#1420): payout, comp-time grants,
	// school-year reset. Reads the live balance through the month service.
	staffBalanceAdjustService := active.NewStaffBalanceAdjustmentService(repos.StaffBalanceAdjust, workTimeMonthService, settingsService, activeLogger)
	if broadcastAware, ok := staffBalanceAdjustService.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}
	// A booking inside a closed month (#1417) could not move its frozen
	// closing balance, so the ledger rejects it.
	staffBalanceAdjustService.SetSnapshotReader(repos.StaffMonthSnapshot)
	// Deletes leave an append-only tombstone in the cross-staff audit log
	// (#1417). Both services fail their delete paths without this wiring.
	if deletionAware, ok := staffBalanceAdjustService.(interface {
		SetDeletionAudit(auditModels.TimeTrackingDeletionRepository)
	}); ok {
		deletionAware.SetDeletionAudit(repos.TimeTrackingDeletion)
	}
	if deletionAware, ok := staffAbsenceService.(interface {
		SetDeletionAudit(auditModels.TimeTrackingDeletionRepository)
	}); ok {
		deletionAware.SetDeletionAudit(repos.TimeTrackingDeletion)
	}
	// Vacation takeover at the moto introduction (#2132): the summary
	// subtracts pre-introduction days, the write paths book/delete them.
	if openingAware, ok := staffAbsenceService.(interface {
		SetVacationOpeningRepository(activeModels.StaffVacationOpeningRepository)
	}); ok {
		openingAware.SetVacationOpeningRepository(repos.StaffVacationOpening)
	}

	// Monatsabschluss (#1417): freezes a month's closing balance so a
	// retroactive correction can no longer rewrite every later Übertrag.
	staffMonthCloseService := active.NewStaffMonthCloseService(
		repos.StaffMonthSnapshot,
		workTimeMonthService,
		repos.Staff,
		settingsService,
		activeLogger,
	)
	if broadcastAware, ok := staffMonthCloseService.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}

	// Tenant-wide time-tracking views (#1417 2a). Prefetches all inputs once
	// and runs the SAME per-staff month math over in-memory readers, so the
	// list can never drift from the /staff/{id} detail view.
	staffOverviewService := active.NewStaffOverviewService(
		repos.Staff,
		repos.WorkSession,
		repos.WorkSessionBreak,
		repos.StaffAbsence,
		repos.StaffBalanceAdjust,
		repos.StaffVacationQuota,
		repos.StaffMonthSnapshot,
		repos.StaffWorkSchedule,
		repos.WorkTimeModel,
		repos.StaffShift,
		settingsService,
		activeLogger,
	)
	staffOverviewService.SetHolidayReader(nonWorkingDayService)
	// Vacation takeover (#2132): the Resturlaub column subtracts
	// pre-introduction days exactly like the /staff/{id} detail view.
	staffOverviewService.SetVacationOpeningReader(repos.StaffVacationOpening)

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

	staffTimeExportService := active.NewStaffTimeExportService(
		staffOverviewService,
		workSessionService,
		repos.Staff,
		repos.DataAccessLog,
		payrollStatusService,
		activeLogger,
	)

	// Cross-staff audit feed (#1417): merges the four change trails into one
	// keyset-paginated view. Read-only; permission gating at the route.
	timeTrackingAuditLogService := active.NewTimeTrackingAuditLogService(
		repos.TimeTrackingAuditLog,
		repos.Staff,
		settingsService,
	)

	// Absence email notifications (#1419 4d). Setter injection keeps the
	// constructor stable and unit tests email-free (mirrors SetShiftPlanSyncer);
	// the interface stays untouched via the assertion (like SetHolidayReader).
	if emailAware, ok := staffAbsenceService.(interface {
		SetAbsenceEmailDeps(active.AbsenceEmailDeps)
	}); ok {
		emailAware.SetAbsenceEmailDeps(active.AbsenceEmailDeps{
			Settings:     settingsService,
			Dispatcher:   dispatcher,
			StaffRepo:    repos.Staff,
			SchoolRepo:   repos.School,
			DefaultFrom:  defaultFrom,
			FrontendURL:  frontendURL,
			MailIdentity: tenantMailIdentity,
			Logger:       activeLogger,
		})
	}

	// Initialize attendance sync service (WP-B10). Implements
	// active.AttendanceSyncer - called from CreateVisit / EndVisit to mirror
	// into schedule.instance_students and enrich SSE events. No circular
	// dependency because it only depends on repos, not on active.Service.
	attendanceSyncService := schedule.NewAttendanceSyncService(
		repos.ActivityInstance,
		repos.InstanceStudent,
		logger.With("service", "attendance-sync"),
	)
	pickupBaselines := schedule.NewPickupBaselineServiceWithSettings(
		repos.StudentPickupSchedule,
		repos.RequestChildOffering,
		repos.CareOffering,
		settingsService,
	)
	// The arrival mirror: the class timetable supplies the regular time and,
	// with enrollment.bookings_authoritative on, the approved bookings supply
	// the care days (#2414, ADR 0005). The care-day resolver reads through it
	// so a stale row on an unbooked weekday stops marking a child expected.
	arrivalBaselines := schedule.NewArrivalBaselineService(
		repos.StudentArrivalSchedule,
		repos.Student,
		repos.ClassArrivalTime,
		repos.RequestChildOffering,
		repos.CareOffering,
		settingsService,
	)

	// Care-day derivation (#1747): intersects timetable assignments with the
	// children's care plans. Read-only, so it can be shared by every consumer
	// (instance lifecycle, operations roster, weekly planner, scheduler).
	// Built here, ahead of the active service, because the timetable bridge
	// below needs it and the active service needs the bridge.
	careDayService := schedule.NewCareDayService(schedule.CareDayDependencies{
		ArrivalBaselines:  arrivalBaselines,
		ArrivalSchedules:  repos.StudentArrivalSchedule,
		ArrivalExceptions: repos.StudentArrivalException,
		PickupBaselines:   pickupBaselines,
		PickupExceptions:  repos.StudentPickupException,
	})

	// Single entry point for completing instances whose active.group somebody
	// else ended (force-start here, the nightly session-end job in the
	// scheduler). Finalizes attendance first, so a completed instance never
	// keeps genuinely expected rows — readers take those as the "not booked
	// into care that day" marker (#1747). Repos only, so no cycle with the
	// active service it is injected into.
	timetableBridgeService := schedule.NewTimetableBridgeService(schedule.TimetableBridgeDependencies{
		Instances:        repos.ActivityInstance,
		InstanceStudents: repos.InstanceStudent,
		CareDays:         careDayService,
	})

	// Initialize active service with SSE broadcaster
	activeService := active.NewService(active.ServiceDependencies{
		GroupRepo:                repos.ActiveGroup,
		VisitRepo:                repos.ActiveVisit,
		SupervisorRepo:           repos.GroupSupervisor,
		CombinedGroupRepo:        repos.CombinedGroup,
		GroupMappingRepo:         repos.GroupMapping,
		AttendanceRepo:           repos.Attendance,
		StudentStatusRepo:        repos.StudentStatusDay,
		CrossTenantRepo:          activeRepo.NewCrossTenantRepository(db),
		StudentRepo:              repos.Student,
		PersonRepo:               repos.Person,
		TeacherRepo:              repos.Teacher,
		StaffRepo:                repos.Staff,
		RoomRepo:                 repos.Room,
		ActivityGroupRepo:        repos.ActivityGroup,
		ActivityCatRepo:          repos.ActivityCategory,
		EducationGroupRepo:       repos.Group,
		DeviceRepo:               repos.Device,
		EducationService:         educationService,
		UsersService:             usersService,
		DB:                       db,
		Broadcaster:              realtimeHub,           // Pass SSE broadcaster
		Tracker:                  tracker,               // Product analytics (PostHog)
		WorkSessionService:       workSessionService,    // NFC auto-check-in
		AttendanceSyncer:         attendanceSyncService, // WP-B10 mirror + SSE enrichment
		TimetableBridgeCompleter: timetableBridgeService,
		Logger:                   activeLogger,
		Now:                      now,
	})

	// Initialize feedback service
	feedbackService := feedback.NewService(
		repos.FeedbackEntry,
	)

	// Initialize meal plan service
	mealPlanService := mealplan.NewService(repos.MealPlanEntry)

	// Initialize IoT service
	iotService := iot.NewService(
		repos.Device,
	)

	// Inject settings resolver into active service so auto-clear of sick /
	// excused flags respects the tenant's operations.sick_clear_mode and
	// operations.excused_clear_mode settings.
	activeService.SetSettingsService(settingsService)

	// Inject settings resolver into the IoT service so the device-online window
	// (iot.device_online_window_minutes) is resolved per tenant (issue #586).
	iotService.SetSettingsService(settingsService)

	// Initialize activities service
	activitiesService, err := activities.NewService(
		repos.ActivityCategory,
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
		Repo:                     repos.CareOffering,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		ActivityGroupRepo:        repos.ActivityGroup,
		ActivityScheduleRepo:     repos.ActivitySchedule,
		CalendarPeriodRepo:       repos.CalendarPeriod,
		TimeframeRepo:            repos.Timeframe,
		ActivityExceptionRepo:    repos.ActivityException,
		PhaseRepo:                repos.Phase,
		Settings:                 settingsService,
		Today:                    today,
		LockTemplateRecurrence: func(ctx context.Context) error {
			return schedule.LockTenantRecurrenceWrites(ctx, db)
		},
		Logger: logger.With("service", "enrollment-care-offering"),
	})
	careOfferingSeriesValidator, ok := enrollmentCareOfferingService.(enrollment.CareOfferingSeriesValidator)
	if !ok {
		return nil, fmt.Errorf("enrollment care offering service does not implement series validation")
	}
	careOfferingCalendarPeriodValidator, ok := enrollmentCareOfferingService.(enrollment.CareOfferingCalendarPeriodValidator)
	if !ok {
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
		RoomRepo:        repos.Room,
		ActiveGroupRepo: repos.ActiveGroup,
		LockTemplateRecurrence: func(ctx context.Context) error {
			return schedule.LockTenantRecurrenceWrites(ctx, db)
		},
		ValidateCareOfferingRoomDeletion: careOfferingResourceValidator.ValidateRoomDeletion,
	})

	// Initialize Schulhof service (depends on facilities, activities, and active services)
	schulhofService := facilities.NewSchulhofService(
		facilitiesService,
		activitiesService,
		activeService,
		facilitiesLogger,
	)

	// Initialize WC service (depends on facilities and activities services)
	wcService := facilities.NewWCService(
		facilitiesService,
		activitiesService,
		facilitiesLogger,
	)

	// Initialize schedule service
	scheduleService := schedule.NewServiceWithConfig(schedule.ServiceConfig{
		DateframeRepo:      repos.Dateframe,
		TimeframeRepo:      repos.Timeframe,
		RecurrenceRuleRepo: repos.RecurrenceRule,
		LockTemplateRecurrence: func(ctx context.Context) error {
			return schedule.LockTenantRecurrenceWrites(ctx, db)
		},
		ValidateCareOfferingTimeframeChange: careOfferingResourceValidator.ValidateTimeframeChange,
	})

	// Initialize shift type service (Schichtarten, #1836)
	shiftTypeService := schedule.NewShiftTypeService(
		repos.ShiftType,
		logger.With("service", "shift_type"),
	)
	planningTrackService := schedule.NewPlanningTrackService(repos.PlanningTrack, db)

	// Initialize staff shift service (Dienstplan, #1376 core slice)
	staffShiftService := schedule.NewStaffShiftService(
		repos.StaffShift,
		repos.Staff,
		shiftTypeService,
		db,
		logger.With("service", "staff_shift"),
	)
	staffShiftService.SetSeriesExceptionRepo(repos.StaffShiftSeriesException)
	// #1884: shift moves append a shift_moved Änderungsprotokoll entry.
	staffShiftService.SetDeviationEventRepo(repos.DeviationEvent)
	if broadcastAware, ok := staffShiftService.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}

	// Recurring shift series (Dienstplan-Serien, #1889)
	staffShiftSeriesService := schedule.NewStaffShiftSeriesService(
		repos.StaffShiftSeries,
		repos.StaffShiftSeriesException,
		repos.StaffShift,
		repos.Staff,
		repos.CalendarPeriod,
		shiftTypeService,
		db,
		logger.With("service", "staff_shift_series"),
		staffShiftService,
		today,
	)
	if broadcastAware, ok := staffShiftSeriesService.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}
	// Self-service Betreuungsplan assignments for a staff member ("Mein Tag",
	// #1844) — the "Ort/Aufgabe" the Dienstplan shift alone cannot express.
	staffAssignmentService := schedule.NewStaffAssignmentService(schedule.StaffAssignmentDependencies{
		InstanceStaffRepo:    repos.InstanceStaff,
		ActivityInstanceRepo: repos.ActivityInstance,
		RoomRepo:             repos.Room,
		ActivityGroupRepo:    repos.ActivityGroup,
	}, logger.With("service", "staff_assignment"))

	staffScheduleOverviewService := schedule.NewStaffScheduleOverviewService(schedule.StaffScheduleOverviewDependencies{
		Shifts:        repos.StaffShift,
		ShiftWeeks:    repos.StaffShift,
		Instances:     repos.ActivityInstance,
		InstanceStaff: repos.InstanceStaff,
		Rooms:         repos.Room,
		Staff:         repos.Staff,
		WorkSchedules: repos.StaffWorkSchedule,
		WorkModels:    repos.WorkTimeModel,
		Holidays:      nonWorkingDayService,
	})

	// Couples pulled-forward day pickup times with the per-block partial
	// absences (#2360). Shared by the staff pickup-exception writers and the
	// parent care-exception writers so both derive the same state.
	pickupAutoExcusal := schedule.NewPickupAutoExcusalSyncer(
		repos.StudentPickupException,
		pickupBaselines,
		repos.InstanceStudent,
		db,
	)

	// Initialize pickup schedule service
	pickupScheduleService := schedule.NewPickupScheduleServiceWithBulk(
		repos.StudentPickupSchedule,
		repos.StudentPickupException,
		repos.StudentPickupNote,
		repos.Student,
		repos.Person,
		pickupAutoExcusal,
		pickupBaselines,
		db,
		logger.With("service", "pickup-schedule"),
	)
	partialAbsenceService := schedule.NewPartialAbsenceService(
		repos.StudentPickupException,
		repos.StudentStatusDay,
		repos.ExcusedAbsenceRequest,
		repos.InstanceStudent,
		pickupAutoExcusal,
		db,
	)

	// Initialize RFID check-in service (issue #575 B8). Orchestrates the
	// active/users/facilities/activities services plus the daily-checkout gate
	// policy (settings + pickup schedule + education group) for the /api/iot
	// check-in workflow. Lives in the services/iot/checkin sub-package to avoid
	// the services/iot ↔ services/active ↔ auth/device import cycle.
	checkinService := iotcheckin.NewCheckinService(iotcheckin.CheckinServiceDeps{
		Active:     activeService,
		Users:      usersService,
		Facilities: facilitiesService,
		Activities: activitiesService,
		Settings:   settingsService,
		Pickup:     pickupScheduleService,
		Education:  educationService,
		Logger:     logger.With("service", "checkin"),
	})

	// Initialize display service (info-point dashboards, issue #1325).
	// Aggregates existing data sources; owns no queries beyond its own repo.
	displayService := display.NewService(display.Dependencies{
		DisplayRepo:       repos.Display,
		SchoolRepo:        repos.School,
		Facilities:        facilitiesService,
		ActiveGroupRepo:   repos.ActiveGroup,
		VisitRepo:         repos.ActiveVisit,
		ActivityGroupRepo: repos.ActivityGroup,
		InstanceRepo:      repos.ActivityInstance,
		AttendanceRepo:    repos.Attendance,
		PickupSchedule:    pickupScheduleService,
		SettingsService:   settingsService,
		DB:                db,
		Logger:            logger.With("service", "display"),
	})

	// Period updates/deletes use the same tenant recurrence transaction and
	// advisory lock as template and care-offering mutations. The preflight
	// callback sees the proposed post-update row (or nil for delete) before any
	// FK can be cleared by ON DELETE SET NULL.
	calendarPeriodService := schedule.NewCalendarPeriodServiceWithConfig(schedule.CalendarPeriodServiceConfig{
		Repo:                       repos.CalendarPeriod,
		DB:                         db,
		ValidateCareOfferingChange: careOfferingCalendarPeriodValidator.ValidateCalendarPeriodChange,
		Logger:                     logger.With("service", "calendar-period"),
	})

	// Initialize materialization service (WP-B8). Turns activity templates into
	// concrete schedule.activity_instances + instance_staff/instance_students
	// for a date window. Consumed by the scheduler task (gated on the
	// timetable.materialization_enabled setting) and the manual admin endpoint.
	materializationService := schedule.NewMaterializationService(
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
		calendarPeriodService,
		db,
		realtimeHub,
		logger.With("service", "materialization"),
	)
	// Per-date care filter (#2487): a child stays on the rosters the
	// materializer builds up to and including their last care day, and drops
	// out of every day after it.
	schedule.WireMaterializationCareBounds(materializationService, repos.Student)

	// Initialize instance lifecycle before template split: the split reuses its
	// deviation snapshot/reapply machinery when replacing future occurrences.
	recoveryRepo := scheduleRepo.NewActivityRecoveryRepository(db)
	instanceService := schedule.NewInstanceService(schedule.InstanceServiceDependencies{
		CareDayService:     careDayService,
		InstanceRepo:       repos.ActivityInstance,
		IdempotencyRepo:    repos.InstanceIdempotency,
		InstanceStaffRepo:  repos.InstanceStaff,
		InstanceStudents:   repos.InstanceStudent,
		ExceptionRepo:      repos.ActivityException,
		ActiveGroupRepo:    repos.ActiveGroup,
		SupervisorRepo:     repos.GroupSupervisor,
		VisitRepo:          repos.ActiveVisit,
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
		// E2E fixtures start future weekday instances. Dedicated unit tests
		// construct the service with EnforceTimePolicy: true.
		EnforceTimePolicy: os.Getenv("APP_ENV") != "test",
	})

	// Initialize template split service (WP-B3). "Dieser und alle folgenden":
	// caps the old template's schedules + rosters at an effective date,
	// creates a successor template and re-plans the affected window via the
	// materialization service.
	templateSplitService := schedule.NewTemplateSplitService(schedule.TemplateSplitDependencies{
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
	})

	// Initialize timetable GDPR cleanup service (WP-B14). Deletes
	// schedule.activity_instances (CASCADE → instance_staff + instance_students)
	// and schedule.activity_exceptions older than the tenant's retention window.
	// Per-student audit rows via DataDeletion; exceptions slog-only.
	timetableCleanupService := schedule.NewTimetableCleanupService(
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
	timeTrackingCleanupService := active.NewTimeTrackingCleanupService(
		repos.WorkSession,
		repos.StaffAbsence,
		repos.DataDeletion,
		settingsService,
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

	autoStartService := schedule.NewAutoStartService(schedule.AutoStartDependencies{
		InstanceRepo:      repos.ActivityInstance,
		InstanceStaffRepo: repos.InstanceStaff,
		InstanceStudents:  repos.InstanceStudent,
		InstanceService:   instanceService,
		RoomRepo:          repos.Room,
		ActiveGroupRepo:   repos.ActiveGroup,
		SupervisorRepo:    repos.GroupSupervisor,
		VisitRepo:         repos.ActiveVisit,
		Logger:            logger.With("service", "timetable-auto-start"),
	})
	autoEndService := schedule.NewAutoEndService(repos.ActivityInstance, instanceService)

	arrivalScheduleService := schedule.NewArrivalScheduleServiceWithBaselines(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
		arrivalBaselines,
		repos.ClassArrivalTime,
		db,
		logger.With("service", "arrival-schedule"),
	)

	timetableOperationsService := schedule.NewTimetableOperationsService(schedule.TimetableOperationsDependencies{
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
		VisitRepo:          repos.ActiveVisit,
		StudentRepo:        repos.Student,
		EducationGroupRepo: repos.Group,
		RoomRepo:           repos.Room,
		PersonService:      usersService,
		PlanningTrackRepo:  repos.PlanningTrack,
		Settings:           settingsService,
		Broadcaster:        realtimeHub,
		DB:                 db,
		Logger:             logger.With("service", "timetable-operations"),
		Now:                now,
		RecoveryRepo:       recoveryRepo,
	})

	// Initialize auth service with validated config
	authConfig, err := auth.NewServiceConfig(
		dispatcher,
		defaultFrom,
		frontendURL,
		passwordResetTokenExpiry,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid auth service config: %w", err)
	}
	authConfig.ParentsURL = parentsURL
	authConfig.SchoolURL = schoolURL
	authConfig.Settings = settingsService
	authService, err := auth.NewService(repos, authConfig, db, authLogger)
	if err != nil {
		return nil, err
	}

	mfaTokenAuth, err := authjwt.NewTokenAuth()
	if err != nil {
		return nil, fmt.Errorf("init mfa token auth: %w", err)
	}
	mfaService, err := auth.NewMFAService(auth.MFAServiceConfig{
		Repos:       repos,
		TokenAuth:   mfaTokenAuth,
		Settings:    settingsService,
		Dispatcher:  dispatcher,
		DefaultFrom: defaultFrom,
		FrontendURL: frontendURL,
		JWTSecret:   viper.GetString("auth_jwt_secret"),
		DB:          db,
		Logger:      authLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("init mfa service: %w", err)
	}
	// Wire the MFA gate into the auth service so /auth/login knows to issue
	// challenge tokens instead of token pairs when MFA is required. Done
	// post-construction so we don't introduce a constructor cycle.
	authService.SetMFAService(mfaService)

	tenantDomain := strings.TrimSpace(viper.GetString("tenant_domain"))
	if tenantDomain == "" {
		return nil, fmt.Errorf("TENANT_DOMAIN is required")
	}
	passkeyService, err := auth.NewPasskeyService(auth.PasskeyServiceConfig{
		Repos:        repos,
		MFAService:   mfaService,
		AuthService:  authService,
		DB:           db,
		Logger:       authLogger,
		RPID:         tenantDomain,
		RPName:       "moto",
		TenantDomain: tenantDomain,
	})
	if err != nil {
		return nil, fmt.Errorf("init passkey service: %w", err)
	}

	invitationService := auth.NewInvitationService(auth.InvitationServiceConfig{
		InvitationRepo:    repos.InvitationToken,
		AccountRepo:       repos.Account,
		AccountTenantRepo: repos.AccountTenant,
		RoleRepo:          repos.Role,
		PermissionRepo:    repos.Permission,
		AccountRoleRepo:   repos.AccountRole,
		PersonRepo:        repos.Person,
		StaffRepo:         repos.Staff,
		TeacherRepo:       repos.Teacher,
		StudentRepo:       repos.Student,
		SchoolRepo:        repos.School,
		Mailer:            mailer,
		Dispatcher:        dispatcher,
		FrontendURL:       frontendURL,
		SchoolURL:         schoolURL,
		DefaultFrom:       defaultFrom,
		InvitationExpiry:  invitationTokenExpiry,
		MailIdentity:      tenantMailIdentity,
		DB:                db,
		Logger:            authLogger,
	})

	// Email outbox (parent-enrollment PR 5). Declared here so the
	// guardian invitation service can wire OutboxEnqueuer below.
	emailTemplateRegistry := platform.NewTemplateRegistry()
	emailOutboxService := platform.NewOutboxService(repos.EmailOutbox)
	emailOutboxWorker := platform.NewOutboxWorker(platform.OutboxWorkerConfig{
		Repo:        repos.EmailOutbox,
		Registry:    emailTemplateRegistry,
		Mailer:      mailer,
		MaxAttempts: 6, // pushed by scheduler from settings each tick
		Logger:      logger.With("service", "outbox"),
		DB:          db,
	})
	emailOutboxWorker.SetMailIdentityResolver(tenantMailIdentity)

	guardianInvitationService := auth.NewGuardianInvitationService(auth.GuardianInvitationServiceConfig{
		InvitationRepo:         repos.GuardianInvitation,
		AccountRepo:            repos.Account,
		AccountTenantRepo:      repos.AccountTenant,
		AccountRoleRepo:        repos.AccountRole,
		RoleRepo:               repos.Role,
		PersonRepo:             repos.Person,
		GuardianProfileRepo:    repos.GuardianProfile,
		StudentGuardianRepo:    repos.StudentGuardian,
		GuardianFinancialAudit: repos.GuardianFinancialChange,
		StudentRepo:            repos.Student,
		SchoolRepo:             repos.School,
		OutboxEnqueuer:         emailOutboxService,
		EnrollmentBackfiller:   repos.ParentEnrollmentRequest,
		SettingsResolver:       settingsService,
		FrontendURL:            parentsURL, // accept link goes to the parents portal, not the staff frontend
		FallbackExpiry:         invitationTokenExpiry,
		DB:                     db,
		Logger:                 authLogger.With("flow", "guardian_invitation"),
	})

	// Register the guardian_invitation renderer at startup so the outbox
	// worker can dispatch enqueued rows. PR 7 adds enrollment_submitted +
	// enrollment_admin_notification renderers below; PR 8 will add the
	// decision-digest renderer alongside its service wiring.
	emailTemplateRegistry.Register(
		platformModels.EmailKindGuardianInvitation,
		platform.RendererFunc(auth.NewGuardianInvitationRenderer(auth.GuardianInvitationRendererConfig{
			DefaultFrom: defaultFrom,
		})),
	)
	emailTemplateRegistry.Register(
		platformModels.EmailKindParentAnnouncement,
		platform.RendererFunc(announcement.NewAnnouncementRenderer(announcement.EmailConfig{
			DefaultFrom: defaultFrom,
		})),
	)
	emailTemplateRegistry.Register(
		platformModels.EmailKindParentMessage,
		platform.RendererFunc(messaging.NewParentMessageRenderer(messaging.ParentMessageRendererConfig{DefaultFrom: defaultFrom})),
	)
	// Calendar appointment (Termine) notifications — one renderer, all four kinds.
	appointmentRenderer := platform.RendererFunc(calendarService.NewAppointmentRenderer(calendarService.EmailConfig{
		DefaultFrom: defaultFrom,
		DB:          db,
		Guardians:   repos.StudentGuardian,
	}))
	for _, kind := range []string{
		platformModels.EmailKindAppointmentPublished,
		platformModels.EmailKindAppointmentUpdated,
		platformModels.EmailKindAppointmentCancelled,
		platformModels.EmailKindAppointmentReminder,
	} {
		emailTemplateRegistry.Register(kind, appointmentRenderer)
	}
	// Enrollment outbox renderers, one per EmailKind sharing the same config.
	// Per-status decision emails (PR 8 slice 2) keep one renderer per kind so
	// subjects + templates stay independent and copy updates stay contained.
	// The rollover (annual phase renewal) pair reuses the submission template
	// as a placeholder until proper branded copy lands in a follow-up PR.
	enrollmentRendererCfg := enrollment.EmailRendererConfig{DefaultFrom: defaultFrom}
	for kind, newRenderer := range map[string]func(enrollment.EmailRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error){
		platformModels.EmailKindEnrollmentSubmitted:                enrollment.NewEnrollmentSubmittedRenderer,
		platformModels.EmailKindEnrollmentAdminNotify:              enrollment.NewEnrollmentAdminNotificationRenderer,
		platformModels.EmailKindEnrollmentApproved:                 enrollment.NewEnrollmentApprovedRenderer,
		platformModels.EmailKindEnrollmentWaitlisted:               enrollment.NewEnrollmentWaitlistedRenderer,
		platformModels.EmailKindEnrollmentRejected:                 enrollment.NewEnrollmentRejectedRenderer,
		platformModels.EmailKindEnrollmentDecisionDigest:           enrollment.NewEnrollmentDecisionDigestRenderer,
		platformModels.EmailKindEnrollmentChangeRequestSubmitted:   enrollment.NewEnrollmentChangeRequestSubmittedRenderer,
		platformModels.EmailKindEnrollmentChangeRequestQuestion:    enrollment.NewEnrollmentChangeRequestQuestionRenderer,
		platformModels.EmailKindEnrollmentChangeRequestParentReply: enrollment.NewEnrollmentChangeRequestParentReplyRenderer,
		platformModels.EmailKindEnrollmentChangeRequestApproved:    enrollment.NewEnrollmentChangeRequestApprovedRenderer,
		platformModels.EmailKindEnrollmentChangeRequestRejected:    enrollment.NewEnrollmentChangeRequestRejectedRenderer,
		platformModels.EmailKindEnrollmentRolloverOptIn:            enrollment.NewEnrollmentRolloverOptInRenderer,
		platformModels.EmailKindEnrollmentRolloverOptOut:           enrollment.NewEnrollmentRolloverOptOutRenderer,
	} {
		emailTemplateRegistry.Register(kind, platform.RendererFunc(newRenderer(enrollmentRendererCfg)))
	}

	caregiverCapabilityService := users.NewCaregiverCapabilityService(users.CaregiverCapabilityServiceDependencies{
		AccountRepo:            repos.Account,
		AccountTenantRepo:      repos.AccountTenant,
		AuthEventRepo:          repos.AuthEvent,
		RoleRepo:               repos.Role,
		PersonRepo:             repos.Person,
		StaffRepo:              repos.Staff,
		TeacherRepo:            repos.Teacher,
		GroupTeacherRepo:       repos.GroupTeacher,
		GroupSubstitutionRepo:  repos.GroupSubstitution,
		GroupSupervisorRepo:    repos.GroupSupervisor,
		ActivitySupervisorRepo: repos.ActivitySupervisor,
		AuthService:            authService,
		DB:                     db,
	})

	staffOffboardingService := users.NewStaffOffboardingService(users.StaffOffboardingServiceDependencies{
		PersonRepo:             repos.Person,
		StaffRepo:              repos.Staff,
		TeacherRepo:            repos.Teacher,
		GroupSupervisorRepo:    repos.GroupSupervisor,
		GroupTeacherRepo:       repos.GroupTeacher,
		ClassTeacherRepo:       repos.ClassTeacher,
		GroupSubstitutionRepo:  repos.GroupSubstitution,
		ActivitySupervisorRepo: repos.ActivitySupervisor,
		InstanceStaffRepo:      repos.InstanceStaff,
		StaffShiftRepo:         repos.StaffShift,
		StaffShiftSeriesRepo:   repos.StaffShiftSeries,
		StaffAbsenceRepo:       repos.StaffAbsence,
		AccountRepo:            repos.Account,
		AccountTenantRepo:      repos.AccountTenant,
		RoleRepo:               repos.Role,
		AccountPermissionRepo:  repos.AccountPermission,
		DataDeletionRepo:       repos.DataDeletion,
		TimeTrackingDeleteRepo: repos.TimeTrackingDeletion,
		AuthService:            authService,
		DB:                     db,
		Logger:                 logger.With("service", "staff_offboarding"),
	})
	if broadcastAware, ok := staffOffboardingService.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}); ok {
		broadcastAware.SetBroadcaster(realtimeHub)
	}

	// Initialize user context service
	userContextService := usercontext.NewUserContextServiceWithRepos(usercontext.UserContextRepositories{
		AccountRepo:        repos.Account,
		PersonRepo:         repos.Person,
		StaffRepo:          repos.Staff,
		TeacherRepo:        repos.Teacher,
		StudentRepo:        repos.Student,
		EducationGroupRepo: repos.Group,
		ActivityGroupRepo:  repos.ActivityGroup,
		ActiveGroupRepo:    repos.ActiveGroup,
		VisitsRepo:         repos.ActiveVisit,
		SupervisorRepo:     repos.GroupSupervisor,
		ProfileRepo:        repos.Profile,
		SubstitutionRepo:   repos.GroupSubstitution,
		ClassTeacherRepo:   repos.ClassTeacher,
		ActiveService:      activeService,
		SSESettings:        settingsService,
	}, usercontextLogger)
	substitutionService := education.NewSubstitutionModule(education.SubstitutionDependencies{
		Groups: repos.Group, Substitutions: repos.GroupSubstitution,
		Teachers: repos.Teacher, Staff: repos.Staff, Actors: substitutionActorResolver{identity: userContextService},
		ActiveGroups: repos.ActiveGroup, ActiveSupervisors: repos.GroupSupervisor,
		ActiveSupervisorCreator: activeService,
		Audit:                   repos.SubstitutionChange, DB: db, Broadcaster: realtimeHub,
		Logger: logger.With("service", "substitution"),
		Schedule: newScheduleSubstitutionBridge(schedule.NewSubstitutionAdapter(schedule.SubstitutionAdapterDependencies{
			Instances: repos.ActivityInstance, InstanceStaff: repos.InstanceStaff,
			Staff: repos.Staff, Engine: instanceService, Broadcaster: realtimeHub,
			Logger: logger.With("service", "schedule-substitution"),
		})),
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
	databaseService := database.NewService(repos, databaseLogger)

	// Initialize cleanup service
	privacyConsentService := users.NewPrivacyConsentService(settingsService, logger.With("service", "privacy-consent"))
	activeCleanupService := active.NewCleanupService(
		repos.ActiveVisit,
		repos.Attendance,
		repos.GroupSupervisor,
		repos.PrivacyConsent,
		repos.DataDeletion,
		privacyConsentService,
		db,
		today,
	)
	unregisteredTagScanService := auditService.NewUnregisteredTagScanService(repos.UnregisteredTagScan, db)

	// Initialize import service
	relationshipResolver := importService.NewRelationshipResolver(repos.Group, repos.Room)
	studentImportConfig := importService.NewStudentImportConfig(
		importService.StudentImportDeps{
			PersonRepo:          repos.Person,
			StudentRepo:         repos.Student,
			GuardianRepo:        repos.GuardianProfile,
			GuardianPhoneRepo:   repos.GuardianPhoneNumber,
			RelationRepo:        repos.StudentGuardian,
			PrivacyRepo:         repos.PrivacyConsent,
			ArrivalScheduleRepo: repos.StudentArrivalSchedule,
			PickupScheduleRepo:  repos.StudentPickupSchedule,
			RFIDCardRepo:        repos.RFIDCard,
			Resolver:            relationshipResolver,
		},
		db,
	)
	studentImportService := importService.NewImportService(studentImportConfig)
	studentImportService.SetAuditRepository(repos.DataImport)

	// Staff import files the Stammdatensatz (Person/Staff/Teacher/master
	// data) immediately and issues an invitation for rows with an e-mail;
	// accepting links the account to the imported person (#2600).
	staffImportConfig := importService.NewStaffImportConfig(
		importService.StaffImportDeps{
			InvitationService: invitationService,
			InvitationRepo:    repos.InvitationToken,
			AccountRepo:       repos.Account,
			AccountTenantRepo: repos.AccountTenant,
			RoleRepo:          repos.Role,
			PermissionRepo:    repos.Permission,
			SchoolRepo:        repos.School,
			PersonRepo:        repos.Person,
			StaffRepo:         repos.Staff,
			TeacherRepo:       repos.Teacher,
			MasterDataRepo:    repos.StaffMasterData,
			QualificationRepo: repos.StaffQualification,
		},
	)
	staffImportService := importService.NewImportService(staffImportConfig)
	staffImportService.SetAuditRepository(repos.DataImport)

	// Class-list entry import (#2382): creates through the entry service so
	// the duplicate guards and the audit trail apply to imported rows too.
	classListImportConfig := importService.NewClassListImportConfig(importService.ClassListImportDeps{
		EntryService: users.NewClassListEntryService(repos.ClassListEntry, repos.Student, repos.ClassListEntryChange),
		EntryRepo:    repos.ClassListEntry,
		StudentRepo:  repos.Student,
	})
	classListImportService := importService.NewImportService(classListImportConfig)
	classListImportService.SetAuditRepository(repos.DataImport)

	// Opening balance import (#2132): the config is request-scoped (Stichtag,
	// Begründung, and acting staff member come from the upload form), so the
	// factory closes over the request-independent deps and builds a fresh
	// service per request.
	openingBalanceImportFactory := importService.OpeningBalanceImportFactory(
		func(effectiveDate timezone.Date, note string, decidedByStaffID int64) *importService.ImportService[importModels.OpeningBalanceImportRow] {
			config := importService.NewOpeningBalanceImportConfig(importService.OpeningBalanceImportDeps{
				StaffRepo:            repos.Staff,
				AdjustmentRepo:       repos.StaffBalanceAdjust,
				VacationOpeningRepo:  repos.StaffVacationOpening,
				BalanceAdjustService: staffBalanceAdjustService,
				StaffAbsenceService:  staffAbsenceService,
			}, effectiveDate, note, decidedByStaffID)
			svc := importService.NewImportService(config)
			svc.SetAuditRepository(repos.DataImport)
			return svc
		})

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

	// Operator frontend URL for invitation emails. The operator subdomain is separate
	// from FRONTEND_URL, so we link directly to the operator host to avoid a
	// cross-origin redirect hop that email content scanners treat as a phishing
	// signal. Constructed conditionally - only required when actually sending
	// invitations. InviteOperator and ResendOperatorInvitation guard on empty
	// operatorFrontendURL.
	var operatorFrontendURL string
	if operatorHostname := viper.GetString("next_public_operator_hostname"); operatorHostname != "" {
		protocol := "http"
		if strings.HasPrefix(frontendURL, "https://") {
			protocol = "https"
		}
		operatorFrontendURL = fmt.Sprintf("%s://%s", protocol, strings.TrimRight(operatorHostname, "/"))
	}
	if operatorFrontendURL == "" {
		return nil, fmt.Errorf("NEXT_PUBLIC_OPERATOR_HOSTNAME is required")
	}

	// Initialize platform services (operator dashboard)
	operatorAuthService, err := platform.NewOperatorAuthService(platform.OperatorAuthServiceConfig{
		OperatorRepo:         repos.Operator,
		AuditLogRepo:         repos.OperatorAuditLog,
		EmailChangeTokenRepo: repos.OperatorEmailChangeToken,
		RefreshTokenRepo:     repos.OperatorRefreshToken,
		InvitationTokenRepo:  repos.OperatorInvitationToken,
		DB:                   db,
		Logger:               platformLogger,
		Dispatcher:           dispatcher,
		DefaultFrom:          defaultFrom,
		FrontendURL:          frontendURL,
		OperatorFrontendURL:  operatorFrontendURL,
		EmailChangeExpiry:    emailChangeExpiry,
		InvitationExpiry:     invitationTokenExpiry,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create operator auth service: %w", err)
	}

	// Operator MFA service (issue #1308 phase 7b-2). Constructed alongside
	// the operator auth service so the login-flow integration in 7b-3 can
	// inject it via SetMFAService.
	operatorMFATokenAuth, err := authjwt.NewTokenAuth()
	if err != nil {
		return nil, fmt.Errorf("init operator mfa token auth: %w", err)
	}
	operatorMFAService, err := platform.NewOperatorMFAService(platform.OperatorMFAServiceConfig{
		Repos:       repos,
		TokenAuth:   operatorMFATokenAuth,
		Dispatcher:  dispatcher,
		DefaultFrom: defaultFrom,
		FrontendURL: frontendURL,
		JWTSecret:   viper.GetString("auth_jwt_secret"),
		DB:          db,
		Logger:      platformLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("init operator mfa service: %w", err)
	}
	// Wire the MFA gate into the operator auth service so /operator/auth/login
	// returns challenge tokens when MFA is required (= always, hardcoded for
	// platform scope). Done post-construction to break the
	// OperatorAuthService ↔ OperatorMFAService cycle.
	operatorAuthService.SetMFAService(operatorMFAService)
	operatorPasskeyService, err := platform.NewOperatorPasskeyService(platform.OperatorPasskeyServiceConfig{
		Repos:               repos,
		MFAService:          operatorMFAService,
		AuthService:         operatorAuthService,
		DB:                  db,
		Logger:              platformLogger,
		RPID:                tenantDomain,
		RPName:              "moto",
		OperatorFrontendURL: operatorFrontendURL,
	})
	if err != nil {
		return nil, fmt.Errorf("init operator passkey service: %w", err)
	}

	announcementService := platform.NewAnnouncementService(platform.AnnouncementServiceConfig{
		AnnouncementRepo:     repos.Announcement,
		AnnouncementViewRepo: repos.AnnouncementView,
		AuditLogRepo:         repos.OperatorAuditLog,
		OrgRepo:              repos.Organization,
		SchoolRepo:           repos.School,
		DB:                   db,
		Logger:               platformLogger,
	})

	enrollmentFormSchemaService := enrollment.NewFormSchemaService(enrollment.FormSchemaServiceConfig{
		Repo:        repos.FormSchema,
		PhaseRepo:   repos.Phase,
		RequestRepo: repos.Request,
		Settings:    settingsService,
		Logger:      logger.With("service", "enrollment-form-schema"),
	})

	enrollmentCaptchaService := enrollment.NewCaptchaService(enrollment.CaptchaServiceConfig{
		Settings: settingsService,
		Logger:   logger.With("service", "enrollment-captcha"),
	})

	enrollmentDeletionService := enrollment.NewEnrollmentDeletionService(
		repos.Request,
		repos.RequestChild,
		repos.EnrollmentDeletion,
		repos.EnrollmentDeletionAudit,
		db,
		logger.With("service", "enrollment-deletion"),
	)
	enrollmentRejectedCleanupService := enrollment.NewRejectedEnrollmentCleanupService(
		repos.Request,
		repos.RequestChild,
		repos.LateInvite,
		repos.EmailOutbox,
		settingsService,
		db,
		logger.With("service", "enrollment-rejected-cleanup"),
		enrollment.RejectedEnrollmentCleanupAuditDependencies{
			Deletion: repos.EnrollmentDeletion,
			Audit:    repos.EnrollmentDeletionAudit,
		},
	)

	enrollmentPhaseService := enrollment.NewPhaseService(enrollment.PhaseServiceConfig{
		Repo:             repos.Phase,
		RequestRepo:      repos.Request,
		RequestChildRepo: repos.RequestChild,
		CareOfferingRepo: repos.CareOffering,
		FormSchemaRepo:   repos.FormSchema,
		CalendarPeriods:  calendarPeriodService,
		LockTemplateRecurrence: func(ctx context.Context) error {
			return schedule.LockTenantRecurrenceWrites(ctx, db)
		},
		ValidateCareOfferingPhaseChange: careOfferingPhaseValidator.ValidatePhaseChange,
		Settings:                        settingsService,
		DB:                              db,
		Logger:                          logger.With("service", "enrollment-phase"),
	})
	enrollmentPhaseExpiryService := enrollment.NewPhaseExpiryService(repos.PhaseExpiry)

	studentAuditService := users.NewStudentAuditService(
		repos.StudentFieldEdit,
		logger.With("service", "student_audit"),
	)
	careLifecycleService := users.NewCareLifecycleService(users.CareLifecycleDependencies{
		StudentRepo:    repos.Student,
		PersonRepo:     repos.Person,
		CareExitRepo:   repos.CareExit,
		CleanupRepo:    repos.CareExitCleanup,
		WithdrawalRepo: repos.CareWithdrawal,
		TagReleaser:    repos.GradeTransition,
		AuditService:   studentAuditService,
		LockCareBookingWrites: func(ctx context.Context) error {
			return schedule.LockTenantRecurrenceWrites(ctx, db)
		},
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return settingsService.ResolveBool(ctx, configModels.KeyEnrollmentBookingsAuthoritative)
		},
		DB:     db,
		Logger: logger.With("service", "care_lifecycle"),
	})
	users.WirePersonCareParticipation(usersService, careLifecycleService)
	schedule.WireCareParticipation(careDayService, careLifecycleService)
	// Chat-pill emitter (#1803): also provides guardian-only invalidations for
	// enrollment writes that change a child's live care data.
	pillEmitter := parentmessaging.NewEmitter(
		db,
		repos.ParentMessageThread,
		repos.ParentMessage,
		settingsService,
		realtimeHub,
		logger.With("service", "parent-events"),
	)

	// Anwesenheitswechsel wecken die Sorgeberechtigten, damit der Tagesstatus in der Eltern-App (#2252) live nachlaedt.
	if waker, ok := activeService.(interface {
		SetGuardianWaker(active.GuardianWaker)
	}); ok {
		waker.SetGuardianWaker(pillEmitter)
	}

	enrollmentDecisionService := enrollment.NewDecisionService(enrollment.DecisionServiceConfig{
		RequestRepo:              repos.Request,
		RequestChildRepo:         repos.RequestChild,
		RequestGuardianRepo:      repos.RequestGuardian,
		LateInviteRepo:           repos.LateInvite,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		CareOfferingRepo:         repos.CareOffering,
		PhaseRepo:                repos.Phase,
		FormSchemaRepo:           repos.FormSchema,
		DataAccessLogRepo:        repos.DataAccessLog,
		OfferingAdjustmentRepo:   repos.EnrollmentOfferingAdjustment,
		RestorationAuditRepo:     repos.EnrollmentRestorationAudit,
		SchoolRepo:               repos.School,
		PersonRepo:               repos.Person,
		StaffRepo:                repos.Staff,
		StudentRepo:              repos.Student,
		StudentGuardianRepo:      repos.StudentGuardian,
		GuardianFinancialAudit:   repos.GuardianFinancialChange,
		GuardianProfileRepo:      repos.GuardianProfile,
		GuardianPhoneRepo:        repos.GuardianPhoneNumber,
		PickupScheduleRepo:       repos.StudentPickupSchedule,
		PickupBaselines:          pickupBaselines,
		ArrivalScheduleRepo:      repos.StudentArrivalSchedule,
		StudentEnrollmentRepo:    repos.StudentEnrollment,
		ActivityGroupRepo:        repos.ActivityGroup,
		ActivityScheduleRepo:     repos.ActivitySchedule,
		CalendarPeriodRepo:       repos.CalendarPeriod,
		TimeframeRepo:            repos.Timeframe,
		ActivityExceptionRepo:    repos.ActivityException,
		AccountRepo:              repos.Account,
		AccountTenantRepo:        repos.AccountTenant,
		AccountRoleRepo:          repos.AccountRole,
		RoleRepo:                 repos.Role,
		OutboxEnqueuer:           emailOutboxService,
		StudentAudit:             studentAuditService,
		CareWithdrawal:           careLifecycleService,
		Broadcaster:              realtimeHub,
		PickupGuardianNotifier:   pillEmitter,
		FrontendURL:              frontendURL,
		ParentsURL:               parentsURL,
		Settings:                 settingsService,
		LockTemplateRecurrence: func(ctx context.Context) error {
			return schedule.LockTenantRecurrenceWrites(ctx, db)
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
		// The reconciler takes these BEFORE writing weekly rows — the same
		// student → schedule-row → care-day lock order the staff weekly
		// editors use, so the two weekly writers cannot deadlock against
		// each other (#2360 review). Uses the ambient tenant transaction;
		// missing students (concurrent offboarding) are skipped, matching
		// the resync's tolerance.
		LockPickupStudents: func(ctx context.Context, studentIDs []int64) error {
			for _, studentID := range studentIDs {
				if err := schedule.LockCareStudent(ctx, db, studentID); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
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
	// A split that moves the Zielgruppe away from 'angebot' drops the
	// successor's source rule; the carried roster must then shed its
	// source-derived rows (#2147 review). Wired late because the decision
	// service is constructed after the split service.
	templateSplitService.SetOfferingRosterResync(offeringRosterResyncer.ResyncTemplateOfferingRoster)
	// Grade transitions rewrite school classes, so they must re-reconcile the
	// offering-sourced templates' Jahrgang-filtered rosters (#2137). Wired
	// here because the decision service is constructed after the grade
	// transition service.
	gradeTransitionResyncer, ok := enrollmentDecisionService.(education.OfferingSourceResyncer)
	if !ok {
		return nil, fmt.Errorf("enrollment decision service does not implement the grade-transition offering resync")
	}
	gradeTransitionService.SetOfferingSourceResyncer(gradeTransitionResyncer)
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
		RequestRepo:              repos.Request,
		RequestChildRepo:         repos.RequestChild,
		RequestGuardianRepo:      repos.RequestGuardian,
		LateInviteRepo:           repos.LateInvite,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		CareOfferingRepo:         repos.CareOffering,
		FormSchemaRepo:           repos.FormSchema,
		PhaseRepo:                repos.Phase,
		SchoolRepo:               repos.School,
		StudentRepo:              repos.Student,
		GuardianAuthorizer:       repos.StudentGuardian,
		RateLimitRepo:            repos.SubmissionRateLimit,
		OutboxEnqueuer:           emailOutboxService,
		Settings:                 settingsService,
		ManualDecider:            enrollmentDecisionService,
		FrontendURL:              frontendURL, // admin notification email
		ParentsURL:               parentsURL,  // parent confirmation/status emails
		DB:                       db,
		Logger:                   logger.With("service", "enrollment-request"),
	})

	enrollmentReportService := enrollment.NewReportService(enrollment.ReportServiceConfig{
		RequestRepo:              repos.Request,
		RequestChildRepo:         repos.RequestChild,
		RequestGuardianRepo:      repos.RequestGuardian,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		CareOfferingRepo:         repos.CareOffering,
		FormSchemaRepo:           repos.FormSchema,
		PhaseRepo:                repos.Phase,
		DataAccessLogRepo:        repos.DataAccessLog,
		StudentRepo:              repos.Student,
		StudentGuardianRepo:      repos.StudentGuardian,
		StudentCompanionRepo:     repos.StudentCompanion,
		PersonRepo:               repos.Person,
		EducationGroupRepo:       repos.Group,
		StudentStatusDayRepo:     repos.StudentStatusDay,
		ClassListEntryRepo:       repos.ClassListEntry,
		PickupScheduleSvc:        pickupScheduleService,
		ArrivalScheduleSvc:       arrivalScheduleService,
		CareDaySvc:               careDayService,
		Settings:                 settingsService,
		CareParticipation:        careLifecycleService,
	})
	enrollmentDecisionApplier, _ := enrollmentDecisionService.(enrollment.ChangeRequestDecisionApplier)

	// Created before the change-request service: its multi-child approval takes
	// the companion lock order through this service.
	studentService := users.NewStudentService(
		repos.Student,
		repos.PrivacyConsent,
		repos.StudentCompanion,
		studentAuditService,
	)
	studentDeletionService := users.NewStudentDeletionService(
		studentService,
		repos.Student,
		repos.Person,
		repos.StudentDeletion,
		repos.GradeTransition,
		repos.DataDeletion,
		repos.StudentDeletionAudit,
		db,
	)
	users.WireStudentDocumentCleanup(studentDeletionService, repos.StudentDocument)
	users.WireStudentDeletionCareWithdrawals(studentDeletionService, repos.CareWithdrawal)
	users.WireCareWithdrawalDeletion(careLifecycleService, studentDeletionService)

	// Child documents (#777): metadata, per-category authority and the
	// per-child access gate for the Dokumente tab. Needs the user context to
	// answer "does this caller supervise this child", so it is wired after it.
	studentDocumentService := users.NewStudentDocumentService(
		db,
		repos.StudentDocument,
		repos.Student,
		repos.StudentFieldEdit,
		repos.DataAccessLog,
		userContextService,
		logger.With("service", "student_documents"),
	)

	// School file storage (#2596): folders, visibility, quota, audit trail.
	fileStoreService := filestore.NewService(
		db,
		repos.FileFolder,
		repos.File,
		repos.FileEvent,
		settingsService,
		logger.With("service", "filestore"),
	)

	enrollmentChangeRequestService := enrollment.NewChangeRequestService(enrollment.ChangeRequestServiceConfig{
		ChangeRequestRepo:        repos.ChangeRequest,
		MessageRepo:              repos.ChangeRequestMessage,
		RequestRepo:              repos.Request,
		RequestChildRepo:         repos.RequestChild,
		RequestGuardianRepo:      repos.RequestGuardian,
		LateInviteRepo:           repos.LateInvite,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		CareOfferingRepo:         repos.CareOffering,
		FormSchemaRepo:           repos.FormSchema,
		PhaseRepo:                repos.Phase,
		SchoolRepo:               repos.School,
		GuardianProfileRepo:      repos.GuardianProfile,
		GuardianPhoneRepo:        repos.GuardianPhoneNumber,
		PersonRepo:               repos.Person,
		StudentRepo:              repos.Student,
		GuardianAuthorizer:       repos.StudentGuardian,
		DecisionService:          enrollmentDecisionApplier,
		CompanionGraphLocker:     studentService,
		Settings:                 settingsService,
		OutboxEnqueuer:           emailOutboxService,
		FrontendURL:              frontendURL,
		ParentsURL:               parentsURL,
		DB:                       db,
		Logger:                   logger.With("service", "enrollment-change-request"),
	})

	// Rollover service depends on DecisionService for the
	// rollover_auto_approve=true deadline path.
	enrollmentRolloverCatalogCloner, ok := enrollmentCareOfferingService.(enrollment.RolloverOfferingCatalogCloner)
	if !ok {
		return nil, fmt.Errorf("enrollment care offering service does not implement rollover catalog cloning")
	}
	enrollmentRolloverService := enrollment.NewRolloverService(enrollment.RolloverServiceConfig{
		PhaseRepo:                repos.Phase,
		RequestRepo:              repos.Request,
		RequestChildRepo:         repos.RequestChild,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		OfferingCatalogCloner:    enrollmentRolloverCatalogCloner,
		SchoolRepo:               repos.School,
		OutboxEnqueuer:           emailOutboxService,
		Settings:                 settingsService,
		DecisionService:          enrollmentDecisionService,
		ParentsURL:               parentsURL,
		DB:                       db,
		Logger:                   logger.With("service", "enrollment-rollover"),
	})
	requestReviewPolicy := usercontext.NewParentRequestReviewPolicy(
		settingsService,
		userContextService,
		configModels.KeyParentRequestGroupLeaderReviewEnabled,
	)

	// One append-only ledger for every parent request, shared by all four
	// domains so a request's history survives edits, decisions and corrections
	// the request rows themselves overwrite (#2267).
	parentRequestEvents := users.NewParentRequestEventRecorder(repos.ParentRequestEvent)

	// Care-schedule change requests (#1803): the schedule-domain request
	// lifecycle (create / withdraw / staff decide + apply), decoupled from the
	// chat.
	careRequestService := schedule.NewCareScheduleRequestServiceWithPickupChangesAndPolicy(
		repos.CareScheduleChangeRequest,
		repos.Student,
		repos.Person,
		arrivalScheduleService,
		pickupScheduleService,
		repos.StudentPickupException,
		repos.Attendance,
		pickupAutoExcusal,
		userContextService,
		pillEmitter,
		realtimeHub,
		requestReviewPolicy,
		parentRequestEvents,
		logger.With("service", "care-requests"),
		studentAuditService,
	)

	// Post-enrollment offering changes (#1665): the parents portal submits them,
	// staff decide them on the same review page, and an approval applies the
	// switch through the decision service's dated adjustment path.
	directOfferingApplier, ok := enrollmentDecisionService.(enrollment.DirectOfferingAdjustmentApplier)
	if !ok {
		return nil, fmt.Errorf("enrollment decision service does not implement direct offering adjustment")
	}
	offeringChangeRequestService := enrollment.NewOfferingChangeRequestServiceWithPolicy(enrollment.OfferingChangeRequestServiceConfig{
		ChangeRepo:               repos.OfferingChangeRequest,
		RequestChildRepo:         repos.RequestChild,
		RequestRepo:              repos.Request,
		PhaseRepo:                repos.Phase,
		CareOfferingRepo:         repos.CareOffering,
		ImpactRepo:               repos.OfferingChangeImpact,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		StudentRepo:              repos.Student,
		PersonRepo:               repos.Person,
		CareWithdrawalRepo:       repos.CareWithdrawal,
		OfferingAdjustmentRepo:   repos.EnrollmentOfferingAdjustment,
		UserContext:              userContextService,
		Applier:                  enrollmentDecisionApplier,
		DirectApplier:            directOfferingApplier,
		Settings:                 settingsService,
		Emitter:                  pillEmitter,
		Logger:                   logger.With("service", "offering-change-requests"),
		EventRecorder:            parentRequestEvents,
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
	})

	// Excused-absence approval requests (#1845): the optional office-approval
	// gate for parent-submitted excused absences. Reuses the same review queue,
	// badge and pill machinery as the care-schedule requests above; on approval
	// it writes the excused status days directly.
	excusedRequestService := absence.NewExcusedAbsenceRequestServiceWithPolicy(
		repos.ExcusedAbsenceRequest,
		repos.StudentStatusDay,
		repos.StudentPickupException,
		repos.Student,
		repos.Person,
		userContextService,
		pillEmitter,
		realtimeHub,
		requestReviewPolicy,
		parentRequestEvents,
		logger.With("service", "excused-requests"),
		db,
	)

	// Review access is one cross-domain policy: admins remain school-wide;
	// group leaders are opt-in and limited to their current groups. Attach it
	// to every request service so the unified queue and the legacy per-type
	// decision routes cannot disagree about who may see or decide a request.
	// The notification router and the consent service are built here, ahead of
	// their consumers: messaging, the calendar (#1671) and the announcement
	// producer all need them, and messaging is constructed first.
	vapidConfig := notifications.VAPIDConfig{
		PublicKey:  strings.TrimSpace(viper.GetString("vapid_public_key")),
		PrivateKey: strings.TrimSpace(viper.GetString("vapid_private_key")),
		Subscriber: strings.TrimSpace(viper.GetString("vapid_subscriber")),
	}
	if err := vapidConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid VAPID configuration: %w", err)
	}
	if !vapidConfig.Configured() {
		logger.Info("web push disabled: VAPID keys not configured (VAPID_PUBLIC_KEY / VAPID_PRIVATE_KEY / VAPID_SUBSCRIBER)")
	}
	notificationsService := notifications.NewService(
		settingsService,
		logger.With("service", "notifications"),
		notifications.NewSSEChannel(realtimeHub, notifications.WithGuardianChildAccess(
			db, repos.StudentGuardian, logger.With("channel", "sse"))),
		notifications.NewWebPushChannel(db, repos.PushSubscription, vapidConfig, logger.With("channel", "web_push")),
	)
	notificationPreferencesService := notifications.NewPreferenceService(
		repos.NotificationPreference,
		settingsService,
		db,
		repos.AccountTenant,
	)
	staffNotificationRecipients := notifications.NewStaffRecipientResolver(
		notificationPreferencesService,
		repos.Student,
		repos.Group,
		repos.Staff,
		repos.Account,
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

	messagingService := messaging.NewService(messaging.Config{
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
		Outbox:           emailOutboxService,
		GuardianProfiles: repos.GuardianProfile,
		Schools:          repos.School,
		LoginImages:      settingsService,
		ParentsURL:       parentsURL,
	})

	// OGS-internal colleague chat (#2598). Shares the transport with the
	// parent-OGS messenger (SSE hub + push) but none of its authorization:
	// access here is thread membership, nothing else.
	staffMessagingService := staffmessaging.NewService(staffmessaging.Config{
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
	})

	calendarSvc := calendarService.NewService(calendarService.Config{
		AppointmentRepo:        repos.CalendarAppointment,
		RecurrenceRepo:         repos.CalendarRecurrenceRule,
		RecipientRepo:          repos.CalendarAppointmentRecipient,
		RecipientStudentRepo:   repos.CalendarAppointmentRecipientChild,
		TargetRepo:             repos.CalendarAppointmentTarget,
		OverrideRepo:           repos.CalendarOccurrenceOverride,
		StaffRepo:              repos.Staff,
		StudentRepo:            repos.Student,
		GuardianProfileRepo:    repos.GuardianProfile,
		StudentGuardianRepo:    repos.StudentGuardian,
		ChildRepo:              repos.ParentChild,
		GroupRepo:              repos.Group,
		InstanceStaffRepo:      repos.InstanceStaff,
		ActivityInstanceRepo:   repos.ActivityInstance,
		RoomRepo:               repos.Room,
		StaffShiftRepo:         repos.StaffShift,
		ShiftTypeRepo:          repos.ShiftType,
		UserContext:            userContextService,
		DB:                     db,
		Outbox:                 emailOutboxService,
		SchoolRepo:             repos.School,
		Settings:               settingsService,
		AccountRepo:            repos.Account,
		StaffFeedRepo:          repos.StaffCalendarFeedToken,
		StaffFeedTombstoneRepo: repos.CalendarStaffFeedTombstone,
		PersonRepo:             repos.Person,
		ParentsURL:             parentsURL,
		FrontendURL:            frontendURL,
		Notifier:               notificationsService,
		ReminderNotifier:       notificationsService,
		Preferences:            notificationPreferencesService,
		Logger:                 logger.With("service", "calendar"),
	})

	parentService := parent.NewService(parent.ServiceConfig{
		ChildRepo:                 repos.ParentChild,
		EnrollablePhaseRepo:       repos.ParentEnrollablePhase,
		EnrollmentSettings:        settingsService,
		EnrollmentRequestRepo:     repos.ParentEnrollmentRequest,
		GuardianProfileRepo:       repos.GuardianProfile,
		AttendanceRepo:            repos.Attendance,
		StatusDayRepo:             repos.StudentStatusDay,
		MealPlanRepo:              repos.MealPlanEntry,
		StudentRepo:               repos.Student,
		PickupExceptionRepo:       repos.StudentPickupException,
		ArrivalExceptionRepo:      repos.StudentArrivalException,
		PickupAutoExcusal:         pickupAutoExcusal,
		Settings:                  settingsService,
		Broadcaster:               realtimeHub,
		PersonRepo:                repos.Person,
		ChangeRequestRepo:         repos.StudentDataChangeRequest,
		CareRequestRepo:           repos.CareScheduleChangeRequest,
		ExcusedRequestRepo:        repos.ExcusedAbsenceRequest,
		OfferingChangeRequestRepo: repos.OfferingChangeRequest,
		FamilyProtectionEvents:    repos.FamilyProtection,
		ParentRequestShares:       repos.ParentRequestShare,
		ParentRequestEvents:       parentRequestEvents,
		StudentAudit:              studentAuditService,
		MessageThreadRepo:         repos.ParentMessageThread,
		MessageRepo:               repos.ParentMessage,
		MessageReadRepo:           repos.ParentMessageRead,
		ParentMessageNotifier:     staffParentMessageNotifier,
		ArrivalSchedules:          arrivalScheduleService,
		PickupSchedules:           pickupScheduleService,
		CareRequests:              careRequestService,
		ExcusedRequests:           excusedRequestService,
		Emitter:                   pillEmitter,
		AnnouncementRepo:          repos.ParentAnnouncement,
		GuardianInvites:           guardianInvitationService,
		GuardianInviteRepo:        repos.GuardianInvitation,
		StudentGuardianRepo:       repos.StudentGuardian,
		GuardianPhoneRepo:         repos.GuardianPhoneNumber,
		GuardianChangeAuditRepo:   repos.GuardianChange,
		RequestChildRepo:          repos.RequestChild,
		RequestChildOfferingRepo:  repos.RequestChildOffering,
		CareOfferingRepo:          repos.CareOffering,
		OfferingChanges:           offeringChangeRequestService,
		DB:                        db,
		Logger:                    logger.With("service", "parent"),
		Now:                       now,
	})

	parentAnnouncementService := announcement.NewService(announcement.ServiceConfig{
		Repo:        repos.ParentAnnouncement,
		Settings:    settingsService,
		Outbox:      emailOutboxService,
		Notifier:    notificationsService,
		Preferences: notificationPreferencesService,
		Deliveries:  repos.EmailDelivery,
		ParentsURL:  parentsURL,
		Logger:      logger.With("service", "announcement"),
	})

	staffNoticeService := schedule.NewStaffNoticeService(schedule.StaffNoticeServiceConfig{
		Repo:    repos.StaffNotice,
		Periods: repos.CalendarPeriod,
		Logger:  logger.With("service", "staffnotice"),
	})

	// The cancellation notice (#2601) rides on the announcement service, which
	// is built after the instance service; inject it now that both exist.
	if setter, ok := instanceService.(schedule.GuardianNoticePublisherSetter); ok {
		setter.SetGuardianNoticePublisher(parentAnnouncementService)
	}

	operatorProvisioningService := platform.NewOperatorProvisioningService(platform.OperatorProvisioningServiceConfig{
		OrganizationRepo:      repos.Organization,
		SchoolRepo:            repos.School,
		SummariesRepo:         repos.OperatorSummaries,
		CategoryRepo:          repos.ActivityCategory,
		DeviceRepo:            repos.Device,
		RoleRepo:              repos.Role,
		AccountTenantRepo:     repos.AccountTenant,
		AccountRoleRepo:       repos.AccountRole,
		AccountPermissionRepo: repos.AccountPermission,
		AuthEventRepo:         repos.AuthEvent,
		PersonRepo:            repos.Person,
		StaffRepo:             repos.Staff,
		AccountRepo:           repos.Account,
		TeacherRepo:           repos.Teacher,
		StudentRepo:           repos.Student,
		GroupSupervisorRepo:   repos.GroupSupervisor,
		ActiveGroupRepo:       repos.ActiveGroup,
		Settings:              settingsService,
		InvitationService:     invitationService,
		AuthService:           authService,
		AuditLogRepo:          repos.OperatorAuditLog,
		DB:                    db,
		Logger:                platformLogger,
	})

	listExportService := listexport.NewService()
	emergencyService := emergency.NewService(emergency.Dependencies{
		AttendanceRepo:      repos.Attendance,
		StudentRepo:         repos.Student,
		PersonRepo:          repos.Person,
		VisitRepo:           repos.ActiveVisit,
		StudentGuardianRepo: repos.StudentGuardian,
		ActiveService:       activeService,
		ListExport:          listExportService,
		Settings:            settingsService,
		Logger:              logger,
	})
	slotListsService := slotlists.NewService(slotlists.Dependencies{
		InstanceRepo:        repos.ActivityInstance,
		InstanceStudentRepo: repos.InstanceStudent,
		VisitRepo:           repos.ActiveVisit,
		AttendanceRepo:      repos.Attendance,
		StatusDayRepo:       repos.StudentStatusDay,
		CareDayService:      careDayService,
		PickupExceptionRepo: repos.StudentPickupException,
		PickupBaselines:     pickupBaselines,
		StudentRepo:         repos.Student,
		PersonRepo:          repos.Person,
		EducationGroupRepo:  repos.Group,
		RoomRepo:            repos.Room,
		PickupService:       pickupScheduleService,
		ArrivalService:      arrivalScheduleService,
		ListExport:          listExportService,
		Settings:            settingsService,
		UserContext:         userContextService,
		Logger:              logger.With("service", "slot_lists"),
	})

	// Printable weekly plans (#2079). A pure projection over the same reads
	// the two planning screens use — it renders, it never writes.
	planExportService := planexport.NewService(planexport.Dependencies{
		Overview:       staffScheduleOverviewService,
		ShiftTypes:     repos.ShiftType,
		Instances:      repos.ActivityInstance,
		InstanceStaff:  repos.InstanceStaff,
		Students:       repos.InstanceStudent,
		Rooms:          repos.Room,
		Staff:          repos.Staff,
		ActivityGroups: repos.ActivityGroup,
		PlanningTracks: repos.PlanningTrack,
		ClosingDays:    closingDayService,
		Holidays:       holidayService,
		Renderer:       listExportService,
	}, logger.With("service", "plan_export"))

	pushSubscriptionsService := notifications.NewPushSubscriptionService(
		db,
		repos.PushSubscription,
		repos.AccountTenant,
		vapidConfig,
		logger.With("service", "push_subscriptions"),
	)

	pwaUsageService := pwa.NewUsageService(
		db,
		repos.PWAStandaloneUsage,
		repos.OperatorSummaries,
		repos.AccountTenant,
		settingsService,
		logger.With("service", "pwa_usage"),
	)

	absenceNotifier := notifications.NewAbsenceNotifier(
		notificationsService,
		staffNotificationRecipients,
		db,
		logger.With("producer", "absence_notifications"),
	)
	// Injected after the fact: the parent service is wired before the
	// notification stack exists.
	if setter, ok := parentService.(parent.AbsenceNotifierSetter); ok {
		setter.SetAbsenceNotifier(absenceNotifier)
	}
	if setter, ok := excusedRequestService.(absence.AbsenceNotifierSetter); ok {
		setter.SetAbsenceNotifier(absenceNotifier)
	}

	remindersService := reminders.NewService(reminders.Dependencies{
		Settings:    settingsService,
		Attendance:  repos.Attendance,
		Pickup:      pickupScheduleService,
		Instance:    repos.ActivityInstance,
		Room:        repos.Room,
		Student:     repos.Student,
		Person:      repos.Person,
		Supervision: activeService,
		Logger:      logger.With("service", "reminders"),

		// Bulk readers for ComputeBatch. They answer the three genuinely
		// per-person facts for the whole tenant in one query each, which is what
		// keeps the per-minute cost flat in the number of staff.
		BulkSupervision:   repos.GroupSupervisor,
		BulkVisits:        repos.ActiveVisit,
		BulkInstanceStaff: repos.InstanceStaff,
	})

	workTimeModelService := config.NewWorkTimeModelService(repos.WorkTimeModel)
	workTimeModelService.SetChangeNotifier(func(ctx context.Context) {
		realtime.QueueStaffTimeTrackingChanged(ctx, realtimeHub, nil)
	})
	studentStatusDayService := active.NewStudentStatusDayServiceWithPartialAbsences(
		repos.StudentStatusDay,
		repos.StudentPickupException,
		db,
		now,
	)
	studentStatusDayOverviewService := active.NewStudentStatusDayOverviewService(repos.StudentStatusDay, usersService)
	ogsGroupLiveService := ogsgrouplive.NewService(ogsgrouplive.Dependencies{
		People:            usersService,
		Education:         educationService,
		Substitutions:     substitutionService,
		UserContext:       userContextService,
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

	supervisionDashboardService := supervisiondashboard.NewService(supervisiondashboard.Dependencies{
		Active:      activeService,
		UserContext: userContextService,
		Education:   educationService,
		Schulhof:    schulhofService,
		Operations:  timetableOperationsService,
		Settings:    settingsService,
		Pickups:     pickupScheduleService,
		Arrivals:    arrivalScheduleService,
	})

	timetableDataService := schedule.NewTimetableDataService(schedule.TimetableDataDependencies{
		InstanceStudentRepo:        repos.InstanceStudent,
		ActivityInstanceRepo:       repos.ActivityInstance,
		ActivityExceptionRepo:      repos.ActivityException,
		ActivityScheduleRepo:       repos.ActivitySchedule,
		InstanceStaffRepo:          repos.InstanceStaff,
		StaffShiftRepo:             repos.StaffShift,
		StaffRepo:                  repos.Staff,
		CalendarPeriodRepo:         repos.CalendarPeriod,
		ActiveGroupRepo:            repos.ActiveGroup,
		SupervisorRepo:             repos.GroupSupervisor,
		ArrivalScheduleRepo:        repos.StudentArrivalSchedule,
		ArrivalBaselines:           arrivalBaselines,
		ArrivalExceptionRepo:       repos.StudentArrivalException,
		PickupScheduleRepo:         repos.StudentPickupSchedule,
		PickupBaselines:            pickupBaselines,
		PickupExceptionRepo:        repos.StudentPickupException,
		VisitRepo:                  repos.ActiveVisit,
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
		ConflictAckRepo:            repos.TimetableConflictAck,
		RecoveryRepo:               recoveryRepo,
		Broadcaster:                realtimeHub,
		Logger:                     logger.With("service", "timetable-data"),
		DB:                         db,
		Today:                      today,
	})
	instanceSeriesConverter := schedule.NewInstanceSeriesConversionService(schedule.InstanceSeriesConversionDependencies{
		DB:              db,
		InstanceRepo:    repos.ActivityInstance,
		InstanceService: instanceService,
		TimetableData:   timetableDataService,
	})
	masterDataReviewService := users.NewMasterDataReviewServiceWithAuditAndPolicy(
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
	// Wired by setter, and deliberately AFTER all four services and
	// parentService exist: the sharing rules live in the parents domain, and
	// the four request domains must not import it just to ask who a request
	// was shared with.
	if resolver, ok := parentService.(parentmessaging.ShareVisibilityResolver); ok {
		for _, service := range []any{
			excusedRequestService,
			careRequestService,
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

	parentRequestCoordinator := users.NewParentRequestCoordinator(
		masterDataReviewService.(users.MasterDataBulkReviewPort),
		excusedRequestService,
	)
	// Conflict-resolution ports (#2267, stories 6-10) — injected by setter, so
	// adding a domain to the resolver never rewrites the bulk-approval
	// constructor above. All five request kinds are wired here or the resolve
	// route answers conflict_kind_unsupported for the missing one.
	parentRequestCoordinator.SetMasterDataConflictPort(masterDataReviewService.(users.ParentRequestConflictPort))
	parentRequestCoordinator.SetExcusedConflictPort(excusedRequestService.(users.ParentRequestConflictPort))
	parentRequestCoordinator.SetCareConflictPort(careRequestService.(users.ParentRequestConflictPort))
	parentRequestCoordinator.SetOfferingConflictPort(offeringChangeRequestService.(users.ParentRequestConflictPort))
	// The resolver records ONLY the staff-entered result. Every verdict it
	// takes goes through a domain Decide, which writes its own decided event.
	parentRequestCoordinator.SetEventRecorder(parentRequestEvents)
	familyProtectionService := users.NewFamilyProtectionService(repos.FamilyProtection, repos.Student)

	factory := &Factory{
		settingsRuntimeDB:        db,
		Auth:                     authService,
		StaffPINAuth:             authService,
		MFA:                      mfaService,
		Passkey:                  passkeyService,
		Active:                   activeService,
		ActiveCleanup:            activeCleanupService,
		WorkSession:              workSessionService,
		WorkTimeMonth:            workTimeMonthService,
		Holidays:                 holidayService,
		ClosingDays:              closingDayService,
		StaffAbsence:             staffAbsenceService,
		StaffAbsenceType:         staffAbsenceTypeService,
		StaffBalanceAdjust:       staffBalanceAdjustService,
		StaffMonthClose:          staffMonthCloseService,
		StaffOverview:            staffOverviewService,
		TimeTrackingAuditLog:     timeTrackingAuditLogService,
		StaffTimeExport:          staffTimeExportService,
		Activities:               activitiesService,
		Education:                educationService,
		Substitution:             substitutionService,
		GradeTransition:          gradeTransitionService,
		Facilities:               facilitiesService,
		Schulhof:                 schulhofService,
		WC:                       wcService,
		Feedback:                 feedbackService,
		MealPlan:                 mealPlanService,
		IoT:                      iotService,
		Checkin:                  checkinService,
		StaffClock:               staffClockService,
		Settings:                 settingsService,
		PayrollStatus:            payrollStatusService,
		Schedule:                 scheduleService,
		StaffShifts:              staffShiftService,
		StaffShiftSeries:         staffShiftSeriesService,
		StaffAssignments:         staffAssignmentService,
		StaffScheduleOverview:    staffScheduleOverviewService,
		ShiftTypes:               shiftTypeService,
		PlanningTracks:           planningTrackService,
		PickupSchedule:           pickupScheduleService,
		PartialAbsence:           partialAbsenceService,
		Display:                  displayService,
		ArrivalSchedule:          arrivalScheduleService,
		CareDay:                  careDayService,
		TimetableBridge:          timetableBridgeService,
		CalendarPeriod:           calendarPeriodService,
		Materialization:          materializationService,
		TemplateSplit:            templateSplitService,
		TimetableCleanup:         timetableCleanupService,
		TimeTrackingCleanup:      timeTrackingCleanupService,
		StudentChangeLogCleanup:  studentChangeLogCleanupService,
		Instance:                 instanceService,
		AutoStart:                autoStartService,
		AutoEnd:                  autoEndService,
		TimetableOperations:      timetableOperationsService,
		Users:                    usersService,
		Birthdays:                birthdayService,
		StaffDocuments:           staffDocumentService,
		StudentDocuments:         studentDocumentService,
		FileStore:                fileStoreService,
		StaffOffboarding:         staffOffboardingService,
		CaregiverCapability:      caregiverCapabilityService,
		Guardian:                 guardianService,
		GuardianProfileLoader:    guardianProfileLoader,
		UserContext:              userContextService,
		Database:                 databaseService,
		Import:                   studentImportService,        // Student import service
		StaffImport:              staffImportService,          // Staff (Mitarbeiter) import service
		ClassListImport:          classListImportService,      // Class-list entry import (#2382)
		OpeningBalanceImport:     openingBalanceImportFactory, // Opening balance import (#2132)
		ListExport:               listExportService,
		PlanExport:               planExportService,
		Emergency:                emergencyService,
		SlotLists:                slotListsService,
		Reminders:                remindersService,
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

		// Platform services - OperatorAuth and OperatorInvitation both point
		// at the same concrete operatorAuthService struct, exposed through
		// two narrower interfaces so that each handler depends only on the
		// methods it actually calls. NewOperatorAuthService returns the
		// combined interface, so both fields can be assigned directly.
		OperatorAuth:         operatorAuthService,
		OperatorInvitation:   operatorAuthService,
		OperatorProvisioning: operatorProvisioningService,
		Announcement:         announcementService,
		Schools:              platform.NewSchoolService(repos.School),
		WorkTimeModels:       workTimeModelService,
		Students:             studentService,
		ClassListEntries:     users.NewClassListEntryService(repos.ClassListEntry, repos.Student, repos.ClassListEntryChange),
		StudentDeletion:      studentDeletionService,
		CareLifecycle:        careLifecycleService,
		StudentAudit:         studentAuditService,
		MasterDataReview:     masterDataReviewService,
		CareRequests:         careRequestService,
		OfferingChanges:      offeringChangeRequestService,
		PickupAdjustments:    pickupAdjustmentService,
		ExcusedRequests:      excusedRequestService,
		ParentRequests:       parentRequestCoordinator,
		FamilyProtection:     familyProtectionService,
		RequestReviewPolicy:  requestReviewPolicy,
		StudentStatusDays:    studentStatusDayService,
		AbsenceOverview:      studentStatusDayOverviewService,
		StudentHistory:       active.NewStudentHistoryService(repos.Attendance, repos.ActiveVisit, repos.DataAccessLog, repos.InstanceStudent),
		Statistics: statistics.NewService(statistics.Config{
			Statistics:      repos.Statistics,
			Holidays:        holidayService,
			ClosingDays:     closingDayService,
			Periods:         repos.CalendarPeriod,
			Students:        repos.Student,
			Rooms:           repos.Room,
			AccessLog:       repos.DataAccessLog,
			Settings:        settingsService,
			PrivacyConsents: repos.PrivacyConsent,
			Logger:          logger.With("service", "statistics"),
			Now:             now,
		}),
		OGSGroupLive:            ogsGroupLiveService,
		SupervisionDashboard:    supervisionDashboardService,
		TimetableData:           timetableDataService,
		InstanceSeriesConverter: instanceSeriesConverter,
		OperatorMFA:             operatorMFAService,
		OperatorPasskey:         operatorPasskeyService,
		UnregisteredTagScans:    unregisteredTagScanService,

		EmailOutbox:           emailOutboxService,
		EmailOutboxWorker:     emailOutboxWorker,
		EmailTemplateRegistry: emailTemplateRegistry,

		EnrollmentFormSchema:      enrollmentFormSchemaService,
		EnrollmentCareOffering:    enrollmentCareOfferingService,
		EnrollmentCaptcha:         enrollmentCaptchaService,
		EnrollmentRequest:         enrollmentRequestService,
		EnrollmentPhase:           enrollmentPhaseService,
		EnrollmentPhaseExpiry:     enrollmentPhaseExpiryService,
		EnrollmentDecision:        enrollmentDecisionService,
		EnrollmentReport:          enrollmentReportService,
		EnrollmentRollover:        enrollmentRolloverService,
		EnrollmentChangeRequest:   enrollmentChangeRequestService,
		EnrollmentDeletion:        enrollmentDeletionService,
		EnrollmentRejectedCleanup: enrollmentRejectedCleanupService,

		Parent:              parentService,
		Messaging:           messagingService,
		StaffMessaging:      staffMessagingService,
		StaffNotice:         staffNoticeService,
		Calendar:            calendarSvc,
		CalendarFeedCleanup: calendarSvc,
		ParentAnnouncement:  parentAnnouncementService,
		ParentEventEmitter:  pillEmitter,
	}

	factory.SettingsSideEffects = sideeffects.NewRegistry()
	facilities.RegisterSettingsSideEffects(factory.SettingsSideEffects, schulhofService, wcService)
	users.RegisterCareWithdrawalSettingsSideEffects(factory.SettingsSideEffects, careLifecycleService)
	tenantSettings := config.NewTenantOperations(
		settingsService,
		payrollStatusService,
		settingsRuntime,
		factory.SettingsSideEffects.Dispatch,
		func(_ context.Context, tenantID int64, key string) {
			event := realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key})
			_ = realtimeHub.BroadcastToTenant(tenantID, event)
		},
	)
	factory.TenantSettings = tenantSettings

	// #1843 sick cascade: setter-injected after assembly because the syncer
	// (services/schedule) needs the schedule services while the absence
	// service (services/active) is constructed long before them.
	staffAbsenceService.SetShiftPlanSyncer(schedule.NewShiftPlanSyncService(
		staffShiftService,
		instanceService,
		factory.TimetableData,
		repos.StaffShift,
		repos.InstanceStaff,
		factory.RealtimeHub,
		db,
		logger.With("service", "shift_plan_sync"),
		today,
	))
	return factory, nil
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

// StudentPhotoBootstrap aggregates the dependencies api/base.go must
// provide to wire the photo lifecycle. The unlinker is api-layer (file IO
// shared with login-image/avatar upload helpers); the StudentRepo is
// passed in to avoid storing the repo factory on the services Factory.
type StudentPhotoBootstrap struct {
	Unlinker    users.PhotoUnlinker
	StudentRepo userModels.StudentRepository
	DB          *bun.DB
	Logger      *slog.Logger
}

// EnableStudentPhotos constructs the StudentPhotoService with the supplied
// dependencies and registers its settings handler on
// f.SettingsSideEffects. Idempotent: repeated calls overwrite the prior
// service. Call once at API bootstrap.
func (f *Factory) EnableStudentPhotos(deps StudentPhotoBootstrap) {
	f.StudentPhotos = users.NewStudentPhotoService(users.StudentPhotoServiceDependencies{
		StudentRepo: deps.StudentRepo,
		Settings:    f.Settings,
		UserContext: f.UserContext,
		Broadcaster: f.RealtimeHub,
		Unlinker:    deps.Unlinker,
		DB:          deps.DB,
		Logger:      deps.Logger,
	})
	users.RegisterStudentPhotoSettingsSideEffects(f.SettingsSideEffects, f.StudentPhotos)
}
