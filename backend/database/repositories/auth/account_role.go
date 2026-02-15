package auth

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	accountRoleTable      = "auth.account_roles"
	accountRoleTableAlias = `auth.account_roles AS "account_role"`
)

// AccountRoleRepository implements auth.AccountRoleRepository interface
type AccountRoleRepository struct {
	*base.Repository[*auth.AccountRole]
	db *bun.DB
}

// NewAccountRoleRepository creates a new AccountRoleRepository
func NewAccountRoleRepository(db *bun.DB) auth.AccountRoleRepository {
	repo := base.NewRepository[*auth.AccountRole](db, accountRoleTable, "AccountRole")
	repo.TenantScoped = true
	return &AccountRoleRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByAccountID retrieves all account-role mappings for an account
func (r *AccountRoleRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.AccountRole, error) {
	var accountRoles []*auth.AccountRole
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&accountRoles).
		ModelTableExpr(accountRoleTableAlias).
		Join(`LEFT JOIN auth.roles AS "role" ON "role".id = "account_role".role_id`).
		ColumnExpr(`"account_role".*`).
		ColumnExpr(`"role".id AS "role__id", "role".created_at AS "role__created_at", "role".updated_at AS "role__updated_at", "role".name AS "role__name", "role".description AS "role__description"`).
		Where(`"account_role".account_id = ?`, accountID)

	if where, val, ok := base.TenantWhere(ctx, "account_role"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: err,
		}
	}

	return accountRoles, nil
}

// FindByRoleID retrieves all account-role mappings for a role
func (r *AccountRoleRepository) FindByRoleID(ctx context.Context, roleID int64) ([]*auth.AccountRole, error) {
	var accountRoles []*auth.AccountRole
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&accountRoles).
		ModelTableExpr(accountRoleTableAlias).
		Where(`"account_role".role_id = ?`, roleID)

	if where, val, ok := base.TenantWhere(ctx, "account_role"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by role ID",
			Err: err,
		}
	}

	return accountRoles, nil
}

// FindByAccountAndRole retrieves a specific account-role mapping
func (r *AccountRoleRepository) FindByAccountAndRole(ctx context.Context, accountID, roleID int64) (*auth.AccountRole, error) {
	accountRole := new(auth.AccountRole)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(accountRole).
		ModelTableExpr(accountRoleTableAlias).
		Where(`"account_role".account_id = ? AND "account_role".role_id = ?`, accountID, roleID)

	if where, val, ok := base.TenantWhere(ctx, "account_role"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account and role",
			Err: err,
		}
	}

	return accountRole, nil
}

// Create overrides the base Create method to handle validation
func (r *AccountRoleRepository) Create(ctx context.Context, accountRole *auth.AccountRole) error {
	if accountRole == nil {
		return fmt.Errorf("account role cannot be nil")
	}

	// Validate accountRole
	if err := accountRole.Validate(); err != nil {
		return err
	}

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(accountRole).
		ModelTableExpr(accountRoleTable).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create",
			Err: err,
		}
	}

	return nil
}

// Update overrides the base Update method for schema consistency
func (r *AccountRoleRepository) Update(ctx context.Context, accountRole *auth.AccountRole) error {
	if accountRole == nil {
		return fmt.Errorf("account role cannot be nil")
	}

	// Validate accountRole
	if err := accountRole.Validate(); err != nil {
		return err
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(accountRole).
		Where("id = ?", accountRole.ID).
		ModelTableExpr(accountRoleTable)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "update account_role")
}

// DeleteByAccountAndRole deletes a specific account-role mapping
func (r *AccountRoleRepository) DeleteByAccountAndRole(ctx context.Context, accountID, roleID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.AccountRole)(nil)).
		ModelTableExpr(accountRoleTableAlias).
		Where(`"account_role".account_id = ? AND "account_role".role_id = ?`, accountID, roleID)

	if where, val, ok := base.TenantWhere(ctx, "account_role"); ok {
		query = query.Where(where, val)
	}

	_, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by account and role",
			Err: err,
		}
	}

	return nil
}

// DeleteByAccountID deletes all account-role mappings for an account
func (r *AccountRoleRepository) DeleteByAccountID(ctx context.Context, accountID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.AccountRole)(nil)).
		ModelTableExpr(accountRoleTableAlias).
		Where(`"account_role".account_id = ?`, accountID)

	if where, val, ok := base.TenantWhere(ctx, "account_role"); ok {
		query = query.Where(where, val)
	}

	_, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by account ID",
			Err: err,
		}
	}

	return nil
}

// DeleteByRoleID deletes all account-role mappings for a role
func (r *AccountRoleRepository) DeleteByRoleID(ctx context.Context, roleID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.AccountRole)(nil)).
		ModelTableExpr(accountRoleTableAlias).
		Where(`"account_role".role_id = ?`, roleID)

	if where, val, ok := base.TenantWhere(ctx, "account_role"); ok {
		query = query.Where(where, val)
	}

	_, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by role ID",
			Err: err,
		}
	}

	return nil
}

// List retrieves account-role mappings matching the provided filters
func (r *AccountRoleRepository) List(ctx context.Context, filters map[string]any) ([]*auth.AccountRole, error) {
	var accountRoles []*auth.AccountRole
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&accountRoles).
		ModelTableExpr(accountRoleTableAlias)

	if where, val, ok := base.TenantWhere(ctx, "account_role"); ok {
		query = query.Where(where, val)
	}

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = query.Where(`"account_role".? = ?`, bun.Ident(field), value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return accountRoles, nil
}

// FindAccountRolesWithDetails retrieves account-role mappings with account and role details
func (r *AccountRoleRepository) FindAccountRolesWithDetails(ctx context.Context, filters map[string]any) ([]*auth.AccountRole, error) {
	var accountRoles []*auth.AccountRole
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&accountRoles).
		ModelTableExpr(accountRoleTableAlias).
		ColumnExpr(`"account_role".*`).
		ColumnExpr(`"account".id AS "account__id", "account".email AS "account__email", "account".username AS "account__username", "account".active AS "account__active", "account".created_at AS "account__created_at", "account".updated_at AS "account__updated_at"`).
		ColumnExpr(`"role".id AS "role__id", "role".name AS "role__name", "role".description AS "role__description", "role".is_system AS "role__is_system", "role".created_at AS "role__created_at", "role".updated_at AS "role__updated_at"`).
		Join(`LEFT JOIN auth.accounts AS "account" ON "account".id = "account_role".account_id`).
		Join(`LEFT JOIN auth.roles AS "role" ON "role".id = "account_role".role_id`)

	if where, val, ok := base.TenantWhere(ctx, "account_role"); ok {
		query = query.Where(where, val)
	}

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = query.Where(`"account_role".`+field+` = ?`, value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with details",
			Err: err,
		}
	}

	return accountRoles, nil
}
