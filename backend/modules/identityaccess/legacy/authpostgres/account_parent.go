package authpostgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	accountParentTable      = "auth.accounts_parents"
	accountParentTableAlias = `auth.accounts_parents AS "account_parent"`
	whereID                 = "id = ?"
)

// AccountParentRepository implements auth.AccountParentRepository interface
type AccountParentRepository struct {
	*base.Repository[*authmodels.AccountParent]
	db *bun.DB
}

// NewAccountParentRepository creates a new AccountParentRepository
func NewAccountParentRepository(db *bun.DB) authmodels.AccountParentRepository {
	repo := base.NewRepository[*authmodels.AccountParent](db, accountParentTable, "AccountParent")
	repo.TenantScoped = true
	return &AccountParentRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByEmail retrieves a parent account by email address
func (r *AccountParentRepository) FindByEmail(ctx context.Context, email string) (*authmodels.AccountParent, error) {
	account := new(authmodels.AccountParent)

	// Explicitly specify the schema and table
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(account).
		ModelTableExpr(accountParentTableAlias).
		Where(`LOWER("account_parent".email) = LOWER(?)`, email)

	query = base.WithTenantFilter(ctx, query, "account_parent")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by email",
			Err: base.TranslateNotFound(err),
		}
	}

	return account, nil
}

// List retrieves parent accounts matching the provided filters
func (r *AccountParentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*authmodels.AccountParent, error) {
	var accounts []*authmodels.AccountParent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&accounts).
		ModelTableExpr(accountParentTableAlias)

	query = base.WithTenantFilter(ctx, query, "account_parent")

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
			Err: base.TranslateNotFound(err),
		}
	}

	return accounts, nil
}

// applyAccountParentFilter applies a single filter to the query
func (r *AccountParentRepository) applyAccountParentFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "email":
		if strValue, ok := value.(string); ok {
			return query.Where("LOWER(email) = LOWER(?)", strValue)
		}
		return query.Where("email = ?", value)
	case "username":
		if strValue, ok := value.(string); ok {
			return query.Where("LOWER(username) = LOWER(?)", strValue)
		}
		return query.Where("username = ?", value)
	case "email_like":
		if strValue, ok := value.(string); ok {
			return query.Where("LOWER(email) LIKE LOWER(?)", "%"+strValue+"%")
		}
		return query
	case "username_like":
		if strValue, ok := value.(string); ok {
			return query.Where("LOWER(username) LIKE LOWER(?)", "%"+strValue+"%")
		}
		return query
	case "active":
		return query.Where("active = ?", value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// Create overrides the base Create method for schema consistency
func (r *AccountParentRepository) Create(ctx context.Context, account *authmodels.AccountParent) error {
	if account == nil {
		return fmt.Errorf("account parent cannot be nil")
	}

	// Validate account
	if err := account.Validate(); err != nil {
		return err
	}

	base.EnsureTenantID(ctx, account)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(account).
		ModelTableExpr(accountParentTable).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// Update overrides the base Update method for schema consistency
func (r *AccountParentRepository) Update(ctx context.Context, account *authmodels.AccountParent) error {
	if account == nil {
		return fmt.Errorf("account parent cannot be nil")
	}

	// Validate account
	if err := account.Validate(); err != nil {
		return err
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(account).
		Where(whereID, account.ID).
		ModelTableExpr(accountParentTable)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "update account_parent")
}
