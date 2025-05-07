package auth

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// PasswordResetTokenRepository implements auth.PasswordResetTokenRepository
type PasswordResetTokenRepository struct {
	db *bun.DB
}

// NewPasswordResetTokenRepository creates a new password reset token repository
func NewPasswordResetTokenRepository(db *bun.DB) auth.PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{db: db}
}

// Create inserts a new password reset token into the database
func (r *PasswordResetTokenRepository) Create(ctx context.Context, token *auth.PasswordResetToken) error {
	if err := token.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(token).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a password reset token by its ID
func (r *PasswordResetTokenRepository) FindByID(ctx context.Context, id interface{}) (*auth.PasswordResetToken, error) {
	token := new(auth.PasswordResetToken)
	err := r.db.NewSelect().Model(token).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return token, nil
}

// FindByToken retrieves a password reset token by its value
func (r *PasswordResetTokenRepository) FindByToken(ctx context.Context, tokenValue string) (*auth.PasswordResetToken, error) {
	token := new(auth.PasswordResetToken)
	err := r.db.NewSelect().Model(token).Where("token = ?", tokenValue).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_token", Err: err}
	}
	return token, nil
}

// Update updates an existing password reset token
func (r *PasswordResetTokenRepository) Update(ctx context.Context, token *auth.PasswordResetToken) error {
	if err := token.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(token).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a password reset token
func (r *PasswordResetTokenRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*auth.PasswordResetToken)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves password reset tokens matching the filters
func (r *PasswordResetTokenRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.PasswordResetToken, error) {
	var tokens []*auth.PasswordResetToken
	query := r.db.NewSelect().Model(&tokens)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return tokens, nil
}

// FindByAccountID retrieves all password reset tokens for an account
func (r *PasswordResetTokenRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.PasswordResetToken, error) {
	var tokens []*auth.PasswordResetToken
	err := r.db.NewSelect().
		Model(&tokens).
		Where("account_id = ?", accountID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_account_id", Err: err}
	}
	return tokens, nil
}

// MarkAsUsed marks a token as used
func (r *PasswordResetTokenRepository) MarkAsUsed(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*auth.PasswordResetToken)(nil)).
		Set("used = ?", true).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "mark_as_used", Err: err}
	}
	return nil
}

// DeleteExpired removes all expired password reset tokens
func (r *PasswordResetTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.NewDelete().
		Model((*auth.PasswordResetToken)(nil)).
		Where("expiry < ?", time.Now()).
		Exec(ctx)

	if err != nil {
		return 0, &base.DatabaseError{Op: "delete_expired", Err: err}
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, &base.DatabaseError{Op: "delete_expired_count", Err: err}
	}

	return affected, nil
}
