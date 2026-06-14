// Package services provides service layer implementations
package services

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policies"
	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/email"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/activities"
	auditService "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
	_ "github.com/moto-nrw/project-phoenix/services/config/defaults"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
	"github.com/moto-nrw/project-phoenix/services/database"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/emergency"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/feedback"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/suggestions"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// Factory provides access to all services
type Factory struct {
	Auth                     auth.AuthService
	MFA                      auth.MFAService
	Active                   active.Service
	ActiveCleanup            active.CleanupService
	WorkSession              active.WorkSessionService
	StaffAbsence             active.StaffAbsenceService
	Activities               activities.ActivityService
	Education                education.Service
	GradeTransition          education.GradeTransitionService
	Facilities               facilities.Service
	Schulhof                 facilities.SchulhofService
	WC                       facilities.WCService
	Invitation               auth.InvitationService
	GuardianInvitation       auth.GuardianInvitationService
	Feedback                 feedback.Service
	Suggestions              suggestions.Service
	IoT                      iot.Service
	Settings                 config.SettingsService
	Schedule                 schedule.Service
	PickupSchedule           schedule.PickupScheduleService
	ArrivalSchedule          schedule.ArrivalScheduleService
	CalendarPeriod           schedule.CalendarPeriodService
	Materialization          schedule.MaterializationService
	TimetableCleanup         schedule.TimetableCleanupService
	TimeTrackingCleanup      active.TimeTrackingCleanupService
	Instance                 schedule.InstanceService
	AutoStart                schedule.AutoStartService
	TimetableOperations      schedule.TimetableOperationsService
	Users                    users.PersonService
	StaffOffboarding         users.StaffOffboardingService
	CaregiverCapability      users.CaregiverCapabilityService
	Guardian                 users.GuardianService
	GuardianProfileLoader    users.GuardianProfileLoader
	UserContext              usercontext.UserContextService
	Database                 database.DatabaseService
	Import                   *importService.ImportService[importModels.StudentImportRow] // Student import service
	StaffImport              *importService.ImportService[importModels.StaffImportRow]   // Staff (Mitarbeiter) import service
	ListExport               listexport.Service
	Emergency                emergency.Service
	RealtimeHub              *realtime.Hub // SSE event hub (shared by services and API)
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
	WorkTimeModels       config.WorkTimeModelService
	Students             users.StudentService
	StudentStatusDays    active.StudentStatusDayService
	StudentHistory       active.StudentHistoryService
	TimetableData        schedule.TimetableDataService
	OperatorSuggestions  platform.OperatorSuggestionsService
	OperatorMFA          platform.OperatorMFAService
	UnregisteredTagScans auditService.UnregisteredTagScanService

	// Email outbox (parent-enrollment PR 5) - shared across features.
	// EmailOutbox enqueues from feature code; EmailOutboxWorker drains
	// the table on a scheduler tick; EmailTemplateRegistry holds the
	// kind→Renderer mapping populated at startup.
	EmailOutbox           *platform.OutboxService
	EmailOutboxWorker     *platform.OutboxWorker
	EmailTemplateRegistry *platform.TemplateRegistry

	// Enrollment domain (parent-enrollment PR 5+).
	EnrollmentFormSchema   enrollment.FormSchemaService
	EnrollmentCareOffering enrollment.CareOfferingService
	EnrollmentCaptcha      enrollment.CaptchaService
	EnrollmentRequest      enrollment.RequestService
	EnrollmentPhase        enrollment.PhaseService
	EnrollmentDecision     enrollment.DecisionService
	EnrollmentRollover     enrollment.RolloverService

	// Parent (cross-tenant guardian portal - PR 9)
	Parent parent.Service

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

// NewFactory creates a new services factory
func NewFactory(repos *repositories.Factory, db *bun.DB, logger *slog.Logger) (*Factory, error) {

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

	// Initialize education service first (needed for active service)
	educationService := education.NewService(
		repos.Group,
		repos.GroupTeacher,
		repos.GroupSubstitution,
		repos.Room,
		repos.Teacher,
		repos.Staff,
		repos.Student,
	)

	// Initialize grade transition service
	gradeTransitionService := education.NewGradeTransitionService(education.GradeTransitionServiceDependencies{
		TransitionRepo: repos.GradeTransition,
		StudentRepo:    repos.Student,
		PersonRepo:     repos.Person,
		DB:             db,
	})

	// Initialize settings service (new schema-driven settings system)
	settingsService := config.NewSettingsService(
		repos.SettingValue,
		repos.SettingAudit,
		repos.School,
		db,
		logger,
	)

	// Initialize users service first (needed for active service)
	usersService := users.NewPersonService(users.PersonServiceDependencies{
		PersonRepo:         repos.Person,
		RFIDRepo:           repos.RFIDCard,
		AccountRepo:        repos.Account,
		PersonGuardianRepo: repos.PersonGuardian,
		StudentRepo:        repos.Student,
		StaffRepo:          repos.Staff,
		TeacherRepo:        repos.Teacher,
		DB:                 db,
		SettingsService:    settingsService,
		Logger:             logger.With("service", "users"),
	})

	// Initialize guardian service
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
		Mailer:                  mailer,
		Dispatcher:              dispatcher,
		FrontendURL:             frontendURL,
		DefaultFrom:             defaultFrom,
		InvitationExpiry:        invitationTokenExpiry,
		DB:                      db,
	})

	// Thin loader that backs the public + parent enrollment me/profile
	// endpoints. Pulls the multi-schema join out of the handlers (Rule 1)
	// and into the existing GuardianProfileRepository.LoadProfileWithChildren.
	guardianProfileLoader := users.NewGuardianProfileLoader(repos.GuardianProfile, db, logger.With("service", "guardian-profile-loader"))

	// Initialize work session service (before active service - needed for NFC auto-check-in)
	workSessionService := active.NewWorkSessionService(repos.WorkSession, repos.WorkSessionBreak, repos.WorkSessionEdit, repos.StaffAbsence, repos.GroupSupervisor, repos.Staff, repos.StaffWorkSchedule, repos.WorkTimeModel, activeLogger)

	// Initialize staff absence service
	staffAbsenceService := active.NewStaffAbsenceService(repos.StaffAbsence, repos.WorkSession, repos.StaffVacationQuota, repos.StaffAbsenceAudit)

	// Initialize attendance sync service (WP-B10). Implements
	// active.AttendanceSyncer - called from CreateVisit / EndVisit to mirror
	// into schedule.instance_students and enrich SSE events. No circular
	// dependency because it only depends on repos, not on active.Service.
	attendanceSyncService := schedule.NewAttendanceSyncService(
		repos.ActivityInstance,
		repos.InstanceStudent,
		logger.With("service", "attendance-sync"),
	)

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
		WorkSessionService:       workSessionService,    // NFC auto-check-in
		AttendanceSyncer:         attendanceSyncService, // WP-B10 mirror + SSE enrichment
		TimetableBridgeCompleter: repos.ActivityInstance,
		Logger:                   activeLogger,
	})

	// Initialize feedback service
	feedbackService := feedback.NewService(
		repos.FeedbackEntry,
	)

	// Initialize suggestions service
	suggestionsNotifyEmail := viper.GetString("suggestion_notify_email")
	suggestionsService := suggestions.NewService(suggestions.ServiceConfig{
		PostRepo:        repos.SuggestionPost,
		VoteRepo:        repos.SuggestionVote,
		CommentRepo:     repos.SuggestionComment,
		CommentReadRepo: repos.SuggestionCommentRead,
		DB:              db,
		Dispatcher:      dispatcher,
		DefaultFrom:     defaultFrom,
		NotifyEmail:     suggestionsNotifyEmail,
		FrontendURL:     frontendURL,
		Logger:          logger.With("service", "suggestions"),
	})

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
	)
	if err != nil {
		return nil, err
	}

	// Initialize facilities service
	facilitiesService := facilities.NewService(
		repos.Room,
		repos.ActiveGroup,
	)

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
	scheduleService := schedule.NewService(
		repos.Dateframe,
		repos.Timeframe,
		repos.RecurrenceRule,
	)

	// Initialize pickup schedule service
	pickupScheduleService := schedule.NewPickupScheduleService(
		repos.StudentPickupSchedule,
		repos.StudentPickupException,
		repos.StudentPickupNote,
	)

	// Initialize calendar period service
	calendarPeriodService := schedule.NewCalendarPeriodService(
		repos.CalendarPeriod,
		logger.With("service", "calendar-period"),
	)

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
		logger.With("service", "materialization"),
	)

	// Initialize timetable GDPR cleanup service (WP-B14). Deletes
	// schedule.activity_instances (CASCADE → instance_staff + instance_students)
	// and schedule.activity_exceptions older than the tenant's retention window.
	// Per-student audit rows via DataDeletion; exceptions slog-only.
	timetableCleanupService := schedule.NewTimetableCleanupService(
		repos.ActivityInstance,
		repos.ActivityException,
		repos.InstanceStudent,
		repos.DataDeletion,
		settingsService,
		logger.With("service", "timetable-cleanup"),
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

	// Initialize instance lifecycle service (WP-B9). Drives the state machine
	// on schedule.activity_instances and its bridge to active.groups. Takes
	// the active service as a dependency (for EndActivitySession) - when the
	// bridge closes, visits + supervisors close and per-student checkout SSE
	// events fire, matching today's observable behavior for a session ending.
	instanceService := schedule.NewInstanceService(schedule.InstanceServiceDependencies{
		InstanceRepo:      repos.ActivityInstance,
		InstanceStaffRepo: repos.InstanceStaff,
		InstanceStudents:  repos.InstanceStudent,
		ActiveGroupRepo:   repos.ActiveGroup,
		SupervisorRepo:    repos.GroupSupervisor,
		VisitRepo:         repos.ActiveVisit,
		RoomRepo:          repos.Room,
		ActivityGroupRepo: repos.ActivityGroup,
		StaffRepo:         repos.Staff,
		StudentRepo:       repos.Student,
		ActiveService:     activeService,
		Materialization:   materializationService,
		Broadcaster:       realtimeHub,
		DB:                db,
		Logger:            logger.With("service", "instance-lifecycle"),
	})

	autoStartService := schedule.NewAutoStartService(schedule.AutoStartDependencies{
		InstanceRepo:      repos.ActivityInstance,
		InstanceStaffRepo: repos.InstanceStaff,
		InstanceStudents:  repos.InstanceStudent,
		InstanceService:   instanceService,
		ActiveGroupRepo:   repos.ActiveGroup,
		SupervisorRepo:    repos.GroupSupervisor,
		VisitRepo:         repos.ActiveVisit,
		Logger:            logger.With("service", "timetable-auto-start"),
	})

	// Initialize arrival schedule service
	arrivalScheduleService := schedule.NewArrivalScheduleService(
		repos.StudentArrivalSchedule,
		repos.StudentArrivalException,
		repos.StudentArrivalNote,
		repos.Student,
		repos.Person,
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
		SupervisorRepo:     repos.GroupSupervisor,
		VisitRepo:          repos.ActiveVisit,
		StudentRepo:        repos.Student,
		EducationGroupRepo: repos.Group,
		RoomRepo:           repos.Room,
		PersonService:      usersService,
		Settings:           settingsService,
		Broadcaster:        realtimeHub,
		DB:                 db,
		Logger:             logger.With("service", "timetable-operations"),
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

	invitationService := auth.NewInvitationService(auth.InvitationServiceConfig{
		InvitationRepo:    repos.InvitationToken,
		AccountRepo:       repos.Account,
		AccountTenantRepo: repos.AccountTenant,
		RoleRepo:          repos.Role,
		AccountRoleRepo:   repos.AccountRole,
		PersonRepo:        repos.Person,
		StaffRepo:         repos.Staff,
		TeacherRepo:       repos.Teacher,
		SchoolRepo:        repos.School,
		Mailer:            mailer,
		Dispatcher:        dispatcher,
		FrontendURL:       frontendURL,
		DefaultFrom:       defaultFrom,
		InvitationExpiry:  invitationTokenExpiry,
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

	guardianInvitationService := auth.NewGuardianInvitationService(auth.GuardianInvitationServiceConfig{
		InvitationRepo:       repos.GuardianInvitation,
		AccountRepo:          repos.Account,
		AccountTenantRepo:    repos.AccountTenant,
		AccountRoleRepo:      repos.AccountRole,
		RoleRepo:             repos.Role,
		PersonRepo:           repos.Person,
		GuardianProfileRepo:  repos.GuardianProfile,
		SchoolRepo:           repos.School,
		Mailer:               mailer,
		Dispatcher:           dispatcher,
		OutboxEnqueuer:       platform.NewAuthOutboxAdapter(emailOutboxService),
		EnrollmentBackfiller: repos.ParentEnrollmentRequest,
		SettingsResolver:     settingsService,
		FrontendURL:          parentsURL, // accept link goes to the parents portal, not the staff frontend
		DefaultFrom:          defaultFrom,
		FallbackExpiry:       invitationTokenExpiry,
		DB:                   db,
		Logger:               authLogger.With("flow", "guardian_invitation"),
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
		platformModels.EmailKindEnrollmentSubmitted,
		platform.RendererFunc(enrollment.NewEnrollmentSubmittedRenderer(enrollment.EmailRendererConfig{
			DefaultFrom: defaultFrom,
		})),
	)
	emailTemplateRegistry.Register(
		platformModels.EmailKindEnrollmentAdminNotify,
		platform.RendererFunc(enrollment.NewEnrollmentAdminNotificationRenderer(enrollment.EmailRendererConfig{
			DefaultFrom: defaultFrom,
		})),
	)
	// Per-status decision emails dispatched by the DecisionService
	// (PR 8 slice 2). One renderer per kind keeps subjects + templates
	// independent and makes future copy updates contained.
	emailTemplateRegistry.Register(
		platformModels.EmailKindEnrollmentApproved,
		platform.RendererFunc(enrollment.NewEnrollmentApprovedRenderer(enrollment.EmailRendererConfig{
			DefaultFrom: defaultFrom,
		})),
	)
	emailTemplateRegistry.Register(
		platformModels.EmailKindEnrollmentWaitlisted,
		platform.RendererFunc(enrollment.NewEnrollmentWaitlistedRenderer(enrollment.EmailRendererConfig{
			DefaultFrom: defaultFrom,
		})),
	)
	emailTemplateRegistry.Register(
		platformModels.EmailKindEnrollmentRejected,
		platform.RendererFunc(enrollment.NewEnrollmentRejectedRenderer(enrollment.EmailRendererConfig{
			DefaultFrom: defaultFrom,
		})),
	)
	// Rollover (annual phase renewal) emails. Slice 1 reuses the
	// submission template as a placeholder. Proper branded copy lands
	// in a follow-up PR.
	emailTemplateRegistry.Register(
		platformModels.EmailKindEnrollmentRolloverOptIn,
		platform.RendererFunc(enrollment.NewEnrollmentRolloverOptInRenderer(enrollment.EmailRendererConfig{
			DefaultFrom: defaultFrom,
		})),
	)
	emailTemplateRegistry.Register(
		platformModels.EmailKindEnrollmentRolloverOptOut,
		platform.RendererFunc(enrollment.NewEnrollmentRolloverOptOutRenderer(enrollment.EmailRendererConfig{
			DefaultFrom: defaultFrom,
		})),
	)

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
		GroupSubstitutionRepo:  repos.GroupSubstitution,
		ActivitySupervisorRepo: repos.ActivitySupervisor,
		InstanceStaffRepo:      repos.InstanceStaff,
		StaffAbsenceRepo:       repos.StaffAbsence,
		AccountTenantRepo:      repos.AccountTenant,
		RoleRepo:               repos.Role,
		AccountPermissionRepo:  repos.AccountPermission,
		DataDeletionRepo:       repos.DataDeletion,
		AuthService:            authService,
		DB:                     db,
		Logger:                 logger.With("service", "staff_offboarding"),
	})

	// Initialize authorization
	authorizationService := authorize.NewAuthorizationService()

	// Create policy registry
	policyRegistry := policies.NewPolicyRegistry(
		educationService,
		usersService,
		activeService,
	)

	// Register all policies
	if err := policyRegistry.RegisterAll(authorizationService); err != nil {
		return nil, err
	}

	// Set global resource authorizer
	authorize.SetResourceAuthorizer(
		authorize.NewResourceAuthorizer(authorizationService),
	)

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
	}, usercontextLogger)

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
			Resolver:            relationshipResolver,
		},
		db,
	)
	studentImportService := importService.NewImportService(studentImportConfig)
	studentImportService.SetAuditRepository(repos.DataImport)

	// Staff import bulk-creates invitations (reuses the invitation service);
	// Person/Account/Staff/Teacher are created when each invitee accepts.
	staffImportConfig := importService.NewStaffImportConfig(
		importService.StaffImportDeps{
			InvitationService: invitationService,
			AccountRepo:       repos.Account,
			AccountTenantRepo: repos.AccountTenant,
			RoleRepo:          repos.Role,
			SchoolRepo:        repos.School,
		},
	)
	staffImportService := importService.NewImportService(staffImportConfig)
	staffImportService.SetAuditRepository(repos.DataImport)

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

	// Initialize platform services (operator dashboard)
	operatorAuthService, err := platform.NewOperatorAuthService(platform.OperatorAuthServiceConfig{
		OperatorRepo:         repos.Operator,
		AuditLogRepo:         repos.OperatorAuditLog,
		EmailChangeTokenRepo: repos.OperatorEmailChangeToken,
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

	announcementService := platform.NewAnnouncementService(platform.AnnouncementServiceConfig{
		AnnouncementRepo:     repos.Announcement,
		AnnouncementViewRepo: repos.AnnouncementView,
		AuditLogRepo:         repos.OperatorAuditLog,
		OrgRepo:              repos.Organization,
		SchoolRepo:           repos.School,
		DB:                   db,
		Logger:               platformLogger,
	})

	operatorSuggestionsService := platform.NewOperatorSuggestionsService(platform.OperatorSuggestionsServiceConfig{
		PostRepo:        repos.SuggestionPost,
		CommentRepo:     repos.SuggestionComment,
		CommentReadRepo: repos.SuggestionCommentRead,
		PostReadRepo:    repos.SuggestionPostRead,
		AuditLogRepo:    repos.OperatorAuditLog,
		DB:              db,
		Logger:          platformLogger,
	})

	enrollmentFormSchemaService := enrollment.NewFormSchemaService(enrollment.FormSchemaServiceConfig{
		Repo:        repos.FormSchema,
		PhaseRepo:   repos.Phase,
		RequestRepo: repos.Request,
		Logger:      logger.With("service", "enrollment-form-schema"),
	})

	enrollmentCareOfferingService := enrollment.NewCareOfferingService(enrollment.CareOfferingServiceConfig{
		Repo:                 repos.CareOffering,
		ActivityGroupRepo:    repos.ActivityGroup,
		ActivityScheduleRepo: repos.ActivitySchedule,
		CalendarPeriodRepo:   repos.CalendarPeriod,
		PhaseRepo:            repos.Phase,
		Logger:               logger.With("service", "enrollment-care-offering"),
	})

	enrollmentCaptchaService := enrollment.NewCaptchaService(enrollment.CaptchaServiceConfig{
		Settings: settingsService,
		Logger:   logger.With("service", "enrollment-captcha"),
	})

	enrollmentRequestService := enrollment.NewRequestService(enrollment.RequestServiceConfig{
		RequestRepo:              repos.Request,
		RequestChildRepo:         repos.RequestChild,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		CareOfferingRepo:         repos.CareOffering,
		FormSchemaRepo:           repos.FormSchema,
		PhaseRepo:                repos.Phase,
		SchoolRepo:               repos.School,
		RateLimitRepo:            repos.SubmissionRateLimit,
		OutboxEnqueuer:           platform.NewEnrollmentOutboxAdapter(emailOutboxService),
		Settings:                 settingsService,
		FrontendURL:              frontendURL, // admin notification email
		ParentsURL:               parentsURL,  // parent confirmation/status emails
		DB:                       db,
		Logger:                   logger.With("service", "enrollment-request"),
	})

	enrollmentPhaseService := enrollment.NewPhaseService(enrollment.PhaseServiceConfig{
		Repo:             repos.Phase,
		RequestRepo:      repos.Request,
		RequestChildRepo: repos.RequestChild,
		CareOfferingRepo: repos.CareOffering,
		DB:               db,
		Logger:           logger.With("service", "enrollment-phase"),
	})

	enrollmentDecisionService := enrollment.NewDecisionService(enrollment.DecisionServiceConfig{
		RequestRepo:              repos.Request,
		RequestChildRepo:         repos.RequestChild,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		CareOfferingRepo:         repos.CareOffering,
		PhaseRepo:                repos.Phase,
		FormSchemaRepo:           repos.FormSchema,
		DataAccessLogRepo:        repos.DataAccessLog,
		SchoolRepo:               repos.School,
		PersonRepo:               repos.Person,
		StudentRepo:              repos.Student,
		StudentGuardianRepo:      repos.StudentGuardian,
		GuardianProfileRepo:      repos.GuardianProfile,
		GuardianPhoneRepo:        repos.GuardianPhoneNumber,
		PickupScheduleRepo:       repos.StudentPickupSchedule,
		ArrivalScheduleRepo:      repos.StudentArrivalSchedule,
		StudentEnrollmentRepo:    repos.StudentEnrollment,
		ActivityGroupRepo:        repos.ActivityGroup,
		ActivityScheduleRepo:     repos.ActivitySchedule,
		CalendarPeriodRepo:       repos.CalendarPeriod,
		AccountRepo:              repos.Account,
		AccountTenantRepo:        repos.AccountTenant,
		AccountRoleRepo:          repos.AccountRole,
		RoleRepo:                 repos.Role,
		OutboxEnqueuer:           platform.NewEnrollmentOutboxAdapter(emailOutboxService),
		FrontendURL:              frontendURL,
		ParentsURL:               parentsURL,
		Settings:                 settingsService,
		Logger:                   logger.With("service", "enrollment-decision"),
	})

	// Rollover service depends on DecisionService for the
	// rollover_auto_approve=true deadline path.
	enrollmentRolloverService := enrollment.NewRolloverService(enrollment.RolloverServiceConfig{
		PhaseRepo:                repos.Phase,
		RequestRepo:              repos.Request,
		RequestChildRepo:         repos.RequestChild,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		SchoolRepo:               repos.School,
		OutboxEnqueuer:           platform.NewEnrollmentOutboxAdapter(emailOutboxService),
		Settings:                 settingsService,
		DecisionService:          enrollmentDecisionService,
		ParentsURL:               parentsURL,
		DB:                       db,
		Logger:                   logger.With("service", "enrollment-rollover"),
	})

	parentService := parent.NewService(parent.ServiceConfig{
		ChildRepo:             repos.ParentChild,
		EnrollablePhaseRepo:   repos.ParentEnrollablePhase,
		EnrollmentRequestRepo: repos.ParentEnrollmentRequest,
		GuardianProfileRepo:   repos.GuardianProfile,
		StatusDayRepo:         repos.StudentStatusDay,
		StudentRepo:           repos.Student,
		NoteRepo:              repos.StudentParentNote,
		Settings:              settingsService,
		Broadcaster:           realtimeHub,
		DB:                    db,
		Logger:                logger.With("service", "parent"),
	})

	operatorProvisioningService := platform.NewOperatorProvisioningService(platform.OperatorProvisioningServiceConfig{
		OrganizationRepo:    repos.Organization,
		SchoolRepo:          repos.School,
		SummariesRepo:       repos.OperatorSummaries,
		CategoryRepo:        repos.ActivityCategory,
		DeviceRepo:          repos.Device,
		RoleRepo:            repos.Role,
		AccountTenantRepo:   repos.AccountTenant,
		PersonRepo:          repos.Person,
		StaffRepo:           repos.Staff,
		AccountRepo:         repos.Account,
		TeacherRepo:         repos.Teacher,
		GroupSupervisorRepo: repos.GroupSupervisor,
		InvitationService:   invitationService,
		AuthService:         authService,
		AuditLogRepo:        repos.OperatorAuditLog,
		DB:                  db,
		Logger:              platformLogger,
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
	})

	factory := &Factory{
		Auth:                     authService,
		MFA:                      mfaService,
		Active:                   activeService,
		ActiveCleanup:            activeCleanupService,
		WorkSession:              workSessionService,
		StaffAbsence:             staffAbsenceService,
		Activities:               activitiesService,
		Education:                educationService,
		GradeTransition:          gradeTransitionService,
		Facilities:               facilitiesService,
		Schulhof:                 schulhofService,
		WC:                       wcService,
		Feedback:                 feedbackService,
		Suggestions:              suggestionsService,
		IoT:                      iotService,
		Settings:                 settingsService,
		Schedule:                 scheduleService,
		PickupSchedule:           pickupScheduleService,
		ArrivalSchedule:          arrivalScheduleService,
		CalendarPeriod:           calendarPeriodService,
		Materialization:          materializationService,
		TimetableCleanup:         timetableCleanupService,
		TimeTrackingCleanup:      timeTrackingCleanupService,
		Instance:                 instanceService,
		AutoStart:                autoStartService,
		TimetableOperations:      timetableOperationsService,
		Users:                    usersService,
		StaffOffboarding:         staffOffboardingService,
		CaregiverCapability:      caregiverCapabilityService,
		Guardian:                 guardianService,
		GuardianProfileLoader:    guardianProfileLoader,
		UserContext:              userContextService,
		Database:                 databaseService,
		Import:                   studentImportService, // Student import service
		StaffImport:              staffImportService,   // Staff (Mitarbeiter) import service
		ListExport:               listExportService,
		Emergency:                emergencyService,
		RealtimeHub:              realtimeHub, // Expose SSE hub for API layer
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
		WorkTimeModels:       config.NewWorkTimeModelService(repos.WorkTimeModel),
		Students:             users.NewStudentService(repos.Student, repos.PrivacyConsent, repos.StudentParentNote),
		StudentStatusDays:    active.NewStudentStatusDayService(repos.StudentStatusDay),
		StudentHistory:       active.NewStudentHistoryService(repos.Attendance, repos.ActiveVisit, repos.DataAccessLog),
		TimetableData: schedule.NewTimetableDataService(schedule.TimetableDataDependencies{
			InstanceStudentRepo:    repos.InstanceStudent,
			ActivityInstanceRepo:   repos.ActivityInstance,
			ActivityExceptionRepo:  repos.ActivityException,
			ActivityScheduleRepo:   repos.ActivitySchedule,
			InstanceStaffRepo:      repos.InstanceStaff,
			ActiveGroupRepo:        repos.ActiveGroup,
			SupervisorRepo:         repos.GroupSupervisor,
			ArrivalScheduleRepo:    repos.StudentArrivalSchedule,
			ArrivalExceptionRepo:   repos.StudentArrivalException,
			PickupScheduleRepo:     repos.StudentPickupSchedule,
			PickupExceptionRepo:    repos.StudentPickupException,
			VisitRepo:              repos.ActiveVisit,
			RoomRepo:               repos.Room,
			ActivityCategoryRepo:   repos.ActivityCategory,
			ActivityGroupRepo:      repos.ActivityGroup,
			ActivitySupervisorRepo: repos.ActivitySupervisor,
			StudentEnrollmentRepo:  repos.StudentEnrollment,
			TimeframeRepo:          repos.Timeframe,
			EducationGroupRepo:     repos.Group,
			DB:                     db,
		}),
		OperatorSuggestions:  operatorSuggestionsService,
		OperatorMFA:          operatorMFAService,
		UnregisteredTagScans: unregisteredTagScanService,

		EmailOutbox:           emailOutboxService,
		EmailOutboxWorker:     emailOutboxWorker,
		EmailTemplateRegistry: emailTemplateRegistry,

		EnrollmentFormSchema:   enrollmentFormSchemaService,
		EnrollmentCareOffering: enrollmentCareOfferingService,
		EnrollmentCaptcha:      enrollmentCaptchaService,
		EnrollmentRequest:      enrollmentRequestService,
		EnrollmentPhase:        enrollmentPhaseService,
		EnrollmentDecision:     enrollmentDecisionService,
		EnrollmentRollover:     enrollmentRolloverService,

		Parent: parentService,
	}

	factory.SettingsSideEffects = sideeffects.NewRegistry()
	facilities.RegisterSettingsSideEffects(factory.SettingsSideEffects, schulhofService, wcService)
	return factory, nil
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
