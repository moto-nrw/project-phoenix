package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
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
	return t.service.run(ctx, t.service.tx.RunAdmin, operation, fn)
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
		return stateChange(found, findErr, identityaccess.ErrOperatorInvitationNotFound)
	})
	return result, err
}

func (t *OperatorTokens) FindRedeemableInvitation(ctx context.Context, token string) (result domain.OperatorInvitation, err error) {
	err = t.run(ctx, "find_redeemable_operator_invitation", func(txCtx context.Context, stats *domain.OperationStats) error {
		if token == "" {
			return identityaccess.ErrOperatorInvitationNotFound
		}
		invitation, found, queryStats, findErr := t.store.FindRedeemableOperatorInvitation(txCtx, token, t.now())
		stats.Add(queryStats)
		result = invitation
		return stateChange(found, findErr, identityaccess.ErrOperatorInvitationNotFound)
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
			return identityaccess.ErrOperatorInvitationNotFound
		}
		invitation, found, queryStats, redeemErr := t.store.RedeemOperatorInvitation(txCtx, token, t.now())
		stats.Add(queryStats)
		result = invitation
		return stateChange(found, redeemErr, identityaccess.ErrOperatorInvitationNotFound)
	})
	return result, err
}

func (t *OperatorTokens) RevokeInvitation(ctx context.Context, id int64) error {
	return t.run(ctx, "revoke_operator_invitation", func(txCtx context.Context, stats *domain.OperationStats) error {
		changed, queryStats, err := t.store.RevokeOperatorInvitation(txCtx, id, t.now())
		stats.Add(queryStats)
		return stateChange(changed, err, identityaccess.ErrOperatorInvitationNotFound)
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
			return identityaccess.ErrOperatorInvitationNotFound
		}
		changed, queryStats, err := t.store.ExtendOperatorInvitation(txCtx, id, expiresAt, now)
		stats.Add(queryStats)
		return stateChange(changed, err, identityaccess.ErrOperatorInvitationNotFound)
	})
}

func (t *OperatorTokens) RecordInvitationDelivery(ctx context.Context, id int64, delivery domain.TokenDelivery) error {
	return t.run(ctx, "record_operator_invitation_delivery", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := t.store.RecordOperatorInvitationDelivery(txCtx, id, delivery.Bounded())
		stats.Add(queryStats)
		return err
	})
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
			return identityaccess.ErrOperatorEmailChangeNotFound
		}
		change, found, queryStats, redeemErr := t.store.RedeemOperatorEmailChange(txCtx, token, t.now())
		stats.Add(queryStats)
		result = change
		return stateChange(found, redeemErr, identityaccess.ErrOperatorEmailChangeNotFound)
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

func (t *OperatorTokens) FindOperatorInvitation(ctx context.Context, id int64) (identityaccess.OperatorInvitation, error) {
	value, err := t.FindInvitation(ctx, id)
	if err != nil {
		return identityaccess.OperatorInvitation{}, fmt.Errorf("identity access: find operator invitation: %w", MapError(err))
	}
	return PublicOperatorInvitation(value), nil
}

func (t *OperatorTokens) FindRedeemableOperatorInvitation(ctx context.Context, token string) (identityaccess.OperatorInvitation, error) {
	value, err := t.FindRedeemableInvitation(ctx, token)
	if err != nil {
		return identityaccess.OperatorInvitation{}, fmt.Errorf("identity access: find redeemable operator invitation: %w", MapError(err))
	}
	return PublicOperatorInvitation(value), nil
}

func (t *OperatorTokens) ListRedeemableOperatorInvitations(ctx context.Context) ([]identityaccess.OperatorInvitation, error) {
	values, err := t.ListRedeemableInvitations(ctx)
	if err != nil {
		return nil, fmt.Errorf("identity access: list redeemable operator invitations: %w", MapError(err))
	}
	result := make([]identityaccess.OperatorInvitation, 0, len(values))
	for _, value := range values {
		result = append(result, PublicOperatorInvitation(value))
	}
	return result, nil
}

func (t *OperatorTokens) CountOperatorInvitationsCreatedAfter(ctx context.Context, createdBy int64, since time.Time) (int, error) {
	count, err := t.CountInvitationsCreatedAfter(ctx, createdBy, since)
	if err != nil {
		return 0, fmt.Errorf("identity access: count operator invitations: %w", MapError(err))
	}
	return count, nil
}

func (t *OperatorTokens) CountOperatorEmailChangesCreatedAfter(ctx context.Context, operatorID int64, since time.Time) (int, error) {
	count, err := t.CountEmailChangesCreatedAfter(ctx, operatorID, since)
	if err != nil {
		return 0, fmt.Errorf("identity access: count operator email changes: %w", MapError(err))
	}
	return count, nil
}

func (t *OperatorTokens) CreateOperatorInvitation(ctx context.Context, invitation identityaccess.OperatorInvitation) (identityaccess.OperatorInvitation, error) {

	value, err := t.CreateInvitation(ctx, domain.OperatorInvitation{
		Email: invitation.Email, Token: invitation.Token, ExpiresAt: invitation.ExpiresAt, UsedAt: invitation.UsedAt,
		CreatedBy: invitation.CreatedBy, DisplayName: invitation.DisplayName,
	})
	if err != nil {
		return identityaccess.OperatorInvitation{}, fmt.Errorf("identity access: create operator invitation: %w", MapError(err))
	}
	return PublicOperatorInvitation(value), nil
}

