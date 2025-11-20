// Package services provides service layer implementations
package services

import (
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policies"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
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
	Guardian                 users.GuardianService
	UserContext              usercontext.UserContextService
	Database                 database.DatabaseService
	Import                   *importService.ImportService[importModels.StudentImportRow] // Student import service
	RealtimeHub              *realtime.Hub                                                // SSE event hub (shared by services and API)
	Mailer                   email.Mailer
	DefaultFrom              email.Email
	FrontendURL              string
	InvitationTokenExpiry    time.Duration
	PasswordResetTokenExpiry time.Duration
}

// NewFactory creates a new services factory
func NewFactory(repos *repositories.Factory, db *bun.DB) (*Factory, error) {

	mailer, err := email.NewMailer()
	if err != nil {
		log.Printf("email: failed to initialize SMTP mailer, falling back to mock mailer: %v", err)
		mailer = email.NewMockMailer()
	}
	if _, ok := mailer.(*email.MockMailer); ok {
		log.Println("email: SMTP mailer not configured; using mock mailer (tokens will not be sent via SMTP)")
	}

	dispatcher := email.NewDispatcher(mailer)

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
		log.Fatalf("FRONTEND_URL must use https:// in production (received %q)", rawFrontendURL)
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
	usersService := users.NewPersonService(
		repos.Person,
		repos.RFIDCard,
		repos.Account,
		repos.PersonGuardian,
		repos.Student,
		repos.Staff,
		repos.Teacher,
		db,
	)

	// Initialize guardian service
	guardianService := users.NewGuardianService(
		repos.GuardianProfile,
		repos.StudentGuardian,
		repos.GuardianInvitation,
		repos.AccountParent,
		repos.Student,
		repos.Person,
		mailer,
		dispatcher,
		frontendURL,
		defaultFrom,
		invitationTokenExpiry,
		db,
	)

	// Initialize active service with SSE broadcaster
	activeService := active.NewService(
		repos.ActiveGroup,
		repos.ActiveVisit,
		repos.GroupSupervisor,
		repos.CombinedGroup,
		repos.GroupMapping,
		repos.ScheduledCheckout,
		repos.Student,
		repos.Room,
		repos.ActivityGroup,
		repos.ActivityCategory,
		repos.Group,
		repos.Person,
		repos.Attendance,
		educationService,
		usersService,
		repos.Teacher,
		repos.Staff,
		db,
		realtimeHub, // Pass SSE broadcaster
	)

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

	// Initialize auth service
	authService, err := auth.NewService(
		repos.Account,
		repos.AccountRole,
		repos.AccountPermission,
		repos.Permission,
		repos.Token,
		repos.AccountParent,      // Add this
		repos.Role,               // Add this
		repos.RolePermission,     // Add this
		repos.PasswordResetToken, // Add this
		repos.PasswordResetRateLimit,
		repos.Person,    // Add this for first name
		repos.AuthEvent, // Add for audit logging
		db,
		mailer,
		dispatcher,
		frontendURL,
		defaultFrom,
		passwordResetTokenExpiry,
	)
	if err != nil {
		return nil, err
	}

	invitationService := auth.NewInvitationService(
		repos.InvitationToken,
		repos.Account,
		repos.Role,
		repos.AccountRole,
		repos.Person,
		repos.Staff,
		repos.Teacher,
		mailer,
		dispatcher,
		frontendURL,
		defaultFrom,
		invitationTokenExpiry,
		db,
	)

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
	userContextService := usercontext.NewUserContextService(
		repos.Account,
		repos.Person,
		repos.Staff,
		repos.Teacher,
		repos.Student,
		repos.Group,
		repos.ActivityGroup,
		repos.ActiveGroup,
		repos.ActiveVisit,
		repos.GroupSupervisor,
		repos.Profile,
		repos.GroupSubstitution,
		db,
	)

	// Initialize database stats service
	databaseService := database.NewService(repos)

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
		repos.Person,
		repos.Student,
		repos.GuardianProfile,
		repos.StudentGuardian,
		repos.PrivacyConsent,
		repos.RFIDCard,
		relationshipResolver,
		db,
	)
	studentImportService := importService.NewImportService(studentImportConfig, db)

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
