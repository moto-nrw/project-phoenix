// Package services provides service layer implementations
package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policies"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/internal/adapter/storage"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/moto-nrw/project-phoenix/logging"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/database"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/feedback"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/services/users"
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
	RealtimeHub              *realtime.Hub                                               // SSE event hub (shared by services and API)
	Mailer                   email.Mailer
	DefaultFrom              email.Email
	FrontendURL              string
	InvitationTokenExpiry    time.Duration
	PasswordResetTokenExpiry time.Duration
}

// NewFactory creates a new services factory
func NewFactory(repos *repositories.Factory, db *bun.DB) (*Factory, error) {

	// Initialize file storage for avatars and other uploads
	// Uses local filesystem for development; replace with S3/MinIO for production
	avatarStorage, err := storage.NewLocalStorage(port.StorageConfig{
		BasePath:        "public/uploads",
		PublicURLPrefix: "/uploads",
	}, logging.Logger)
	if err != nil {
		logging.Logger.WithError(err).Warn("storage: failed to initialize local storage for avatars")
	} else {
		usercontext.SetAvatarStorage(avatarStorage)
		logging.Logger.Info("storage: initialized local storage for avatars")
	}

	mailer, err := email.NewMailer()
	if err != nil {
		logging.Logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Warn("email: failed to initialize SMTP mailer, falling back to mock mailer")
		mailer = email.NewMockMailer()
	}
	if _, ok := mailer.(*email.MockMailer); ok {
		logging.Logger.Info("email: SMTP mailer not configured; using mock mailer (tokens will not be sent via SMTP)")
	}

	dispatcher := email.NewDispatcher(mailer)

	defaultFrom := email.NewEmail(viper.GetString("email_from_name"), viper.GetString("email_from_address"))
	if defaultFrom.Address == "" {
		defaultFrom = email.NewEmail("moto", "no-reply@moto.local")
	}

	rawFrontendURL := viper.GetString("frontend_url")
	frontendURL := strings.TrimRight(rawFrontendURL, "/")
	if frontendURL == "" {
		return nil, fmt.Errorf("FRONTEND_URL environment variable is required")
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
	realtimeHub := realtime.NewHub()

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
		Mailer:                 mailer,
		Dispatcher:             dispatcher,
		FrontendURL:            frontendURL,
		DefaultFrom:            defaultFrom,
		InvitationExpiry:       invitationTokenExpiry,
		DB:                     db,
	})

	// Initialize active service with SSE broadcaster
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
		Broadcaster:        realtimeHub, // Pass SSE broadcaster
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
	)
	if err != nil {
		return nil, fmt.Errorf("invalid auth service config: %w", err)
	}
	authService, err := auth.NewService(repos, authConfig, db)
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
		Mailer:           mailer,
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
	databaseService := database.NewService(repos)

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
		RealtimeHub:              realtimeHub,          // Expose SSE hub for API layer
		Invitation:               invitationService,
		Mailer:                   mailer,
		DefaultFrom:              defaultFrom,
		FrontendURL:              frontendURL,
		InvitationTokenExpiry:    invitationTokenExpiry,
		PasswordResetTokenExpiry: passwordResetTokenExpiry,
	}, nil
}
