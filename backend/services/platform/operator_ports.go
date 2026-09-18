package platform

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/platform"
)

// The operator identity rows (platform.operators) and the operator login
// flows are owned by Identity & Access (#2720, #3252). The retained e-mail
// change, invitation, MFA and passkey flows in this package reach them
// through the consumer-owned ports below; the serving root binds the ports
// to the public module. The same holds for the operator MFA records (#2723)
// the operator invitation and e-mail change links (#2722) and the operator
// passkey records (#2724).

// OperatorDirectory reads and writes operator rows for the retained flows.
// A missing row is (nil, nil), validation runs on the retained model before
// the owner sees the value, and the identity and timestamps are written
// back into the caller's value.
type OperatorDirectory interface {
	Create(ctx context.Context, operator *platform.Operator) error
	FindByID(ctx context.Context, id int64) (*platform.Operator, error)
	// FindByIDForUpdate locks the operator row for the caller's transaction
	// so concurrent credential changes serialize.
	FindByIDForUpdate(ctx context.Context, id int64) (*platform.Operator, error)
	FindByEmail(ctx context.Context, email string) (*platform.Operator, error)
	Update(ctx context.Context, operator *platform.Operator) error
	List(ctx context.Context) ([]*platform.Operator, error)
	// IncrementMFAAttempts atomically bumps mfa_attempts and applies the
	// lockout window once threshold is reached, so concurrent failed
	// verifications cannot collapse into a single counted attempt.
	IncrementMFAAttempts(ctx context.Context, id int64, threshold int, lockoutDuration time.Duration) (OperatorMFAAttempts, error)
	// ResetMFAAttempts atomically clears mfa_attempts and mfa_locked_until
	// after a successful verification.
	ResetMFAAttempts(ctx context.Context, id int64) error
}

// OperatorMFAAttempts is the counter snapshot after one failed MFA
// verification: the new attempt count and the lockout, if it was applied.
type OperatorMFAAttempts struct {
	Attempts    int
	LockedUntil *time.Time
}

// OperatorInvitationTokens reads and writes the operator invitation links,
// which Identity & Access owns (#2722). A missing or no longer redeemable
// row is (nil, nil) for the lookups and redemption and false for the state
// changes. Create runs the retained model validation, refuses an already
// expired link and writes the identity and timestamps back into the
// caller's value. Every call joins the caller's transaction when one is
// active.
type OperatorInvitationTokens interface {
	Create(ctx context.Context, token *platform.OperatorInvitationToken) error
	FindByID(ctx context.Context, id int64) (*platform.OperatorInvitationToken, error)
	FindValidByToken(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error)
	ConsumeByToken(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error)
	MarkAsUsed(ctx context.Context, id int64) (bool, error)
	ListPending(ctx context.Context) ([]*platform.OperatorInvitationToken, error)
	InvalidateByEmail(ctx context.Context, email string) (int, error)
	// ExtendExpiry never revives an expired or spent invitation.
	ExtendExpiry(ctx context.Context, id int64, newExpiresAt time.Time) (bool, error)
	UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error
	DeleteExpired(ctx context.Context) (int, error)
	CountRecentByCreatedBy(ctx context.Context, createdByID int64, since time.Time) (int, error)
}

// OperatorSessions is the operator login capability the retained MFA and
// passkey exchanges consume: a token pair for an operator whose identity
// was proven via a non-password channel. Errors arrive in the retained
// shapes (OperatorNotFoundError, OperatorInactiveError).
type OperatorSessions interface {
	IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
}

// OperatorInvitationFlows is the operator invitation capability Identity &
// Access owns since #3332: the link that lets an invitee become an
// operator, its public validation and acceptance, resend and revoke. The
// rate limit, the transaction ordering, the audit entry and the mail belong
// to the owner; what remains here is the envelope the operator routes
// classify on, which the composition root translates into. clientIP is
// recorded in the operator ledger.
type OperatorInvitationFlows interface {
	Invite(ctx context.Context, email string, displayName *string, createdByID int64, clientIP string) error
	ValidateInvitation(ctx context.Context, token string) (*platform.OperatorInvitationToken, error)
	AcceptInvitation(ctx context.Context, token, displayName, password, clientIP string) (*platform.Operator, error)
	// ListPendingInvitations returns the links that can still be spent.
	ListPendingInvitations(ctx context.Context) ([]*platform.OperatorInvitationToken, error)
	RevokeInvitation(ctx context.Context, invitationID, actorID int64, clientIP string) error
	ResendInvitation(ctx context.Context, invitationID, actorID int64, clientIP string) error
}

// OperatorEmailChangeFlows is the verified operator e-mail change Identity
// & Access owns since #3332, with the same division: the owner runs the
// flow, this package carries the envelope. ConfirmEmailChange answers with
// the address that is now the operator's.
type OperatorEmailChangeFlows interface {
	InitiateEmailChange(ctx context.Context, operatorID int64, newEmail, currentPassword, clientIP string) error
	ConfirmEmailChange(ctx context.Context, token, clientIP string) (string, error)
	// CleanupEmailChangeTokens spends the expired links so their operator
	// can request a new one, then deletes the rows nobody counts any more.
	CleanupEmailChangeTokens(ctx context.Context) (int, error)
}
