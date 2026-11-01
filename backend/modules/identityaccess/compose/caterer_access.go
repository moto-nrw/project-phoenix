package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// newCatererAccess composes the caterer flows (ADR 0077) and their second
// factor (#4497); without the sessions or the lifecycle there are none.
func newCatererAccess(auth *application.AccountAuthentication, store *postgres.Store, sessions *identityPorts.SessionDependencies, lifecycle *LifecycleDependencies, flows application.MFAFlows) (*identityaccess.CatererAccess, error) {
	if auth == nil || sessions == nil || lifecycle == nil || lifecycle.Caterers == nil {
		return nil, nil
	}
	access, err := application.NewCatererAccess(auth, application.CatererAccessDependencies{
		Store:     store,
		Delivery:  lifecycle.Caterers,
		Passwords: lifecycle.Passwords,
		Verifier:  sessions.Passwords,
		Schools:   application.SchoolDirectory{SchoolDirectory: sessions.Schools}, MFA: flows.Account, Passkeys: flows.AccountPasskey,
		Runtime: tenantRuntime{attach: sessions.TenantRuntime, runner: application.NewTransactionRunner()},
		Attach:  sessions.TenantRuntime,
		Logger:  lifecycle.Logger,
	})
	if err != nil {
		return nil, err
	}
	return identityaccess.NewCatererAccess(access, access, access, access), nil
}
