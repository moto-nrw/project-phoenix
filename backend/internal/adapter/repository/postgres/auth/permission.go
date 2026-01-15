package auth

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/uptrace/bun"
)

const (
	permissionTable           = "auth.permissions"
	permissionTableAlias      = `auth.permissions AS "permission"`
	rolePermissionsTable      = "auth.role_permissions"
	whereAccountAndPermission = "account_id = ? AND permission_id = ?"
)

// PermissionRepository implements auth.PermissionRepository interface
type PermissionRepository struct {
	*base.Repository[*auth.Permission]
	db *bun.DB
}

// NewPermissionRepository creates a new PermissionRepository
func NewPermissionRepository(db *bun.DB) auth.PermissionRepository {
	return &PermissionRepository{
		Repository: base.NewRepository[*auth.Permission](db, permissionTable, "Permission"),
		db:         db,
	}
}

// FindByName retrieves a permission by its name
func (r *PermissionRepository) FindByName(ctx context.Context, name string) (*auth.Permission, error) {
	permission := new(auth.Permission)
	err := r.db.NewSelect().
		Model(permission).
		ModelTableExpr(permissionTableAlias).
		Where(`LOWER("permission".name) = LOWER(?)`, name).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: err,
		}
	}

	return permission, nil
}

// FindByResourceAction retrieves a permission by resource and action
func (r *PermissionRepository) FindByResourceAction(ctx context.Context, resource, action string) (*auth.Permission, error) {
	permission := new(auth.Permission)
	err := r.db.NewSelect().
		Model(permission).
		ModelTableExpr(permissionTableAlias).
		Where(`LOWER("permission".resource) = LOWER(?) AND LOWER("permission".action) = LOWER(?)`, resource, action).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by resource and action",
			Err: err,
		}
	}

	return permission, nil
}

// FindByAccountID retrieves all permissions assigned to an account (direct + role-based)
func (r *PermissionRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.Permission, error) {
	var permissions []*auth.Permission

	// This query combines permissions from direct assignments and role-based permissions
	err := r.db.NewSelect().
		Model(&permissions).
		ModelTableExpr(permissionTableAlias).
		Distinct().
		With("account_permissions_direct", r.db.NewSelect().
			Table("auth.account_permissions").
			Where("account_id = ? AND granted = true", accountID)).
		With("account_permissions_from_roles", r.db.NewSelect().
			Table(rolePermissionsTable).
			Join("JOIN auth.account_roles ar ON ar.role_id = role_permissions.role_id").
			Where("ar.account_id = ?", accountID)).
		With("all_account_permissions", r.db.NewSelect().
			Column("permission_id").
			TableExpr("account_permissions_direct").
			UnionAll(r.db.NewSelect().TableExpr("account_permissions_from_roles").Column("permission_id"))).
		Join(`JOIN all_account_permissions aap ON aap.permission_id = "permission".id`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: err,
		}
	}

	return permissions, nil
}

// FindDirectByAccountID retrieves only direct permissions assigned to an account (not role-based)
func (r *PermissionRepository) FindDirectByAccountID(ctx context.Context, accountID int64) ([]*auth.Permission, error) {
	var permissions []*auth.Permission

	// This query gets ONLY direct permissions, not role-based ones
	err := r.db.NewSelect().
		Model(&permissions).
		ModelTableExpr(permissionTableAlias).
		Join(`JOIN auth.account_permissions ap ON ap.permission_id = "permission".id`).
		Where("ap.account_id = ? AND ap.granted = true", accountID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find direct permissions by account ID",
			Err: err,
		}
	}

	return permissions, nil
}

// FindByRoleID retrieves all permissions assigned to a role
func (r *PermissionRepository) FindByRoleID(ctx context.Context, roleID int64) ([]*auth.Permission, error) {
	var permissions []*auth.Permission
	err := r.db.NewSelect().
		Model(&permissions).
		ModelTableExpr(permissionTableAlias).
		Join(`JOIN auth.role_permissions rp ON rp.permission_id = "permission".id`).
		Where("rp.role_id = ?", roleID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by role ID",
			Err: err,
		}
	}

	return permissions, nil
}

// AssignPermissionToAccount assigns a permission directly to an account
func (r *PermissionRepository) AssignPermissionToAccount(ctx context.Context, accountID int64, permissionID int64) error {
	// Check if the permission assignment already exists
	exists, err := r.db.NewSelect().
		Model((*auth.AccountPermission)(nil)).
		ModelTableExpr(`auth.account_permissions AS "account_permission"`).
		Where(whereAccountAndPermission, accountID, permissionID).
		Exists(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "check permission assignment",
			Err: err,
		}
	}

	if exists {
		// Update the existing assignment to ensure it's granted
		_, err = r.db.NewUpdate().
			Model((*auth.AccountPermission)(nil)).
			ModelTableExpr(`auth.account_permissions AS "account_permission"`).
			Set("granted = true").
			Where(whereAccountAndPermission, accountID, permissionID).
			Exec(ctx)

		if err != nil {
			return &modelBase.DatabaseError{
				Op:  "update permission assignment",
				Err: err,
			}
		}

		return nil
	}

	// Create the permission assignment
	_, err = r.db.NewInsert().
		Model(&auth.AccountPermission{
			AccountID:    accountID,
			PermissionID: permissionID,
			Granted:      true,
		}).
		ModelTableExpr(`auth.account_permissions AS "account_permission"`).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "assign permission to account",
			Err: err,
		}
	}

	return nil
}

