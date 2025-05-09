package auth

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// RolePermissionRepository implements auth.RolePermissionRepository interface
type RolePermissionRepository struct {
	*base.Repository[*auth.RolePermission]
	db *bun.DB
}

// NewRolePermissionRepository creates a new RolePermissionRepository
func NewRolePermissionRepository(db *bun.DB) auth.RolePermissionRepository {
	return &RolePermissionRepository{
		Repository: base.NewRepository[*auth.RolePermission](db, "auth.role_permissions", "RolePermission"),
		db:         db,
	}
}

// FindByRoleID retrieves all role-permission mappings for a role
func (r *RolePermissionRepository) FindByRoleID(ctx context.Context, roleID int64) ([]*auth.RolePermission, error) {
	var rolePermissions []*auth.RolePermission
	err := r.db.NewSelect().
		Model(&rolePermissions).
		Where("role_id = ?", roleID).
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
		Where("permission_id = ?", permissionID).
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
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
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

	// Use the base Create method
	return r.Repository.Create(ctx, rolePermission)
}

// DeleteByRoleAndPermission deletes a specific role-permission mapping
func (r *RolePermissionRepository) DeleteByRoleAndPermission(ctx context.Context, roleID, permissionID int64) error {
	_, err := r.db.NewDelete().
		Model((*auth.RolePermission)(nil)).
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

// List retrieves role-permission mappings matching the provided filters
func (r *RolePermissionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.RolePermission, error) {
	var rolePermissions []*auth.RolePermission
	query := r.db.NewSelect().Model(&rolePermissions)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = query.Where("? = ?", bun.Ident(field), value)
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
func (r *RolePermissionRepository) FindRolePermissionsWithDetails(ctx context.Context, filters map[string]interface{}) ([]*auth.RolePermission, error) {
	var rolePermissions []*auth.RolePermission
	query := r.db.NewSelect().
		Model(&rolePermissions).
		Relation("Role").
		Relation("Permission")

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = query.Where("role_permission.? = ?", bun.Ident(field), value)
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
