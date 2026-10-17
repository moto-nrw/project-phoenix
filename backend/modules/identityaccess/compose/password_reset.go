package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// PasswordResetDependencies compose the password reset flows (#2722). They
// require the session dependencies: the flows reuse the portal-role lookups
// and the session revocation.
type PasswordResetDependencies struct {
	Delivery         identityPorts.PasswordResetDelivery
	Passwords        PasswordPolicy
	Expiry           time.Duration
	RateLimitEnabled bool
	Logger           *slog.Logger
}

func newPasswordReset(auth *application.AccountAuthentication, store *postgres.Store, sessions *SessionDependencies, deps *PasswordResetDependencies) (*application.PasswordReset, error) {
	if deps == nil {
		return nil, nil
	}
	if auth == nil || sessions == nil {
		return nil, errors.New("identity access compose: the password reset flows require the session dependencies")
	}
	if deps.Delivery == nil || deps.Passwords == nil {
		return nil, errors.New("identity access compose: every password reset dependency is required")
	}
	attach := sessions.TenantRuntime
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	return application.NewPasswordReset(auth, application.PasswordResetDependencies{
		Store: store, Logins: store, Passwords: deps.Passwords,
		Delivery:         passwordResetDelivery{deps.Delivery},
		Runtime:          tenantRuntime{attach: attach, runner: newTransactionRunner()},
		Expiry:           deps.Expiry,
		RateLimitEnabled: deps.RateLimitEnabled,
		Logger:           deps.Logger,
	})
}

type passwordResetDelivery struct {
	source identityPorts.PasswordResetDelivery
}

func (d passwordResetDelivery) DispatchPasswordReset(ctx context.Context, token domain.PasswordResetToken, recipient string, scope domain.PasswordResetScope, expiry time.Duration) {
	d.source.DispatchPasswordReset(ctx, application.PublicPasswordResetLink(token), recipient, identityaccess.PasswordResetScope(scope), expiry)
}
