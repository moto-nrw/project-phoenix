package authpostgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
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
	*base.Repository[*authmodels.Permission]
	db *bun.DB
}

// NewPermissionRepository creates a new PermissionRepository
func NewPermissionRepository(db *bun.DB) authmodels.PermissionRepository {
	return &PermissionRepository{
		Repository: base.NewRepository[*authmodels.Permission](db, permissionTable, "Permission"),
		db:         db,
	}
}

// FindByName retrieves a permission by its name
func (r *PermissionRepository) FindByName(ctx context.Context, name string) (*authmodels.Permission, error) {
	permission := new(authmodels.Permission)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(permission).
		ModelTableExpr(permissionTableAlias).
		Where(`LOWER("permission".name) = LOWER(?)`, name).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: base.TranslateNotFound(err),
		}
	}

	return permission, nil
}

// FindByAccountID retrieves all permissions assigned to an account (direct + role-based).
// When tenant context is available, filters assignments by tenant_id for tenant isolation.
func (r *PermissionRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*authmodels.Permission, error) {
	var permissions []*authmodels.Permission

	// Direct grants, filtered by tenant
	direct := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr(`"account_permission".permission_id`).
		TableExpr(`auth.account_permissions AS "account_permission"`).
		Where(`"account_permission".account_id = ? AND "account_permission".granted = true`, accountID)

	// Role-granted permissions, filtered by tenant
	fromRoles := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr(`"role_permission".permission_id`).
		TableExpr(`auth.role_permissions AS "role_permission"`).
		Join(`JOIN auth.account_roles AS "ar" ON "ar".role_id = "role_permission".role_id`).
		Where(`"ar".account_id = ?`, accountID)

	// Apply the tenant filter the context carries.
	if _, val, ok := base.TenantWhere(ctx, "account_permission"); ok {
		direct = direct.Where(`"account_permission".tenant_id = ?`, val)
		// Use same tenant ID for the role-granted set
		fromRoles = fromRoles.Where(`"ar".tenant_id = ?`, val)
	}

	// The union is joined as a derived table so the evaluator resolves every
	// table this read touches.
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&permissions).
		ModelTableExpr(permissionTableAlias).
		Distinct().
		Join(`JOIN (?) AS "aap" ON "aap".permission_id = "permission".id`, direct.UnionAll(fromRoles)).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return permissions, nil
}

// FindDirectByAccountID retrieves only direct permissions assigned to an account (not role-based)
func (r *PermissionRepository) FindDirectByAccountID(ctx context.Context, accountID int64) ([]*authmodels.Permission, error) {
	var permissions []*authmodels.Permission

	// This query gets ONLY direct permissions, not role-based ones
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&permissions).
		ModelTableExpr(permissionTableAlias).
		Join(`JOIN auth.account_permissions ap ON ap.permission_id = "permission".id`).
		Where("ap.account_id = ? AND ap.granted = true", accountID)
	query = base.WithTenantFilter(ctx, query, "ap")
	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find direct permissions by account ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return permissions, nil
}

// FindByRoleID retrieves all permissions assigned to a role
func (r *PermissionRepository) FindByRoleID(ctx context.Context, roleID int64) ([]*authmodels.Permission, error) {
	var permissions []*authmodels.Permission
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&permissions).
		ModelTableExpr(permissionTableAlias).
		Join(`JOIN auth.role_permissions rp ON rp.permission_id = "permission".id`).
		Where("rp.role_id = ?", roleID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by role ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return permissions, nil
}

// AssignPermissionToRole assigns a permission to a role
func (r *PermissionRepository) AssignPermissionToRole(ctx context.Context, roleID int64, permissionID int64) error {
	db := base.GetDB(ctx, r.db)

	// Check if the permission assignment already exists
	count, err := db.NewSelect().
		Table(rolePermissionsTable).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Count(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "check permission assignment to role",
			Err: base.TranslateNotFound(err),
		}
	}

	if count > 0 {
		// Already assigned, nothing to do
		return nil
	}

	// Create the permission assignment
	_, err = db.NewInsert().
		Model(&authmodels.RolePermission{
			RoleID:       roleID,
			PermissionID: permissionID,
		}).
		ModelTableExpr(rolePermissionsTable).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "assign permission to role",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// RemovePermissionFromRole removes a permission assignment from a role
func (r *PermissionRepository) RemovePermissionFromRole(ctx context.Context, roleID int64, permissionID int64) error {
	_, err := base.GetDB(ctx, r.db).NewDelete().
		Table(rolePermissionsTable).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "remove permission from role",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// Update overrides the base Update method for schema consistency
func (r *PermissionRepository) Update(ctx context.Context, permission *authmodels.Permission) error {
	if permission == nil {
		return fmt.Errorf("permission cannot be nil")
	}

	// Validate permission - this will also normalize the name
	if err := permission.Validate(); err != nil {
		return err
	}

	// Execute the query using GetDB for transaction support
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(permission).
		Where(whereID, permission.ID).
		ModelTableExpr(permissionTable).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// List retrieves permissions matching the provided filters
func (r *PermissionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*authmodels.Permission, error) {
	var permissions []*authmodels.Permission
	query := base.GetDB(ctx, r.db).NewSelect().
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
			Err: base.TranslateNotFound(err),
		}
	}

	return permissions, nil
}

// applyPermissionFilter applies a single filter to the query
func (r *PermissionRepository) applyPermissionFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "name":
		return r.applyPermissionStringEqualFilter(query, bun.Safe(`"permission".name`), value)
	case "resource":
		return r.applyPermissionStringEqualFilter(query, bun.Safe(`"permission".resource`), value)
	case "action":
		return r.applyPermissionStringEqualFilter(query, bun.Safe(`"permission".action`), value)
	case "name_like":
		return r.applyPermissionStringLikeFilter(query, bun.Safe(`"permission".name`), value)
	case "is_system":
		return query.Where(`"permission".is_system = ?`, value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// applyPermissionStringEqualFilter applies case-insensitive equality filter for permission fields
// The field is a column written in this file, never request input.
func (r *PermissionRepository) applyPermissionStringEqualFilter(query *bun.SelectQuery, field bun.Safe, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER(?) = LOWER(?)", field, strValue)
	}
	return query.Where("? = ?", field, value)
}

// applyPermissionStringLikeFilter applies case-insensitive LIKE filter for permission fields
func (r *PermissionRepository) applyPermissionStringLikeFilter(query *bun.SelectQuery, field bun.Safe, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER(?) LIKE LOWER(?)", field, "%"+strValue+"%")
	}
	return query
}
