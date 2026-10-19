package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
)

// newCatererAccess composes the caterer flows (ADR 0074) over the sessions
// and the lifecycle's password policy; without either there are none.
func newCatererAccess(auth *application.AccountAuthentication, store *postgres.Store, sessions *SessionDependencies, lifecycle *LifecycleDependencies) (*identityaccess.CatererAccess, error) {
	if auth == nil || sessions == nil || lifecycle == nil {
		return nil, nil
	}
	access, err := application.NewCatererAccess(auth, application.CatererAccessDependencies{
		Store:     store,
		Passwords: lifecycle.Passwords,
		Verifier:  sessions.Passwords,
		Schools:   application.SchoolDirectory{Source: sessions.Schools},
		Runtime:   tenantRuntime{attach: sessions.TenantRuntime, runner: newTransactionRunner()},
		Attach:    sessions.TenantRuntime,
		Logger:    lifecycle.Logger,
	})
	if err != nil {
		return nil, err
	}
	return identityaccess.NewCatererAccess(access), nil
}
