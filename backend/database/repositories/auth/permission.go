package auth

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// PermissionRepository implements auth.PermissionRepository interface
type PermissionRepository struct {
	*base.Repository[*auth.Permission]
	db *bun.DB
}

// NewPermissionRepository creates a new PermissionRepository
func NewPermissionRepository(db *bun.DB) auth.PermissionRepository {
	return &PermissionRepository{
		Repository: base.NewRepository[*auth.Permission](db, "auth.permissions", "Permission"),
		db:         db,
	}
}

// FindByName retrieves a permission by its name
func (r *PermissionRepository) FindByName(ctx context.Context, name string) (*auth.Permission, error) {
	permission := new(auth.Permission)
	err := r.db.NewSelect().
		Model(permission).
		Where("LOWER(name) = LOWER(?)", name).
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
		Where("LOWER(resource) = LOWER(?) AND LOWER(action) = LOWER(?)", resource, action).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by resource and action",
			Err: err,
		}
	}

	return permission, nil
}

// FindByAccountID retrieves all permissions assigned to an account
func (r *PermissionRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.Permission, error) {
	var permissions []*auth.Permission

	// This query combines permissions from direct assignments and role-based permissions
	err := r.db.NewSelect().
		Model(&permissions).
		Distinct().
		With("account_permissions_direct", r.db.NewSelect().
			Table("auth.account_permissions").
			Where("account_id = ? AND granted = true", accountID)).
		With("account_permissions_from_roles", r.db.NewSelect().
			Table("auth.role_permissions").
			Join("JOIN auth.account_roles ar ON ar.role_id = role_permissions.role_id").
			Where("ar.account_id = ?", accountID)).
		With("all_account_permissions", r.db.NewSelect().
			Column("permission_id").
			TableExpr("account_permissions_direct").
			UnionAll(r.db.NewSelect().TableExpr("account_permissions_from_roles").Column("permission_id"))).
		Join("JOIN all_account_permissions aap ON aap.permission_id = permission.id").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
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
		Join("JOIN auth.role_permissions rp ON rp.permission_id = permission.id").
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
		Where("account_id = ? AND permission_id = ?", accountID, permissionID).
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
			Set("granted = true").
			Where("account_id = ? AND permission_id = ?", accountID, permissionID).
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
		Where("account_id = ? AND permission_id = ?", accountID, permissionID).
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
	// Check if the permission assignment already exists
	exists, err := r.db.NewSelect().
		Model((*auth.RolePermission)(nil)).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Exists(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "check permission assignment to role",
			Err: err,
		}
	}

	if exists {
		// Already assigned, nothing to do
		return nil
	}

	// Create the permission assignment
	_, err = r.db.NewInsert().
		Model(&auth.RolePermission{
			RoleID:       roleID,
			PermissionID: permissionID,
		}).
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
	_, err := r.db.NewDelete().
		Model((*auth.RolePermission)(nil)).
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
		Where("id = ?", permission.ID).
		ModelTableExpr("auth.permissions")

	// Extract transaction from context if it exists
	if tx, ok := ctx.Value("tx").(*bun.Tx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(permission).
			Where("id = ?", permission.ID).
			ModelTableExpr("auth.permissions")
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
	query := r.db.NewSelect().Model(&permissions)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			switch field {
			case "name":
				// Case-insensitive name search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(name) = LOWER(?)", strValue)
				} else {
					query = query.Where("name = ?", value)
				}
			case "resource":
				// Case-insensitive resource search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(resource) = LOWER(?)", strValue)
				} else {
					query = query.Where("resource = ?", value)
				}
			case "action":
				// Case-insensitive action search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(action) = LOWER(?)", strValue)
				} else {
					query = query.Where("action = ?", value)
				}
			case "name_like":
				// Case-insensitive name pattern search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(name) LIKE LOWER(?)", "%"+strValue+"%")
				}
			case "is_system":
				query = query.Where("is_system = ?", value)
			default:
				// Default to exact match for other fields
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

	return permissions, nil
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
