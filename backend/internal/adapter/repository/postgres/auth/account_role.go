package auth

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	authPort "github.com/moto-nrw/project-phoenix/internal/core/port/auth"
	"github.com/uptrace/bun"
)

const (
	accountRoleTable      = "auth.account_roles"
	accountRoleTableAlias = `auth.account_roles AS "account_role"`
)

// AccountRoleRepository implements authPort.AccountRoleRepository interface
type AccountRoleRepository struct {
	*base.Repository[*auth.AccountRole]
	db *bun.DB
}

// NewAccountRoleRepository creates a new AccountRoleRepository
func NewAccountRoleRepository(db *bun.DB) authPort.AccountRoleRepository {
	return &AccountRoleRepository{
		Repository: base.NewRepository[*auth.AccountRole](db, accountRoleTable, "AccountRole"),
		db:         db,
	}
}

// FindByAccountID retrieves all account-role mappings for an account
func (r *AccountRoleRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.AccountRole, error) {
	var accountRoles []*auth.AccountRole
	err := r.db.NewSelect().
		Model(&accountRoles).
		ModelTableExpr(accountRoleTableAlias).
		Join(`LEFT JOIN auth.roles AS "role" ON "role".id = "account_role".role_id`).
		ColumnExpr(`"account_role".*`).
		ColumnExpr(`"role".id AS "role__id", "role".created_at AS "role__created_at", "role".updated_at AS "role__updated_at", "role".name AS "role__name", "role".description AS "role__description"`).
		Where(`"account_role".account_id = ?`, accountID).
		Scan(ctx)

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
	err := r.db.NewSelect().
		Model(&accountRoles).
		ModelTableExpr(accountRoleTableAlias).
		Where(`"account_role".role_id = ?`, roleID).
		Scan(ctx)

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
	err := r.db.NewSelect().
		Model(accountRole).
		ModelTableExpr(accountRoleTableAlias).
		Where(`"account_role".account_id = ? AND "account_role".role_id = ?`, accountID, roleID).
		Scan(ctx)

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

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewInsert().
		Model(accountRole).
		ModelTableExpr(accountRoleTable)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewInsert().
			Model(accountRole).
			ModelTableExpr(accountRoleTable)
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
func (r *AccountRoleRepository) Update(ctx context.Context, accountRole *auth.AccountRole) error {
	if accountRole == nil {
		return fmt.Errorf("account role cannot be nil")
	}

	// Validate accountRole
	if err := accountRole.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(accountRole).
		Where("id = ?", accountRole.ID).
		ModelTableExpr(accountRoleTable)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(accountRole).
			Where("id = ?", accountRole.ID).
			ModelTableExpr(accountRoleTable)
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

// DeleteByAccountAndRole deletes a specific account-role mapping
func (r *AccountRoleRepository) DeleteByAccountAndRole(ctx context.Context, accountID, roleID int64) error {
	_, err := r.db.NewDelete().
		Model((*auth.AccountRole)(nil)).
		ModelTableExpr(accountRoleTableAlias).
		Where(`"account_role".account_id = ? AND "account_role".role_id = ?`, accountID, roleID).
		Exec(ctx)

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
	_, err := r.db.NewDelete().
		Model((*auth.AccountRole)(nil)).
		ModelTableExpr(accountRoleTableAlias).
		Where(`"account_role".account_id = ?`, accountID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by account ID",
			Err: err,
		}
	}

	return nil
}

// List retrieves account-role mappings matching the provided filters
func (r *AccountRoleRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.AccountRole, error) {
	var accountRoles []*auth.AccountRole
	query := r.db.NewSelect().
		Model(&accountRoles).
		ModelTableExpr(accountRoleTableAlias)

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
func (r *AccountRoleRepository) FindAccountRolesWithDetails(ctx context.Context, filters map[string]interface{}) ([]*auth.AccountRole, error) {
	var accountRoles []*auth.AccountRole
	query := r.db.NewSelect().
		Model(&accountRoles).
		ModelTableExpr(accountRoleTableAlias).
		Relation("Account").
		Relation("Role")

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = query.Where(`"account_role".? = ?`, bun.Ident(field), value)
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
