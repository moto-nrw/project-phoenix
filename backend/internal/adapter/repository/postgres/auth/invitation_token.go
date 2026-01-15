package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	modelAuth "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	invitationTable      = "auth.invitation_tokens"
	invitationTableAlias = `auth.invitation_tokens AS "invitation_token"`
)

// InvitationTokenRepository provides persistence for invitation tokens.
type InvitationTokenRepository struct {
	*base.Repository[*modelAuth.InvitationToken]
	db *bun.DB
}

// NewInvitationTokenRepository constructs a new repository instance.
func NewInvitationTokenRepository(db *bun.DB) modelAuth.InvitationTokenRepository {
	return &InvitationTokenRepository{
		Repository: base.NewRepository[*modelAuth.InvitationToken](db, "auth.invitation_tokens", "InvitationToken"),
		db:         db,
	}
}

// FindByToken fetches an invitation by its token value.
func (r *InvitationTokenRepository) FindByToken(ctx context.Context, token string) (*modelAuth.InvitationToken, error) {
	entity := new(modelAuth.InvitationToken)
	err := r.db.NewSelect().
		Model(entity).
		ModelTableExpr(invitationTableAlias).
		Where(`"invitation_token".token = ?`, token).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find invitation by token",
			Err: err,
		}
	}

	return entity, nil
}

// FindByID retrieves an invitation token by primary key.
func (r *InvitationTokenRepository) FindByID(ctx context.Context, id interface{}) (*modelAuth.InvitationToken, error) {
	entity := new(modelAuth.InvitationToken)
	if err := r.db.NewSelect().
		Model(entity).
		ModelTableExpr(invitationTableAlias).
		Where(`"invitation_token".id = ?`, id).
		Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find invitation by id",
			Err: err,
		}
	}
	return entity, nil
}

// Update persists changes to an invitation token.
func (r *InvitationTokenRepository) Update(ctx context.Context, token *modelAuth.InvitationToken) error {
	if token == nil {
		return fmt.Errorf("invitation token cannot be nil")
	}

	if _, err := r.db.NewUpdate().
		Model(token).
		ModelTableExpr(invitationTableAlias).
		WherePK().
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{
			Op:  "update invitation",
			Err: err,
		}
	}
	return nil
}

// FindValidByToken returns an invitation if it is not expired or used.
func (r *InvitationTokenRepository) FindValidByToken(ctx context.Context, token string, now time.Time) (*modelAuth.InvitationToken, error) {
	entity := new(modelAuth.InvitationToken)
	err := r.db.NewSelect().
		Model(entity).
		ModelTableExpr(invitationTableAlias).
		Where(`"invitation_token".token = ?`, token).
		Where(`"invitation_token".expires_at > ?`, now).
		Where(`"invitation_token".used_at IS NULL`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find valid invitation by token",
			Err: err,
		}
	}

	return entity, nil
}

// FindByEmail returns invitations associated with an email address.
func (r *InvitationTokenRepository) FindByEmail(ctx context.Context, email string) ([]*modelAuth.InvitationToken, error) {
	var tokens []*modelAuth.InvitationToken
	err := r.db.NewSelect().
		Model(&tokens).
		ModelTableExpr(invitationTableAlias).
		Where(`LOWER("invitation_token".email) = LOWER(?)`, email).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find invitations by email",
			Err: err,
		}
	}
	return tokens, nil
}

// MarkAsUsed sets the used_at timestamp for a token.
func (r *InvitationTokenRepository) MarkAsUsed(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*modelAuth.InvitationToken)(nil)).
		ModelTableExpr(invitationTable).
		Set(`used_at = NOW()`).
		Where(`id = ?`, id).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "mark invitation as used",
			Err: err,
		}
	}
	return nil
}

// InvalidateByEmail marks all invitations for an email as used.
func (r *InvitationTokenRepository) InvalidateByEmail(ctx context.Context, email string) (int, error) {
	res, err := r.db.NewUpdate().
		Model((*modelAuth.InvitationToken)(nil)).
		ModelTableExpr(invitationTable).
		Set(`used_at = NOW()`).
		Where(`LOWER(email) = LOWER(?)`, email).
		Where(`used_at IS NULL`).
		Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "invalidate invitations by email",
			Err: err,
		}
	}

	count, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve affected rows for invalidate invitations: %w", err)
	}

	return int(count), nil
}

// DeleteExpired removes invitations that can no longer be used.
func (r *InvitationTokenRepository) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	res, err := r.db.NewDelete().
		Model((*modelAuth.InvitationToken)(nil)).
		ModelTableExpr(invitationTable).
		Where(`expires_at <= ?`, now).
		WhereOr(`used_at IS NOT NULL`).
		Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete expired invitations",
			Err: err,
		}
	}

	count, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve affected rows for delete expired invitations: %w", err)
	}
	return int(count), nil
}

