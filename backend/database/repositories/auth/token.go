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

// TokenRepository implements auth.TokenRepository interface
type TokenRepository struct {
	*base.Repository[*auth.Token]
	db *bun.DB
}

// NewTokenRepository creates a new TokenRepository
func NewTokenRepository(db *bun.DB) auth.TokenRepository {
	return &TokenRepository{
		Repository: base.NewRepository[*auth.Token](db, "auth.tokens", "Token"),
		db:         db,
	}
}

// FindByToken retrieves a token by its token value
func (r *TokenRepository) FindByToken(ctx context.Context, token string) (*auth.Token, error) {
	authToken := new(auth.Token)
	err := r.db.NewSelect().
		Model(authToken).
		Where("token = ?", token).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by token",
			Err: err,
		}
	}

	return authToken, nil
}

// FindByAccountID retrieves all tokens for an account
func (r *TokenRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.Token, error) {
	var tokens []*auth.Token
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

// FindByAccountIDAndIdentifier retrieves a token by account ID and identifier
func (r *TokenRepository) FindByAccountIDAndIdentifier(ctx context.Context, accountID int64, identifier string) (*auth.Token, error) {
	token := new(auth.Token)
	err := r.db.NewSelect().
		Model(token).
		Where("account_id = ? AND identifier = ?", accountID, identifier).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID and identifier",
			Err: err,
		}
	}

	return token, nil
}

// DeleteExpiredTokens removes all expired tokens
func (r *TokenRepository) DeleteExpiredTokens(ctx context.Context) (int, error) {
	res, err := r.db.NewDelete().
		Model((*auth.Token)(nil)).
		Where("expiry < ?", time.Now()).
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

// DeleteByAccountID removes all tokens for an account
func (r *TokenRepository) DeleteByAccountID(ctx context.Context, accountID int64) error {
	_, err := r.db.NewDelete().
		Model((*auth.Token)(nil)).
		Where("account_id = ?", accountID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by account ID",
			Err: err,
		}
	}

	return nil
}

// DeleteByAccountIDAndIdentifier removes a token by account ID and identifier
func (r *TokenRepository) DeleteByAccountIDAndIdentifier(ctx context.Context, accountID int64, identifier string) error {
	_, err := r.db.NewDelete().
		Model((*auth.Token)(nil)).
		Where("account_id = ? AND identifier = ?", accountID, identifier).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by account ID and identifier",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *TokenRepository) Create(ctx context.Context, token *auth.Token) error {
	if token == nil {
		return fmt.Errorf("token cannot be nil")
	}

	// Validate token
	if err := token.Validate(); err != nil {
		return err
	}

	// Use the base Create method which now uses ModelTableExpr
	return r.Repository.Create(ctx, token)
}

// FindValidTokens retrieves all valid (non-expired) tokens matching the filters
func (r *TokenRepository) FindValidTokens(ctx context.Context, filters map[string]interface{}) ([]*auth.Token, error) {
	var tokens []*auth.Token
	query := r.db.NewSelect().
		Model(&tokens).
		Where("expiry > ?", time.Now())

	// Apply additional filters
	for field, value := range filters {
		if value != nil {
			query = query.Where("? = ?", bun.Ident(field), value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find valid tokens",
			Err: err,
		}
	}

	return tokens, nil
}

// List retrieves tokens matching the provided filters
func (r *TokenRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.Token, error) {
	var tokens []*auth.Token
	query := r.db.NewSelect().Model(&tokens)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			switch field {
			case "mobile":
				query = query.Where("mobile = ?", value)
			case "active":
				if val, ok := value.(bool); ok && val {
					query = query.Where("expiry > ?", time.Now())
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

// FindTokensWithAccount retrieves tokens with their associated account details
func (r *TokenRepository) FindTokensWithAccount(ctx context.Context, filters map[string]interface{}) ([]*auth.Token, error) {
	var tokens []*auth.Token
	query := r.db.NewSelect().
		Model(&tokens).
		Relation("Account")

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = query.Where("token.? = ?", bun.Ident(field), value)
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
