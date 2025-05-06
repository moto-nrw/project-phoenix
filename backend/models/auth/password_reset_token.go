package auth

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// PasswordResetToken represents a password reset token
type PasswordResetToken struct {
	base.Model
	AccountID int64     `bun:"account_id,notnull" json:"account_id"`
	Token     string    `bun:"token,notnull" json:"token"`
	Expiry    time.Time `bun:"expiry,notnull" json:"expiry"`
	Used      bool      `bun:"used,notnull,default:false" json:"used"`

	// Relations
	Account *Account `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
}

// TableName returns the table name for the PasswordResetToken model
func (p *PasswordResetToken) TableName() string {
	return "auth.password_reset_tokens"
}

// GetID returns the token ID
func (p *PasswordResetToken) GetID() interface{} {
	return p.ID
}

// GetCreatedAt returns the creation timestamp
func (p *PasswordResetToken) GetCreatedAt() time.Time {
	return p.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (p *PasswordResetToken) GetUpdatedAt() time.Time {
	return p.UpdatedAt
}

// Validate validates the token fields
func (p *PasswordResetToken) Validate() error {
	if p.AccountID <= 0 {
		return errors.New("account ID is required")
	}

	if p.Token == "" {
		return errors.New("token value is required")
	}

	if p.Expiry.IsZero() || p.Expiry.Before(time.Now()) {
		return errors.New("token must have a valid expiry date in the future")
	}

	return nil
}

// IsExpired checks if the token has expired
func (p *PasswordResetToken) IsExpired() bool {
	return time.Now().After(p.Expiry)
}

// IsValid checks if the token is valid (not expired and not used)
func (p *PasswordResetToken) IsValid() bool {
	return !p.IsExpired() && !p.Used
}

// PasswordResetTokenRepository defines operations for working with password reset tokens
type PasswordResetTokenRepository interface {
	base.Repository[*PasswordResetToken]
	FindByToken(ctx context.Context, token string) (*PasswordResetToken, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*PasswordResetToken, error)
	MarkAsUsed(ctx context.Context, id int64) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// DefaultPasswordResetTokenRepository is the default implementation of PasswordResetTokenRepository
type DefaultPasswordResetTokenRepository struct {
	db *bun.DB
}

// NewPasswordResetTokenRepository creates a new password reset token repository
func NewPasswordResetTokenRepository(db *bun.DB) PasswordResetTokenRepository {
	return &DefaultPasswordResetTokenRepository{db: db}
}

// Create inserts a new password reset token into the database
func (r *DefaultPasswordResetTokenRepository) Create(ctx context.Context, token *PasswordResetToken) error {
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
func (r *DefaultPasswordResetTokenRepository) FindByID(ctx context.Context, id interface{}) (*PasswordResetToken, error) {
	token := new(PasswordResetToken)
	err := r.db.NewSelect().Model(token).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return token, nil
}

// FindByToken retrieves a password reset token by its value
func (r *DefaultPasswordResetTokenRepository) FindByToken(ctx context.Context, tokenValue string) (*PasswordResetToken, error) {
	token := new(PasswordResetToken)
	err := r.db.NewSelect().Model(token).Where("token = ?", tokenValue).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_token", Err: err}
	}
	return token, nil
}

// Update updates an existing password reset token
func (r *DefaultPasswordResetTokenRepository) Update(ctx context.Context, token *PasswordResetToken) error {
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
func (r *DefaultPasswordResetTokenRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*PasswordResetToken)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves password reset tokens matching the filters
func (r *DefaultPasswordResetTokenRepository) List(ctx context.Context, filters map[string]interface{}) ([]*PasswordResetToken, error) {
	var tokens []*PasswordResetToken
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
func (r *DefaultPasswordResetTokenRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*PasswordResetToken, error) {
	var tokens []*PasswordResetToken
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
func (r *DefaultPasswordResetTokenRepository) MarkAsUsed(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*PasswordResetToken)(nil)).
		Set("used = ?", true).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "mark_as_used", Err: err}
	}
	return nil
}

// DeleteExpired removes all expired password reset tokens
func (r *DefaultPasswordResetTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.NewDelete().
		Model((*PasswordResetToken)(nil)).
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
