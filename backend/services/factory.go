// Package services provides service layer implementations
package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/feedback"
	"github.com/uptrace/bun"
)

// Factory provides access to all services
type Factory struct {
	Auth      auth.AuthService
	Active    active.Service
	Education education.Service
	Feedback  feedback.Service
	// Add other services as they are created
	// Student StudentService
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

	return &Factory{
		Auth:      authService,
		Active:    activeService,
		Education: educationService,
		Feedback:  feedbackService,
		// Initialize other services as they are created
	}, nil
}