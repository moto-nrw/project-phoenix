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
)

// PasswordResetDelivery mails a reset link asynchronously. The root resolves
// the portal host for the scope and records the outcome through
// RecordPasswordResetDelivery once the send settles.
type PasswordResetDelivery interface {
	DispatchPasswordReset(ctx context.Context, link identityaccess.PasswordResetLink, recipient string, scope identityaccess.PasswordResetScope, expiry time.Duration)
}

// PasswordResetDependencies compose the password reset flows (#2722). They
// require the session dependencies: the flows reuse the portal-role lookups
// and the session revocation.
type PasswordResetDependencies struct {
	Delivery         PasswordResetDelivery
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

type passwordResetDelivery struct{ source PasswordResetDelivery }

func (d passwordResetDelivery) DispatchPasswordReset(ctx context.Context, token domain.PasswordResetToken, recipient string, scope domain.PasswordResetScope, expiry time.Duration) {
	d.source.DispatchPasswordReset(ctx, publicPasswordResetLink(token), recipient, identityaccess.PasswordResetScope(scope), expiry)
}

func publicPasswordResetLink(token domain.PasswordResetToken) identityaccess.PasswordResetLink {
	return identityaccess.PasswordResetLink{
		ID: token.ID, AccountID: token.AccountID, Token: token.Token, Expiry: token.Expiry,
		Delivery: identityaccess.TokenDelivery(token.Delivery), CreatedAt: token.CreatedAt,
	}
}

var errPasswordResetUnavailable = identityaccess.ErrPasswordResetUnavailable

func (e engine) InitiatePasswordReset(ctx context.Context, email string, scope identityaccess.PasswordResetScope) (*identityaccess.PasswordResetLink, error) {
	if e.resets == nil {
		return nil, errPasswordResetUnavailable
	}
	token, err := e.resets.InitiatePasswordReset(e.attach(ctx), email, domain.PasswordResetScope(scope))
	if err != nil {
		return nil, passwordResetError(err)
	}
	if token == nil {
		return nil, nil
	}
	link := publicPasswordResetLink(*token)
	return &link, nil
}

func (e engine) ResetPassword(ctx context.Context, token, newPassword string) error {
	if e.resets == nil {
		return errPasswordResetUnavailable
	}
	return passwordResetError(e.resets.ResetPassword(e.attach(ctx), token, newPassword))
}

func (e engine) RecordPasswordResetDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error {
	if e.resets == nil {
		return errPasswordResetUnavailable
	}
	return passwordResetError(e.resets.RecordPasswordResetDelivery(e.attach(ctx), id, domain.TokenDelivery(delivery)))
}

func (e engine) DeleteSpentPasswordResetTokens(ctx context.Context) (int, error) {
	if e.resets == nil {
		return 0, errPasswordResetUnavailable
	}
	deleted, err := e.resets.DeleteSpentPasswordResetTokens(e.attach(ctx))
	return deleted, passwordResetError(err)
}

func (e engine) DeleteStalePasswordResetWindows(ctx context.Context) (int, error) {
	if e.resets == nil {
		return 0, errPasswordResetUnavailable
	}
	deleted, err := e.resets.DeleteStalePasswordResetWindows(e.attach(ctx))
	return deleted, passwordResetError(err)
}

// passwordResetError translates the rate-limit rejection to its public
// shape and everything else like the authentication flows.
func passwordResetError(err error) error { return recordMissing(translatePasswordResetError(err)) }

func translatePasswordResetError(err error) error {
	if err == nil {
		return nil
	}
	var operation *application.OperationError
	if errors.As(err, &operation) && operation == err {
		return &identityaccess.AuthenticationError{Op: operation.Op, Err: passwordResetError(operation.Err)}
	}
	var limited *domain.PasswordResetRateLimitError
	if errors.As(err, &limited) {
		return &identityaccess.PasswordResetRateLimitError{Attempts: limited.Attempts, RetryAt: limited.RetryAt}
	}
	return authenticationError(err)
}
