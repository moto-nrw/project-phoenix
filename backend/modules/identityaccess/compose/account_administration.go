package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	internalports "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityBindingPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// newAccountAdministration composes the account administration flows
// (#3332) over the session flows whose revocation a deactivation schedules.
// The manageable-school set an organisation administrator is bounded by
// comes from the school directory the session dependencies already bind.
func newAccountAdministration(
	store internalports.AccountAdministrationStore,
	auth *application.AccountAuthentication,
	sessions *identityBindingPorts.SessionDependencies,
	deps *LifecycleDependencies,
) (*application.AccountAdministration, error) {
	if deps == nil {
		return nil, nil
	}
	if auth == nil || sessions == nil {
		return nil, errors.New("identity access compose: the administration flows require the session dependencies")
	}
	if sessions.Schools == nil || sessions.Passwords == nil {
		return nil, errors.New("identity access compose: the administration flows require the school directory and the password verifier")
	}
	return application.NewAccountAdministration(auth, application.AccountAdministrationDependencies{
		Store:     store,
		Schools:   application.SchoolDirectory{Source: sessions.Schools},
		Verifier:  sessions.Passwords,
		Passwords: deps.Passwords,
		Runtime:   tenantRuntime{attach: sessions.TenantRuntime, runner: application.NewTransactionRunner()},
		Logger:    deps.Logger,
	})
}

// --- engine methods --------------------------------------------------------
