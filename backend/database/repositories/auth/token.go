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
	"github.com/moto-nrw/project-phoenix/tenant"
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

// MarkRotated records the successor of a consumed refresh handle. Keeping the
// handoff in the database lets another frontend process recover an interrupted
// rotation without accepting arbitrary or unbounded replay.
func (r *TokenRepository) MarkRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Set(`rotated_at = ?`, rotatedAt).
		Set(`replacement_token = ?`, replacementToken).
		Set(`recovery_proof_hash = ?`, recoveryProofHash).
		Where(`"token".id = ?`, id).
		Where(`"token".rotated_at IS NULL`)

	query = base.WithTenantFilter(ctx, query, "token")
	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "mark refresh token rotated", Err: err}
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return &modelBase.DatabaseError{Op: "count rotated refresh tokens", Err: err}
	}
	if affected != 1 {
		return &modelBase.DatabaseError{Op: "mark refresh token rotated", Err: errors.New("refresh token was already rotated or not found")}
	}
	return nil
}

// DeleteExpiredRotatedForAccount removes predecessor rows only after their
// refresh JWTs expire. Until then they are replay-detection evidence and must
// remain attributable to their token family.
func (r *TokenRepository) DeleteExpiredRotatedForAccount(ctx context.Context, accountID int64, now time.Time) error {
	db := base.GetDB(ctx, r.db)
	candidates := db.NewSelect().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		ColumnExpr(`"token".id`).
		Where(`"token".account_id = ?`, accountID).
		Where(`"token".rotated_at IS NOT NULL`).
		Where(`"token".expiry <= ?`, now)
	candidates = base.WithTenantFilter(ctx, candidates, "token").
		For("UPDATE SKIP LOCKED")

	query := db.NewDelete().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".id IN (?)`, candidates)

	query = base.WithTenantFilter(ctx, query, "token")
	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "delete expired rotated account refresh tokens", Err: err}
	}
	return nil
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
	deleted, err := r.DeleteBefore(ctx, "expiry", time.Now(), "delete expired tokens")
	return int(deleted), err
}

// ListInactiveAccountIDsWithLiveTokens returns accounts that are deactivated
// but still have unexpired refresh tokens. Used to recover a failed
// account-wide wipe.
func (r *TokenRepository) ListInactiveAccountIDsWithLiveTokens(ctx context.Context) ([]int64, error) {
	var ids []int64
	err := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr("DISTINCT t.account_id").
		TableExpr(`auth.tokens AS t`).
		Join(`INNER JOIN auth.accounts AS a ON a.id = t.account_id`).
		Where(`a.active = ?`, false).
		Where(`t.rotated_at IS NULL`).
		Where(`t.expiry > ?`, time.Now()).
		Scan(ctx, &ids)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list inactive accounts with live tokens", Err: err}
	}
	return ids, nil
}

// DeleteByAccountIDReturning atomically deletes and returns every affected
// token so the service can persist a complete revocation audit in the same
// transaction; the generic repository cannot express DELETE ... RETURNING.
func (r *TokenRepository) DeleteByAccountIDReturning(ctx context.Context, accountID int64) ([]*auth.Token, error) {
	var deleted []*auth.Token
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".account_id = ?`, accountID).
		Returning("*")
	if !tenant.IsAdminTx(ctx) {
		query = base.WithTenantFilter(ctx, query, "token")
	}
	if err := query.Scan(ctx, &deleted); err != nil {
		return nil, &modelBase.DatabaseError{Op: "delete and return by account ID", Err: err}
	}
	return deleted, nil
}

// DeleteByTenantIDReturning uses DELETE ... RETURNING so tenant revocation and
// its per-family audit evidence can commit atomically; generic CRUD cannot.
func (r *TokenRepository) DeleteByTenantIDReturning(ctx context.Context, tenantID int64) ([]*auth.Token, error) {
	var deleted []*auth.Token
	err := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".tenant_id = ?`, tenantID).
		Returning("*").
		Scan(ctx, &deleted)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "delete and return tokens by tenant ID", Err: err}
	}
	return deleted, nil
}

// Create overrides the base Create method to handle validation
func (r *TokenRepository) Create(ctx context.Context, token *auth.Token) error {
	if token == nil {
		return fmt.Errorf("token cannot be nil")
	}
	if token.PortalScope == "" {
		token.PortalScope = auth.PortalScopeUnknown
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

// CleanupOldTokensForAccountReturning enforces the active-session cap for one
// portal group and returns only sessions that were actually revoked for
// same-transaction audit; generic CRUD cannot combine the ordered cap policy
// with DELETE ... RETURNING.
//
// Tenant and org share the staff allowance. Unknown legacy rows are isolated
// so a known-portal login cannot evict a session whose portal is unknown.
func (r *TokenRepository) CleanupOldTokensForAccountReturning(ctx context.Context, accountID int64, portalScope string, keepCount int) ([]*auth.Token, error) {
	scopes := auth.CapPortalScopes(portalScope)
	var tokens []*auth.Token
	selectQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&tokens).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".account_id = ?`, accountID).
		Where(`"token".portal_scope IN (?)`, bun.List(scopes)).
		Where(`"token".rotated_at IS NULL`).
		Where(`"token".expiry > ?`, time.Now()).
		OrderExpr(`"token".id DESC`)
	if !tenant.IsAdminTx(ctx) {
		selectQuery = base.WithTenantFilter(ctx, selectQuery, "token")
	}
	if err := selectQuery.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find tokens for audited cleanup", Err: err}
	}
	if len(tokens) <= keepCount {
		return nil, nil
	}
	ids := make([]int64, 0, len(tokens)-keepCount)
	for _, token := range tokens[keepCount:] {
		ids = append(ids, token.ID)
	}
	var deleted []*auth.Token
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".id IN (?)`, bun.List(ids)).
		Returning("*")
	if !tenant.IsAdminTx(ctx) {
		query = base.WithTenantFilter(ctx, query, "token")
	}
	if err := query.Scan(ctx, &deleted); err != nil {
		return nil, &modelBase.DatabaseError{Op: "delete and return old tokens", Err: err}
	}
	return deleted, nil
}

// DeleteByFamilyIDReturning uses DELETE ... RETURNING so family revocation and
// its audit evidence share one transaction; generic CRUD cannot express it.
func (r *TokenRepository) DeleteByFamilyIDReturning(ctx context.Context, familyID string) ([]*auth.Token, error) {
	var deleted []*auth.Token
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.Token)(nil)).
		ModelTableExpr(`auth.tokens AS "token"`).
		Where(`"token".family_id = ?`, familyID).
		Returning("*")
	query = base.WithTenantFilter(ctx, query, "token")
	if err := query.Scan(ctx, &deleted); err != nil {
		return nil, &modelBase.DatabaseError{Op: "delete and return tokens by family ID", Err: err}
	}
	return deleted, nil
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
