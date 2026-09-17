package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// PasswordResetStore is the persistence port over the identity-owned rows
// the password reset flows (#2722) read and write: auth.password_reset_tokens,
// auth.password_reset_rate_limits and the credential columns of
// auth.accounts. Every statement runs on the connection the caller's context
// carries. "Redeemable" compares against the now the caller passes.
type PasswordResetStore interface {
	// PasswordResetWindow reads the address's window without counting.
	PasswordResetWindow(ctx context.Context, email string) (domain.PasswordResetWindow, bool, domain.OperationStats, error)
	// CountPasswordResetRequest counts one request in the rolling window,
	// restarting an elapsed window, and returns the window afterwards.
	CountPasswordResetRequest(ctx context.Context, email string) (domain.PasswordResetWindow, domain.OperationStats, error)
	// DeleteStalePasswordResetWindows removes windows that started before.
	DeleteStalePasswordResetWindows(ctx context.Context, before time.Time) (int, domain.OperationStats, error)

	// RevokePasswordResetTokens spends every link of the account.
	RevokePasswordResetTokens(ctx context.Context, accountID int64) (domain.OperationStats, error)
	InsertPasswordResetToken(ctx context.Context, token domain.PasswordResetToken) (domain.PasswordResetToken, domain.OperationStats, error)
	FindRedeemablePasswordResetToken(ctx context.Context, token string, now time.Time) (domain.PasswordResetToken, bool, domain.OperationStats, error)
	// RedeemPasswordResetToken spends the link and reports whether it was
	// still unused.
	RedeemPasswordResetToken(ctx context.Context, id int64) (bool, domain.OperationStats, error)
	RecordPasswordResetDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) (domain.OperationStats, error)
	// DeleteSpentPasswordResetTokens removes links that expired before now or
	// were used.
	DeleteSpentPasswordResetTokens(ctx context.Context, now time.Time) (int, domain.OperationStats, error)

	// SetAccountPassword replaces the credential and clears the one-time
	// password flag; it reports whether the account exists.
	SetAccountPassword(ctx context.Context, accountID int64, hash string) (bool, domain.OperationStats, error)
}

// PasswordResetDelivery is the consumer-owned port over the Delivery
// platform's asynchronous e-mail send of a reset link. The composition
// resolves the portal host for the scope and records the delivery outcome
// through the module once the send settles.
type PasswordResetDelivery interface {
	DispatchPasswordReset(ctx context.Context, token domain.PasswordResetToken, recipient string, scope domain.PasswordResetScope, expiry time.Duration)
}
