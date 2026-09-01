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
	repo := base.NewRepository[*auth.AccountPermission](db, accountPermissionTable, "AccountPermission")
	repo.TenantScoped = true
	return &AccountPermissionRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByAccountID retrieves all account-permission mappings for an account
func (r *AccountPermissionRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.AccountPermission, error) {
	var accountPermissions []*auth.AccountPermission
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&accountPermissions).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".account_id = ?`, accountID)

	query = base.WithTenantFilter(ctx, query, "account_permission")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return accountPermissions, nil
}

// GrantPermission grants a permission to an account
func (r *AccountPermissionRepository) GrantPermission(ctx context.Context, accountID, permissionID int64) error {
	db := base.GetDB(ctx, r.db)

	// Check if the permission mapping already exists
	existsQuery := db.NewSelect().
		Model((*auth.AccountPermission)(nil)).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID)

	existsQuery = base.WithTenantFilter(ctx, existsQuery, "account_permission")

	exists, err := existsQuery.Exists(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "check permission mapping",
			Err: base.TranslateNotFound(err),
		}
	}

	if exists {
		// Update the existing mapping to grant the permission
		updateQuery := db.NewUpdate().
			Model((*auth.AccountPermission)(nil)).
			ModelTableExpr(accountPermissionTableAlias).
			Set("granted = ?", true).
			Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID)

		updateQuery = base.WithTenantFilter(ctx, updateQuery, "account_permission")

		_, err = updateQuery.Exec(ctx)
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
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// DenyPermission explicitly denies a permission to an account
func (r *AccountPermissionRepository) DenyPermission(ctx context.Context, accountID, permissionID int64) error {
	db := base.GetDB(ctx, r.db)

	// Check if the permission mapping already exists
	existsQuery := db.NewSelect().
		Model((*auth.AccountPermission)(nil)).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID)

	existsQuery = base.WithTenantFilter(ctx, existsQuery, "account_permission")

	exists, err := existsQuery.Exists(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "check permission mapping",
			Err: base.TranslateNotFound(err),
		}
	}

	if exists {
		// Update the existing mapping to deny the permission
		updateQuery := db.NewUpdate().
			Model((*auth.AccountPermission)(nil)).
			ModelTableExpr(accountPermissionTableAlias).
			Set("granted = ?", false).
			Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID)

		updateQuery = base.WithTenantFilter(ctx, updateQuery, "account_permission")

		_, err = updateQuery.Exec(ctx)
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
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// RemovePermission removes a permission mapping for an account
func (r *AccountPermissionRepository) RemovePermission(ctx context.Context, accountID, permissionID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.AccountPermission)(nil)).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".account_id = ? AND "account_permission".permission_id = ?`, accountID, permissionID)

	query = base.WithTenantFilter(ctx, query, "account_permission")

	_, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "remove permission",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// DeleteByPermissionID deletes all account-permission mappings for a permission
func (r *AccountPermissionRepository) DeleteByPermissionID(ctx context.Context, permissionID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.AccountPermission)(nil)).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".permission_id = ?`, permissionID)

	query = base.WithTenantFilter(ctx, query, "account_permission")

	_, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by permission ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// DeleteByAccountID deletes all tenant-scoped direct permission grants for an
// account. Used by staff offboarding so a re-invited account does not regain
// old elevated permissions. Returns the number of deleted rows.
func (r *AccountPermissionRepository) DeleteByAccountID(ctx context.Context, accountID int64) (int64, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.AccountPermission)(nil)).
		ModelTableExpr(accountPermissionTableAlias).
		Where(`"account_permission".account_id = ?`, accountID)

	query = base.WithTenantFilter(ctx, query, "account_permission")

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete by account ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return result.RowsAffected()
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

	base.EnsureTenantID(ctx, accountPermission)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(accountPermission).
		ModelTableExpr(accountPermissionTable).
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
func (r *AccountPermissionRepository) Update(ctx context.Context, accountPermission *auth.AccountPermission) error {
	if accountPermission == nil {
		return fmt.Errorf("account permission cannot be nil")
	}

	// Validate accountPermission
	if err := accountPermission.Validate(); err != nil {
		return err
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(accountPermission).
		Where("id = ?", accountPermission.ID).
		ModelTableExpr(accountPermissionTable)

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

	return base.AssertRowsAffected(result, 1, "update account_permission")
}

// List retrieves account-permission mappings matching the provided filters
func (r *AccountPermissionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.AccountPermission, error) {
	var accountPermissions []*auth.AccountPermission
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&accountPermissions).
		ModelTableExpr(accountPermissionTableAlias)

	query = base.WithTenantFilter(ctx, query, "account_permission")

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
			Err: base.TranslateNotFound(err),
		}
	}

	return accountPermissions, nil
}
