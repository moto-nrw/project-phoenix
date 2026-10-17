package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func PublicPasswordResetLink(token domain.PasswordResetToken) identityaccess.PasswordResetLink {
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
	link := PublicPasswordResetLink(*token)
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
	var operation *OperationError
	if errors.As(err, &operation) && operation == err {
		return &identityaccess.AuthenticationError{Op: operation.Op, Err: passwordResetError(operation.Err)}
	}
	var limited *domain.PasswordResetRateLimitError
	if errors.As(err, &limited) {
		return &identityaccess.PasswordResetRateLimitError{Attempts: limited.Attempts, RetryAt: limited.RetryAt}
	}
	return authenticationError(err)
}
