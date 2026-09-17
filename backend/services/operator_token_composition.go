package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services/platform"
)

// Identity & Access owns the operator invitation and e-mail change links
// (#2722). The adapters below serve the retained operator service's
// consumer-owned ports over the public module: a missing or no longer
// redeemable row is (nil, nil) or false, the retained model validation runs
// before the owner sees a new link, and the stored identity and timestamps
// are written back into the caller's value. Store errors keep their cause,
// so the retained flows still recognize unique violations.

type operatorInvitationTokens struct {
	tokens identityaccess.OperatorTokens
}

func newOperatorInvitationTokens(tokens identityaccess.OperatorTokens) platform.OperatorInvitationTokens {
	return operatorInvitationTokens{tokens: tokens}
}

func (t operatorInvitationTokens) Create(ctx context.Context, token *platformModels.OperatorInvitationToken) error {
	if token == nil {
		return fmt.Errorf("operator invitation token cannot be nil")
	}
	if err := token.Validate(); err != nil {
		return err
	}
	stored, err := t.tokens.CreateOperatorInvitation(ctx, identityaccess.OperatorInvitation{
		Email: token.Email, Token: token.Token, ExpiresAt: token.ExpiresAt, CreatedBy: token.CreatedBy, DisplayName: token.DisplayName,
	})
	if err != nil {
		return operatorDatabaseError("create operator invitation token", err)
	}
	*token = *operatorInvitationTokenModel(stored)
	return nil
}

func (t operatorInvitationTokens) FindByID(ctx context.Context, id int64) (*platformModels.OperatorInvitationToken, error) {
	invitation, err := t.tokens.FindOperatorInvitation(ctx, id)
	return invitationLookup(invitation, err, "find invitation token by ID")
}

func (t operatorInvitationTokens) FindValidByToken(ctx context.Context, tokenStr string) (*platformModels.OperatorInvitationToken, error) {
	invitation, err := t.tokens.FindRedeemableOperatorInvitation(ctx, tokenStr)
	return invitationLookup(invitation, err, "find valid invitation token")
}

func (t operatorInvitationTokens) ConsumeByToken(ctx context.Context, tokenStr string) (*platformModels.OperatorInvitationToken, error) {
	invitation, err := t.tokens.RedeemOperatorInvitation(ctx, tokenStr)
	return invitationLookup(invitation, err, "consume invitation token")
}

func invitationLookup(invitation identityaccess.OperatorInvitation, err error, op string) (*platformModels.OperatorInvitationToken, error) {
	if errors.Is(err, identityaccess.ErrOperatorInvitationNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, operatorDatabaseError(op, err)
	}
	return operatorInvitationTokenModel(invitation), nil
}

func (t operatorInvitationTokens) MarkAsUsed(ctx context.Context, id int64) (bool, error) {
	return stateChanged(t.tokens.RevokeOperatorInvitation(ctx, id), identityaccess.ErrOperatorInvitationNotFound, "mark invitation token as used")
}

func (t operatorInvitationTokens) ListPending(ctx context.Context) ([]*platformModels.OperatorInvitationToken, error) {
	invitations, err := t.tokens.ListRedeemableOperatorInvitations(ctx)
	if err != nil {
		return nil, operatorDatabaseError("list pending invitation tokens", err)
	}
	result := make([]*platformModels.OperatorInvitationToken, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, operatorInvitationTokenModel(invitation))
	}
	return result, nil
}

func (t operatorInvitationTokens) InvalidateByEmail(ctx context.Context, email string) (int, error) {
	revoked, err := t.tokens.RevokeOperatorInvitationsForEmail(ctx, email)
	if err != nil {
		return 0, operatorDatabaseError("invalidate invitation tokens by email", err)
	}
	return revoked, nil
}

func (t operatorInvitationTokens) ExtendExpiry(ctx context.Context, id int64, newExpiresAt time.Time) (bool, error) {
	return stateChanged(t.tokens.ExtendOperatorInvitation(ctx, id, newExpiresAt), identityaccess.ErrOperatorInvitationNotFound, "extend invitation token expiry")
}

func (t operatorInvitationTokens) UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error {
	delivery := identityaccess.TokenDelivery{SentAt: sentAt, Error: emailError, RetryCount: retryCount}
	if err := t.tokens.RecordOperatorInvitationDelivery(ctx, tokenID, delivery); err != nil {
		return operatorDatabaseError("update invitation delivery result", err)
	}
	return nil
}

func (t operatorInvitationTokens) DeleteExpired(ctx context.Context) (int, error) {
	deleted, err := t.tokens.DeleteExpiredOperatorInvitations(ctx)
	if err != nil {
		return 0, operatorDatabaseError("delete expired invitation tokens", err)
	}
	return deleted, nil
}