func (t *OperatorTokens) RedeemOperatorInvitation(ctx context.Context, token string) (identityaccess.OperatorInvitation, error) {
	value, err := t.RedeemInvitation(ctx, token)
	if err != nil {
		return identityaccess.OperatorInvitation{}, fmt.Errorf("identity access: redeem operator invitation: %w", MapError(err))
	}
	return PublicOperatorInvitation(value), nil
}

func (t *OperatorTokens) RevokeOperatorInvitation(ctx context.Context, id int64) error {
	if err := t.RevokeInvitation(ctx, id); err != nil {
		return fmt.Errorf("identity access: revoke operator invitation: %w", MapError(err))
	}
	return nil
}

func (t *OperatorTokens) RevokeOperatorInvitationsForEmail(ctx context.Context, email string) (int, error) {
	revoked, err := t.RevokeInvitationsForEmail(ctx, email)
	if err != nil {
		return 0, fmt.Errorf("identity access: revoke operator invitations for email: %w", MapError(err))
	}
	return revoked, nil
}

func (t *OperatorTokens) ExtendOperatorInvitation(ctx context.Context, id int64, expiresAt time.Time) error {
	if err := t.ExtendInvitation(ctx, id, expiresAt); err != nil {
		return fmt.Errorf("identity access: extend operator invitation: %w", MapError(err))
	}
	return nil
}

func (t *OperatorTokens) RecordOperatorInvitationDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error {
	if err := t.RecordInvitationDelivery(ctx, id, domain.TokenDelivery(delivery)); err != nil {
		return fmt.Errorf("identity access: record operator invitation delivery: %w", MapError(err))
	}
	return nil
}

func (t *OperatorTokens) DeleteExpiredOperatorInvitations(ctx context.Context) (int, error) {
	var deleted int
	err := t.run(ctx, "delete_expired_operator_invitations", func(txCtx context.Context, stats *domain.OperationStats) error {
		count, queryStats, deleteErr := t.store.DeleteExpiredOperatorInvitations(txCtx, t.now())
		stats.Add(queryStats)
		deleted = count
		return deleteErr
	})
	if err != nil {
		return 0, fmt.Errorf("identity access: delete expired operator invitations: %w", MapError(err))
	}
	return deleted, nil
}

func (t *OperatorTokens) CreateOperatorEmailChange(ctx context.Context, change identityaccess.OperatorEmailChange) (identityaccess.OperatorEmailChange, error) {
	value, err := t.CreateEmailChange(ctx, domain.OperatorEmailChange{
		OperatorID: change.OperatorID, NewEmail: change.NewEmail, Token: change.Token, Expiry: change.Expiry, Used: change.Used,
	})
	if err != nil {
		return identityaccess.OperatorEmailChange{}, fmt.Errorf("identity access: create operator email change: %w", MapError(err))
	}
	return PublicOperatorEmailChange(value), nil
}

func (t *OperatorTokens) RedeemOperatorEmailChange(ctx context.Context, token string) (identityaccess.OperatorEmailChange, error) {
	value, err := t.RedeemEmailChange(ctx, token)
	if err != nil {
		return identityaccess.OperatorEmailChange{}, fmt.Errorf("identity access: redeem operator email change: %w", MapError(err))
	}
	return PublicOperatorEmailChange(value), nil
}

func (t *OperatorTokens) RevokeOperatorEmailChanges(ctx context.Context, operatorID int64) error {
	if err := t.RevokeEmailChanges(ctx, operatorID); err != nil {
		return fmt.Errorf("identity access: revoke operator email changes: %w", MapError(err))
	}
	return nil
}

func (t *OperatorTokens) RecordOperatorEmailChangeDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error {
	err := t.run(ctx, "record_operator_email_change_delivery", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := t.store.RecordOperatorEmailChangeDelivery(txCtx, id, domain.TokenDelivery(delivery).Bounded())
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return fmt.Errorf("identity access: record operator email change delivery: %w", MapError(err))
	}
	return nil
}

func (t *OperatorTokens) RevokeExpiredOperatorEmailChanges(ctx context.Context) (int, error) {
	revoked, err := t.RevokeExpiredEmailChanges(ctx)
	if err != nil {
		return 0, fmt.Errorf("identity access: revoke expired operator email changes: %w", MapError(err))
	}
	return revoked, nil
}

func (t *OperatorTokens) DeleteStaleOperatorEmailChanges(ctx context.Context) (int, error) {
	deleted, err := t.DeleteStaleEmailChanges(ctx)
	if err != nil {
		return 0, fmt.Errorf("identity access: delete stale operator email changes: %w", MapError(err))
	}
	return deleted, nil
}

func PublicOperatorInvitation(value domain.OperatorInvitation) identityaccess.OperatorInvitation {
	return identityaccess.OperatorInvitation{
		ID: value.ID, Email: value.Email, Token: value.Token, ExpiresAt: value.ExpiresAt, UsedAt: value.UsedAt,
		CreatedBy: value.CreatedBy, DisplayName: value.DisplayName, Delivery: identityaccess.TokenDelivery(value.Delivery),
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func PublicOperatorEmailChange(value domain.OperatorEmailChange) identityaccess.OperatorEmailChange {
	return identityaccess.OperatorEmailChange{
		ID: value.ID, OperatorID: value.OperatorID, NewEmail: value.NewEmail, Token: value.Token, Expiry: value.Expiry,
		Used: value.Used, Delivery: identityaccess.TokenDelivery(value.Delivery), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}
