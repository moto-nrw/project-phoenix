package identityaccess

import (
	"context"
	"errors"
	"fmt"
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
	// ErrOperatorEmailChangeActive reports a create that met the
	// one-active-link-per-operator index: that operator already has a live
	// link. The initiation flow answers its caller with the rate limit.
	ErrOperatorEmailChangeActive = errors.New("an active email change link already exists")
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

// OperatorTokens is the capability the operator invitation and e-mail
// change flows consume.
type OperatorTokens interface {
	OperatorTokenQuery
	OperatorTokenCommand
}

func (m *Module) FindOperatorInvitation(ctx context.Context, id int64) (OperatorInvitation, error) {
	invitation, err := m.engine.FindOperatorInvitation(ctx, id)
	if err != nil {
		return OperatorInvitation{}, fmt.Errorf("identity access: find operator invitation: %w", err)
	}
	return invitation, nil
}

func (m *Module) FindRedeemableOperatorInvitation(ctx context.Context, token string) (OperatorInvitation, error) {
	invitation, err := m.engine.FindRedeemableOperatorInvitation(ctx, token)
	if err != nil {
		return OperatorInvitation{}, fmt.Errorf("identity access: find redeemable operator invitation: %w", err)
	}
	return invitation, nil
}

func (m *Module) ListRedeemableOperatorInvitations(ctx context.Context) ([]OperatorInvitation, error) {
	invitations, err := m.engine.ListRedeemableOperatorInvitations(ctx)
	if err != nil {
		return nil, fmt.Errorf("identity access: list redeemable operator invitations: %w", err)
	}
	return invitations, nil
}

func (m *Module) CountOperatorInvitationsCreatedAfter(ctx context.Context, createdBy int64, since time.Time) (int, error) {
	count, err := m.engine.CountOperatorInvitationsCreatedAfter(ctx, createdBy, since)
	if err != nil {
		return 0, fmt.Errorf("identity access: count operator invitations: %w", err)
	}
	return count, nil
}

func (m *Module) CountOperatorEmailChangesCreatedAfter(ctx context.Context, operatorID int64, since time.Time) (int, error) {
	count, err := m.engine.CountOperatorEmailChangesCreatedAfter(ctx, operatorID, since)
	if err != nil {
		return 0, fmt.Errorf("identity access: count operator email changes: %w", err)
	}
	return count, nil
}

func (m *Module) CreateOperatorInvitation(ctx context.Context, invitation OperatorInvitation) (OperatorInvitation, error) {
	stored, err := m.engine.CreateOperatorInvitation(ctx, invitation)
	if err != nil {
		return OperatorInvitation{}, fmt.Errorf("identity access: create operator invitation: %w", err)
	}
	return stored, nil
}

func (m *Module) RedeemOperatorInvitation(ctx context.Context, token string) (OperatorInvitation, error) {
	invitation, err := m.engine.RedeemOperatorInvitation(ctx, token)
	if err != nil {
		return OperatorInvitation{}, fmt.Errorf("identity access: redeem operator invitation: %w", err)
	}
	return invitation, nil
}

func (m *Module) RevokeOperatorInvitation(ctx context.Context, id int64) error {
	if err := m.engine.RevokeOperatorInvitation(ctx, id); err != nil {
		return fmt.Errorf("identity access: revoke operator invitation: %w", err)
	}
	return nil
}

func (m *Module) RevokeOperatorInvitationsForEmail(ctx context.Context, email string) (int, error) {
	revoked, err := m.engine.RevokeOperatorInvitationsForEmail(ctx, email)
	if err != nil {
		return 0, fmt.Errorf("identity access: revoke operator invitations for email: %w", err)
	}
	return revoked, nil
}

func (m *Module) ExtendOperatorInvitation(ctx context.Context, id int64, expiresAt time.Time) error {
	if err := m.engine.ExtendOperatorInvitation(ctx, id, expiresAt); err != nil {
		return fmt.Errorf("identity access: extend operator invitation: %w", err)
	}
	return nil
}

func (m *Module) RecordOperatorInvitationDelivery(ctx context.Context, id int64, delivery TokenDelivery) error {
	if err := m.engine.RecordOperatorInvitationDelivery(ctx, id, delivery); err != nil {
		return fmt.Errorf("identity access: record operator invitation delivery: %w", err)
	}
	return nil
}

func (m *Module) DeleteExpiredOperatorInvitations(ctx context.Context) (int, error) {
	deleted, err := m.engine.DeleteExpiredOperatorInvitations(ctx)
	if err != nil {
		return 0, fmt.Errorf("identity access: delete expired operator invitations: %w", err)
	}
	return deleted, nil
}

func (m *Module) CreateOperatorEmailChange(ctx context.Context, change OperatorEmailChange) (OperatorEmailChange, error) {
	stored, err := m.engine.CreateOperatorEmailChange(ctx, change)
	if err != nil {
		return OperatorEmailChange{}, fmt.Errorf("identity access: create operator email change: %w", err)
	}
	return stored, nil
}

func (m *Module) RedeemOperatorEmailChange(ctx context.Context, token string) (OperatorEmailChange, error) {
	change, err := m.engine.RedeemOperatorEmailChange(ctx, token)
	if err != nil {
		return OperatorEmailChange{}, fmt.Errorf("identity access: redeem operator email change: %w", err)
	}
	return change, nil
}

func (m *Module) RevokeOperatorEmailChanges(ctx context.Context, operatorID int64) error {
	if err := m.engine.RevokeOperatorEmailChanges(ctx, operatorID); err != nil {
		return fmt.Errorf("identity access: revoke operator email changes: %w", err)
	}
	return nil
}

func (m *Module) RecordOperatorEmailChangeDelivery(ctx context.Context, id int64, delivery TokenDelivery) error {
	if err := m.engine.RecordOperatorEmailChangeDelivery(ctx, id, delivery); err != nil {
		return fmt.Errorf("identity access: record operator email change delivery: %w", err)
	}
	return nil
}

func (m *Module) RevokeExpiredOperatorEmailChanges(ctx context.Context) (int, error) {
	revoked, err := m.engine.RevokeExpiredOperatorEmailChanges(ctx)
	if err != nil {
		return 0, fmt.Errorf("identity access: revoke expired operator email changes: %w", err)
	}
	return revoked, nil
}

func (m *Module) DeleteStaleOperatorEmailChanges(ctx context.Context) (int, error) {
	deleted, err := m.engine.DeleteStaleOperatorEmailChanges(ctx)
	if err != nil {
		return 0, fmt.Errorf("identity access: delete stale operator email changes: %w", err)
	}
	return deleted, nil
}
