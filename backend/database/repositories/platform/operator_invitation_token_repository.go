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
	operatorInvitationTokenTable = "platform.operator_invitation_tokens"

	maxInvitationEmailErrorLength = 1024
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

// FindByID retrieves an invitation token by its ID
func (r *OperatorInvitationTokenRepository) FindByID(ctx context.Context, id int64) (*platform.OperatorInvitationToken, error) {
	token := new(platform.OperatorInvitationToken)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(token).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find invitation token by ID",
			Err: err,
		}
	}

	return token, nil
}

// FindValidByToken retrieves a valid (not expired, not used) token by its token string
func (r *OperatorInvitationTokenRepository) FindValidByToken(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
	token := new(platform.OperatorInvitationToken)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(token).
		Where("token = ?", tokenStr).
		Where("expires_at > NOW()").
		Where("used_at IS NULL").
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find valid invitation token",
			Err: err,
		}
	}

	return token, nil
}

// ConsumeByToken atomically marks a valid token as used and returns it.
// Returns (nil, nil) when no matching valid token exists.
// Only one concurrent transaction can succeed — the loser sees zero rows.
func (r *OperatorInvitationTokenRepository) ConsumeByToken(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
	token := new(platform.OperatorInvitationToken)
	err := base.GetDB(ctx, r.db).NewUpdate().
		Model(token).
		ModelTableExpr(operatorInvitationTokenTable).
		Set("used_at = NOW()").
		Where("token = ? AND expires_at > NOW() AND used_at IS NULL", tokenStr).
		Returning("*").
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "consume invitation token",
			Err: err,
		}
	}

	return token, nil
}

// MarkAsUsed marks an invitation token as used.
// Returns false if the token was already consumed (0 rows affected).
func (r *OperatorInvitationTokenRepository) MarkAsUsed(ctx context.Context, id int64) (bool, error) {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorInvitationToken)(nil)).
		ModelTableExpr(operatorInvitationTokenTable).
		Set("used_at = NOW()").
		Where("id = ? AND used_at IS NULL", id).
		Exec(ctx)

	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "mark invitation token as used",
			Err: err,
		}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "count affected rows",
			Err: err,
		}
	}

	return affected > 0, nil
}

// ListPending returns all pending (not used, not expired) invitation tokens
func (r *OperatorInvitationTokenRepository) ListPending(ctx context.Context) ([]*platform.OperatorInvitationToken, error) {
	var tokens []*platform.OperatorInvitationToken
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&tokens).
		Where("used_at IS NULL").
		Where("expires_at > NOW()").
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list pending invitation tokens",
			Err: err,
		}
	}

	return tokens, nil
}

// ExtendExpiry updates the expiration time of an invitation token.
// Only updates if the token is still unused. Returns false if already consumed.
func (r *OperatorInvitationTokenRepository) ExtendExpiry(ctx context.Context, id int64, newExpiresAt time.Time) (bool, error) {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorInvitationToken)(nil)).
		ModelTableExpr(operatorInvitationTokenTable).
		Set("expires_at = ?", newExpiresAt).
		Where("id = ? AND used_at IS NULL AND expires_at > NOW()", id).
		Exec(ctx)

	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "extend invitation token expiry",
			Err: err,
		}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "count affected rows",
			Err: err,
		}
	}

	return affected > 0, nil
}

// InvalidateByEmail marks all pending tokens for an email as used
func (r *OperatorInvitationTokenRepository) InvalidateByEmail(ctx context.Context, email string) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorInvitationToken)(nil)).
		ModelTableExpr(operatorInvitationTokenTable).
		Set("used_at = NOW()").
		Where("email = ? AND used_at IS NULL", email).
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "invalidate invitation tokens by email",
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

// UpdateDeliveryResult updates the email delivery metadata for a token
func (r *OperatorInvitationTokenRepository) UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error {
	token := &platform.OperatorInvitationToken{Model: modelBase.Model{ID: tokenID}, EmailSentAt: sentAt, EmailRetryCount: retryCount}
	if emailError != nil {
		truncated := strutil.TruncateRunes(*emailError, maxInvitationEmailErrorLength, "")
		token.EmailError = &truncated
	}

	_, err := r.UpdateColumns(ctx, token, "email_sent_at", "email_error", "email_retry_count")
	return err
}

// CountRecentByCreatedBy counts tokens created after `since` by the given
// inviter for per-operator rate limiting. Counts ALL rows (used or pending)
// because each successful Create dispatches an invitation email, so the
// rate limit must reflect total email-send volume, not currently-active tokens.
func (r *OperatorInvitationTokenRepository) CountRecentByCreatedBy(ctx context.Context, createdByID int64, since time.Time) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*platform.OperatorInvitationToken)(nil)).
		Where("created_by = ? AND created_at > ?", createdByID, since).
		Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count recent invitation tokens",
			Err: err,
		}
	}

	return count, nil
}

// DeleteExpired deletes all expired invitation tokens
func (r *OperatorInvitationTokenRepository) DeleteExpired(ctx context.Context) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*platform.OperatorInvitationToken)(nil)).
		ModelTableExpr(operatorInvitationTokenTable).
		Where("expires_at < NOW()").
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete expired invitation tokens",
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
