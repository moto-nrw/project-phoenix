package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Identity-owned reads and writes behind the operator-led school access
// flows (#3252). Every statement runs on the administrative transaction the
// flows open, so the RLS-guarded mapping, role and permission tables are
// fully visible; the mapping and role statements name their school
// explicitly instead of applying the context scope.

type managedAccountRow struct {
	ID     int64  `bun:"id"`
	Email  string `bun:"email"`
	Active bool   `bun:"active"`
}

func (s *Store) FindManagedAccount(ctx context.Context, id int64, forUpdate bool) (domain.ManagedAccount, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.ManagedAccount{}, false, domain.OperationStats{}, err
	}
	var rows []managedAccountRow
	query := db.NewSelect().
		TableExpr(`auth.accounts AS "account"`).
		ColumnExpr(`"account".id, "account".email, "account".active`).
		Where(`"account".id = ?`, id)
	if forUpdate {
		query = query.For("UPDATE")
	}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.ManagedAccount{}, false, stats, fmt.Errorf("identity access postgres: find managed account: %w", err)
	}
	if len(rows) == 0 {
		return domain.ManagedAccount{}, false, stats, nil
	}
	return domain.ManagedAccount{ID: rows[0].ID, Email: rows[0].Email, Active: rows[0].Active}, true, stats, nil
}

func (s *Store) SetAccountActive(ctx context.Context, id int64, active bool) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().
		TableExpr(`auth.accounts AS "account"`).
		Set("active = ?", active).
		Set("updated_at = NOW()").
		Where(`"account".id = ?`, id).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: set account active: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

type tenantMappingRow struct {
	TenantID      int64      `bun:"tenant_id"`
	Status        string     `bun:"status"`
	ActivatedAt   *time.Time `bun:"activated_at"`
	DeactivatedAt *time.Time `bun:"deactivated_at"`
}

// ListTenantMappings returns every mapping of the account, ascending by
// school, as the retained repository did.
func (s *Store) ListTenantMappings(ctx context.Context, accountID int64) ([]domain.TenantMapping, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []tenantMappingRow
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.account_tenants AS "account_tenant"`).
		ColumnExpr(`"account_tenant".tenant_id, "account_tenant".status, "account_tenant".activated_at, "account_tenant".deactivated_at`).
		Where(`"account_tenant".account_id = ?`, accountID).
		OrderExpr(`"account_tenant".tenant_id ASC`).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list tenant mappings: %w", err)
	}
	mappings := make([]domain.TenantMapping, 0, len(rows))
	for _, row := range rows {
		mappings = append(mappings, domain.TenantMapping{TenantID: row.TenantID, Status: row.Status, ActivatedAt: row.ActivatedAt, DeactivatedAt: row.DeactivatedAt})
	}
	return mappings, stats, nil
}

// DeactivateTenantMapping marks the mapping inactive and drops the staff
// calendar feed token the membership carried.
func (s *Store) DeactivateTenantMapping(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().
		TableExpr(`auth.account_tenants AS "account_tenant"`).
		Set("status = ?", "inactive").
		Set("deactivated_at = NOW()").
		Set("staff_calendar_feed_token = NULL").
		Set("updated_at = NOW()").
		Where(`"account_tenant".account_id = ?`, accountID).
		Where(`"account_tenant".tenant_id = ?`, tenantID).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: deactivate tenant mapping: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

type accountRoleAssignmentRow struct {
	TenantID int64   `bun:"tenant_id"`
	RoleID   int64   `bun:"role_id"`
	Name     string  `bun:"name"`
	IsSystem bool    `bun:"is_system"`
	BaseRole *string `bun:"base_role"`
}

// ListAccountRoleAssignments returns the account's assignments at every
// school with their role facts; a dangling assignment keeps its role id and
// an empty name, as the retained left join did.
func (s *Store) ListAccountRoleAssignments(ctx context.Context, accountID int64) ([]domain.AccountRoleAssignment, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []accountRoleAssignmentRow
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.account_roles AS "account_role"`).
		ColumnExpr(`"account_role".tenant_id, "account_role".role_id, COALESCE("role".name, '') AS name, COALESCE("role".is_system, false) AS is_system, "role".base_role`).
		Join(`LEFT JOIN auth.roles AS "role" ON "role".id = "account_role".role_id`).
		Where(`"account_role".account_id = ?`, accountID).
		OrderExpr(`"account_role".tenant_id ASC, "account_role".role_id ASC`).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list account role assignments: %w", err)
	}
	assignments := make([]domain.AccountRoleAssignment, 0, len(rows))
	for _, row := range rows {
		assignments = append(assignments, domain.AccountRoleAssignment{
			TenantID: row.TenantID,
			Role:     domain.AccountTenantRole{ID: row.RoleID, Name: row.Name, IsSystem: row.IsSystem, BaseRole: row.BaseRole},
		})
	}
	return assignments, stats, nil
}

func (s *Store) RemoveAccountRole(ctx context.Context, accountID, roleID, tenantID int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().
		TableExpr(`auth.account_roles AS "account_role"`).
		Where(`"account_role".account_id = ?`, accountID).
		Where(`"account_role".role_id = ?`, roleID).
		Where(`"account_role".tenant_id = ?`, tenantID).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: remove account role: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) DeleteAccountPermissionsAtTenant(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().
		TableExpr(`auth.account_permissions AS "account_permission"`).
		Where(`"account_permission".account_id = ?`, accountID).
		Where(`"account_permission".tenant_id = ?`, tenantID).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: delete account permissions at tenant: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

type roleFactRow struct {
	ID       int64   `bun:"id"`
	Name     string  `bun:"name"`
	IsSystem bool    `bun:"is_system"`
	BaseRole *string `bun:"base_role"`
	TenantID *int64  `bun:"tenant_id"`
}

func (r roleFactRow) toDomain() domain.RoleFact {
	return domain.RoleFact{ID: r.ID, Name: r.Name, IsSystem: r.IsSystem, BaseRole: r.BaseRole, TenantID: r.TenantID}
}

const roleFactColumns = `"role".id, "role".name, "role".is_system, "role".base_role, "role".tenant_id`

// FindRole resolves a role of any school; the access policy decides whether
// it may be handed out at the target school.
func (s *Store) FindRole(ctx context.Context, roleID int64) (domain.RoleFact, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.RoleFact{}, false, domain.OperationStats{}, err
	}
	var rows []roleFactRow
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.roles AS "role"`).
		ColumnExpr(roleFactColumns).
		Where(`"role".id = ?`, roleID).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.RoleFact{}, false, stats, fmt.Errorf("identity access postgres: find role: %w", err)
	}
	if len(rows) == 0 {
		return domain.RoleFact{}, false, stats, nil
	}
	return rows[0].toDomain(), true, stats, nil
}

func (s *Store) ListRoles(ctx context.Context) ([]domain.RoleFact, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []roleFactRow
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.roles AS "role"`).
		ColumnExpr(roleFactColumns).
		OrderExpr(`"role".id ASC`).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list roles: %w", err)
	}
	roles := make([]domain.RoleFact, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, row.toDomain())
	}
	return roles, stats, nil
}
