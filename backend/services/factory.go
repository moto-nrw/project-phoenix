// Package services provides service layer implementations
package services

import (
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policies"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/database"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/feedback"
	"github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Factory provides access to all services
type Factory struct {
	Auth          auth.AuthService
	Active        active.Service
	ActiveCleanup active.CleanupService
	Activities    activities.ActivityService
	Education     education.Service
	Facilities    facilities.Service
	Feedback      feedback.Service
	IoT           iot.Service
	Config        config.Service
	Schedule      schedule.Service
	Users         users.PersonService
	UserContext   usercontext.UserContextService
	Database      database.DatabaseService
}

// NewFactory creates a new services factory
func NewFactory(repos *repositories.Factory, db *bun.DB) (*Factory, error) {

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

	// Initialize active service
	activeService := active.NewService(
		repos.ActiveGroup,
		repos.ActiveVisit,
		repos.GroupSupervisor,
		repos.CombinedGroup,
		repos.GroupMapping,
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
		repos.Person,             // Add this for first name
		db,
	)
	if err != nil {
		return nil, err
	}

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

	return &Factory{
		Auth:          authService,
		Active:        activeService,
		ActiveCleanup: activeCleanupService,
		Activities:    activitiesService,
		Education:     educationService,
		Facilities:    facilitiesService,
		Feedback:      feedbackService,
		IoT:           iotService,
		Config:        configService,
		Schedule:      scheduleService,
		Users:         usersService,
		UserContext:   userContextService,
		Database:      databaseService,
	}, nil
}
