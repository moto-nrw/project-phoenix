package compose

import (
	application "github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/application"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// NewOffboarding binds the Membership-owned part of staff offboarding and of
// the automatic deletion of retired staff (#3963). It uses the same store and
// ambient transaction as the Membership service.
func NewOffboarding(dependencies Dependencies) (*schoolmembership.Offboarding, error) {
	service, err := newApplication(dependencies)
	if err != nil {
		return nil, err
	}
	return application.NewOffboardingFacade(service), nil
}
