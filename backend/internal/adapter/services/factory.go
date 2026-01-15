// Package services provides service layer implementations
package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/adapter/mailer"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/policies"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	importModels "github.com/moto-nrw/project-phoenix/internal/core/domain/import"
	"github.com/moto-nrw/project-phoenix/internal/core/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
	"github.com/moto-nrw/project-phoenix/internal/core/service/activities"
	"github.com/moto-nrw/project-phoenix/internal/core/service/auth"
	"github.com/moto-nrw/project-phoenix/internal/core/service/config"
	"github.com/moto-nrw/project-phoenix/internal/core/service/database"
	"github.com/moto-nrw/project-phoenix/internal/core/service/education"
	"github.com/moto-nrw/project-phoenix/internal/core/service/facilities"
	"github.com/moto-nrw/project-phoenix/internal/core/service/feedback"
	importService "github.com/moto-nrw/project-phoenix/internal/core/service/import"
	"github.com/moto-nrw/project-phoenix/internal/core/service/iot"
	"github.com/moto-nrw/project-phoenix/internal/core/service/schedule"
	"github.com/moto-nrw/project-phoenix/internal/core/service/usercontext"
	"github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// Factory provides access to all services
type Factory struct {
	Auth                     auth.AuthService
	Active                   active.Service
	ActiveCleanup            active.CleanupService
	Activities               activities.ActivityService
	Education                education.Service
	Facilities               facilities.Service
	Invitation               auth.InvitationService
	Feedback                 feedback.Service
	IoT                      iot.Service
	Config                   config.Service
	Schedule                 schedule.Service
	Users                    users.PersonService
	Student                  users.StudentService
	Guardian                 users.GuardianService
	UserContext              usercontext.UserContextService
	Database                 database.DatabaseService
	Import                   *importService.ImportService[importModels.StudentImportRow] // Student import service
	Mailer                   port.EmailSender
	DefaultFrom              port.EmailAddress
	FrontendURL              string
	InvitationTokenExpiry    time.Duration
	PasswordResetTokenExpiry time.Duration
}

