package identityaccess

import (
	"context"
)

// SignInDataRetention deletes short-lived sign-in data after the fixed
// windows of the retention list (#3954): invitations for parents 30 days
// after redemption or expiry, e-mail codes and passkey ceremonies of accounts
// and operators 7 days after expiry. The hourly token cleanup runs it in an
// administrative transaction that reaches every school.
type SignInDataRetention interface {
	DeleteExpiredGuardianInvitations(ctx context.Context) (int, error)
	DeleteExpiredMFAEmailChallenges(ctx context.Context) (int, error)
	DeleteExpiredPasskeySessions(ctx context.Context) (int, error)
}
