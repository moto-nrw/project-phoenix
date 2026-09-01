package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	modelAuth "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	repo := base.NewRepository[*modelAuth.InvitationToken](db, "auth.invitation_tokens", "InvitationToken")
	repo.TenantScoped = true
	return &InvitationTokenRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByToken fetches an invitation by its token value.
func (r *InvitationTokenRepository) FindByToken(ctx context.Context, token string) (*modelAuth.InvitationToken, error) {
	entity := new(modelAuth.InvitationToken)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(entity).
		ModelTableExpr(invitationTableAlias).
		Where(`"invitation_token".token = ?`, token)

	query = base.WithTenantFilter(ctx, query, "invitation_token")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find invitation by token",
			Err: base.TranslateNotFound(err),
		}
	}

	return entity, nil
}

// FindByID retrieves an invitation token by primary key.
func (r *InvitationTokenRepository) FindByID(ctx context.Context, id interface{}) (*modelAuth.InvitationToken, error) {
	entity := new(modelAuth.InvitationToken)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(entity).
		ModelTableExpr(invitationTableAlias).
		Where(`"invitation_token".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "invitation_token")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find invitation by id",
			Err: base.TranslateNotFound(err),
		}
	}
	return entity, nil
}

// Update persists changes to an invitation token.
func (r *InvitationTokenRepository) Update(ctx context.Context, token *modelAuth.InvitationToken) error {
	if token == nil {
		return fmt.Errorf("invitation token cannot be nil")
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(token).
		ModelTableExpr(invitationTableAlias).
		WherePK()

	query = base.WithTenantFilter(ctx, query, "invitation_token")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update invitation",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "update invitation")
}

// FindValidByToken returns an invitation if it is not expired or used.
func (r *InvitationTokenRepository) FindValidByToken(ctx context.Context, token string, now time.Time) (*modelAuth.InvitationToken, error) {
	entity := new(modelAuth.InvitationToken)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(entity).
		ModelTableExpr(invitationTableAlias).
		Where(`"invitation_token".token = ?`, token).
		Where(`"invitation_token".expires_at > ?`, now).
		Where(`"invitation_token".used_at IS NULL`)

	query = base.WithTenantFilter(ctx, query, "invitation_token")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find valid invitation by token",
			Err: base.TranslateNotFound(err),
		}
	}

	return entity, nil
}

// FindByEmail returns invitations associated with an email address.
func (r *InvitationTokenRepository) FindByEmail(ctx context.Context, email string) ([]*modelAuth.InvitationToken, error) {
	var tokens []*modelAuth.InvitationToken
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&tokens).
		ModelTableExpr(invitationTableAlias).
		Where(`LOWER("invitation_token".email) = LOWER(?)`, email)

	query = base.WithTenantFilter(ctx, query, "invitation_token")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find invitations by email",
			Err: base.TranslateNotFound(err),
		}
	}
	return tokens, nil
}

// MarkAsUsed sets the used_at timestamp for a token.
func (r *InvitationTokenRepository) MarkAsUsed(ctx context.Context, id int64) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*modelAuth.InvitationToken)(nil)).
		ModelTableExpr(invitationTable).
		Set(`used_at = NOW()`).
		Where(`id = ?`, id)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "mark invitation as used",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "mark invitation as used")
}

// InvalidateByEmail marks all invitations for an email as used.
func (r *InvitationTokenRepository) InvalidateByEmail(ctx context.Context, email string) (int, error) {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*modelAuth.InvitationToken)(nil)).
		ModelTableExpr(invitationTable).
		Set(`used_at = NOW()`).
		Where(`LOWER(email) = LOWER(?)`, email).
		Where(`used_at IS NULL`)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	res, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "invalidate invitations by email",
			Err: base.TranslateNotFound(err),
		}
	}

	count, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve affected rows for invalidate invitations: %w", err)
	}

	return int(count), nil
}

// InvalidateByTenantID marks all pending invitations for a tenant as used.
// Used during soft-delete to prevent redemption of invitations for deleted schools.
func (r *InvitationTokenRepository) InvalidateByTenantID(ctx context.Context, tenantID int64) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*modelAuth.InvitationToken)(nil)).
		ModelTableExpr(invitationTable).
		Set(`used_at = NOW()`).
		Where(`tenant_id = ?`, tenantID).
		Where(`used_at IS NULL`).
		Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "invalidate invitations by tenant ID",
			Err: base.TranslateNotFound(err),
		}
	}
	count, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve affected rows for invalidate invitations by tenant: %w", err)
	}
	return int(count), nil
}

// DeleteExpired removes invitations that can no longer be used.
func (r *InvitationTokenRepository) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*modelAuth.InvitationToken)(nil)).
		ModelTableExpr(invitationTable).
		WhereGroup(" AND ", func(group *bun.DeleteQuery) *bun.DeleteQuery {
			return group.
				Where(`expires_at <= ?`, now).
				WhereOr(`used_at IS NOT NULL`)
		})

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	res, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete expired invitations",
			Err: base.TranslateNotFound(err),
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
	query := base.GetDB(ctx, r.db).NewSelect().
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

	query = base.WithTenantFilter(ctx, query, "invitation_token")

	now := time.Now()

	for key, value := range filters {
		query = r.applyInvitationFilter(query, key, value, now)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list invitation tokens",
			Err: base.TranslateNotFound(err),
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
	token := &modelAuth.InvitationToken{Model: modelBase.Model{ID: id}, EmailSentAt: sentAt, EmailRetryCount: retryCount}
	if emailError != nil {
		truncated := strutil.TruncateBytes(*emailError, maxEmailErrorLength, "")
		token.EmailError = &truncated
	}

	n, err := r.UpdateColumns(ctx, token, "email_sent_at", "email_error", "email_retry_count")
	if err != nil {
		return err
	}
	if n != 1 {
		return &modelBase.DatabaseError{
			Op:  "update invitation delivery result",
			Err: fmt.Errorf("expected 1 rows affected, got %d", n),
		}
	}
	return nil
}
