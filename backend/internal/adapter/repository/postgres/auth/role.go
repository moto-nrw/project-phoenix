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
	roleTable      = "auth.roles"
	roleTableAlias = "auth.roles AS role"
	whereRoleID    = "role.id = ?"
)

// RoleRepository implements auth.RoleRepository interface
type RoleRepository struct {
	*base.Repository[*auth.Role]
	db *bun.DB
}

// NewRoleRepository creates a new RoleRepository
func NewRoleRepository(db *bun.DB) auth.RoleRepository {
	return &RoleRepository{
		Repository: base.NewRepository[*auth.Role](db, roleTable, "Role"),
		db:         db,
	}
}

// FindByName retrieves a role by its name
func (r *RoleRepository) FindByName(ctx context.Context, name string) (*auth.Role, error) {
	role := new(auth.Role)
	err := r.db.NewSelect().
		Model(role).
		ModelTableExpr(roleTableAlias).
		Where("LOWER(role.name) = LOWER(?)", name).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: err,
		}
	}

	return role, nil
}

// FindByAccountID retrieves all roles assigned to an account
func (r *RoleRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.Role, error) {
	var roles []*auth.Role
	err := r.db.NewSelect().
		Model(&roles).
		ModelTableExpr(roleTableAlias).
		Join("JOIN auth.account_roles ar ON ar.role_id = role.id").
		Where("ar.account_id = ?", accountID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: err,
		}
	}

	return roles, nil
}

// GetRoleWithPermissions retrieves a role with its associated permissions
func (r *RoleRepository) GetRoleWithPermissions(ctx context.Context, roleID int64) (*auth.Role, error) {
	role := new(auth.Role)
	err := r.db.NewSelect().
		Model(role).
		ModelTableExpr(roleTableAlias).
		Where(whereRoleID, roleID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get role",
			Err: err,
		}
	}

	// Load permissions for the role
	var permissions []*auth.Permission
	err = r.db.NewSelect().
		Model(&permissions).
		ModelTableExpr("auth.permissions AS permission").
		Join("JOIN auth.role_permissions rp ON rp.permission_id = permission.id").
		Where("rp.role_id = ?", roleID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get role permissions",
			Err: err,
		}
	}

	role.Permissions = permissions
	return role, nil
}

// Create overrides the base Create method to handle validation
func (r *RoleRepository) Create(ctx context.Context, role *auth.Role) error {
	if role == nil {
		return fmt.Errorf("role cannot be nil")
	}

	// Validate role
	if err := role.Validate(); err != nil {
		return err
	}

	// Use the base Create method which now uses ModelTableExpr
	return r.Repository.Create(ctx, role)
}

// Update overrides the base Update method for schema consistency
func (r *RoleRepository) Update(ctx context.Context, role *auth.Role) error {
	if role == nil {
		return fmt.Errorf("role cannot be nil")
	}

	// Validate role
	if err := role.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(role).
		Where(whereRoleID, role.ID).
		ModelTableExpr(roleTableAlias)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(role).
			Where(whereRoleID, role.ID).
			ModelTableExpr(roleTableAlias)
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

// List retrieves roles matching the provided filters
func (r *RoleRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.Role, error) {
	var roles []*auth.Role
	query := r.db.NewSelect().
		Model(&roles).
		ModelTableExpr(roleTableAlias)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = r.applyRoleFilter(query, field, value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return roles, nil
}

// applyRoleFilter applies a single filter to the query
func (r *RoleRepository) applyRoleFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "name":
		return r.applyRoleStringEqualFilter(query, "role.name", value)
	case "name_like":
		return r.applyRoleStringLikeFilter(query, "role.name", value)
	case "is_system":
		return query.Where("role.is_system = ?", value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// applyRoleStringEqualFilter applies case-insensitive equality filter for role fields
func (r *RoleRepository) applyRoleStringEqualFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER("+field+") = LOWER(?)", strValue)
	}
	return query.Where(field+" = ?", value)
}

// applyRoleStringLikeFilter applies case-insensitive LIKE filter for role fields
func (r *RoleRepository) applyRoleStringLikeFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER("+field+") LIKE LOWER(?)", "%"+strValue+"%")
	}
	return query
}
