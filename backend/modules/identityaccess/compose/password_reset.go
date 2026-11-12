package compose

import (
	"errors"
	"log/slog"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-ogs/project-phoenix/modules/identityaccess/ports"
)

// PasswordResetDependencies add reset policy and delivery to the session flows.
type PasswordResetDependencies struct {
	// DeliveryFor builds delivery over its outcome recorder once.
	DeliveryFor      func(identityaccess.PasswordResetDeliveryRecorder) identityPorts.PasswordResetDelivery
	Passwords        identityPorts.PasswordPolicy
	Expiry           time.Duration
	RateLimitEnabled bool
	RateLimitSecret  []byte
	Logger           *slog.Logger
}

func newPasswordReset(auth *application.AccountAuthentication, store *postgres.Store, sessions *identityPorts.SessionDependencies, deps *PasswordResetDependencies, runtime tenantRuntime) (*application.PasswordReset, error) {
	if deps == nil {
		return nil, nil
	}
	if auth == nil || sessions == nil {
		return nil, errors.New("identity access compose: the password reset flows require the session dependencies")
	}
	attach := sessions.TenantRuntime
	var delivery identityPorts.PasswordResetDelivery
	if deps.DeliveryFor != nil {
		delivery = deps.DeliveryFor(application.NewPasswordResetDeliveryRecords(store, attach, runtime))
	}
	if delivery == nil || deps.Passwords == nil {
		return nil, errors.New("identity access compose: every password reset dependency is required")
	}
	return application.NewPasswordReset(auth, application.PasswordResetDependencies{
		Store: store, Logins: store, Passwords: deps.Passwords,
		Delivery:         application.PasswordResetDelivery{Source: delivery},
		Runtime:          runtime,
		Expiry:           deps.Expiry,
		RateLimitEnabled: deps.RateLimitEnabled,
		RateLimitSecret:  deps.RateLimitSecret,
		Logger:           deps.Logger,
	})
}
