package auth

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// TokenRepository implements auth.TokenRepository
type TokenRepository struct {
	db *bun.DB
}

// NewTokenRepository creates a new token repository
func NewTokenRepository(db *bun.DB) auth.TokenRepository {
	return &TokenRepository{db: db}
}

// Create inserts a new token into the database
func (r *TokenRepository) Create(ctx context.Context, token *auth.Token) error {
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
func (r *TokenRepository) FindByID(ctx context.Context, id interface{}) (*auth.Token, error) {
	token := new(auth.Token)
	err := r.db.NewSelect().Model(token).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return token, nil
}

// FindByToken retrieves a token by its value
func (r *TokenRepository) FindByToken(ctx context.Context, tokenValue string) (*auth.Token, error) {
	token := new(auth.Token)
	err := r.db.NewSelect().Model(token).Where("token = ?", tokenValue).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_token", Err: err}
	}
	return token, nil
}

// Update updates an existing token
func (r *TokenRepository) Update(ctx context.Context, token *auth.Token) error {
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
func (r *TokenRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*auth.Token)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves tokens matching the filters
func (r *TokenRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.Token, error) {
	var tokens []*auth.Token
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
func (r *TokenRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.Token, error) {
	var tokens []*auth.Token
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
func (r *TokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.db.NewDelete().
		Model((*auth.Token)(nil)).
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
func (r *TokenRepository) RevokeAllForAccount(ctx context.Context, accountID int64) error {
	_, err := r.db.NewDelete().
		Model((*auth.Token)(nil)).
		Where("account_id = ?", accountID).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "revoke_all", Err: err}
	}
	return nil
}

// FindByIdentifier retrieves a token by account ID and identifier
func (r *TokenRepository) FindByIdentifier(ctx context.Context, accountID int64, identifier string) (*auth.Token, error) {
	token := new(auth.Token)
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
