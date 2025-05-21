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
		ModelTableExpr("auth.roles AS role").
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
		ModelTableExpr("auth.roles AS role").
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
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM auth.account_roles WHERE account_id = $1 AND role_id = $2", 
		accountID, roleID).Scan(&count)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "check role assignment",
			Err: err,
		}
	}

	if count > 0 {
		// Role already assigned, no action needed
		return nil
	}

	// Create the role assignment
	_, err = r.db.Exec(
		"INSERT INTO auth.account_roles (account_id, role_id) VALUES ($1, $2)", 
		accountID, roleID)

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
	_, err := r.db.Exec(
		"DELETE FROM auth.account_roles WHERE account_id = $1 AND role_id = $2",
		accountID, roleID)

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
		ModelTableExpr("auth.roles AS role").
		Where("role.id = ?", roleID).
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
		Where("role.id = ?", role.ID).
		ModelTableExpr("auth.roles AS role")

	// Extract transaction from context if it exists
	if tx, ok := ctx.Value("tx").(*bun.Tx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(role).
			Where("role.id = ?", role.ID).
			ModelTableExpr("auth.roles AS role")
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
		ModelTableExpr("auth.roles AS role")

	// Apply filters
	for field, value := range filters {
		if value != nil {
			switch field {
			case "name":
				// Case-insensitive name search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(role.name) = LOWER(?)", strValue)
				} else {
					query = query.Where("role.name = ?", value)
				}
			case "name_like":
				// Case-insensitive name pattern search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(role.name) LIKE LOWER(?)", "%"+strValue+"%")
				}
			case "is_system":
				query = query.Where("role.is_system = ?", value)
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