// NewFactory creates a new services factory.
// The fileStorage parameter is optional (can be nil) - if provided, it will be used
// for avatar storage. Pass nil for CLI commands that don't need avatar functionality.
// The broadcaster parameter is optional (can be nil) - if provided, it will be used
// for real-time event broadcasting. Pass nil for CLI commands that don't need SSE.
// This follows the Hexagonal Architecture pattern where adapters are injected from outside.
func NewFactory(repos *repositories.Factory, db *bun.DB, fileStorage port.FileStorage, broadcaster port.Broadcaster) (*Factory, error) {

	appEnv := strings.ToLower(strings.TrimSpace(viper.GetString("app_env")))
	if appEnv == "" {
		return nil, fmt.Errorf("APP_ENV environment variable is required")
	}

	// Configure avatar storage if provided (injected from adapter layer)
	if fileStorage != nil {
		usercontext.SetAvatarStorage(fileStorage)
		logger.Logger.Info("storage: avatar storage configured")
	} else {
		logger.Logger.Debug("storage: no file storage provided, avatar operations will be disabled")
	}

	m, err := mailer.NewSMTPMailer()
	if err != nil {
		if appEnv == "production" {
			return nil, fmt.Errorf("failed to initialize SMTP mailer: %w", err)
		}
		logger.Logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Warn("email: failed to initialize SMTP mailer, falling back to mock mailer")
		m = mailer.NewMockMailer()
	}
	if _, ok := m.(*mailer.MockMailer); ok {
		logger.Logger.Info("email: SMTP mailer not configured; using mock mailer (tokens will not be sent via SMTP)")
	}

	dispatcher := mailer.NewDispatcher(m)

	defaultFrom := port.EmailAddress{
		Name:    strings.TrimSpace(viper.GetString("email_from_name")),
		Address: strings.TrimSpace(viper.GetString("email_from_address")),
	}
	if defaultFrom.Name == "" || defaultFrom.Address == "" {
		return nil, fmt.Errorf("EMAIL_FROM_NAME and EMAIL_FROM_ADDRESS environment variables are required")
	}

	rawFrontendURL := viper.GetString("frontend_url")
	frontendURL := strings.TrimRight(rawFrontendURL, "/")
	if frontendURL == "" {
		return nil, fmt.Errorf("FRONTEND_URL environment variable is required")
	}

	if appEnv == "production" && !strings.HasPrefix(frontendURL, "https://") {
		return nil, fmt.Errorf("FRONTEND_URL must use https:// in production (received %q)", rawFrontendURL)
	}

	if strings.TrimSpace(viper.GetString("invitation_token_expiry_hours")) == "" {
		return nil, fmt.Errorf("INVITATION_TOKEN_EXPIRY_HOURS environment variable is required")
	}
	invitationExpiryHours := viper.GetInt("invitation_token_expiry_hours")
	if invitationExpiryHours <= 0 {
		return nil, fmt.Errorf("INVITATION_TOKEN_EXPIRY_HOURS must be a positive integer (1-168)")
	}
	if invitationExpiryHours > 168 {
		return nil, fmt.Errorf("INVITATION_TOKEN_EXPIRY_HOURS must be less than or equal to 168")
	}
	invitationTokenExpiry := time.Duration(invitationExpiryHours) * time.Hour

	if strings.TrimSpace(viper.GetString("password_reset_token_expiry_minutes")) == "" {
		return nil, fmt.Errorf("PASSWORD_RESET_TOKEN_EXPIRY_MINUTES environment variable is required")
	}
	passwordResetExpiryMinutes := viper.GetInt("password_reset_token_expiry_minutes")
	if passwordResetExpiryMinutes <= 0 {
		return nil, fmt.Errorf("PASSWORD_RESET_TOKEN_EXPIRY_MINUTES must be a positive integer (1-1440)")
	}
	if passwordResetExpiryMinutes > 1440 {
		return nil, fmt.Errorf("PASSWORD_RESET_TOKEN_EXPIRY_MINUTES must be less than or equal to 1440")
	}
	passwordResetTokenExpiry := time.Duration(passwordResetExpiryMinutes) * time.Minute

	// Rate limiting configuration (12-Factor: read at startup, inject into services)
	rateLimitEnabled := viper.GetBool("rate_limit_enabled")
	rateLimitMaxRequests := viper.GetInt("rate_limit_max_requests")
	if rateLimitEnabled {
		rawRateLimitMax := strings.TrimSpace(viper.GetString("rate_limit_max_requests"))
		if rawRateLimitMax == "" {
			return nil, fmt.Errorf("RATE_LIMIT_MAX_REQUESTS environment variable is required when rate limiting is enabled")
		}
		if rateLimitMaxRequests <= 0 {
			return nil, fmt.Errorf("RATE_LIMIT_MAX_REQUESTS must be a positive integer (1-100)")
		}
		if rateLimitMaxRequests > 100 {
			return nil, fmt.Errorf("RATE_LIMIT_MAX_REQUESTS must be less than or equal to 100")
		}
	}

	// Initialize education service first (needed for active service)
	educationService := education.NewService(
		repos.Group,
		repos.GroupTeacher,
		repos.GroupSubstitution,
		repos.Room,
		repos.Teacher,
		repos.Staff,
		db,
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
	})

	// Initialize guardian service
	guardianService := users.NewGuardianService(users.GuardianServiceDependencies{
		GuardianProfileRepo:    repos.GuardianProfile,
		StudentGuardianRepo:    repos.StudentGuardian,
		GuardianInvitationRepo: repos.GuardianInvitation,
		AccountParentRepo:      repos.AccountParent,
		StudentRepo:            repos.Student,
		PersonRepo:             repos.Person,
		Dispatcher:             dispatcher,
		FrontendURL:            frontendURL,
		DefaultFrom:            defaultFrom,
		InvitationExpiry:       invitationTokenExpiry,
		DB:                     db,
	})

	// Initialize active service with SSE broadcaster (injected from adapter layer)
	activeService := active.NewService(active.ServiceDependencies{
		GroupRepo:          repos.ActiveGroup,
		VisitRepo:          repos.ActiveVisit,
		SupervisorRepo:     repos.GroupSupervisor,
		CombinedGroupRepo:  repos.CombinedGroup,
		GroupMappingRepo:   repos.GroupMapping,
		AttendanceRepo:     repos.Attendance,
		StudentRepo:        repos.Student,
		PersonRepo:         repos.Person,
		TeacherRepo:        repos.Teacher,
		StaffRepo:          repos.Staff,
		RoomRepo:           repos.Room,
		ActivityGroupRepo:  repos.ActivityGroup,
		ActivityCatRepo:    repos.ActivityCategory,
		EducationGroupRepo: repos.Group,
		EducationService:   educationService,
		UsersService:       usersService,
		DB:                 db,
		Broadcaster:        broadcaster, // Injected from adapter layer (Hexagonal Architecture)
	})

	// Initialize feedback service
	feedbackService := feedback.NewService(
		repos.FeedbackEntry,
		db,
	)

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

	// Initialize activities service
	activitiesService, err := activities.NewService(
		repos.ActivityCategory,
		repos.ActivityGroup,
		repos.ActivitySchedule,
		repos.ActivitySupervisor,
		repos.StudentEnrollment,
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

	// Initialize schedule service
	scheduleService := schedule.NewService(
		repos.Dateframe,
		repos.Timeframe,
		repos.RecurrenceRule,
		db,
	)

	// Initialize auth service with validated config
	authConfig, err := auth.NewServiceConfig(
		dispatcher,
		defaultFrom,
		frontendURL,
		passwordResetTokenExpiry,
		rateLimitEnabled,
		rateLimitMaxRequests,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid auth service config: %w", err)
	}
	tokenProvider, err := jwt.NewTokenAuth()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize token auth: %w", err)
	}

	authRepos := auth.Repositories{
		Account:                repos.Account,
		AccountParent:          repos.AccountParent,
		Role:                   repos.Role,
		Permission:             repos.Permission,
		RolePermission:         repos.RolePermission,
		AccountRole:            repos.AccountRole,
		AccountPermission:      repos.AccountPermission,
		Token:                  repos.Token,
		PasswordResetToken:     repos.PasswordResetToken,
		PasswordResetRateLimit: repos.PasswordResetRateLimit,
		InvitationToken:        repos.InvitationToken,
		GuardianInvitation:     repos.GuardianInvitation,
		Person:                 repos.Person,
		AuthEvent:              repos.AuthEvent,
	}

	authService, err := auth.NewService(authRepos, authConfig, db, tokenProvider)
	if err != nil {
		return nil, err
	}

	invitationService := auth.NewInvitationService(auth.InvitationServiceConfig{
		InvitationRepo:   repos.InvitationToken,
		AccountRepo:      repos.Account,
		RoleRepo:         repos.Role,
		AccountRoleRepo:  repos.AccountRole,
		PersonRepo:       repos.Person,
		StaffRepo:        repos.Staff,
		TeacherRepo:      repos.Teacher,
		Dispatcher:       dispatcher,
		FrontendURL:      frontendURL,
		DefaultFrom:      defaultFrom,
		InvitationExpiry: invitationTokenExpiry,
		DB:               db,
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
	}, db)

	// Initialize database stats service
	databaseService := database.NewService(database.Repositories{
		Student:       repos.Student,
		Teacher:       repos.Teacher,
		Room:          repos.Room,
		ActivityGroup: repos.ActivityGroup,
		Group:         repos.Group,
		Role:          repos.Role,
		Device:        repos.Device,
		Permission:    repos.Permission,
	})

	// Initialize cleanup service
	activeCleanupService := active.NewCleanupService(
		repos.ActiveVisit,
		repos.Attendance,
		repos.PrivacyConsent,
		repos.DataDeletion,
		db,
	)

	// Initialize import service
	relationshipResolver := importService.NewRelationshipResolver(repos.Group, repos.Room)
	studentImportConfig := importService.NewStudentImportConfig(
		repos.Person,
		repos.Student,
		repos.GuardianProfile,
		repos.StudentGuardian,
		repos.PrivacyConsent,
		relationshipResolver,
		db,
	)
	studentImportService := importService.NewImportService(studentImportConfig, db)
	studentImportService.SetAuditRepository(repos.DataImport)

	// Initialize student service
	studentService := users.NewStudentService(users.StudentServiceDependencies{
		StudentRepo:        repos.Student,
		PrivacyConsentRepo: repos.PrivacyConsent,
		DB:                 db,
	})

	return &Factory{
		Auth:                     authService,
		Active:                   activeService,
		ActiveCleanup:            activeCleanupService,
		Activities:               activitiesService,
		Education:                educationService,
		Facilities:               facilitiesService,
		Feedback:                 feedbackService,
		IoT:                      iotService,
		Config:                   configService,
		Schedule:                 scheduleService,
		Users:                    usersService,
		Student:                  studentService,
		Guardian:                 guardianService,
		UserContext:              userContextService,
		Database:                 databaseService,
		Import:                   studentImportService, // Student import service
		Invitation:               invitationService,
		Mailer:                   m,
		DefaultFrom:              defaultFrom,
		FrontendURL:              frontendURL,
		InvitationTokenExpiry:    invitationTokenExpiry,
		PasswordResetTokenExpiry: passwordResetTokenExpiry,
	}, nil
}
