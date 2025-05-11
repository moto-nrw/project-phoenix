// Package services provides service layer implementations
package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/activ
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/education"
tion"
	"github.com/moto-nrw/project-phoenix/services/fac
	"github.com/moto-nrw/project-phoenix/services/feedback"
edback"
	"github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/moto
	"github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/uptrace/bun"
)

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
	// Group   GroupService
	// etc.
}

// NewFactory creates a new services factory
func NewFactory(repos *repositories.Factory, db *bun.DB) (*Factory, error) {
	// Initialize auth service
	authService, err := auth.NewService(
		repos.Account,
		repos.AccountRole,
		repos.Token,
		db,
	)
	if err != nil {
		return nil, err
	}

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

	return &Factory{
		Auth:      authService,
		Active:    activeService,
		Education: educationService,
		Feedback:  feedbackService,
		IoT:       iotService,
		Config:    configService,
		// Initialize other services as they are created
	}, nil
}