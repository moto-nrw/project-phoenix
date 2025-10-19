package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelAuth "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
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
		Where(`"invitation".token = ?`, token).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find invitation by token",
			Err: err,
		}
	}

	return entity, nil
}

// FindValidByToken returns an invitation if it is not expired or used.
func (r *InvitationTokenRepository) FindValidByToken(ctx context.Context, token string, now time.Time) (*modelAuth.InvitationToken, error) {
	entity := new(modelAuth.InvitationToken)
	err := r.db.NewSelect().
		Model(entity).
		Where(`"invitation".token = ?`, token).
		Where(`"invitation".expires_at > ?`, now).
		Where(`"invitation".used_at IS NULL`).
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
		Where(`LOWER("invitation".email) = LOWER(?)`, email).
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
		ModelTableExpr(`auth.invitation_tokens`).
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
		ModelTableExpr(`auth.invitation_tokens`).
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
		ModelTableExpr(`auth.invitation_tokens`).
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
		Relation("Role").
		Relation("Creator")

	now := time.Now()

	for key, value := range filters {
		switch key {
		case "email":
			if v, ok := value.(string); ok && v != "" {
				query = query.Where(`LOWER("invitation".email) = LOWER(?)`, v)
			}
		case "pending":
			if pending, ok := value.(bool); ok && pending {
				query = query.Where(`"invitation".used_at IS NULL`).Where(`"invitation".expires_at > ?`, now)
			}
		case "expired":
			if expired, ok := value.(bool); ok && expired {
				query = query.Where(`"invitation".expires_at <= ?`, now)
			}
		case "used":
			if used, ok := value.(bool); ok && used {
				query = query.Where(`"invitation".used_at IS NOT NULL`)
			}
		}
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list invitation tokens",
			Err: err,
		}
	}

	return tokens, nil
}
