package identityaccess

import (
	"context"
	"errors"
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
	ErrOperatorInvitationNotFound = errors.New("operator invitation not found")
	// ErrOperatorEmailChangeNotFound reports an e-mail change link that does
	// not exist, is expired or was already used. The loser of two concurrent
	// confirmations receives it.
	ErrOperatorEmailChangeNotFound = errors.New("operator email change not found")
)

// TokenDelivery is the recorded outcome of mailing a one-time link.
type TokenDelivery struct {
	SentAt     *time.Time
	Error      *string
	RetryCount int
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

// OperatorTokenQuery resolves operator invitation and e-mail change links.
type OperatorTokenQuery interface {
	// FindOperatorInvitation returns the invitation in any state.
	FindOperatorInvitation(ctx context.Context, id int64) (OperatorInvitation, error)
	FindRedeemableOperatorInvitation(ctx context.Context, token string) (OperatorInvitation, error)
	// ListRedeemableOperatorInvitations orders by creation, newest first.
	ListRedeemableOperatorInvitations(ctx context.Context) ([]OperatorInvitation, error)
	// CountOperatorInvitationsCreatedAfter counts every invitation the
	// operator created after since, spent or not; the invite rate limit
	// reads it.
	CountOperatorInvitationsCreatedAfter(ctx context.Context, createdBy int64, since time.Time) (int, error)
	// CountOperatorEmailChangesCreatedAfter counts every link created for
	// the operator after since, spent or not; the initiation rate limit
	// reads it.
	CountOperatorEmailChangesCreatedAfter(ctx context.Context, operatorID int64, since time.Time) (int, error)
}

// OperatorTokenCommand changes operator invitation and e-mail change links.
// The creates validate the link and return it with its identity and
// timestamps.
type OperatorTokenCommand interface {
	OperatorInvitationCommand
	OperatorEmailChangeCommand
}

// OperatorInvitationCommand issues, redeems and retires operator invitations.
type OperatorInvitationCommand interface {
	CreateOperatorInvitation(ctx context.Context, invitation OperatorInvitation) (OperatorInvitation, error)
	// RedeemOperatorInvitation spends a redeemable invitation exactly once.
	RedeemOperatorInvitation(ctx context.Context, token string) (OperatorInvitation, error)
	// RevokeOperatorInvitation spends an unused invitation.
	RevokeOperatorInvitation(ctx context.Context, id int64) error
	// RevokeOperatorInvitationsForEmail spends every unused invitation for
	// the address and returns how many it spent.
	RevokeOperatorInvitationsForEmail(ctx context.Context, email string) (int, error)
	// ExtendOperatorInvitation moves the expiry of a redeemable invitation
	// to the future expiresAt.
	ExtendOperatorInvitation(ctx context.Context, id int64, expiresAt time.Time) error
	RecordOperatorInvitationDelivery(ctx context.Context, id int64, delivery TokenDelivery) error
	DeleteExpiredOperatorInvitations(ctx context.Context) (int, error)
}

// OperatorEmailChangeCommand issues and redeems operator e-mail changes.
type OperatorEmailChangeCommand interface {
	CreateOperatorEmailChange(ctx context.Context, change OperatorEmailChange) (OperatorEmailChange, error)
	// RedeemOperatorEmailChange spends a redeemable link exactly once.
	RedeemOperatorEmailChange(ctx context.Context, token string) (OperatorEmailChange, error)
	// RevokeOperatorEmailChanges spends every unused link of the operator.
	RevokeOperatorEmailChanges(ctx context.Context, operatorID int64) error
	RecordOperatorEmailChangeDelivery(ctx context.Context, id int64, delivery TokenDelivery) error
	// RevokeExpiredOperatorEmailChanges spends expired, unused links so the
	// operator can request a new one, and returns how many it spent.
	RevokeExpiredOperatorEmailChanges(ctx context.Context) (int, error)
	// DeleteStaleOperatorEmailChanges deletes expired or spent links older
	// than the initiation rate-limit window.
	DeleteStaleOperatorEmailChanges(ctx context.Context) (int, error)
}

// OperatorLinkDeliveryRecorder records how the mail of an operator invitation
// or e-mail change link fared; the root's mail delivery reports through it.
type OperatorLinkDeliveryRecorder interface {
	RecordOperatorInvitationDelivery(ctx context.Context, id int64, delivery TokenDelivery) error
	RecordOperatorEmailChangeDelivery(ctx context.Context, id int64, delivery TokenDelivery) error
}

// OperatorTokens is the capability the operator invitation and e-mail
// change flows consume.
type OperatorTokens interface {
	OperatorTokenQuery
	OperatorTokenCommand
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
