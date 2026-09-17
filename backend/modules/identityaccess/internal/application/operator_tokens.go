package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// staleEmailChangeAge keeps used and expired e-mail change links for the
// hour the initiation rate limit counts them.
const staleEmailChangeAge = time.Hour

// OperatorTokens serves the operator invitation and e-mail change links
// (#2722). The rows are platform-wide, so every operation runs through
// RunPlatform: it joins the administrative transaction a caller opened for a
// multi-statement flow (initiation, redemption) and otherwise executes on
// the root connection. The clock lives here, so expiry is decided once,
// against the same instant, for lookup, redemption and cleanup.
type OperatorTokens struct {
	service *Service
	store   ports.OperatorTokenStore
	now     func() time.Time
}

func NewOperatorTokens(service *Service, store ports.OperatorTokenStore) *OperatorTokens {
	if service == nil || store == nil {
		panic("identity access application: operator tokens require the service and their store")
	}
	return &OperatorTokens{service: service, store: store, now: time.Now}
}

func (t *OperatorTokens) run(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return t.service.run(ctx, t.service.tx.RunPlatform, operation, fn)
}

func (t *OperatorTokens) CreateInvitation(ctx context.Context, invitation domain.OperatorInvitation) (result domain.OperatorInvitation, err error) {
	err = t.run(ctx, "create_operator_invitation", func(txCtx context.Context, stats *domain.OperationStats) error {
		if validateErr := invitation.Validate(t.now()); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := t.store.InsertOperatorInvitation(txCtx, invitation)
		stats.Add(queryStats)
		result = stored
		return insertErr
	})
	return result, err
}

// FindInvitation returns the invitation in any state; the caller decides
// whether it is still usable.
func (t *OperatorTokens) FindInvitation(ctx context.Context, id int64) (result domain.OperatorInvitation, err error) {
	err = t.run(ctx, "find_operator_invitation", func(txCtx context.Context, stats *domain.OperationStats) error {
		invitation, found, queryStats, findErr := t.store.FindOperatorInvitation(txCtx, id)
		stats.Add(queryStats)
		result = invitation
		return stateChange(found, findErr, domain.ErrOperatorInvitationNotFound)
	})
	return result, err
}

func (t *OperatorTokens) FindRedeemableInvitation(ctx context.Context, token string) (result domain.OperatorInvitation, err error) {
	err = t.run(ctx, "find_redeemable_operator_invitation", func(txCtx context.Context, stats *domain.OperationStats) error {
		if token == "" {
			return domain.ErrOperatorInvitationNotFound
		}
		invitation, found, queryStats, findErr := t.store.FindRedeemableOperatorInvitation(txCtx, token, t.now())
		stats.Add(queryStats)
		result = invitation
		return stateChange(found, findErr, domain.ErrOperatorInvitationNotFound)
	})
	return result, err
}

func (t *OperatorTokens) ListRedeemableInvitations(ctx context.Context) (result []domain.OperatorInvitation, err error) {
	err = t.run(ctx, "list_redeemable_operator_invitations", func(txCtx context.Context, stats *domain.OperationStats) error {
		invitations, queryStats, listErr := t.store.ListRedeemableOperatorInvitations(txCtx, t.now())
		stats.Add(queryStats)
		result = invitations
		return listErr
	})
	return result, err
}

func (t *OperatorTokens) CountInvitationsCreatedAfter(ctx context.Context, createdBy int64, since time.Time) (result int, err error) {
	err = t.run(ctx, "count_operator_invitations", func(txCtx context.Context, stats *domain.OperationStats) error {
		count, queryStats, countErr := t.store.CountOperatorInvitationsCreatedAfter(txCtx, createdBy, since)
		stats.Add(queryStats)
		result = count
		return countErr
	})
	return result, err
}

// RedeemInvitation spends a redeemable invitation exactly once; the loser
// of two concurrent redemptions receives ErrOperatorInvitationNotFound.
func (t *OperatorTokens) RedeemInvitation(ctx context.Context, token string) (result domain.OperatorInvitation, err error) {
	err = t.run(ctx, "redeem_operator_invitation", func(txCtx context.Context, stats *domain.OperationStats) error {
		if token == "" {
			return domain.ErrOperatorInvitationNotFound
		}
		invitation, found, queryStats, redeemErr := t.store.RedeemOperatorInvitation(txCtx, token, t.now())
		stats.Add(queryStats)
		result = invitation
		return stateChange(found, redeemErr, domain.ErrOperatorInvitationNotFound)
	})
	return result, err
}

func (t *OperatorTokens) RevokeInvitation(ctx context.Context, id int64) error {
	return t.run(ctx, "revoke_operator_invitation", func(txCtx context.Context, stats *domain.OperationStats) error {
		changed, queryStats, err := t.store.RevokeOperatorInvitation(txCtx, id, t.now())
		stats.Add(queryStats)
		return stateChange(changed, err, domain.ErrOperatorInvitationNotFound)
	})
}

