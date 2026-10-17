package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Engine methods for the operator invitation and e-mail change links
// (#2722). They need no dependencies beyond the database, so every
// composition serves them.

func (e engine) FindOperatorInvitation(ctx context.Context, id int64) (identityaccess.OperatorInvitation, error) {
	value, err := e.tokens.FindInvitation(ctx, id)
	return PublicOperatorInvitation(value), MapError(err)
}

func (e engine) FindRedeemableOperatorInvitation(ctx context.Context, token string) (identityaccess.OperatorInvitation, error) {
	value, err := e.tokens.FindRedeemableInvitation(ctx, token)
	return PublicOperatorInvitation(value), MapError(err)
}

func (e engine) ListRedeemableOperatorInvitations(ctx context.Context) ([]identityaccess.OperatorInvitation, error) {
	values, err := e.tokens.ListRedeemableInvitations(ctx)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]identityaccess.OperatorInvitation, 0, len(values))
	for _, value := range values {
		result = append(result, PublicOperatorInvitation(value))
	}
	return result, nil
}

func (e engine) CountOperatorInvitationsCreatedAfter(ctx context.Context, createdBy int64, since time.Time) (int, error) {
	count, err := e.tokens.CountInvitationsCreatedAfter(ctx, createdBy, since)
	return count, MapError(err)
}

func (e engine) CountOperatorEmailChangesCreatedAfter(ctx context.Context, operatorID int64, since time.Time) (int, error) {
	count, err := e.tokens.CountEmailChangesCreatedAfter(ctx, operatorID, since)
	return count, MapError(err)
}

func (e engine) CreateOperatorInvitation(ctx context.Context, invitation identityaccess.OperatorInvitation) (identityaccess.OperatorInvitation, error) {
	// UsedAt is passed so validation refuses a spent link; identity,
	// delivery and timestamps are assigned by the store.
	value, err := e.tokens.CreateInvitation(ctx, domain.OperatorInvitation{
		Email: invitation.Email, Token: invitation.Token, ExpiresAt: invitation.ExpiresAt, UsedAt: invitation.UsedAt,
		CreatedBy: invitation.CreatedBy, DisplayName: invitation.DisplayName,
	})
	return PublicOperatorInvitation(value), MapError(err)
}

func (e engine) RedeemOperatorInvitation(ctx context.Context, token string) (identityaccess.OperatorInvitation, error) {
	value, err := e.tokens.RedeemInvitation(ctx, token)
	return PublicOperatorInvitation(value), MapError(err)
}

func (e engine) RevokeOperatorInvitation(ctx context.Context, id int64) error {
	return MapError(e.tokens.RevokeInvitation(ctx, id))
}

func (e engine) RevokeOperatorInvitationsForEmail(ctx context.Context, email string) (int, error) {
	revoked, err := e.tokens.RevokeInvitationsForEmail(ctx, email)
	return revoked, MapError(err)
}

func (e engine) ExtendOperatorInvitation(ctx context.Context, id int64, expiresAt time.Time) error {
	return MapError(e.tokens.ExtendInvitation(ctx, id, expiresAt))
}

func (e engine) RecordOperatorInvitationDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error {
	return MapError(e.tokens.RecordInvitationDelivery(ctx, id, domain.TokenDelivery(delivery)))
}

func (e engine) DeleteExpiredOperatorInvitations(ctx context.Context) (int, error) {
	deleted, err := e.tokens.DeleteExpiredInvitations(ctx)
	return deleted, MapError(err)
}

func (e engine) CreateOperatorEmailChange(ctx context.Context, change identityaccess.OperatorEmailChange) (identityaccess.OperatorEmailChange, error) {
	value, err := e.tokens.CreateEmailChange(ctx, domain.OperatorEmailChange{
		OperatorID: change.OperatorID, NewEmail: change.NewEmail, Token: change.Token, Expiry: change.Expiry, Used: change.Used,
	})
	return PublicOperatorEmailChange(value), MapError(err)
}

func (e engine) RedeemOperatorEmailChange(ctx context.Context, token string) (identityaccess.OperatorEmailChange, error) {
	value, err := e.tokens.RedeemEmailChange(ctx, token)
	return PublicOperatorEmailChange(value), MapError(err)
}

func (e engine) RevokeOperatorEmailChanges(ctx context.Context, operatorID int64) error {
	return MapError(e.tokens.RevokeEmailChanges(ctx, operatorID))
}

func (e engine) RecordOperatorEmailChangeDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error {
	return MapError(e.tokens.RecordEmailChangeDelivery(ctx, id, domain.TokenDelivery(delivery)))
}

func (e engine) RevokeExpiredOperatorEmailChanges(ctx context.Context) (int, error) {
	revoked, err := e.tokens.RevokeExpiredEmailChanges(ctx)
	return revoked, MapError(err)
}

func (e engine) DeleteStaleOperatorEmailChanges(ctx context.Context) (int, error) {
	deleted, err := e.tokens.DeleteStaleEmailChanges(ctx)
	return deleted, MapError(err)
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
