package identityaccess

import (
	"context"
	"time"
)

// Operator invitation and e-mail change links (#2722): the one-time tokens
// that let an invitee become an operator and that confirm an operator's new
// address. The rows are platform-wide like the operator itself. A link is
// redeemable while it is unused and unexpired; the capability never stores a
// link that is already expired or spent, so no delivery carries a dead
// credential, and it never extends one that expired in the meantime. Every
// operation joins the caller's transaction when one is active.
var (
	// ErrOperatorInvitationNotFound reports an invitation that does not exist
	// or is not in a state that allows the redemption, revocation or
	// extension. The loser of two concurrent redemptions receives it.
	ErrOperatorInvitationNotFound = notFoundError("operator invitation not found")
	// ErrOperatorEmailChangeNotFound reports an e-mail change link that does
	// not exist, is expired or was already used. The loser of two concurrent
	// confirmations receives it.
	ErrOperatorEmailChangeNotFound = notFoundError("operator email change not found")
)

// TokenDelivery is the recorded outcome of mailing a one-time link.
type TokenDelivery struct {
	SentAt     *time.Time
	Error      *string
	RetryCount int
	// Outcome is what Delivery knows about the last mail; empty for a token
	// whose owner did not ask (#5934). AddressMark is why the school's list holds
	// the address.
	Outcome     string
	AddressMark string
	// ProviderBlock is the provider's block now; only a mail it withheld asks.
	ProviderBlock string
}

// OperatorInvitation is the link that lets Email become an operator.
type OperatorInvitation struct {
	ID          int64
	Email       string
	Token       string
	ExpiresAt   time.Time
	UsedAt      *time.Time
	CreatedBy   int64
	DisplayName *string
	Delivery    TokenDelivery
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// OperatorEmailChange is the link that confirms NewEmail for an operator.
type OperatorEmailChange struct {
	ID         int64
	OperatorID int64
	NewEmail   string
	Token      string
	Expiry     time.Time
	Used       bool
	Delivery   TokenDelivery
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OperatorLinkDeliveryRecorder records how the mail of an operator invitation
// or e-mail change link fared; the root's mail delivery reports through it.
type OperatorLinkDeliveryRecorder interface {
	RecordOperatorInvitationDelivery(ctx context.Context, id int64, delivery TokenDelivery) error
	RecordOperatorEmailChangeDelivery(ctx context.Context, id int64, delivery TokenDelivery) error
}

// minimumOperatorEmailChangeExpiry accounts for mail delivery latency and the
// time the operator needs to open the link.
const minimumOperatorEmailChangeExpiry = 15 * time.Minute

// OperatorEmailChangeExpiry is the lifetime of an operator e-mail change
// link. It deliberately reuses the password reset lifetime: both are
// one-time verification links with the same delivery constraints and
// security profile, floored at 15 minutes. If the two ever need to diverge,
// introduce EMAIL_CHANGE_TOKEN_EXPIRY_MINUTES.
func OperatorEmailChangeExpiry(passwordResetExpiry time.Duration) time.Duration {
	if passwordResetExpiry >= minimumOperatorEmailChangeExpiry {
		return passwordResetExpiry
	}
	return minimumOperatorEmailChangeExpiry
}

// OperatorRecordCleanup drops the operator invitations and refresh sessions
// that expired; the hourly worker runs it.
type OperatorRecordCleanup interface {
	DeleteExpiredOperatorInvitations(ctx context.Context) (int, error)
	DeleteExpiredOperatorSessions(ctx context.Context, now time.Time) (int, error)
}
