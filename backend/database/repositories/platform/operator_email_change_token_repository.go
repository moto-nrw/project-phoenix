package platform

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

const (
	operatorEmailChangeTokenTable      = "platform.operator_email_change_tokens"
	operatorEmailChangeTokenTableAlias = `platform.operator_email_change_tokens AS "operator_email_change_token"`

	maxEmailChangeErrorLength = 1024
)

// OperatorEmailChangeTokenRepository implements platform.OperatorEmailChangeTokenRepository
type OperatorEmailChangeTokenRepository struct {
	*base.Repository[*platform.OperatorEmailChangeToken]
	db *bun.DB
}

// NewOperatorEmailChangeTokenRepository creates a new OperatorEmailChangeTokenRepository
func NewOperatorEmailChangeTokenRepository(db *bun.DB) platform.OperatorEmailChangeTokenRepository {
	return &OperatorEmailChangeTokenRepository{
		Repository: base.NewRepository[*platform.OperatorEmailChangeToken](db, operatorEmailChangeTokenTable, "OperatorEmailChangeToken"),
		db:         db,
	}
}

// ConsumeByToken atomically marks a valid token as used and returns it.
// Returns (nil, nil) when no matching valid token exists.
// This prevents concurrent confirmations from consuming the same token twice,
// because the UPDATE ... WHERE used = FALSE only succeeds for one transaction.
func (r *OperatorEmailChangeTokenRepository) ConsumeByToken(ctx context.Context, tokenStr string) (*platform.OperatorEmailChangeToken, error) {
	token := new(platform.OperatorEmailChangeToken)
	err := base.GetDB(ctx, r.db).NewUpdate().
		Model(token).
		ModelTableExpr(operatorEmailChangeTokenTable).
		Set("used = TRUE").
		Where("token = ? AND expiry > ? AND used = FALSE", tokenStr, time.Now()).
		Returning("*").
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "consume email change token",
			Err: err,
		}
	}

	return token, nil
}

// InvalidateByOperatorID marks all pending tokens for an operator as used
func (r *OperatorEmailChangeTokenRepository) InvalidateByOperatorID(ctx context.Context, operatorID int64) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorEmailChangeToken)(nil)).
		ModelTableExpr(operatorEmailChangeTokenTable).
		Set("used = TRUE").
		Where("operator_id = ? AND used = FALSE", operatorID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "invalidate email change tokens by operator ID",
			Err: err,
		}
	}

	return nil
}

// UpdateDeliveryResult updates the email delivery metadata for a token
func (r *OperatorEmailChangeTokenRepository) UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error {
	token := &platform.OperatorEmailChangeToken{Model: modelBase.Model{ID: tokenID}, EmailSentAt: sentAt, EmailRetryCount: retryCount}
	if emailError != nil {
		truncated := strutil.TruncateRunes(*emailError, maxEmailChangeErrorLength, "")
		token.EmailError = &truncated
	}

	_, err := r.UpdateColumns(ctx, token, "email_sent_at", "email_error", "email_retry_count")
	return err
}

// CountRecentByOperatorID counts tokens created after `since` for rate limiting.
// Counts ALL tokens (including invalidated) because each initiation sends an email.
func (r *OperatorEmailChangeTokenRepository) CountRecentByOperatorID(ctx context.Context, operatorID int64, since time.Time) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*platform.OperatorEmailChangeToken)(nil)).
		ModelTableExpr(operatorEmailChangeTokenTableAlias).
		Where(`"operator_email_change_token".operator_id = ? AND "operator_email_change_token".created_at > ?`, operatorID, since).
		Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count recent email change tokens",
			Err: err,
		}
	}

	return count, nil
}

// InvalidateExpiredTokens marks expired-but-unused tokens as used so they no
// longer occupy the partial unique index (one active token per operator).
// Without this, an expired token blocks new email-change requests until
// DeleteStaleTokens removes the row (up to 1 hour after creation).
// The rate limit count (CountRecentByOperatorID) is unaffected because it
// counts ALL rows by created_at, regardless of used status.
func (r *OperatorEmailChangeTokenRepository) InvalidateExpiredTokens(ctx context.Context) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorEmailChangeToken)(nil)).
		ModelTableExpr(operatorEmailChangeTokenTable).
		Set("used = TRUE").
		Where("expiry < NOW() AND used = FALSE").
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "invalidate expired email change tokens",
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

// DeleteStaleTokens deletes tokens that are expired or used, but only if they
// are older than the 1-hour rate limit window. This preserves the rate limit
// count (CountRecentByOperatorID) which counts ALL tokens created in the last hour.
func (r *OperatorEmailChangeTokenRepository) DeleteStaleTokens(ctx context.Context) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*platform.OperatorEmailChangeToken)(nil)).
		ModelTableExpr(operatorEmailChangeTokenTable).
		Where("created_at < NOW() - INTERVAL '1 hour' AND (expiry < NOW() OR used = TRUE)").
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete stale email change tokens",
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