// List returns invitations filtered by the provided criteria.
func (r *InvitationTokenRepository) List(ctx context.Context, filters map[string]interface{}) ([]*modelAuth.InvitationToken, error) {
	var tokens []*modelAuth.InvitationToken
	query := r.db.NewSelect().
		Model(&tokens).
		ModelTableExpr(invitationTableAlias).
		ColumnExpr(`"invitation_token".*`).
		ColumnExpr(`"role"."id" AS "role__id"`).
		ColumnExpr(`"role"."created_at" AS "role__created_at"`).
		ColumnExpr(`"role"."updated_at" AS "role__updated_at"`).
		ColumnExpr(`"role"."name" AS "role__name"`).
		ColumnExpr(`"role"."description" AS "role__description"`).
		ColumnExpr(`"creator"."id" AS "creator__id"`).
		ColumnExpr(`"creator"."created_at" AS "creator__created_at"`).
		ColumnExpr(`"creator"."updated_at" AS "creator__updated_at"`).
		ColumnExpr(`"creator"."email" AS "creator__email"`).
		ColumnExpr(`"creator"."username" AS "creator__username"`).
		ColumnExpr(`"creator"."active" AS "creator__active"`).
		ColumnExpr(`"creator"."is_password_otp" AS "creator__is_password_otp"`).
		ColumnExpr(`"creator"."last_login" AS "creator__last_login"`).
		ColumnExpr(`"creator"."pin_attempts" AS "creator__pin_attempts"`).
		ColumnExpr(`"creator"."pin_locked_until" AS "creator__pin_locked_until"`).
		Join(`LEFT JOIN auth.roles AS "role" ON "role"."id" = "invitation_token"."role_id"`).
		Join(`LEFT JOIN auth.accounts AS "creator" ON "creator"."id" = "invitation_token"."created_by"`)

	now := time.Now()

	for key, value := range filters {
		query = r.applyInvitationFilter(query, key, value, now)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list invitation tokens",
			Err: err,
		}
	}

	return tokens, nil
}

// applyInvitationFilter applies a single filter to the query
func (r *InvitationTokenRepository) applyInvitationFilter(query *bun.SelectQuery, key string, value interface{}, now time.Time) *bun.SelectQuery {
	switch key {
	case "email":
		return r.applyEmailFilter(query, value)
	case "pending":
		return r.applyPendingFilter(query, value, now)
	case "expired":
		return r.applyExpiredFilter(query, value, now)
	case "used":
		return r.applyUsedFilter(query, value)
	default:
		return query
	}
}

// applyEmailFilter applies email filter with case-insensitive search
func (r *InvitationTokenRepository) applyEmailFilter(query *bun.SelectQuery, value interface{}) *bun.SelectQuery {
	if v, ok := value.(string); ok && v != "" {
		return query.Where(`LOWER("invitation_token".email) = LOWER(?)`, v)
	}
	return query
}

// applyPendingFilter applies pending status filter (not used and not expired)
func (r *InvitationTokenRepository) applyPendingFilter(query *bun.SelectQuery, value interface{}, now time.Time) *bun.SelectQuery {
	if pending, ok := value.(bool); ok && pending {
		return query.Where(`"invitation_token".used_at IS NULL`).Where(`"invitation_token".expires_at > ?`, now)
	}
	return query
}

// applyExpiredFilter applies expired status filter
func (r *InvitationTokenRepository) applyExpiredFilter(query *bun.SelectQuery, value interface{}, now time.Time) *bun.SelectQuery {
	if expired, ok := value.(bool); ok && expired {
		return query.Where(`"invitation_token".expires_at <= ?`, now)
	}
	return query
}

// applyUsedFilter applies used status filter
func (r *InvitationTokenRepository) applyUsedFilter(query *bun.SelectQuery, value interface{}) *bun.SelectQuery {
	if used, ok := value.(bool); ok && used {
		return query.Where(`"invitation_token".used_at IS NOT NULL`)
	}
	return query
}

// UpdateDeliveryResult updates the email delivery metadata for an invitation token.
func (r *InvitationTokenRepository) UpdateDeliveryResult(ctx context.Context, id int64, sentAt *time.Time, emailError *string, retryCount int) error {
	update := r.db.NewUpdate().
		Model((*modelAuth.InvitationToken)(nil)).
		ModelTableExpr(invitationTable).
		Where(`id = ?`, id).
		Set(`email_retry_count = ?`, retryCount)

	if sentAt != nil {
		update = update.Set(`email_sent_at = ?`, *sentAt)
	} else {
		update = update.Set(`email_sent_at = NULL`)
	}

	if emailError != nil {
		update = update.Set(`email_error = ?`, truncateError(*emailError))
	} else {
		update = update.Set(`email_error = NULL`)
	}

	if _, err := update.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{
			Op:  "update invitation delivery result",
			Err: err,
		}
	}
	return nil
}
