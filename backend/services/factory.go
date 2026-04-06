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
	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/email"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
	_ "github.com/moto-nrw/project-phoenix/services/config/defaults"
	"github.com/moto-nrw/project-phoenix/services/database"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/feedback"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/suggestions"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// Factory provides access to all services
type Factory struct {
	Auth                     auth.AuthService
	Active                   active.Service
	ActiveCleanup            active.CleanupService
	WorkSession              active.WorkSessionService
	StaffAbsence             active.StaffAbsenceService
	Activities               activities.ActivityService
	Education                education.Service
	GradeTransition          education.GradeTransitionService
	Facilities               facilities.Service
	Schulhof                 facilities.SchulhofService
	Invitation               auth.InvitationService
	Feedback                 feedback.Service
	Suggestions              suggestions.Service
	IoT                      iot.Service
	Config                   config.Service
	Settings                 config.SettingsService
	Schedule                 schedule.Service
	PickupSchedule           schedule.PickupScheduleService
	Users                    users.PersonService
	CaregiverCapability      users.CaregiverCapabilityService
	Guardian                 users.GuardianService
	UserContext              usercontext.UserContextService
	Database                 database.DatabaseService
	Import                   *importService.ImportService[importModels.StudentImportRow] // Student import service
	RealtimeHub              *realtime.Hub                                               // SSE event hub (shared by services and API)
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
	OperatorSuggestions  platform.OperatorSuggestionsService
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
		frontendURL = "http://localhost:3000"
	}

	appEnv := strings.ToLower(viper.GetString("app_env"))
	if appEnv == "production" && !strings.HasPrefix(frontendURL, "https://") {
		return nil, fmt.Errorf("FRONTEND_URL must use https:// in production (received %q)", rawFrontendURL)
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
		db,
	)

	// Initialize grade transition service
	gradeTransitionService := education.NewGradeTransitionService(education.GradeTransitionServiceDependencies{
		TransitionRepo: repos.GradeTransition,
		StudentRepo:    repos.Student,
		PersonRepo:     repos.Person,
		DB:             db,
	})

	// Initialize users service first (needed for active service)
	usersService := users.NewPersonService(users.PersonServiceDependencies{
		PersonRepo:          repos.Person,
		RFIDRepo:            repos.RFIDCard,
		AccountRepo:         repos.Account,
		PersonGuardianRepo:  repos.PersonGuardian,
		StudentRepo:         repos.Student,
		StaffRepo:           repos.Staff,
		TeacherRepo:         repos.Teacher,
		GroupSupervisorRepo: repos.GroupSupervisor,
		DB:                  db,
	})

	// Initialize guardian service
	guardianService := users.NewGuardianService(users.GuardianServiceDependencies{
		GuardianProfileRepo:     repos.GuardianProfile,
		GuardianPhoneNumberRepo: repos.GuardianPhoneNumber,
		StudentGuardianRepo:     repos.StudentGuardian,
		GuardianInvitationRepo:  repos.GuardianInvitation,
		AccountParentRepo:       repos.AccountParent,
		StudentRepo:             repos.Student,
		PersonRepo:              repos.Person,
		Mailer:                  mailer,
		Dispatcher:              dispatcher,
		FrontendURL:             frontendURL,
		DefaultFrom:             defaultFrom,
		InvitationExpiry:        invitationTokenExpiry,
		DB:                      db,
	})

	// Initialize work session service (before active service - needed for NFC auto-check-in)
	workSessionService := active.NewWorkSessionService(repos.WorkSession, repos.WorkSessionBreak, repos.WorkSessionEdit, repos.StaffAbsence, repos.GroupSupervisor, activeLogger)

	// Initialize staff absence service
	staffAbsenceService := active.NewStaffAbsenceService(repos.StaffAbsence, repos.WorkSession)

	// Initialize active service with SSE broadcaster
	activeService := active.NewService(active.ServiceDependencies{
		GroupRepo:          repos.ActiveGroup,
		VisitRepo:          repos.ActiveVisit,
		SupervisorRepo:     repos.GroupSupervisor,
		CombinedGroupRepo:  repos.CombinedGroup,
		GroupMappingRepo:   repos.GroupMapping,
		AttendanceRepo:     repos.Attendance,
		CrossTenantRepo:    activeRepo.NewCrossTenantRepository(db),
		StudentRepo:        repos.Student,
		PersonRepo:         repos.Person,
		TeacherRepo:        repos.Teacher,
		StaffRepo:          repos.Staff,
		RoomRepo:           repos.Room,
		ActivityGroupRepo:  repos.ActivityGroup,
		ActivityCatRepo:    repos.ActivityCategory,
		EducationGroupRepo: repos.Group,
		DeviceRepo:         repos.Device,
		EducationService:   educationService,
		UsersService:       usersService,
		DB:                 db,
		Broadcaster:        realtimeHub,        // Pass SSE broadcaster
		WorkSessionService: workSessionService, // NFC auto-check-in
		Logger:             activeLogger,
	})

	// Initialize feedback service
	feedbackService := feedback.NewService(
		repos.FeedbackEntry,
		db,
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
		db,
	)

	// Initialize config service
	configService := config.NewService(
		repos.Setting,
		db,
	)

	// Initialize settings service (new schema-driven settings system)
	settingsService := config.NewSettingsService(
		repos.SettingValue,
		repos.SettingAudit,
		db,
		logger,
	)

	// Initialize activities service
	activitiesService, err := activities.NewService(
		repos.ActivityCategory,
		repos.ActivityGroup,
		repos.ActivitySchedule,
		repos.ActivitySupervisor,
		repos.StudentEnrollment,
		repos.ActiveGroup,
		db,
	)
	if err != nil {
		return nil, err
	}

	// Initialize facilities service
	facilitiesService := facilities.NewService(
		repos.Room,
		repos.ActiveGroup,
		db,
	)

	// Initialize Schulhof service (depends on facilities, activities, and active services)
	schulhofService := facilities.NewSchulhofService(
		facilitiesService,
		activitiesService,
		activeService,
		db,
		facilitiesLogger,
	)

	// Initialize schedule service
	scheduleService := schedule.NewService(
		repos.Dateframe,
		repos.Timeframe,
		repos.RecurrenceRule,
		db,
	)

	// Initialize pickup schedule service
	pickupScheduleService := schedule.NewPickupScheduleService(
		repos.StudentPickupSchedule,
		repos.StudentPickupException,
		repos.StudentPickupNote,
		db,
	)

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
	authService, err := auth.NewService(repos, authConfig, db, authLogger)
	if err != nil {
		return nil, err
	}

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
		ActivitySupervisorRepo: repos.ActivitySupervisor,
		AuthService:            authService,
		DB:                     db,
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
	}, db, usercontextLogger)

	// Initialize database stats service
	databaseService := database.NewService(repos, databaseLogger)

	// Initialize cleanup service
	activeCleanupService := active.NewCleanupService(
		repos.ActiveVisit,
		repos.PrivacyConsent,
		repos.DataDeletion,
		db,
	)

	// Initialize import service
	relationshipResolver := importService.NewRelationshipResolver(repos.Group, repos.Room)
	studentImportConfig := importService.NewStudentImportConfig(
		importService.StudentImportDeps{
			PersonRepo:         repos.Person,
			StudentRepo:        repos.Student,
			GuardianRepo:       repos.GuardianProfile,
			GuardianPhoneRepo:  repos.GuardianPhoneNumber,
			RelationRepo:       repos.StudentGuardian,
			PrivacyRepo:        repos.PrivacyConsent,
			PickupScheduleRepo: repos.StudentPickupSchedule,
			Resolver:           relationshipResolver,
		},
		db,
	)
	studentImportService := importService.NewImportService(studentImportConfig, db)

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
	// signal. Constructed conditionally — only required when actually sending
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

	operatorProvisioningService := platform.NewOperatorProvisioningService(platform.OperatorProvisioningServiceConfig{
		OrganizationRepo:    repos.Organization,
		SchoolRepo:          repos.School,
		CategoryRepo:        repos.ActivityCategory,
		DeviceRepo:          repos.Device,
		RoleRepo:            repos.Role,
		AccountTenantRepo:   repos.AccountTenant,
		PersonRepo:          repos.Person,
		StaffRepo:           repos.Staff,
		TeacherRepo:         repos.Teacher,
		GroupSupervisorRepo: repos.GroupSupervisor,
		InvitationService:   invitationService,
		AuthService:         authService,
		AuditLogRepo:        repos.OperatorAuditLog,
		DB:                  db,
		Logger:              platformLogger,
	})

	return &Factory{
		Auth:                     authService,
		Active:                   activeService,
		ActiveCleanup:            activeCleanupService,
		WorkSession:              workSessionService,
		StaffAbsence:             staffAbsenceService,
		Activities:               activitiesService,
		Education:                educationService,
		GradeTransition:          gradeTransitionService,
		Facilities:               facilitiesService,
		Schulhof:                 schulhofService,
		Feedback:                 feedbackService,
		Suggestions:              suggestionsService,
		IoT:                      iotService,
		Config:                   configService,
		Settings:                 settingsService,
		Schedule:                 scheduleService,
		PickupSchedule:           pickupScheduleService,
		Users:                    usersService,
		CaregiverCapability:      caregiverCapabilityService,
		Guardian:                 guardianService,
		UserContext:              userContextService,
		Database:                 databaseService,
		Import:                   studentImportService, // Student import service
		RealtimeHub:              realtimeHub,          // Expose SSE hub for API layer
		Invitation:               invitationService,
		Mailer:                   mailer,
		DefaultFrom:              defaultFrom,
		FrontendURL:              frontendURL,
		InvitationTokenExpiry:    invitationTokenExpiry,
		PasswordResetTokenExpiry: passwordResetTokenExpiry,

		// Platform services — OperatorAuth and OperatorInvitation both point
		// at the same concrete operatorAuthService struct, exposed through
		// two narrower interfaces so that each handler depends only on the
		// methods it actually calls. NewOperatorAuthService returns the
		// combined interface, so both fields can be assigned directly.
		OperatorAuth:         operatorAuthService,
		OperatorInvitation:   operatorAuthService,
		OperatorProvisioning: operatorProvisioningService,
		Announcement:         announcementService,
		OperatorSuggestions:  operatorSuggestionsService,
	}, nil
}
