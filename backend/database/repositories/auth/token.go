package auth

import (
	"context"
	"database/sql"
	"errors"
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
	repo := base.NewRepository[*auth.Token](db, "auth.tokens", "Token")
	repo.TenantScoped = true
	return &TokenRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByToken retrieves a token by its token value
func (r *TokenRepository) FindByToken(ctx context.Context, token string) (*auth.Token, error) {
	authToken := new(auth.Token)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(authToken).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".token = ?`, token)

	query = base.WithTenantFilter(ctx, query, "token")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by token",
			Err: err,
		}
	}

	return authToken, nil
}

// FindByTokenForUpdate retrieves a token by its token value with a row lock
// Must be called within a transaction
func (r *TokenRepository) FindByTokenForUpdate(ctx context.Context, token string) (*auth.Token, error) {
	authToken := new(auth.Token)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(authToken).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".token = ?`, token)

	query = base.WithTenantFilter(ctx, query, "token")

	err := query.
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by token for update",
			Err: err,
		}
	}

	return authToken, nil
}

// FindByAccountID retrieves all tokens for an account
func (r *TokenRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.Token, error) {
	var tokens []*auth.Token
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&tokens).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".account_id = ?`, accountID)

	query = base.WithTenantFilter(ctx, query, "token")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: err,
		}
	}

	return tokens, nil
}

// DeleteExpiredTokens removes all expired tokens
func (r *TokenRepository) DeleteExpiredTokens(ctx context.Context) (int, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".expiry < ?`, time.Now())

	query = base.WithTenantFilter(ctx, query, "token")

	res, err := query.Exec(ctx)

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
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".account_id = ?`, accountID)

	query = base.WithTenantFilter(ctx, query, "token")

	_, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by account ID",
			Err: err,
		}
	}

	return nil
}

// DeleteByTenantID removes all tokens for a given tenant (school).
// Used during soft-delete to immediately revoke all refresh tokens.
func (r *TokenRepository) DeleteByTenantID(ctx context.Context, tenantID int64) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".tenant_id = ?`, tenantID).
		Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete tokens by tenant ID",
			Err: err,
		}
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count affected rows",
			Err: err,
		}
	}
	return int(n), nil
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

	base.EnsureTenantID(ctx, token)

	// Explicitly set the table name with schema
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(token).
		ModelTableExpr(`auth.tokens`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create",
			Err: err,
		}
	}

	return nil
}

// Delete overrides the base Delete method to support transactions
func (r *TokenRepository) Delete(ctx context.Context, id interface{}) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "token")

	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete",
			Err: err,
		}
	}

	return nil
}

// List retrieves tokens matching the provided filters
func (r *TokenRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.Token, error) {
	var tokens []*auth.Token
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&tokens).
		ModelTableExpr(`auth.tokens AS "token"`)

	query = base.WithTenantFilter(ctx, query, "token")

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = r.applyTokenFilter(query, field, value)
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

// applyTokenFilter applies a single filter to the query
func (r *TokenRepository) applyTokenFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "mobile":
		return query.Where(`"token".mobile = ?`, value)
	case "active":
		return r.applyActiveTokenFilter(query, value)
	case "expired":
		return r.applyExpiredTokenFilter(query, value)
	default:
		return query.Where(`"token".? = ?`, bun.Ident(field), value)
	}
}

// applyActiveTokenFilter applies active token filter (not expired)
func (r *TokenRepository) applyActiveTokenFilter(query *bun.SelectQuery, value interface{}) *bun.SelectQuery {
	if val, ok := value.(bool); ok && val {
		return query.Where(`"token".expiry > ?`, time.Now())
	}
	return query
}

// applyExpiredTokenFilter applies expired token filter
func (r *TokenRepository) applyExpiredTokenFilter(query *bun.SelectQuery, value interface{}) *bun.SelectQuery {
	if val, ok := value.(bool); ok && val {
		return query.Where(`"token".expiry <= ?`, time.Now())
	}
	return query
}

// CleanupOldTokensForAccount keeps only the most recent N tokens for an account
// This is useful to allow multiple sessions while preventing unlimited token accumulation
func (r *TokenRepository) CleanupOldTokensForAccount(ctx context.Context, accountID int64, keepCount int) error {
	// First, get all tokens for the account ordered by creation date (newest first)
	var tokens []*auth.Token
	selectQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&tokens).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".account_id = ?`, accountID).
		OrderExpr(`"token".id DESC`) // Assuming ID is auto-incrementing, so higher ID = newer

	selectQuery = base.WithTenantFilter(ctx, selectQuery, "token")

	err := selectQuery.Scan(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "find tokens for cleanup",
			Err: err,
		}
	}

	// If we have more tokens than we want to keep, delete the old ones
	if len(tokens) > keepCount {
		// Get the IDs of tokens to delete (all except the most recent keepCount)
		var idsToDelete []int64
		for i := keepCount; i < len(tokens); i++ {
			idsToDelete = append(idsToDelete, tokens[i].ID)
		}

		// Delete the old tokens
		if len(idsToDelete) > 0 {
			delQuery := base.GetDB(ctx, r.db).NewDelete().
				Model((*auth.Token)(nil)).
				ModelTableExpr(`auth.tokens AS "token"`).
				Where(`"token".id IN (?)`, bun.List(idsToDelete))

			delQuery = base.WithTenantFilter(ctx, delQuery, "token")

			_, err = delQuery.Exec(ctx)

			if err != nil {
				return &modelBase.DatabaseError{
					Op:  "delete old tokens",
					Err: err,
				}
			}
		}
	}

	return nil
}

// DeleteByFamilyID deletes all tokens in a specific family
func (r *TokenRepository) DeleteByFamilyID(ctx context.Context, familyID string) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".family_id = ?`, familyID)

	query = base.WithTenantFilter(ctx, query, "token")

	_, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete tokens by family ID",
			Err: err,
		}
	}

	return nil
}

// GetLatestTokenInFamily gets the token with the highest generation in a family
func (r *TokenRepository) GetLatestTokenInFamily(ctx context.Context, familyID string) (*auth.Token, error) {
	var token auth.Token

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&token).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".family_id = ?`, familyID)

	query = base.WithTenantFilter(ctx, query, "token")

	err := query.
		OrderExpr(`"token".generation DESC`).
		Limit(1).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &modelBase.DatabaseError{
				Op:  "get latest token in family",
				Err: errors.New("token not found"),
			}
		}
		return nil, &modelBase.DatabaseError{
			Op:  "get latest token in family",
			Err: err,
		}
	}

	return &token, nil
}

// FindByFamilyID finds all tokens belonging to a specific family
func (r *TokenRepository) FindByFamilyID(ctx context.Context, familyID string) ([]*auth.Token, error) {
	var tokens []*auth.Token

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&tokens).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".family_id = ?`, familyID)

	query = base.WithTenantFilter(ctx, query, "token")

	err := query.
		OrderExpr(`"token".generation DESC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find tokens by family ID",
			Err: err,
		}
	}

	return tokens, nil
}
