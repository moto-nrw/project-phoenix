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
	err := base.GetDB(ctx, r.db).NewSelect().
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

	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(rolePermission).
		Where("id = ?", rolePermission.ID).
		ModelTableExpr(rolePermissionTable).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return nil
}

// DeleteByRoleID deletes all role-permission mappings for a role
func (r *RolePermissionRepository) DeleteByRoleID(ctx context.Context, roleID int64) error {
	_, err := base.GetDB(ctx, r.db).NewDelete().
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
	_, err := base.GetDB(ctx, r.db).NewDelete().
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rolePermissions).
		ModelTableExpr(rolePermissionTableAlias)

	// Apply filters with proper table alias prefix
	for field, value := range filters {
		if value != nil {
			query = query.Where(`"role_permission".? = ?`, bun.Ident(field), value)
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