func (t operatorInvitationTokens) CountRecentByCreatedBy(ctx context.Context, createdByID int64, since time.Time) (int, error) {
	count, err := t.tokens.CountOperatorInvitationsCreatedAfter(ctx, createdByID, since)
	if err != nil {
		return 0, operatorDatabaseError("count recent invitation tokens", err)
	}
	return count, nil
}

type operatorEmailChangeTokens struct {
	tokens identityaccess.OperatorTokens
}

func newOperatorEmailChangeTokens(tokens identityaccess.OperatorTokens) platform.OperatorEmailChangeTokens {
	return operatorEmailChangeTokens{tokens: tokens}
}

func (t operatorEmailChangeTokens) Create(ctx context.Context, token *platformModels.OperatorEmailChangeToken) error {
	if token == nil {
		return fmt.Errorf("operator email change token cannot be nil")
	}
	if err := token.Validate(); err != nil {
		return err
	}
	stored, err := t.tokens.CreateOperatorEmailChange(ctx, identityaccess.OperatorEmailChange{
		OperatorID: token.OperatorID, NewEmail: token.NewEmail, Token: token.Token, Expiry: token.Expiry, Used: token.Used,
	})
	if err != nil {
		return operatorDatabaseError("create operator email change token", err)
	}
	*token = *operatorEmailChangeTokenModel(stored)
	return nil
}

func (t operatorEmailChangeTokens) ConsumeByToken(ctx context.Context, tokenStr string) (*platformModels.OperatorEmailChangeToken, error) {
	change, err := t.tokens.RedeemOperatorEmailChange(ctx, tokenStr)
	if errors.Is(err, identityaccess.ErrOperatorEmailChangeNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, operatorDatabaseError("consume email change token", err)
	}
	return operatorEmailChangeTokenModel(change), nil
}

func (t operatorEmailChangeTokens) InvalidateByOperatorID(ctx context.Context, operatorID int64) error {
	if err := t.tokens.RevokeOperatorEmailChanges(ctx, operatorID); err != nil {
		return operatorDatabaseError("invalidate email change tokens by operator ID", err)
	}
	return nil
}

func (t operatorEmailChangeTokens) UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error {
	delivery := identityaccess.TokenDelivery{SentAt: sentAt, Error: emailError, RetryCount: retryCount}
	if err := t.tokens.RecordOperatorEmailChangeDelivery(ctx, tokenID, delivery); err != nil {
		return operatorDatabaseError("update email change delivery result", err)
	}
	return nil
}

func (t operatorEmailChangeTokens) CountRecentByOperatorID(ctx context.Context, operatorID int64, since time.Time) (int, error) {
	count, err := t.tokens.CountOperatorEmailChangesCreatedAfter(ctx, operatorID, since)
	if err != nil {
		return 0, operatorDatabaseError("count recent email change tokens", err)
	}
	return count, nil
}

func (t operatorEmailChangeTokens) InvalidateExpiredTokens(ctx context.Context) (int, error) {
	revoked, err := t.tokens.RevokeExpiredOperatorEmailChanges(ctx)
	if err != nil {
		return 0, operatorDatabaseError("invalidate expired email change tokens", err)
	}
	return revoked, nil
}

func (t operatorEmailChangeTokens) DeleteStaleTokens(ctx context.Context) (int, error) {
	deleted, err := t.tokens.DeleteStaleOperatorEmailChanges(ctx)
	if err != nil {
		return 0, operatorDatabaseError("delete stale email change tokens", err)
	}
	return deleted, nil
}

// stateChanged reports a state change the owner refused as false.
func stateChanged(err, refused error, op string) (bool, error) {
	if errors.Is(err, refused) {
		return false, nil
	}
	if err != nil {
		return false, operatorDatabaseError(op, err)
	}
	return true, nil
}

func operatorInvitationTokenModel(src identityaccess.OperatorInvitation) *platformModels.OperatorInvitationToken {
	token := &platformModels.OperatorInvitationToken{
		Email: src.Email, Token: src.Token, ExpiresAt: src.ExpiresAt, UsedAt: src.UsedAt, CreatedBy: src.CreatedBy,
		DisplayName: src.DisplayName, EmailSentAt: src.Delivery.SentAt, EmailError: src.Delivery.Error,
		EmailRetryCount: src.Delivery.RetryCount,
	}
	token.ID, token.CreatedAt, token.UpdatedAt = src.ID, src.CreatedAt, src.UpdatedAt
	return token
}

func operatorEmailChangeTokenModel(src identityaccess.OperatorEmailChange) *platformModels.OperatorEmailChangeToken {
	token := &platformModels.OperatorEmailChangeToken{
		OperatorID: src.OperatorID, NewEmail: src.NewEmail, Token: src.Token, Expiry: src.Expiry, Used: src.Used,
		EmailSentAt: src.Delivery.SentAt, EmailError: src.Delivery.Error, EmailRetryCount: src.Delivery.RetryCount,
	}
	token.ID, token.CreatedAt, token.UpdatedAt = src.ID, src.CreatedAt, src.UpdatedAt
	return token
}
