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
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&accountPermissions).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".account_id = ?`, accountID).
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
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&accountPermissions).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".permission_id = ?`, permissionID).
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
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(accountPermission).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID).
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
	db := base.GetDB(ctx, r.db)

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
			Set("granted = ?", true).
			Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID).
			Exec(ctx)
	} else {
		// Create a new permission mapping
		perm := &auth.AccountPermission{
			AccountID:    accountID,
			PermissionID: permissionID,
			Granted:      true,
		}
		perm.SetTenantID(tenant.FromContext(ctx))
		_, err = db.NewInsert().
			Model(perm).
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
	db := base.GetDB(ctx, r.db)

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
			Set("granted = ?", false).
			Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID).
			Exec(ctx)
	} else {
		// Create a new permission mapping with denied status
		// Use raw SQL to ensure granted=false is explicitly set (BUN may skip zero values with defaults)
		_, err = db.NewRaw(
			"INSERT INTO auth.account_permissions (account_id, permission_id, granted, tenant_id) VALUES (?, ?, false, ?)",
			accountID, permissionID, tenant.FromContext(ctx),
		).Exec(ctx)
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
	_, err := base.GetDB(ctx, r.db).NewDelete().
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

// DeleteByPermissionID deletes all account-permission mappings for a permission
func (r *AccountPermissionRepository) DeleteByPermissionID(ctx context.Context, permissionID int64) error {
	_, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.AccountPermission)(nil)).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".permission_id = ?`, permissionID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by permission ID",
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

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(accountPermission).
		ModelTableExpr(accountPermissionTable).
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
func (r *AccountPermissionRepository) Update(ctx context.Context, accountPermission *auth.AccountPermission) error {
	if accountPermission == nil {
		return fmt.Errorf("account permission cannot be nil")
	}

	// Validate accountPermission
	if err := accountPermission.Validate(); err != nil {
		return err
	}

	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(accountPermission).
		Where("id = ?", accountPermission.ID).
		ModelTableExpr(accountPermissionTable).
		Exec(ctx)
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&accountPermissions).
		ModelTableExpr(accountPermissionTableAlias)

	// Apply filters with proper table alias prefix
	for field, value := range filters {
		if value != nil {
			switch field {
			case "granted":
				query = query.Where(`"account_permission".granted = ?`, value)
			default:
				query = query.Where(`"account_permission".? = ?`, bun.Ident(field), value)
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&accountPermissions).
		ModelTableExpr(accountPermissionTableAlias).
		ColumnExpr(`"account_permission".*`).
		ColumnExpr(`"account".id AS "account__id", "account".email AS "account__email", "account".username AS "account__username", "account".active AS "account__active", "account".created_at AS "account__created_at", "account".updated_at AS "account__updated_at"`).
		ColumnExpr(`"permission".id AS "permission__id", "permission".name AS "permission__name", "permission".description AS "permission__description", "permission".resource AS "permission__resource", "permission".action AS "permission__action", "permission".created_at AS "permission__created_at", "permission".updated_at AS "permission__updated_at"`).
		Join(`LEFT JOIN auth.accounts AS "account" ON "account".id = "account_permission".account_id`).
		Join(`LEFT JOIN auth.permissions AS "permission" ON "permission".id = "account_permission".permission_id`)

	// Apply filters with proper table alias prefix
	for field, value := range filters {
		if value != nil {
			query = query.Where(`"account_permission".? = ?`, bun.Ident(field), value)
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
