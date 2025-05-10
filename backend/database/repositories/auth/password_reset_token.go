package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// PasswordResetTokenRepository implements auth.PasswordResetTokenRepository interface
type PasswordResetTokenRepository struct {
	*base.Repository[*auth.PasswordResetToken]
	db *bun.DB
}

// NewPasswordResetTokenRepository creates a new PasswordResetTokenRepository
func NewPasswordResetTokenRepository(db *bun.DB) auth.PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{
		Repository: base.NewRepository[*auth.PasswordResetToken](db, "auth.password_reset_tokens", "PasswordResetToken"),
		db:         db,
	}
}

// FindByToken retrieves a password reset token by its token value
func (r *PasswordResetTokenRepository) FindByToken(ctx context.Context, token string) (*auth.PasswordResetToken, error) {
	resetToken := new(auth.PasswordResetToken)
	err := r.db.NewSelect().
		Model(resetToken).
		Where("token = ?", token).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by token",
			Err: err,
		}
	}

	return resetToken, nil
}

// FindByAccountID retrieves all password reset tokens for an account
func (r *PasswordResetTokenRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.PasswordResetToken, error) {
	var tokens []*auth.PasswordResetToken
	err := r.db.NewSelect().
		Model(&tokens).
		Where("account_id = ?", accountID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: err,
		}
	}

	return tokens, nil
}

// FindValidByToken retrieves a valid (not expired, not used) password reset token
func (r *PasswordResetTokenRepository) FindValidByToken(ctx context.Context, token string) (*auth.PasswordResetToken, error) {
	resetToken := new(auth.PasswordResetToken)
	err := r.db.NewSelect().
		Model(resetToken).
		Where("token = ? AND expiry > ? AND used = FALSE", token, time.Now()).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find valid by token",
			Err: err,
		}
	}

	return resetToken, nil
}

// MarkAsUsed marks a password reset token as used
func (r *PasswordResetTokenRepository) MarkAsUsed(ctx context.Context, tokenID int64) error {
	_, err := r.db.NewUpdate().
		Model((*auth.PasswordResetToken)(nil)).
		Set("used = TRUE").
		Where("id = ?", tokenID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "mark as used",
			Err: err,
		}
	}

	return nil
}

// DeleteExpiredTokens removes all expired or used tokens
func (r *PasswordResetTokenRepository) DeleteExpiredTokens(ctx context.Context) (int, error) {
	res, err := r.db.NewDelete().
		Model((*auth.PasswordResetToken)(nil)).
		Where("expiry < ? OR used = TRUE", time.Now()).
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete expired tokens",
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

// InvalidateTokensByAccountID marks all tokens for an account as used
func (r *PasswordResetTokenRepository) InvalidateTokensByAccountID(ctx context.Context, accountID int64) error {
	_, err := r.db.NewUpdate().
		Model((*auth.PasswordResetToken)(nil)).
		Set("used = TRUE").
		Where("account_id = ?", accountID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "invalidate tokens by account ID",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *PasswordResetTokenRepository) Create(ctx context.Context, token *auth.PasswordResetToken) error {
	if token == nil {
		return fmt.Errorf("password reset token cannot be nil")
	}

	// Validate token
	if err := token.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewInsert().
		Model(token).
		ModelTableExpr("auth.password_reset_tokens")

	// Extract transaction from context if it exists
	if tx, ok := ctx.Value("tx").(*bun.Tx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewInsert().
			Model(token).
			ModelTableExpr("auth.password_reset_tokens")
	}

	// Execute the query
	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create",
			Err: err,
		}
	}

	return nil
}

// Update overrides the base Update method for schema consistency
func (r *PasswordResetTokenRepository) Update(ctx context.Context, token *auth.PasswordResetToken) error {
	if token == nil {
		return fmt.Errorf("password reset token cannot be nil")
	}

	// Validate token
	if err := token.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(token).
		Where("id = ?", token.ID).
		ModelTableExpr("auth.password_reset_tokens")

	// Extract transaction from context if it exists
	if tx, ok := ctx.Value("tx").(*bun.Tx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(token).
			Where("id = ?", token.ID).
			ModelTableExpr("auth.password_reset_tokens")
	}

	// Execute the query
	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return nil
}

// List retrieves password reset tokens matching the provided filters
func (r *PasswordResetTokenRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.PasswordResetToken, error) {
	var tokens []*auth.PasswordResetToken
	query := r.db.NewSelect().Model(&tokens)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			switch field {
			case "used":
				query = query.Where("used = ?", value)
			case "valid":
				if val, ok := value.(bool); ok && val {
					query = query.Where("expiry > ? AND used = FALSE", time.Now())
				}
			case "expired":
				if val, ok := value.(bool); ok && val {
					query = query.Where("expiry <= ?", time.Now())
				}
			default:
				query = query.Where("? = ?", bun.Ident(field), value)
			}
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return tokens, nil
}

// FindTokensWithAccount retrieves password reset tokens with their associated account details
func (r *PasswordResetTokenRepository) FindTokensWithAccount(ctx context.Context, filters map[string]interface{}) ([]*auth.PasswordResetToken, error) {
	var tokens []*auth.PasswordResetToken
	query := r.db.NewSelect().
		Model(&tokens).
		Relation("Account")

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = query.Where("password_reset_token.? = ?", bun.Ident(field), value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with account",
			Err: err,
		}
	}

	return tokens, nil
}
