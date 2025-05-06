package auth

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Token represents an authentication token in the system
type Token struct {
	base.Model
	AccountID  int64     `bun:"account_id,notnull" json:"account_id"`
	Token      string    `bun:"token,notnull" json:"token"`
	Expiry     time.Time `bun:"expiry,notnull" json:"expiry"`
	Mobile     bool      `bun:"mobile,notnull,default:false" json:"mobile"`
	Identifier string    `bun:"identifier" json:"identifier,omitempty"`

	// Relations
	Account *Account `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
}

// TableName returns the table name for the Token model
func (t *Token) TableName() string {
	return "auth.tokens"
}

// GetID returns the token ID
func (t *Token) GetID() interface{} {
	return t.ID
}

// GetCreatedAt returns the creation timestamp
func (t *Token) GetCreatedAt() time.Time {
	return t.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (t *Token) GetUpdatedAt() time.Time {
	return t.UpdatedAt
}

// Validate validates the token fields
func (t *Token) Validate() error {
	if t.AccountID <= 0 {
		return errors.New("account ID is required")
	}

	if t.Token == "" {
		return errors.New("token value is required")
	}

	if t.Expiry.IsZero() || t.Expiry.Before(time.Now()) {
		return errors.New("token must have a valid expiry date in the future")
	}

	return nil
}

// IsExpired checks if the token has expired
func (t *Token) IsExpired() bool {
	return time.Now().After(t.Expiry)
}

// TokenRepository defines operations for working with tokens
type TokenRepository interface {
	base.Repository[*Token]
	FindByToken(ctx context.Context, token string) (*Token, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*Token, error)
	DeleteExpired(ctx context.Context) (int64, error)
	RevokeAllForAccount(ctx context.Context, accountID int64) error
	FindByIdentifier(ctx context.Context, accountID int64, identifier string) (*Token, error)
}

// DefaultTokenRepository is the default implementation of TokenRepository
type DefaultTokenRepository struct {
	db *bun.DB
}

// NewTokenRepository creates a new token repository
func NewTokenRepository(db *bun.DB) TokenRepository {
	return &DefaultTokenRepository{db: db}
}

// Create inserts a new token into the database
func (r *DefaultTokenRepository) Create(ctx context.Context, token *Token) error {
	if err := token.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(token).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a token by its ID
func (r *DefaultTokenRepository) FindByID(ctx context.Context, id interface{}) (*Token, error) {
	token := new(Token)
	err := r.db.NewSelect().Model(token).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return token, nil
}

// FindByToken retrieves a token by its value
func (r *DefaultTokenRepository) FindByToken(ctx context.Context, tokenValue string) (*Token, error) {
	token := new(Token)
	err := r.db.NewSelect().Model(token).Where("token = ?", tokenValue).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_token", Err: err}
	}
	return token, nil
}

// Update updates an existing token
func (r *DefaultTokenRepository) Update(ctx context.Context, token *Token) error {
	if err := token.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(token).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a token
func (r *DefaultTokenRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Token)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves tokens matching the filters
func (r *DefaultTokenRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Token, error) {
	var tokens []*Token
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

// FindByAccountID retrieves all tokens for an account
func (r *DefaultTokenRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*Token, error) {
	var tokens []*Token
	err := r.db.NewSelect().
		Model(&tokens).
		Where("account_id = ?", accountID).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_account_id", Err: err}
	}
	return tokens, nil
}

// DeleteExpired removes all expired tokens
func (r *DefaultTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.NewDelete().
		Model((*Token)(nil)).
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

// RevokeAllForAccount revokes all tokens for an account
func (r *DefaultTokenRepository) RevokeAllForAccount(ctx context.Context, accountID int64) error {
	_, err := r.db.NewDelete().
		Model((*Token)(nil)).
		Where("account_id = ?", accountID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "revoke_all", Err: err}
	}
	return nil
}

// FindByIdentifier retrieves a token by account ID and identifier
func (r *DefaultTokenRepository) FindByIdentifier(ctx context.Context, accountID int64, identifier string) (*Token, error) {
	token := new(Token)
	err := r.db.NewSelect().
		Model(token).
		Where("account_id = ?", accountID).
		Where("identifier = ?", identifier).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_identifier", Err: err}
	}
	return token, nil
}