// RemovePermissionFromAccount removes a permission assignment from an account
func (r *PermissionRepository) RemovePermissionFromAccount(ctx context.Context, accountID int64, permissionID int64) error {
	_, err := r.db.NewDelete().
		Model((*auth.AccountPermission)(nil)).
		ModelTableExpr(`auth.account_permissions AS "account_permission"`).
		Where(whereAccountAndPermission, accountID, permissionID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "remove permission from account",
			Err: err,
		}
	}

	return nil
}

// AssignPermissionToRole assigns a permission to a role
func (r *PermissionRepository) AssignPermissionToRole(ctx context.Context, roleID int64, permissionID int64) error {
	// Get the database connection (or transaction if in context)
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	// Check if the permission assignment already exists
	count, err := db.NewSelect().
		Table(rolePermissionsTable).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Count(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "check permission assignment to role",
			Err: err,
		}
	}

	if count > 0 {
		// Already assigned, nothing to do
		return nil
	}

	// Create the permission assignment
	_, err = db.NewInsert().
		Model(&auth.RolePermission{
			RoleID:       roleID,
			PermissionID: permissionID,
		}).
		ModelTableExpr(rolePermissionsTable).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "assign permission to role",
			Err: err,
		}
	}

	return nil
}

// RemovePermissionFromRole removes a permission assignment from a role
func (r *PermissionRepository) RemovePermissionFromRole(ctx context.Context, roleID int64, permissionID int64) error {
	// Get the database connection (or transaction if in context)
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	_, err := db.NewDelete().
		Table(rolePermissionsTable).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "remove permission from role",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *PermissionRepository) Create(ctx context.Context, permission *auth.Permission) error {
	if permission == nil {
		return fmt.Errorf("permission cannot be nil")
	}

	// Validate permission - this will also normalize the name
	if err := permission.Validate(); err != nil {
		return err
	}

	// Use the base Create method which now uses ModelTableExpr
	return r.Repository.Create(ctx, permission)
}

// Update overrides the base Update method for schema consistency
func (r *PermissionRepository) Update(ctx context.Context, permission *auth.Permission) error {
	if permission == nil {
		return fmt.Errorf("permission cannot be nil")
	}

	// Validate permission - this will also normalize the name
	if err := permission.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(permission).
		Where(whereID, permission.ID).
		ModelTableExpr(permissionTable)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(permission).
			Where(whereID, permission.ID).
			ModelTableExpr(permissionTable)
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

// List retrieves permissions matching the provided filters
func (r *PermissionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.Permission, error) {
	var permissions []*auth.Permission
	query := r.db.NewSelect().
		Model(&permissions).
		ModelTableExpr(permissionTableAlias)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = r.applyPermissionFilter(query, field, value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return permissions, nil
}

// applyPermissionFilter applies a single filter to the query
func (r *PermissionRepository) applyPermissionFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "name":
		return r.applyPermissionStringEqualFilter(query, `"permission".name`, value)
	case "resource":
		return r.applyPermissionStringEqualFilter(query, `"permission".resource`, value)
	case "action":
		return r.applyPermissionStringEqualFilter(query, `"permission".action`, value)
	case "name_like":
		return r.applyPermissionStringLikeFilter(query, `"permission".name`, value)
	case "is_system":
		return query.Where(`"permission".is_system = ?`, value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// applyPermissionStringEqualFilter applies case-insensitive equality filter for permission fields
func (r *PermissionRepository) applyPermissionStringEqualFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER("+field+") = LOWER(?)", strValue)
	}
	return query.Where(field+" = ?", value)
}

// applyPermissionStringLikeFilter applies case-insensitive LIKE filter for permission fields
func (r *PermissionRepository) applyPermissionStringLikeFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER("+field+") LIKE LOWER(?)", "%"+strValue+"%")
	}
	return query
}

// FindByRoleByName retrieves a role by its name
func (r *PermissionRepository) FindByRoleByName(ctx context.Context, roleName string) (*auth.Role, error) {
	role := new(auth.Role)
	err := r.db.NewSelect().
		Model(role).
		ModelTableExpr("auth.roles AS role").
		Where("LOWER(role.name) = LOWER(?)", roleName).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find role by name",
			Err: err,
		}
	}

	return role, nil
}
