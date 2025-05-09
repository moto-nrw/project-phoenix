package auth

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// RoleRepository implements auth.RoleRepository interface
type RoleRepository struct {
	*base.Repository[*auth.Role]
	db *bun.DB
}

// NewRoleRepository creates a new RoleRepository
func NewRoleRepository(db *bun.DB) auth.RoleRepository {
	return &RoleRepository{
		Repository: base.NewRepository[*auth.Role](db, "auth.roles", "Role"),
		db:         db,
	}
}

// FindByName retrieves a role by its name
func (r *RoleRepository) FindByName(ctx context.Context, name string) (*auth.Role, error) {
	role := new(auth.Role)
	err := r.db.NewSelect().
		Model(role).
		Where("LOWER(name) = LOWER(?)", name).
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

// AssignRoleToAccount assigns a role to an account
func (r *RoleRepository) AssignRoleToAccount(ctx context.Context, accountID int64, roleID int64) error {
	// Check if the role assignment already exists
	exists, err := r.db.NewSelect().
		Model((*auth.AccountRole)(nil)).
		Where("account_id = ? AND role_id = ?", accountID, roleID).
		Exists(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "check role assignment",
			Err: err,
		}
	}

	if exists {
		// Role already assigned, no action needed
		return nil
	}

	// Create the role assignment
	_, err = r.db.NewInsert().
		Model(&auth.AccountRole{
			AccountID: accountID,
			RoleID:    roleID,
		}).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "assign role to account",
			Err: err,
		}
	}

	return nil
}

// RemoveRoleFromAccount removes a role assignment from an account
func (r *RoleRepository) RemoveRoleFromAccount(ctx context.Context, accountID int64, roleID int64) error {
	_, err := r.db.NewDelete().
		Model((*auth.AccountRole)(nil)).
		Where("account_id = ? AND role_id = ?", accountID, roleID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "remove role from account",
			Err: err,
		}
	}

	return nil
}

// GetRoleWithPermissions retrieves a role with its associated permissions
func (r *RoleRepository) GetRoleWithPermissions(ctx context.Context, roleID int64) (*auth.Role, error) {
	role := new(auth.Role)
	err := r.db.NewSelect().
		Model(role).
		Where("id = ?", roleID).
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

// Create overrides the base Create method to handle name normalization
func (r *RoleRepository) Create(ctx context.Context, role *auth.Role) error {
	if role == nil {
		return fmt.Errorf("role cannot be nil")
	}

	// Validate role - this will also normalize the name
	if err := role.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, role)
}

// Update overrides the base Update method to handle name normalization
func (r *RoleRepository) Update(ctx context.Context, role *auth.Role) error {
	if role == nil {
		return fmt.Errorf("role cannot be nil")
	}

	// Validate role - this will also normalize the name
	if err := role.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, role)
}

// List retrieves roles matching the provided filters
func (r *RoleRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.Role, error) {
	var roles []*auth.Role
	query := r.db.NewSelect().Model(&roles)

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

	return roles, nil
}
