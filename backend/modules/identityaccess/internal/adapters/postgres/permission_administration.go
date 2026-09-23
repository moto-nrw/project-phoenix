package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

type permissionRow struct {
	bun.BaseModel `bun:"table:auth.permissions,alias:permission"`
	ID            int64     `bun:"id,pk,autoincrement"`
	Name          string    `bun:"name"`
	Description   string    `bun:"description"`
	Resource      string    `bun:"resource"`
	Action        string    `bun:"action"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func permissionRecord(p domain.ManagedPermission) permissionRow {
	return permissionRow{ID: p.ID, Name: p.Name, Description: p.Description, Resource: p.Resource,
		Action: p.Action, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

func (p permissionRow) domain() domain.ManagedPermission {
	return domain.ManagedPermission{ID: p.ID, Name: p.Name, Description: p.Description, Resource: p.Resource,
		Action: p.Action, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

func (s *Store) CreatePermission(ctx context.Context, permission domain.ManagedPermission) (domain.ManagedPermission, error) {
	if err := permission.Validate(); err != nil {
		return domain.ManagedPermission{}, err
	}
	db, err := s.database(ctx)
	if err != nil {
		return domain.ManagedPermission{}, err
	}
	row := permissionRecord(permission)
	if _, err := db.NewInsert().Model(&row).ModelTableExpr("auth.permissions").Returning("id, created_at, updated_at").Exec(ctx); err != nil {
		return domain.ManagedPermission{}, fmt.Errorf("create: %w", err)
	}
	return row.domain(), nil
}

func (s *Store) FindPermission(ctx context.Context, id int64) (domain.ManagedPermission, bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.ManagedPermission{}, false, err
	}
	row := new(permissionRow)
	err = db.NewSelect().Model(row).ModelTableExpr("auth.permissions AS permission").Where("permission.id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ManagedPermission{}, false, nil
	}
	if err != nil {
		return domain.ManagedPermission{}, false, fmt.Errorf("find by id: %w", err)
	}
	return row.domain(), true, nil
}

func (s *Store) FindPermissionByName(ctx context.Context, name string) (domain.ManagedPermission, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.ManagedPermission{}, err
	}
	row := new(permissionRow)
	err = db.NewSelect().Model(row).ModelTableExpr("auth.permissions AS permission").
		Where("LOWER(permission.name) = LOWER(?)", name).Scan(ctx)
	if err != nil {
		return domain.ManagedPermission{}, fmt.Errorf("find by name: %w", err)
	}
	return row.domain(), nil
}

func (s *Store) UpdatePermission(ctx context.Context, permission domain.ManagedPermission) error {
	if err := permission.Validate(); err != nil {
		return err
	}
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	row := permissionRecord(permission)
	_, err = db.NewUpdate().Model(&row).ModelTableExpr("auth.permissions").
		Where("id = ?", permission.ID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	return nil
}

func (s *Store) DeletePermission(ctx context.Context, id int64) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Table("auth.permissions").Where("id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

func (s *Store) ListPermissions(ctx context.Context, filter domain.PermissionFilter) ([]domain.ManagedPermission, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	query := db.NewSelect().TableExpr("auth.permissions AS permission").ColumnExpr("permission.id, permission.name, permission.description, permission.resource, permission.action, permission.created_at, permission.updated_at")
	if filter.Resource != "" {
		query = query.Where("LOWER(permission.resource) = LOWER(?)", filter.Resource)
	}
	if filter.Action != "" {
		query = query.Where("LOWER(permission.action) = LOWER(?)", filter.Action)
	}
	return scanPermissions(ctx, query, "list")
}

func scanPermissions(ctx context.Context, query *bun.SelectQuery, operation string) ([]domain.ManagedPermission, error) {
	var permissions []domain.ManagedPermission
	if err := query.Scan(ctx, &permissions); err != nil {
		return nil, fmt.Errorf("%s: %w", operation, err)
	}
	return permissions, nil
}

func (s *Store) ListRolePermissions(ctx context.Context, roleID int64) ([]domain.ManagedPermission, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	return scanPermissions(ctx, db.NewSelect().TableExpr("auth.permissions AS permission").ColumnExpr("permission.id, permission.name, permission.description, permission.resource, permission.action, permission.created_at, permission.updated_at").
		Join("JOIN auth.role_permissions rp ON rp.permission_id = permission.id").
		Where("rp.role_id = ?", roleID), "find by role ID")
}

// ListAccountPermissions preserves the administration view: direct grants plus role grants.
// Explicit denials are applied by the authentication evaluator, not by this catalog view.
func (s *Store) ListAccountPermissions(ctx context.Context, accountID int64) ([]domain.ManagedPermission, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	direct := db.NewSelect().TableExpr("auth.account_permissions AS account_permission").
		ColumnExpr("account_permission.permission_id").Where("account_permission.account_id = ? AND account_permission.granted = true", accountID)
	roles := db.NewSelect().TableExpr("auth.role_permissions AS role_permission").
		ColumnExpr("role_permission.permission_id").
		Join("JOIN auth.account_roles AS ar ON ar.role_id = role_permission.role_id").Where("ar.account_id = ?", accountID)
	if tenantID := s.scope(ctx).TenantID; tenantID > 0 {
		direct = direct.Where("account_permission.tenant_id = ?", tenantID)
		roles = roles.Where("ar.tenant_id = ?", tenantID)
	}
	return scanPermissions(ctx, db.NewSelect().TableExpr("auth.permissions AS permission").
		ColumnExpr("permission.id, permission.name, permission.description, permission.resource, permission.action, permission.created_at, permission.updated_at").Distinct().
		Join("JOIN (?) AS aap ON aap.permission_id = permission.id", direct.UnionAll(roles)), "find by account ID")
}

func (s *Store) ListAccountDirectPermissions(ctx context.Context, accountID int64) ([]domain.ManagedPermission, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	query := db.NewSelect().TableExpr("auth.permissions AS permission").ColumnExpr("permission.id, permission.name, permission.description, permission.resource, permission.action, permission.created_at, permission.updated_at").
		Join("JOIN auth.account_permissions ap ON ap.permission_id = permission.id").
		Where("ap.account_id = ? AND ap.granted = true", accountID)
	if tenantID := s.scope(ctx).TenantID; tenantID > 0 {
		query = query.Where("ap.tenant_id = ?", tenantID)
	}
	return scanPermissions(ctx, query, "find direct permissions by account ID")
}

// AssignRolePermission is idempotent under the caller's role-row lock.
func (s *Store) AssignRolePermission(ctx context.Context, roleID, permissionID int64) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	count, err := db.NewSelect().Table("auth.role_permissions").Where("role_id = ? AND permission_id = ?", roleID, permissionID).Count(ctx)
	if err != nil {
		return fmt.Errorf("check permission assignment to role: %w", err)
	}
	if count > 0 {
		return nil
	}
	_, err = db.NewRaw("INSERT INTO auth.role_permissions (role_id, permission_id) VALUES (?, ?)", roleID, permissionID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("assign permission to role: %w", err)
	}
	return nil
}

func (s *Store) RemoveRolePermission(ctx context.Context, roleID, permissionID int64) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Table("auth.role_permissions").Where("role_id = ? AND permission_id = ?", roleID, permissionID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("remove permission from role: %w", err)
	}
	return nil
}

func (s *Store) DeleteRolePermissions(ctx context.Context, roleID int64) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Table("auth.role_permissions").Where("role_id = ?", roleID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete by role ID: %w", err)
	}
	return nil
}

func (s *Store) DeletePermissionAssignments(ctx context.Context, permissionID int64) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Table("auth.role_permissions").Where("permission_id = ?", permissionID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete by permission ID: %w", err)
	}
	return nil
}

func (s *Store) DeletePermissionGrants(ctx context.Context, permissionID int64) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewDelete().TableExpr("auth.account_permissions AS account_permission").Where("account_permission.permission_id = ?", permissionID)
	if tenantID := s.scope(ctx).TenantID; tenantID > 0 {
		query = query.Where("account_permission.tenant_id = ?", tenantID)
	}
	_, err = query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete by permission ID: %w", err)
	}
	return nil
}

func (s *Store) GrantAccountPermission(ctx context.Context, accountID, permissionID int64) error {
	return s.setAccountPermission(ctx, accountID, permissionID, true, "grant permission")
}

func (s *Store) DenyAccountPermission(ctx context.Context, accountID, permissionID int64) error {
	return s.setAccountPermission(ctx, accountID, permissionID, false, "deny permission")
}

// setAccountPermission retains the existence/update sequence under the account-row lock.
// Tenantless administrative callers update all matching mappings, as before; school callers
// always constrain both the existence check and update to their school.
func (s *Store) setAccountPermission(ctx context.Context, accountID, permissionID int64, granted bool, operation string) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	tenantID := s.scope(ctx).TenantID
	query := db.NewSelect().TableExpr("auth.account_permissions AS account_permission").
		Where("account_permission.account_id = ? AND account_permission.permission_id = ?", accountID, permissionID)
	if tenantID > 0 {
		query = query.Where("account_permission.tenant_id = ?", tenantID)
	}
	exists, err := query.Exists(ctx)
	if err != nil {
		return fmt.Errorf("check permission mapping: %w", err)
	}
	if exists {
		update := db.NewUpdate().TableExpr("auth.account_permissions AS account_permission").
			Set("granted = ?", granted).Where("account_permission.account_id = ? AND account_permission.permission_id = ?", accountID, permissionID)
		if tenantID > 0 {
			update = update.Where("account_permission.tenant_id = ?", tenantID)
		}
		_, err = update.Exec(ctx)
	} else {
		_, err = db.NewRaw("INSERT INTO auth.account_permissions (account_id, permission_id, granted, tenant_id) VALUES (?, ?, ?, ?)",
			accountID, permissionID, granted, tenantID).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

func (s *Store) RemoveAccountPermission(ctx context.Context, accountID, permissionID int64) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewDelete().TableExpr("auth.account_permissions AS account_permission").
		Where("account_permission.account_id = ? AND account_permission.permission_id = ?", accountID, permissionID)
	if tenantID := s.scope(ctx).TenantID; tenantID > 0 {
		query = query.Where("account_permission.tenant_id = ?", tenantID)
	}
	_, err = query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("remove permission: %w", err)
	}
	return nil
}
