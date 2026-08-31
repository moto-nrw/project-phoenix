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
func (r *PermissionRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.Permission, error) {
	return r.FindByAccountIDForTenant(ctx, accountID, 0)
}

// FindByAccountIDForTenant retrieves all permissions for an account scoped to a specific tenant.
// tenantID > 0: filter account_permissions and account_roles by that tenant (login/switch flows).
// tenantID == 0: fall back to context-based filtering via TenantWhere (authenticated requests).
func (r *PermissionRepository) FindByAccountIDForTenant(ctx context.Context, accountID int64, tenantID int64) ([]*auth.Permission, error) {
	var permissions []*auth.Permission

	// Build direct permissions CTE with tenant filter
	directCTE := base.GetDB(ctx, r.db).NewSelect().
		Table("auth.account_permissions").
		Where("account_id = ? AND granted = true", accountID)

	// Build role-based permissions CTE with tenant filter
	roleCTE := base.GetDB(ctx, r.db).NewSelect().
		Table(rolePermissionsTable).
		Join("JOIN auth.account_roles ar ON ar.role_id = role_permissions.role_id").
		Where("ar.account_id = ?", accountID)

	// Apply tenant filtering: explicit tenant ID takes priority, then context
	if tenantID > 0 {
		directCTE = directCTE.Where("account_permissions.tenant_id = ?", tenantID)
		roleCTE = roleCTE.Where("ar.tenant_id = ?", tenantID)
	} else if where, val, ok := base.TenantWhere(ctx, "account_permissions"); ok {
		directCTE = directCTE.Where(where, val)
		// Use same tenant ID for role CTE
		roleCTE = roleCTE.Where(`ar.tenant_id = ?`, val)
	}

	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&permissions).
		ModelTableExpr(permissionTableAlias).
		Distinct().
		With("account_permissions_direct", directCTE).
		With("account_permissions_from_roles", roleCTE).
		With("all_account_permissions", base.GetDB(ctx, r.db).NewSelect().
			Column("permission_id").
			TableExpr("account_permissions_direct").
			UnionAll(base.GetDB(ctx, r.db).NewSelect().TableExpr("account_permissions_from_roles").Column("permission_id"))).
		Join(`JOIN all_account_permissions aap ON aap.permission_id = "permission".id`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return permissions, nil
}

// LockAccountPermissionSourcesForTenant takes FOR SHARE row locks on every row
// that feeds an account's effective permission set at one tenant: the direct
// grants in auth.account_permissions and the auth.role_permissions rows of the
// roles the account holds there. Must be called inside a transaction; it
// returns no data, only the locks.
//
// FindByAccountIDForTenant reads through two CTEs and a UNION, which Postgres
// refuses to attach a locking clause to. Taking the locks in two plain
// statements first is equivalent for the purpose they serve: once they are
// held, no concurrent revocation can commit until this transaction ends, so
// the permission set read afterwards — and written into a JWT — is provably
// the one that still existed at commit time. Role revocation and membership
// revocation are already pinned this way (FindByAccountIDForTenantForShare,
// ExistsActiveByAccountAndTenantForShare); without this the permission half of
// the same token was still read from an unlocked snapshot.
//
// Only revocations are serialized, deliberately: a FOR SHARE lock cannot cover
// rows that do not exist yet, so a permission GRANTED mid-mint may or may not
// make it into the token. That direction is harmless — the next token has it.
//
// LOCK ORDER — callers must already hold the account row (auth.accounts FOR
// UPDATE) and take these locks AFTER the account-role read, matching the order
// every revocation path walks (account → roles → permissions; see
// operator_account_access.go and staff_offboarding.go).
// Both statements are consumed with Scan into a throwaway slice rather than
// Exec. A locking SELECT returns rows, and pgdriver's Exec path (readQuery)
// has no case for the DataRow message — it survives today only because the
// protocol tag 'D' collides with Describe, which that switch happens to
// discard. Scanning the rows uses readQueryData, which handles them by
// contract instead of by coincidence. Do not "simplify" this back to Exec.
func (r *PermissionRepository) LockAccountPermissionSourcesForTenant(ctx context.Context, accountID int64, tenantID int64) error {
	db := base.GetDB(ctx, r.db)

	var lockedDirect []int
	if err := db.NewSelect().
		ColumnExpr("1").
		TableExpr("auth.account_permissions AS ap").
		Where("ap.account_id = ? AND ap.tenant_id = ?", accountID, tenantID).
		For("SHARE OF ap").
		Scan(ctx, &lockedDirect); err != nil {
		return &modelBase.DatabaseError{
			Op:  "lock direct account permissions",
			Err: base.TranslateNotFound(err),
		}
	}

	var lockedFromRoles []int
	if err := db.NewSelect().
		ColumnExpr("1").
		TableExpr(rolePermissionsTable+" AS rp").
		Where(`rp.role_id IN (SELECT ar.role_id FROM auth.account_roles AS ar WHERE ar.account_id = ? AND ar.tenant_id = ?)`, accountID, tenantID).
		For("SHARE OF rp").
		Scan(ctx, &lockedFromRoles); err != nil {
		return &modelBase.DatabaseError{
			Op:  "lock role permissions",
			Err: base.TranslateNotFound(err),
		}
	}

	return nil
}

// FindDirectByAccountID retrieves only direct permissions assigned to an account (not role-based)
func (r *PermissionRepository) FindDirectByAccountID(ctx context.Context, accountID int64) ([]*auth.Permission, error) {
	var permissions []*auth.Permission

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
func (r *PermissionRepository) FindByRoleID(ctx context.Context, roleID int64) ([]*auth.Permission, error) {
	var permissions []*auth.Permission
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
		Model(&auth.RolePermission{
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
func (r *PermissionRepository) Update(ctx context.Context, permission *auth.Permission) error {
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
func (r *PermissionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.Permission, error) {
	var permissions []*auth.Permission
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
