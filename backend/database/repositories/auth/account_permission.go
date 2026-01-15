package auth

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	accountPermissionTable      = "auth.account_permissions"
	accountPermissionTableAlias = `auth.account_permissions AS "account_permission"`
)

// AccountPermissionRepository implements auth.AccountPermissionRepository interface
type AccountPermissionRepository struct {
	*base.Repository[*auth.AccountPermission]
	db *bun.DB
}

// NewAccountPermissionRepository creates a new AccountPermissionRepository
func NewAccountPermissionRepository(db *bun.DB) auth.AccountPermissionRepository {
	return &AccountPermissionRepository{
		Repository: base.NewRepository[*auth.AccountPermission](db, accountPermissionTable, "AccountPermission"),
		db:         db,
	}
}

// FindByAccountID retrieves all account-permission mappings for an account
func (r *AccountPermissionRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.AccountPermission, error) {
	var accountPermissions []*auth.AccountPermission
	err := r.db.NewSelect().
		Model(&accountPermissions).
		ModelTableExpr(accountPermissionTable).
		Where("account_id = ?", accountID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: err,
		}
	}

	return accountPermissions, nil
}

// FindByPermissionID retrieves all account-permission mappings for a permission
func (r *AccountPermissionRepository) FindByPermissionID(ctx context.Context, permissionID int64) ([]*auth.AccountPermission, error) {
	var accountPermissions []*auth.AccountPermission
	err := r.db.NewSelect().
		Model(&accountPermissions).
		ModelTableExpr(accountPermissionTable).
		Where("permission_id = ?", permissionID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by permission ID",
			Err: err,
		}
	}

	return accountPermissions, nil
}

// FindByAccountAndPermission retrieves a specific account-permission mapping
func (r *AccountPermissionRepository) FindByAccountAndPermission(ctx context.Context, accountID, permissionID int64) (*auth.AccountPermission, error) {
	accountPermission := new(auth.AccountPermission)
	err := r.db.NewSelect().
		Model(accountPermission).
		ModelTableExpr(accountPermissionTable).
		Where("account_id = ? AND permission_id = ?", accountID, permissionID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account and permission",
			Err: err,
		}
	}

	return accountPermission, nil
}

// GrantPermission grants a permission to an account
func (r *AccountPermissionRepository) GrantPermission(ctx context.Context, accountID, permissionID int64) error {
	// Get the database connection (or transaction if in context)
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	// Check if the permission mapping already exists
	exists, err := db.NewSelect().
		Model((*auth.AccountPermission)(nil)).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID).
		Exists(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "check permission mapping",
			Err: err,
		}
	}

	if exists {
		// Update the existing mapping to grant the permission
		_, err = db.NewUpdate().
			Model((*auth.AccountPermission)(nil)).
			ModelTableExpr(accountPermissionTableAlias).
			Set(`"account_permission".granted = ?`, true).
			Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID).
			Exec(ctx)
	} else {
		// Create a new permission mapping
		_, err = db.NewInsert().
			Model(&auth.AccountPermission{
				AccountID:    accountID,
				PermissionID: permissionID,
				Granted:      true,
			}).
			ModelTableExpr(accountPermissionTable).
			Exec(ctx)
	}

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "grant permission",
			Err: err,
		}
	}

	return nil
}

// DenyPermission explicitly denies a permission to an account
func (r *AccountPermissionRepository) DenyPermission(ctx context.Context, accountID, permissionID int64) error {
	// Get the database connection (or transaction if in context)
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	// Check if the permission mapping already exists
	exists, err := db.NewSelect().
		Model((*auth.AccountPermission)(nil)).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID).
		Exists(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "check permission mapping",
			Err: err,
		}
	}

	if exists {
		// Update the existing mapping to deny the permission
		_, err = db.NewUpdate().
			Model((*auth.AccountPermission)(nil)).
			ModelTableExpr(accountPermissionTableAlias).
			Set(`"account_permission".granted = ?`, false).
			Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID).
			Exec(ctx)
	} else {
		// Create a new permission mapping with denied status
		_, err = db.NewInsert().
			Model(&auth.AccountPermission{
				AccountID:    accountID,
				PermissionID: permissionID,
				Granted:      false,
			}).
			ModelTableExpr(accountPermissionTable).
			Exec(ctx)
	}

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "deny permission",
			Err: err,
		}
	}

	return nil
}

// RemovePermission removes a permission mapping for an account
func (r *AccountPermissionRepository) RemovePermission(ctx context.Context, accountID, permissionID int64) error {
	// Get the database connection (or transaction if in context)
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	_, err := db.NewDelete().
		Model((*auth.AccountPermission)(nil)).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "remove permission",
			Err: err,
		}
	}

	return nil
}

// / Create overrides the base Create method for schema consistency
func (r *AccountPermissionRepository) Create(ctx context.Context, accountPermission *auth.AccountPermission) error {
	if accountPermission == nil {
		return fmt.Errorf("account permission cannot be nil")
	}

	// Validate accountPermission
	if err := accountPermission.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewInsert().
		Model(accountPermission).
		ModelTableExpr(accountPermissionTable)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewInsert().
			Model(accountPermission).
			ModelTableExpr(accountPermissionTable)
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
func (r *AccountPermissionRepository) Update(ctx context.Context, accountPermission *auth.AccountPermission) error {
	if accountPermission == nil {
		return fmt.Errorf("account permission cannot be nil")
	}

	// Validate accountPermission
	if err := accountPermission.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(accountPermission).
		Where("id = ?", accountPermission.ID).
		ModelTableExpr(accountPermissionTable)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(accountPermission).
			Where("id = ?", accountPermission.ID).
			ModelTableExpr(accountPermissionTable)
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

// List retrieves account-permission mappings matching the provided filters
func (r *AccountPermissionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.AccountPermission, error) {
	var accountPermissions []*auth.AccountPermission
	query := r.db.NewSelect().
		Model(&accountPermissions).
		ModelTableExpr(accountPermissionTable)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			switch field {
			case "granted":
				query = query.Where("granted = ?", value)
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

	return accountPermissions, nil
}

// FindAccountPermissionsWithDetails retrieves account-permission mappings with account and permission details
func (r *AccountPermissionRepository) FindAccountPermissionsWithDetails(ctx context.Context, filters map[string]interface{}) ([]*auth.AccountPermission, error) {
	var accountPermissions []*auth.AccountPermission
	query := r.db.NewSelect().
		Model(&accountPermissions).
		ModelTableExpr(accountPermissionTable).
		Relation("Account").
		Relation("Permission")

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = query.Where("account_permission.? = ?", bun.Ident(field), value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with details",
			Err: err,
		}
	}

	return accountPermissions, nil
}
