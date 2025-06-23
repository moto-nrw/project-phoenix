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
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".token = ?`, token).
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
	res, err := r.db.ExecContext(ctx, "DELETE FROM auth.tokens WHERE expiry < ?", time.Now())

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
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".account_id = ?`, accountID).
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
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".account_id = ? AND "token".identifier = ?`, accountID, identifier).
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

// CleanupOldTokensForAccount keeps only the most recent N tokens for an account
// This is useful to allow multiple sessions while preventing unlimited token accumulation
func (r *TokenRepository) CleanupOldTokensForAccount(ctx context.Context, accountID int64, keepCount int) error {
	// First, get all tokens for the account ordered by creation date (newest first)
	var tokens []*auth.Token
	err := r.db.NewSelect().
		Model(&tokens).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".account_id = ?`, accountID).
		Order(`"token".id DESC`). // Assuming ID is auto-incrementing, so higher ID = newer
		Scan(ctx)

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
			_, err = r.db.NewDelete().
				Model((*auth.Token)(nil)).
				ModelTableExpr(`auth.tokens AS "token"`).
				Where(`"token".id IN (?)`, bun.In(idsToDelete)).
				Exec(ctx)

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

// FindByFamilyID finds all tokens belonging to a specific family
func (r *TokenRepository) FindByFamilyID(ctx context.Context, familyID string) ([]*auth.Token, error) {
	var tokens []*auth.Token
	
	err := r.db.NewSelect().
		Model(&tokens).
		Where(`"token".family_id = ?`, familyID).
		Order(`"token".generation DESC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find tokens by family ID",
			Err: err,
		}
	}

	return tokens, nil
}

// DeleteByFamilyID deletes all tokens in a specific family
func (r *TokenRepository) DeleteByFamilyID(ctx context.Context, familyID string) error {
	_, err := r.db.NewDelete().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".family_id = ?`, familyID).
		Exec(ctx)

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
	
	err := r.db.NewSelect().
		Model(&token).
		Where(`"token".family_id = ?`, familyID).
		Order(`"token".generation DESC`).
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
