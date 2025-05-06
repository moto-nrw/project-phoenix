package repositories

import (
	"github.com/moto-nrw/project-phoenix/database/repositories/users"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Factory provides access to all repositories
type Factory struct {
	// Users domain
	Person   userModels.PersonRepository
	RFIDCard userModels.RFIDCardRepository

	// Add other repositories here as they are implemented
	// Auth domain
	// Account  auth.AccountRepository

	// Activities domain
	// Activity   activities.ActivityRepository
	// Category   activities.CategoryRepository

	// ... and so on
}

// NewFactory creates a new repository factory with all repositories
func NewFactory(db *bun.DB) *Factory {
	return &Factory{
		// Initialize all repositories
		Person:   users.NewPersonRepository(db),
		RFIDCard: users.NewRFIDCardRepository(db),

		// Add other repositories as they are implemented
	}
}
