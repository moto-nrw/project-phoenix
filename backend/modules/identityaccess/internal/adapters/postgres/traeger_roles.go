package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// Träger roles (ADR 0084, #4649) live in auth.organization_account_roles and
// carry no tenant: only the administrative transaction of the Träger sign-in,
// refresh and membership commands reaches them.

const listTraegerRolesQuery = `SELECT role FROM auth.organization_account_roles
	WHERE organization_id = ? AND account_id = ?
	ORDER BY role`

// ListTraegerRoles returns the account's Träger roles at the Träger.
func (s *Store) ListTraegerRoles(ctx context.Context, organizationID, accountID int64) ([]string, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	roles := []string{}
	started := time.Now()
	err = db.NewRaw(listTraegerRolesQuery, organizationID, accountID).Scan(ctx, &roles)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(roles))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list Träger roles: %w", err)
	}
	return roles, stats, nil
}

const lockLiveTraegerAccountQuery = `SELECT id FROM auth.accounts
	WHERE id = ? AND personal_data_erased_at IS NULL
	FOR UPDATE`

// LockLiveTraegerAccount locks the account row until the grant commits and
// reports whether the account is still live. auth.erase_unused_accounts locks
// with SKIP LOCKED and passes over it; when the erasure locked first, the
// re-checked row is erased and the grant finds no live account.
func (s *Store) LockLiveTraegerAccount(ctx context.Context, accountID int64) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewRaw(lockLiveTraegerAccountQuery, accountID).Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(ids))}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: lock Träger account: %w", err)
	}
	return len(ids) == 1, stats, nil
}

const insertTraegerRoleQuery = `INSERT INTO auth.organization_account_roles (organization_id, account_id, role)
	VALUES (?, ?, ?)
	ON CONFLICT ON CONSTRAINT uq_organization_account_roles_role DO NOTHING`

// InsertTraegerRole grants the role once; a held role stays unchanged.
func (s *Store) InsertTraegerRole(ctx context.Context, organizationID, accountID int64, role string) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(insertTraegerRoleQuery, organizationID, accountID, role).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: insert Träger role: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

// The Träger sessions and the school sessions opened through them (ADR 0084
// section 5): both name the Träger in organization_id.
const deleteTraegerSessionsQuery = `DELETE FROM auth.tokens
	WHERE organization_id = ?`

// DeleteTraegerSessions deletes every Träger session of the Träger (#4657)
// and every school session opened through it (#4654): none of them can be
// refreshed or handed off again.
func (s *Store) DeleteTraegerSessions(ctx context.Context, organizationID int64) (int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(deleteTraegerSessionsQuery, organizationID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("identity access postgres: delete Träger sessions: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows, stats, nil
}

type traegerAccountRow struct {
	AccountID int64    `bun:"account_id"`
	Email     string   `bun:"email"`
	Roles     []string `bun:"roles,array"`
}

const listTraegerAccountsQuery = `SELECT "account".id AS account_id, "account".email,
		COALESCE(array_agg("role".role ORDER BY "role".role) FILTER (WHERE "role".role IS NOT NULL), '{}') AS roles
	FROM auth.accounts AS "account"
	LEFT JOIN auth.organization_account_roles AS "role"
		ON "role".account_id = "account".id AND "role".organization_id = ?
	WHERE "account".id IN (?)
	GROUP BY "account".id, "account".email
	ORDER BY "account".email, "account".id`

// ListTraegerAccounts returns e-mail and Träger roles of the given accounts.
func (s *Store) ListTraegerAccounts(ctx context.Context, organizationID int64, accountIDs []int64) ([]domain.TraegerAccount, domain.OperationStats, error) {
	if len(accountIDs) == 0 {
		return []domain.TraegerAccount{}, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []traegerAccountRow
	started := time.Now()
	err = db.NewRaw(listTraegerAccountsQuery, organizationID, bun.List(accountIDs)).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list Träger accounts: %w", err)
	}
	accounts := make([]domain.TraegerAccount, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, domain.TraegerAccount{AccountID: row.AccountID, Email: row.Email, Roles: row.Roles})
	}
	return accounts, stats, nil
}

const findTraegerSchoolRoleQuery = `SELECT id AS role_id, name, is_system, tenant_id, school_portal FROM auth.roles
	WHERE LOWER(name) = LOWER(?) AND (tenant_id = ? OR tenant_id IS NULL)
	ORDER BY tenant_id IS NULL ASC, id ASC LIMIT 1`

const listTraegerSchoolRolePermissionsQuery = `SELECT DISTINCT "permission".resource || ':' || "permission".action
	FROM auth.role_permissions AS "role_permission"
	JOIN auth.permissions AS "permission" ON "permission".id = "role_permission".permission_id
	WHERE "role_permission".role_id = ?
	ORDER BY 1`

// FindTraegerSchoolRole resolves the role a Träger school session holds at
// the school (ADR 0084 section 5): the school's own role of that name before
// the system role, with its effective permissions in the resource:action
// form the session claims carry. found is false when the school sees no such
// role.
func (s *Store) FindTraegerSchoolRole(ctx context.Context, name string, tenantID int64) (domain.RoleAssignment, []string, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.RoleAssignment{}, nil, false, domain.OperationStats{}, err
	}
	var rows []roleAssignmentRow
	started := time.Now()
	err = db.NewRaw(findTraegerSchoolRoleQuery, name, tenantID).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.RoleAssignment{}, nil, false, stats, fmt.Errorf("identity access postgres: find Träger school role: %w", err)
	}
	if len(rows) == 0 {
		return domain.RoleAssignment{}, nil, false, stats, nil
	}
	row := rows[0]
	role := domain.RoleAssignment{RoleID: row.RoleID, Name: row.Name, IsSystem: row.IsSystem, TenantID: row.TenantID, SchoolPortal: row.SchoolPortal}
	permissions := []string{}
	started = time.Now()
	err = db.NewRaw(listTraegerSchoolRolePermissionsQuery, role.RoleID).Scan(ctx, &permissions)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	stats.Rows += int64(len(permissions))
	if err != nil {
		return domain.RoleAssignment{}, nil, false, stats, fmt.Errorf("identity access postgres: list Träger school role permissions: %w", err)
	}
	return role, permissions, true, stats, nil
}
