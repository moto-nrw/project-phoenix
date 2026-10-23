package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
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

// copyRolePermissionsAttempts bounds the retries of AssignRolePermissions.
// Each retry follows a catalog deletion that committed during the statement.
const copyRolePermissionsAttempts = 3

// AssignRolePermissions gives a role every listed permission the catalog
// still holds, in one statement, so a permission deleted after the caller
// read the catalog is skipped instead of failing the role (#4574). A deletion
// that commits while the statement runs breaks its foreign key check; the
// statement then rolls back to its savepoint and runs again on the new
// catalog. phoenix_tenant may not lock catalog rows (no UPDATE grant), so the
// retry stands in for a row lock.
func (s *Store) AssignRolePermissions(ctx context.Context, roleID int64, permissionIDs []int64) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	for attempt := 1; ; attempt++ {
		err = copyCatalogPermissions(ctx, db, roleID, permissionIDs)
		if err == nil || attempt == copyRolePermissionsAttempts || !isDeletedPermission(err) {
			return err
		}
	}
}

func copyCatalogPermissions(ctx context.Context, db bun.IDB, roleID int64, permissionIDs []int64) error {
	if _, err := db.ExecContext(ctx, "SAVEPOINT copy_role_permissions"); err != nil {
		return fmt.Errorf("copy permissions to role: %w", err)
	}
	_, err := db.NewRaw(`INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT ?, permission.id FROM auth.permissions AS permission WHERE permission.id IN (?)
		ON CONFLICT (role_id, permission_id) DO NOTHING`, roleID, bun.List(permissionIDs)).Exec(ctx)
	if err != nil {
		if _, rollbackErr := db.ExecContext(ctx, "ROLLBACK TO SAVEPOINT copy_role_permissions"); rollbackErr != nil {
			return fmt.Errorf("copy permissions to role: %w", errors.Join(err, rollbackErr))
		}
		return fmt.Errorf("copy permissions to role: %w", err)
	}
	if _, err := db.ExecContext(ctx, "RELEASE SAVEPOINT copy_role_permissions"); err != nil {
		return fmt.Errorf("copy permissions to role: %w", err)
	}
	return nil
}

// isDeletedPermission reports the foreign key violation of a role permission
// whose catalog permission was deleted concurrently.
func isDeletedPermission(err error) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) && postgresError.Field('C') == "23503" &&
		postgresError.Field('n') == "fk_role_permissions_permission"
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
	query = scopeTenantQuery(s.scope(ctx), query, "account_permission.tenant_id = ?")
	_, err = query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete by permission ID: %w", err)
	}
	return nil
}
