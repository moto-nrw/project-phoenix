package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// SignInDataRetentionStore deletes the short-lived sign-in data whose window
// ended before the given instant (#3954). The sweeps run in the token
// cleanup's administrative transaction and reach every school.
type SignInDataRetentionStore interface {
	// DeleteExpiredGuardianInvitations removes the invitations redeemed, or
	// else expired, before the instant.
	DeleteExpiredGuardianInvitations(ctx context.Context, before time.Time) (int, domain.OperationStats, error)
	// DeleteExpiredCatererInvitations removes the caterer invitations
	// accepted, revoked or else expired before the instant.
	DeleteExpiredCatererInvitations(ctx context.Context, before time.Time) (int, domain.OperationStats, error)
	DeleteExpiredAccountMFAEmailChallenges(ctx context.Context, before time.Time) (int, domain.OperationStats, error)
	DeleteExpiredOperatorMFAEmailChallenges(ctx context.Context, before time.Time) (int, domain.OperationStats, error)
	DeleteExpiredAccountPasskeySessions(ctx context.Context, before time.Time) (int, domain.OperationStats, error)
	DeleteExpiredOperatorPasskeySessions(ctx context.Context, before time.Time) (int, domain.OperationStats, error)
}
