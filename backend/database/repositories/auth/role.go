package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	repo := base.NewRepository[*auth.Role](db, roleTable, "Role")
	repo.TenantScoped = true
	return &RoleRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByName retrieves a role by its name, scoped to the current tenant.
// Prefers tenant-specific roles over system roles when both exist.
func (r *RoleRepository) FindByName(ctx context.Context, name string) (*auth.Role, error) {
	role := new(auth.Role)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(role).
		ModelTableExpr(roleTableAlias).
		Where("LOWER(role.name) = LOWER(?)", name)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("(role.tenant_id = ? OR role.tenant_id IS NULL)", tenantID)
		// Prefer tenant-specific role over system role
		query = query.OrderExpr("role.tenant_id IS NULL ASC")
	}

	query = query.Limit(1)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: err,
		}
	}

	return role, nil
}

// FindByAccountID retrieves all roles assigned to an account.
// When tenant context is present, account-role assignments are scoped to that tenant.
func (r *RoleRepository) FindByAccountID(ctx context.Context, accountID int64) ([]*auth.Role, error) {
	var roles []*auth.Role
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&roles).
		ModelTableExpr(roleTableAlias).
		Join("JOIN auth.account_roles ar ON ar.role_id = role.id").
		Where("ar.account_id = ?", accountID)

	query = base.WithTenantFilter(ctx, query, "ar")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: err,
		}
	}

	return roles, nil
}

// FindRoleNamesByAccountIDs batch-loads the primary role name for each account ID.
// Returns a map of accountID → role name (first role found per account).
func (r *RoleRepository) FindRoleNamesByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	if len(accountIDs) == 0 {
		return make(map[int64]string), nil
	}

	type accountRoleRow struct {
		AccountID int64  `bun:"account_id"`
		RoleName  string `bun:"role_name"`
	}

	var rows []accountRoleRow
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr("auth.account_roles AS ar").
		Join("JOIN auth.roles AS role ON role.id = ar.role_id").
		ColumnExpr("ar.account_id").
		ColumnExpr("role.name AS role_name").
		Where("ar.account_id IN (?)", bun.List(accountIDs)).
		OrderExpr("ar.created_at ASC")

	query = base.WithTenantFilter(ctx, query, "ar")

	err := query.Scan(ctx, &rows)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find role names by account IDs",
			Err: err,
		}
	}

	result := make(map[int64]string, len(rows))
	for _, row := range rows {
		// Keep first role found per account (don't overwrite)
		if _, exists := result[row.AccountID]; !exists {
			result[row.AccountID] = row.RoleName
		}
	}

	return result, nil
}

// AssignRoleToAccount assigns a role to an account
// DEPRECATED: Use AccountRoleRepository.Create instead
func (r *RoleRepository) AssignRoleToAccount(_ context.Context, _ int64, _ int64) error {
	// This method should not be used directly
	// Use the AccountRoleRepository instead for proper ORM-based operations
	return &modelBase.DatabaseError{
		Op:  "assign role to account",
		Err: errors.New("deprecated: use AccountRoleRepository.Create instead"),
	}
}

// RemoveRoleFromAccount removes a role assignment from an account
// DEPRECATED: Use AccountRoleRepository.DeleteByAccountAndRole instead
func (r *RoleRepository) RemoveRoleFromAccount(_ context.Context, _ int64, _ int64) error {
	// This method should not be used directly
	// Use the AccountRoleRepository instead for proper ORM-based operations
	return &modelBase.DatabaseError{
		Op:  "remove role from account",
		Err: errors.New("deprecated: use AccountRoleRepository.DeleteByAccountAndRole instead"),
	}
}

// FindByID overrides the base FindByID to include system roles (tenant_id IS NULL)
// alongside tenant-specific roles in tenant-scoped lookups.
func (r *RoleRepository) FindByID(ctx context.Context, id any) (*auth.Role, error) {
	role := new(auth.Role)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(role).
		ModelTableExpr(roleTableAlias).
		Where(whereRoleID, id)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("(role.tenant_id = ? OR role.tenant_id IS NULL)", tenantID)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by id",
			Err: err,
		}
	}

	return role, nil
}

// Delete overrides the base Delete to restrict deletions to tenant-owned roles only.
// System roles (tenant_id IS NULL) cannot be deleted at the repository level.
func (r *RoleRepository) Delete(ctx context.Context, id any) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*auth.Role)(nil)).
		ModelTableExpr(roleTableAlias).
		Where(whereRoleID, id)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("role.tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete",
			Err: err,
		}
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return &modelBase.DatabaseError{
			Op:  "delete",
			Err: sql.ErrNoRows,
		}
	}

	return nil
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

	// Execute the query using GetDB for transaction support.
	// Only tenant-owned roles can be updated; system roles (tenant_id IS NULL) are excluded.
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(role).
		Where(whereRoleID, role.ID).
		ModelTableExpr(roleTableAlias)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("role.tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: sql.ErrNoRows,
		}
	}

	return nil
}

// List retrieves roles matching the provided filters.
// For tenant-scoped requests, returns both system roles (tenant_id IS NULL)
// and the tenant's own roles.
func (r *RoleRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.Role, error) {
	var roles []*auth.Role
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&roles).
		ModelTableExpr(roleTableAlias)

	// Tenant filter: show system roles + tenant's own roles
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("(role.tenant_id = ? OR role.tenant_id IS NULL)", tenantID)
	}

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
