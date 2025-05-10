// Package services provides service layer implementations
package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/uptrace/bun"
)

// Factory provides access to all services
type Factory struct {
	Auth auth.AuthService
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

	return &Factory{
		Auth: authService,
		// Initialize other services as they are created
	}, nil
}
