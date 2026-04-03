package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

const (
	operatorInvitationTokenTable      = "platform.operator_invitation_tokens"
	operatorInvitationTokenTableAlias = `platform.operator_invitation_tokens AS "operator_invitation_token"`

	maxInvitationErrorLength = 1024
)

// OperatorInvitationTokenRepository implements platform.OperatorInvitationTokenRepository
type OperatorInvitationTokenRepository struct {
	*base.Repository[*platform.OperatorInvitationToken]
	db *bun.DB
}

// NewOperatorInvitationTokenRepository creates a new OperatorInvitationTokenRepository
func NewOperatorInvitationTokenRepository(db *bun.DB) platform.OperatorInvitationTokenRepository {
	return &OperatorInvitationTokenRepository{
		Repository: base.NewRepository[*platform.OperatorInvitationToken](db, operatorInvitationTokenTable, "OperatorInvitationToken"),
		db:         db,
	}
}

// Create creates a new operator invitation token
func (r *OperatorInvitationTokenRepository) Create(ctx context.Context, token *platform.OperatorInvitationToken) error {
	if token == nil {
		return fmt.Errorf("invitation token cannot be nil")
	}

	if err := token.Validate(); err != nil {
		return err
	}

	return r.Repository.Create(ctx, token)
}

// FindByID retrieves an invitation token by its primary key
func (r *OperatorInvitationTokenRepository) FindByID(ctx context.Context, id int64) (*platform.OperatorInvitationToken, error) {
	token := new(platform.OperatorInvitationToken)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(token).
		ModelTableExpr(operatorInvitationTokenTableAlias).
		Relation("Inviter").
		Where(`"operator_invitation_token".id = ?`, id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find operator invitation token by ID",
			Err: err,
		}
	}

	return token, nil
}

// FindValidByToken retrieves a valid (not expired, not used) invitation token with its inviter.
// Returns (nil, nil) when no matching valid token exists.
func (r *OperatorInvitationTokenRepository) FindValidByToken(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
	token := new(platform.OperatorInvitationToken)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(token).
		ModelTableExpr(operatorInvitationTokenTableAlias).
		Relation("Inviter").
		Where(`"operator_invitation_token".token = ?`, tokenStr).
		Where(`"operator_invitation_token".expiry > ?`, time.Now()).
		Where(`"operator_invitation_token".used = FALSE`).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find valid operator invitation token",
			Err: err,
		}
	}

	return token, nil
}

// ConsumeByToken atomically marks a valid token as used and returns it.
// Returns (nil, nil) when no matching valid token exists.
func (r *OperatorInvitationTokenRepository) ConsumeByToken(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
	token := new(platform.OperatorInvitationToken)
	err := base.GetDB(ctx, r.db).NewUpdate().
		Model(token).
		ModelTableExpr(operatorInvitationTokenTable).
		Set("used = TRUE").
		Where("token = ? AND expiry > ? AND used = FALSE", tokenStr, time.Now()).
		Returning("*").
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "consume operator invitation token",
			Err: err,
		}
	}

	return token, nil
}

// InvalidateByEmail marks all pending tokens for an email as used
func (r *OperatorInvitationTokenRepository) InvalidateByEmail(ctx context.Context, email string) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorInvitationToken)(nil)).
		ModelTableExpr(operatorInvitationTokenTable).
		Set("used = TRUE").
		Where("email = ? AND used = FALSE", email).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "invalidate operator invitation tokens by email",
			Err: err,
		}
	}

	return nil
}

// UpdateDeliveryResult updates the email delivery metadata for a token
func (r *OperatorInvitationTokenRepository) UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error {
	update := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorInvitationToken)(nil)).
		ModelTableExpr(operatorInvitationTokenTable).
		Where("id = ?", tokenID).
		Set("email_retry_count = ?", retryCount)

	if sentAt != nil {
		update = update.Set("email_sent_at = ?", *sentAt)
	} else {
		update = update.Set("email_sent_at = NULL")
	}

	if emailError != nil {
		update = update.Set("email_error = ?", truncateInvitationError(*emailError))
	} else {
		update = update.Set("email_error = NULL")
	}

	if _, err := update.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{
			Op:  "update operator invitation delivery result",
			Err: err,
		}
	}

	return nil
}

// ListPending returns all non-expired, non-used invitation tokens with inviter info
func (r *OperatorInvitationTokenRepository) ListPending(ctx context.Context) ([]*platform.OperatorInvitationToken, error) {
	var tokens []*platform.OperatorInvitationToken
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&tokens).
		ModelTableExpr(operatorInvitationTokenTableAlias).
		Relation("Inviter").
		Where(`"operator_invitation_token".used = FALSE`).
		Where(`"operator_invitation_token".expiry > ?`, time.Now()).
		OrderExpr(`"operator_invitation_token".created_at DESC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list pending operator invitation tokens",
			Err: err,
		}
	}

	return tokens, nil
}

// InvalidateExpiredTokens marks expired-but-unused tokens as used so they no
// longer occupy the partial unique index (one active token per email).
func (r *OperatorInvitationTokenRepository) InvalidateExpiredTokens(ctx context.Context) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorInvitationToken)(nil)).
		ModelTableExpr(operatorInvitationTokenTable).
		Set("used = TRUE").
		Where("expiry < NOW() AND used = FALSE").
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "invalidate expired operator invitation tokens",
			Err: err,
		}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count affected rows",
			Err: err,
		}
	}

	return int(affected), nil
}

// DeleteStaleTokens deletes tokens that are expired or used and older than 48 hours.
func (r *OperatorInvitationTokenRepository) DeleteStaleTokens(ctx context.Context) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*platform.OperatorInvitationToken)(nil)).
		ModelTableExpr(operatorInvitationTokenTable).
		Where("created_at < NOW() - INTERVAL '48 hours' AND (expiry < NOW() OR used = TRUE)").
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete stale operator invitation tokens",
			Err: err,
		}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count affected rows",
			Err: err,
		}
	}

	return int(affected), nil
}

func truncateInvitationError(msg string) string {
	if msg == "" {
		return ""
	}
	runes := []rune(msg)
	if len(runes) <= maxInvitationErrorLength {
		return msg
	}
	return string(runes[:maxInvitationErrorLength])
}
