// Package services provides service layer implementations
package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/feedback"
	"github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Factory provides access to all services
type Factory struct {
	Auth       auth.AuthService
	Active     active.Service
	Activities activities.ActivityService
	Education  education.Service
	Facilities facilities.Service
	Feedback   feedback.Service
	IoT        iot.Service
	Config     config.Service
	Schedule   schedule.Service
	Users      users.PersonService
}

// NewFactory creates a new services factory
func NewFactory(repos *repositories.Factory, db *bun.DB) (*Factory, error) {

	// Initialize active service
	activeService := active.NewService(
		repos.ActiveGroup,
		repos.ActiveVisit,
		repos.GroupSupervisor,
		repos.CombinedGroup,
		repos.GroupMapping,
		db,
	)

	// Initialize education service
	educationService := education.NewService(
		repos.Group,
		repos.GroupTeacher,
		repos.GroupSubstitution,
		repos.Room,
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

	// Initialize users service
	usersService := users.NewPersonService(
		repos.Person,
		repos.RFIDCard,
		repos.Account,
		repos.PersonGuardian,
		repos.Staff,
		repos.Teacher,
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
		db,
	)
	if err != nil {
		return nil, err
	}

	return &Factory{
		Auth:       authService,
		Active:     activeService,
		Activities: activitiesService,
		Education:  educationService,
		Facilities: facilitiesService,
		Feedback:   feedbackService,
		IoT:        iotService,
		Config:     configService,
		Schedule:   scheduleService,
		Users:      usersService,
	}, nil
}
