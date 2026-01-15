package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/uptrace/bun"
)

const (
	passwordResetTokenTable      = "auth.password_reset_tokens"
	passwordResetTokenTableAlias = "auth.password_reset_tokens AS password_reset_token"
)

// PasswordResetTokenRepository implements auth.PasswordResetTokenRepository interface
type PasswordResetTokenRepository struct {
	*base.Repository[*auth.PasswordResetToken]
	db *bun.DB
}

// NewPasswordResetTokenRepository creates a new PasswordResetTokenRepository
func NewPasswordResetTokenRepository(db *bun.DB) auth.PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{
		Repository: base.NewRepository[*auth.PasswordResetToken](db, passwordResetTokenTable, "PasswordResetToken"),
		db:         db,
	}
}

// FindByToken retrieves a password reset token by its token value
func (r *PasswordResetTokenRepository) FindByToken(ctx context.Context, token string) (*auth.PasswordResetToken, error) {
	resetToken := new(auth.PasswordResetToken)
	err := r.db.NewSelect().
		Model(resetToken).
		ModelTableExpr(passwordResetTokenTableAlias).
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
		ModelTableExpr(passwordResetTokenTableAlias).
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
		ModelTableExpr(passwordResetTokenTableAlias).
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
		ModelTableExpr(passwordResetTokenTable).
		Set("used = TRUE").
		Where(whereID, tokenID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "mark as used",
			Err: err,
		}
	}

	return nil
}

// UpdateDeliveryResult updates the delivery metadata for a password reset token.
func (r *PasswordResetTokenRepository) UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error {
	update := r.db.NewUpdate().
		Model((*auth.PasswordResetToken)(nil)).
		ModelTableExpr(passwordResetTokenTable).
		Where(whereID, tokenID).
		Set("email_retry_count = ?", retryCount)

	if sentAt != nil {
		update = update.Set("email_sent_at = ?", *sentAt)
	} else {
		update = update.Set("email_sent_at = NULL")
	}

	if emailError != nil {
		update = update.Set("email_error = ?", truncateError(*emailError))
	} else {
		update = update.Set("email_error = NULL")
	}

	if _, err := update.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{
			Op:  "update password reset delivery result",
			Err: err,
		}
	}

	return nil
}

// DeleteExpiredTokens removes all expired or used tokens
func (r *PasswordResetTokenRepository) DeleteExpiredTokens(ctx context.Context) (int, error) {
	res, err := r.db.NewDelete().
		Model((*auth.PasswordResetToken)(nil)).
		ModelTableExpr(passwordResetTokenTable).
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
		ModelTableExpr(passwordResetTokenTable).
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

	// Use the base Create method which now uses ModelTableExpr
	return r.Repository.Create(ctx, token)
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
		ModelTableExpr(passwordResetTokenTableAlias).
		Where(whereID, token.ID)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(token).
			ModelTableExpr(passwordResetTokenTableAlias).
			Where(whereID, token.ID)
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
	query := r.db.NewSelect().
		Model(&tokens).
		ModelTableExpr(passwordResetTokenTableAlias)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = r.applyPasswordResetTokenFilter(query, field, value)
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

// applyPasswordResetTokenFilter applies a single filter to the query
func (r *PasswordResetTokenRepository) applyPasswordResetTokenFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "used":
		return query.Where("used = ?", value)
	case "valid":
		return r.applyValidFilter(query, value)
	case "expired":
		return r.applyExpiredTokenFilter(query, value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// applyValidFilter applies valid token filter (not expired and not used)
func (r *PasswordResetTokenRepository) applyValidFilter(query *bun.SelectQuery, value interface{}) *bun.SelectQuery {
	if val, ok := value.(bool); ok && val {
		return query.Where("expiry > ? AND used = FALSE", time.Now())
	}
	return query
}

// applyExpiredTokenFilter applies expired token filter
func (r *PasswordResetTokenRepository) applyExpiredTokenFilter(query *bun.SelectQuery, value interface{}) *bun.SelectQuery {
	if val, ok := value.(bool); ok && val {
		return query.Where("expiry <= ?", time.Now())
	}
	return query
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
