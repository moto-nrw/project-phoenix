// backend/auth/authorize/policies/registry.go
package policies

import (
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/policy"
	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
	"github.com/moto-nrw/project-phoenix/internal/core/service/education"
	"github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// PolicyRegistry manages policy registration
type PolicyRegistry struct {
	educationService education.Service
	usersService     users.PersonService
	activeService    active.Service
}

// NewPolicyRegistry creates a new policy registry
func NewPolicyRegistry(
	educationService education.Service,
	usersService users.PersonService,
	activeService active.Service,
) *PolicyRegistry {
	return &PolicyRegistry{
		educationService: educationService,
		usersService:     usersService,
		activeService:    activeService,
	}
}

// RegisterAll registers all policies with the authorization service
func (r *PolicyRegistry) RegisterAll(authService authorize.AuthorizationService) error {
	policies := []policy.Policy{
		NewStudentVisitPolicy(r.educationService, r.usersService, r.activeService),
		// Add more policies here as you create them
		// NewActiveGroupPolicy(r.educationService, r.usersService),
		// NewStudentPolicy(r.educationService, r.usersService),
	}

	for _, p := range policies {
		if err := authService.RegisterPolicy(p); err != nil {
			return err
		}
	}

	return nil
}
