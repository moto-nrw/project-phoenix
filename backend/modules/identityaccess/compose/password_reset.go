package compose

import (
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// PasswordResetDependencies compose the password reset flows (#2722). They
// require the session dependencies: the flows reuse the portal-role lookups
// and the session revocation.
type PasswordResetDependencies struct {
	// DeliveryFor builds the delivery over the recorder of its outcome; the
	// composition calls it once.
	DeliveryFor      func(identityaccess.PasswordResetDeliveryRecorder) identityPorts.PasswordResetDelivery
	Passwords        identityPorts.PasswordPolicy
	Expiry           time.Duration
	RateLimitEnabled bool
	Logger           *slog.Logger
}

func newPasswordReset(auth *application.AccountAuthentication, store *postgres.Store, sessions *identityPorts.SessionDependencies, deps *PasswordResetDependencies) (*application.PasswordReset, error) {
	if deps == nil {
		return nil, nil
	}
	if auth == nil || sessions == nil {
		return nil, errors.New("identity access compose: the password reset flows require the session dependencies")
	}
	attach := sessions.TenantRuntime
	var delivery identityPorts.PasswordResetDelivery
	if deps.DeliveryFor != nil {
		delivery = deps.DeliveryFor(application.NewPasswordResetDeliveryRecords(store, attach, tenantRuntime{attach: attach, runner: application.NewTransactionRunner()}))
	}
	if delivery == nil || deps.Passwords == nil {
		return nil, errors.New("identity access compose: every password reset dependency is required")
	}
	return application.NewPasswordReset(auth, application.PasswordResetDependencies{
		Store: store, Logins: store, Passwords: deps.Passwords,
		Delivery:         application.PasswordResetDelivery{Source: delivery},
		Runtime:          tenantRuntime{attach: attach, runner: application.NewTransactionRunner()},
		Expiry:           deps.Expiry,
		RateLimitEnabled: deps.RateLimitEnabled,
		Logger:           deps.Logger,
	})
}
