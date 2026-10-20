package compose

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	rotationBindings "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application/rotation"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The operator flows (#3252) need facts other owners hold and the retained
// rules the root still composes. The seams below are expressed in public
// values so the root can implement them without the module's internal
// vocabulary; this package adapts them to the consumer-owned ports.

// PasswordHasher hashes a new password and applies the strength policy.
type PasswordHasher interface {
	HashPassword(password string) (string, error)
	ValidatePasswordStrength(password string) error
}

// OperatorDependencies are the seams the operator flows need beyond the
// account-session dependencies they share (the password verifier, the JWT
// codec, the auth ledger, the schools and the tenant runtime).
type OperatorDependencies struct {
	Audit         identityPorts.OperatorAudit
	Passwords     PasswordHasher
	Organizations identityPorts.OrganizationDirectory
	Identities    identityPorts.SchoolIdentityProvisioner
	Logger        *slog.Logger
}

func newOperatorFlows(service *application.Service, store *postgres.Store, tokens *application.OperatorTokens, auth *application.AccountAuthentication, sessions *identityPorts.SessionDependencies, deps *OperatorDependencies, lifecycle *application.AccountLifecycle, mfa ports.OperatorMFAGate) (*application.OperatorAuthentication, *application.OperatorAccountAccess, error) {
	if deps == nil {
		return nil, nil, nil
	}
	if sessions == nil || auth == nil {
		return nil, nil, errors.New("identity access compose: the operator flows require the session dependencies")
	}
	switch {
	case deps.Audit == nil, deps.Passwords == nil,
		deps.Organizations == nil, deps.Identities == nil:
		return nil, nil, errors.New("identity access compose: every operator dependency is required")
	}
	runtime := tenantRuntime{attach: sessions.TenantRuntime, runner: tenant.NewTransactionRunner()}
	operatorAuth, err := application.NewOperatorAuthentication(service, application.OperatorAuthenticationDependencies{
		Passwords:   sessions.Passwords,
		Hasher:      deps.Passwords,
		Codec:       application.TokenCodec{Source: sessions.Codec},
		MFA:         mfa,
		Audit:       application.OperatorAudit{Source: deps.Audit},
		Credentials: tokens,
		Runtime:     runtime,
		Attach:      sessions.TenantRuntime,
		Rotation:    rotationBindings.RotationPolicy{},
		Logger:      deps.Logger,
	})
	if err != nil {
		return nil, nil, err
	}
	accountAccess, err := application.NewOperatorAccountAccess(application.OperatorAccountAccessDependencies{
		Store:         store,
		Login:         store,
		Access:        store,
		Sessions:      auth,
		Schools:       application.SchoolDirectory{Source: sessions.Schools},
		Organizations: application.OrganizationDirectory{Source: deps.Organizations},
		Identities:    application.SchoolIdentityProvisioner{Source: deps.Identities, Lifecycle: lifecycle},
		Policy:        application.SchoolRolePolicy{},
		Audit:         application.AuthAudit{Source: sessions.Audit},
		OperatorAudit: application.OperatorAudit{Source: deps.Audit},
		Runtime:       runtime,
		Attach:        sessions.TenantRuntime,
		Logger:        deps.Logger,
	})
	if err != nil {
		return nil, nil, err
	}
	return operatorAuth, accountAccess, nil
}

// Engine methods: every operator flow answers
// ErrOperatorAuthenticationUnavailable when the module was composed without
// operator dependencies.
