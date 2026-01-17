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
	rolePermissionTable      = "auth.role_permissions"
	rolePermissionTableAlias = `auth.role_permissions AS "role_permission"`
)

// RolePermissionRepository implements auth.RolePermissionRepository interface
type RolePermissionRepository struct {
	*base.Repository[*auth.RolePermission]
	db *bun.DB
}

// NewRolePermissionRepository creates a new RolePermissionRepository
func NewRolePermissionRepository(db *bun.DB) auth.RolePermissionRepository {
	return &RolePermissionRepository{
		Repository: base.NewRepository[*auth.RolePermission](db, rolePermissionTable, "RolePermission"),
		db:         db,
	}
}

// FindByRoleID retrieves all role-permission mappings for a role
func (r *RolePermissionRepository) FindByRoleID(ctx context.Context, roleID int64) ([]*auth.RolePermission, error) {
	var rolePermissions []*auth.RolePermission
	err := r.db.NewSelect().
		Model(&rolePermissions).
		ModelTableExpr(rolePermissionTableAlias).
		Where(`"role_permission".role_id = ?`, roleID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by role ID",
			Err: err,
		}
	}

	return rolePermissions, nil
}

// FindByPermissionID retrieves all role-permission mappings for a permission
func (r *RolePermissionRepository) FindByPermissionID(ctx context.Context, permissionID int64) ([]*auth.RolePermission, error) {
	var rolePermissions []*auth.RolePermission
	err := r.db.NewSelect().
		Model(&rolePermissions).
		ModelTableExpr(rolePermissionTableAlias).
		Where(`"role_permission".permission_id = ?`, permissionID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by permission ID",
			Err: err,
		}
	}

	return rolePermissions, nil
}

// FindByRoleAndPermission retrieves a specific role-permission mapping
func (r *RolePermissionRepository) FindByRoleAndPermission(ctx context.Context, roleID, permissionID int64) (*auth.RolePermission, error) {
	rolePermission := new(auth.RolePermission)
	err := r.db.NewSelect().
		Model(rolePermission).
		ModelTableExpr(rolePermissionTableAlias).
		Where(`"role_permission".role_id = ? AND "role_permission".permission_id = ?`, roleID, permissionID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by role and permission",
			Err: err,
		}
	}

	return rolePermission, nil
}

// Create overrides the base Create method to handle validation
func (r *RolePermissionRepository) Create(ctx context.Context, rolePermission *auth.RolePermission) error {
	if rolePermission == nil {
		return fmt.Errorf("role permission cannot be nil")
	}

	// Validate rolePermission
	if err := rolePermission.Validate(); err != nil {
		return err
	}

	// Use the base Create method which now uses ModelTableExpr
	return r.Repository.Create(ctx, rolePermission)
}

// Update overrides the base Update method for schema consistency
func (r *RolePermissionRepository) Update(ctx context.Context, rolePermission *auth.RolePermission) error {
	if rolePermission == nil {
		return fmt.Errorf("role permission cannot be nil")
	}

	// Validate rolePermission
	if err := rolePermission.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(rolePermission).
		Where("id = ?", rolePermission.ID).
		ModelTableExpr(rolePermissionTable)

	// Extract transaction from context if it exists
	if tx, ok := ctx.Value("tx").(*bun.Tx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(rolePermission).
			Where("id = ?", rolePermission.ID).
			ModelTableExpr(rolePermissionTable)
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

// DeleteByRoleAndPermission deletes a specific role-permission mapping
func (r *RolePermissionRepository) DeleteByRoleAndPermission(ctx context.Context, roleID, permissionID int64) error {
	_, err := r.db.NewDelete().
		Model((*auth.RolePermission)(nil)).
		ModelTableExpr(rolePermissionTable).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by role and permission",
			Err: err,
		}
	}

	return nil
}

// DeleteByRoleID deletes all role-permission mappings for a role
func (r *RolePermissionRepository) DeleteByRoleID(ctx context.Context, roleID int64) error {
	_, err := r.db.NewDelete().
		Model((*auth.RolePermission)(nil)).
		ModelTableExpr(rolePermissionTable).
		Where("role_id = ?", roleID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by role ID",
			Err: err,
		}
	}

	return nil
}

// DeleteByPermissionID deletes all role-permission mappings for a permission
func (r *RolePermissionRepository) DeleteByPermissionID(ctx context.Context, permissionID int64) error {
	_, err := r.db.NewDelete().
		Model((*auth.RolePermission)(nil)).
		ModelTableExpr(rolePermissionTable).
		Where("permission_id = ?", permissionID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete by permission ID",
			Err: err,
		}
	}

	return nil
}

// List retrieves role-permission mappings matching the provided filters
func (r *RolePermissionRepository) List(ctx context.Context, filters map[string]any) ([]*auth.RolePermission, error) {
	var rolePermissions []*auth.RolePermission
	query := r.db.NewSelect().
		Model(&rolePermissions).
		ModelTableExpr(rolePermissionTableAlias)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = query.Where(`"role_permission".`+field+` = ?`, value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return rolePermissions, nil
}

// FindRolePermissionsWithDetails retrieves role-permission mappings with role and permission details
func (r *RolePermissionRepository) FindRolePermissionsWithDetails(ctx context.Context, filters map[string]any) ([]*auth.RolePermission, error) {
	var rolePermissions []*auth.RolePermission
	query := r.db.NewSelect().
		Model(&rolePermissions).
		ModelTableExpr(rolePermissionTableAlias).
		ColumnExpr(`"role_permission".*`).
		ColumnExpr(`"role".id AS "role__id", "role".name AS "role__name", "role".description AS "role__description", "role".is_system AS "role__is_system", "role".created_at AS "role__created_at", "role".updated_at AS "role__updated_at"`).
		ColumnExpr(`"permission".id AS "permission__id", "permission".name AS "permission__name", "permission".description AS "permission__description", "permission".resource AS "permission__resource", "permission".action AS "permission__action", "permission".created_at AS "permission__created_at", "permission".updated_at AS "permission__updated_at"`).
		Join(`LEFT JOIN auth.roles AS "role" ON "role".id = "role_permission".role_id`).
		Join(`LEFT JOIN auth.permissions AS "permission" ON "permission".id = "role_permission".permission_id`)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = query.Where(`"role_permission".`+field+` = ?`, value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with details",
			Err: err,
		}
	}

	return rolePermissions, nil
}
