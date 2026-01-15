package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	accountParentTable      = "auth.accounts_parents"
	accountParentTableAlias = `auth.accounts_parents AS "account_parent"`
	whereID                 = "id = ?"
)

// AccountParentRepository implements auth.AccountParentRepository interface
type AccountParentRepository struct {
	*base.Repository[*auth.AccountParent]
	db *bun.DB
}

// NewAccountParentRepository creates a new AccountParentRepository
func NewAccountParentRepository(db *bun.DB) auth.AccountParentRepository {
	return &AccountParentRepository{
		Repository: base.NewRepository[*auth.AccountParent](db, accountParentTable, "AccountParent"),
		db:         db,
	}
}

// FindByEmail retrieves a parent account by email address
func (r *AccountParentRepository) FindByEmail(ctx context.Context, email string) (*auth.AccountParent, error) {
	account := new(auth.AccountParent)
	err := r.db.NewSelect().
		Model(account).
		Where("LOWER(email) = LOWER(?)", email).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by email",
			Err: err,
		}
	}

	return account, nil
}

// FindByUsername retrieves a parent account by username
func (r *AccountParentRepository) FindByUsername(ctx context.Context, username string) (*auth.AccountParent, error) {
	account := new(auth.AccountParent)
	err := r.db.NewSelect().
		Model(account).
		Where("LOWER(username) = LOWER(?)", username).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by username",
			Err: err,
		}
	}

	return account, nil
}

// UpdateLastLogin updates the last login timestamp for a parent account
func (r *AccountParentRepository) UpdateLastLogin(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*auth.AccountParent)(nil)).
		ModelTableExpr(accountParentTable).
		Set("last_login = ?", time.Now()).
		Where(whereID, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update last login",
			Err: err,
		}
	}

	return nil
}

// UpdatePassword updates the password hash for a parent account
func (r *AccountParentRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.db.NewUpdate().
		Model((*auth.AccountParent)(nil)).
		ModelTableExpr(accountParentTable).
		Set("password_hash = ?", passwordHash).
		Where(whereID, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update password",
			Err: err,
		}
	}

	return nil
}

// List retrieves parent accounts matching the provided filters
func (r *AccountParentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.AccountParent, error) {
	var accounts []*auth.AccountParent
	query := r.db.NewSelect().
		Model(&accounts).
		ModelTableExpr(accountParentTableAlias)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = r.applyAccountParentFilter(query, field, value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return accounts, nil
}

// applyAccountParentFilter applies a single filter to the query
func (r *AccountParentRepository) applyAccountParentFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "email":
		return r.applyStringEqualFilter(query, "email", value)
	case "username":
		return r.applyStringEqualFilter(query, "username", value)
	case "email_like":
		return r.applyStringLikeFilter(query, "email", value)
	case "username_like":
		return r.applyStringLikeFilter(query, "username", value)
	case "active":
		return query.Where("active = ?", value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// applyStringEqualFilter applies case-insensitive equality filter for string fields
func (r *AccountParentRepository) applyStringEqualFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER("+field+") = LOWER(?)", strValue)
	}
	return query.Where(field+" = ?", value)
}

// applyStringLikeFilter applies case-insensitive LIKE filter for string fields
func (r *AccountParentRepository) applyStringLikeFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER("+field+") LIKE LOWER(?)", "%"+strValue+"%")
	}
	return query
}

// Create overrides the base Create method for schema consistency
func (r *AccountParentRepository) Create(ctx context.Context, account *auth.AccountParent) error {
	if account == nil {
		return fmt.Errorf("account parent cannot be nil")
	}

	// Validate account
	if err := account.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewInsert().
		Model(account).
		ModelTableExpr(accountParentTable)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewInsert().
			Model(account).
			ModelTableExpr(accountParentTable)
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
func (r *AccountParentRepository) Update(ctx context.Context, account *auth.AccountParent) error {
	if account == nil {
		return fmt.Errorf("account parent cannot be nil")
	}

	// Validate account
	if err := account.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(account).
		Where(whereID, account.ID).
		ModelTableExpr(accountParentTable)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(account).
			Where(whereID, account.ID).
			ModelTableExpr(accountParentTable)
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