func (t *OperatorTokens) RevokeInvitationsForEmail(ctx context.Context, email string) (result int, err error) {
	err = t.run(ctx, "revoke_operator_invitations_for_email", func(txCtx context.Context, stats *domain.OperationStats) error {
		revoked, queryStats, revokeErr := t.store.RevokeOperatorInvitationsForEmail(txCtx, email, t.now())
		stats.Add(queryStats)
		result = revoked
		return revokeErr
	})
	return result, err
}

// ExtendInvitation moves the expiry of a still redeemable invitation. An
// invitation that expired or was spent in the meantime is not revived.
func (t *OperatorTokens) ExtendInvitation(ctx context.Context, id int64, expiresAt time.Time) error {
	return t.run(ctx, "extend_operator_invitation", func(txCtx context.Context, stats *domain.OperationStats) error {
		now := t.now()
		if !expiresAt.After(now) {
			return domain.ErrOperatorInvitationNotFound
		}
		changed, queryStats, err := t.store.ExtendOperatorInvitation(txCtx, id, expiresAt, now)
		stats.Add(queryStats)
		return stateChange(changed, err, domain.ErrOperatorInvitationNotFound)
	})
}

func (t *OperatorTokens) RecordInvitationDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) error {
	return t.run(ctx, "record_operator_invitation_delivery", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := t.store.RecordOperatorInvitationDelivery(txCtx, id, delivery.Bounded())
		stats.Add(queryStats)
		return err
	})
}

func (t *OperatorTokens) DeleteExpiredInvitations(ctx context.Context) (result int, err error) {
	err = t.run(ctx, "delete_expired_operator_invitations", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleted, queryStats, deleteErr := t.store.DeleteExpiredOperatorInvitations(txCtx, t.now())
		stats.Add(queryStats)
		result = deleted
		return deleteErr
	})
	return result, err
}

func (t *OperatorTokens) CreateEmailChange(ctx context.Context, change domain.OperatorEmailChange) (result domain.OperatorEmailChange, err error) {
	err = t.run(ctx, "create_operator_email_change", func(txCtx context.Context, stats *domain.OperationStats) error {
		if validateErr := change.Validate(t.now()); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := t.store.InsertOperatorEmailChange(txCtx, change)
		stats.Add(queryStats)
		result = stored
		return insertErr
	})
	return result, err
}

func (t *OperatorTokens) CountEmailChangesCreatedAfter(ctx context.Context, operatorID int64, since time.Time) (result int, err error) {
	err = t.run(ctx, "count_operator_email_changes", func(txCtx context.Context, stats *domain.OperationStats) error {
		count, queryStats, countErr := t.store.CountOperatorEmailChangesCreatedAfter(txCtx, operatorID, since)
		stats.Add(queryStats)
		result = count
		return countErr
	})
	return result, err
}

// RedeemEmailChange spends a redeemable link exactly once; the loser of two
// concurrent confirmations receives ErrOperatorEmailChangeNotFound.
func (t *OperatorTokens) RedeemEmailChange(ctx context.Context, token string) (result domain.OperatorEmailChange, err error) {
	err = t.run(ctx, "redeem_operator_email_change", func(txCtx context.Context, stats *domain.OperationStats) error {
		if token == "" {
			return domain.ErrOperatorEmailChangeNotFound
		}
		change, found, queryStats, redeemErr := t.store.RedeemOperatorEmailChange(txCtx, token, t.now())
		stats.Add(queryStats)
		result = change
		return stateChange(found, redeemErr, domain.ErrOperatorEmailChangeNotFound)
	})
	return result, err
}

func (t *OperatorTokens) RevokeEmailChanges(ctx context.Context, operatorID int64) error {
	return t.run(ctx, "revoke_operator_email_changes", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := t.store.RevokeOperatorEmailChanges(txCtx, operatorID)
		stats.Add(queryStats)
		return err
	})
}

func (t *OperatorTokens) RecordEmailChangeDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) error {
	return t.run(ctx, "record_operator_email_change_delivery", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := t.store.RecordOperatorEmailChangeDelivery(txCtx, id, delivery.Bounded())
		stats.Add(queryStats)
		return err
	})
}

func (t *OperatorTokens) RevokeExpiredEmailChanges(ctx context.Context) (result int, err error) {
	err = t.run(ctx, "revoke_expired_operator_email_changes", func(txCtx context.Context, stats *domain.OperationStats) error {
		revoked, queryStats, revokeErr := t.store.RevokeExpiredOperatorEmailChanges(txCtx, t.now())
		stats.Add(queryStats)
		result = revoked
		return revokeErr
	})
	return result, err
}

func (t *OperatorTokens) DeleteStaleEmailChanges(ctx context.Context) (result int, err error) {
	err = t.run(ctx, "delete_stale_operator_email_changes", func(txCtx context.Context, stats *domain.OperationStats) error {
		now := t.now()
		deleted, queryStats, deleteErr := t.store.DeleteStaleOperatorEmailChanges(txCtx, now.Add(-staleEmailChangeAge), now)
		stats.Add(queryStats)
		result = deleted
		return deleteErr
	})
	return result, err
}
