package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// Identity-owned reads behind tenant, parent and school login, refresh,
// switching and session validation (#3251). Every statement runs on the
// connection the caller's context carries: the pre-authentication flows open
// an administrative transaction (auth.account_tenants, auth.account_roles and
// auth.account_permissions are RLS-guarded), and the locking variants pin
// the rows a mint authorizes on until that transaction commits.

type loginAccountRow struct {
	ID           int64     `bun:"id"`
	Email        string    `bun:"email"`
	Username     *string   `bun:"username"`
	PasswordHash *string   `bun:"password_hash"`
	Active       bool      `bun:"active"`
	UpdatedAt    time.Time `bun:"updated_at"`
}

func (r loginAccountRow) toDomain() domain.LoginAccount {
	account := domain.LoginAccount{ID: r.ID, Email: r.Email, Active: r.Active, UpdatedAt: r.UpdatedAt}
	if r.Username != nil {
		account.Username = *r.Username
	}
	if r.PasswordHash != nil {
		account.PasswordHash = *r.PasswordHash
	}
	return account
}

const loginAccountColumns = `"account".id, "account".email, "account".username, "account".password_hash, "account".active, "account".updated_at`

// FindLoginAccountByEmail matches case-insensitively, as the retained
// repository did.
func (s *Store) FindLoginAccountByEmail(ctx context.Context, email string) (domain.LoginAccount, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.LoginAccount{}, false, domain.OperationStats{}, err
	}
	var rows []loginAccountRow
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.accounts AS "account"`).
		ColumnExpr(loginAccountColumns).
		Where(`LOWER("account".email) = LOWER(?)`, email).
		Limit(1).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.LoginAccount{}, false, stats, fmt.Errorf("identity access postgres: find login account by email: %w", err)
	}
	if len(rows) == 0 {
		return domain.LoginAccount{}, false, stats, nil
	}
	return rows[0].toDomain(), true, stats, nil
}

func (s *Store) FindLoginAccount(ctx context.Context, id int64, forUpdate bool) (domain.LoginAccount, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.LoginAccount{}, false, domain.OperationStats{}, err
	}
	var rows []loginAccountRow
	query := db.NewSelect().
		TableExpr(`auth.accounts AS "account"`).
		ColumnExpr(loginAccountColumns).
		Where(`"account".id = ?`, id)
	if forUpdate {
		query = query.For("UPDATE")
	}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.LoginAccount{}, false, stats, fmt.Errorf("identity access postgres: find login account: %w", err)
	}
	if len(rows) == 0 {
		return domain.LoginAccount{}, false, stats, nil
	}
	return rows[0].toDomain(), true, stats, nil
}

// RecordAccountLogin writes the last-login stamp. On the login path this is
// the first write of the mint transaction and takes the account row lock
// concurrent issuers serialize the session cap on.
func (s *Store) RecordAccountLogin(ctx context.Context, id int64, at time.Time) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().
		TableExpr(`auth.accounts AS "account"`).
		Set("last_login = ?", at).
		Where(`"account".id = ?`, id).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: record account login: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

// ListActiveTenantIDs returns the account's active school mappings in
// creation order, which is the order the default tenant resolution walks.
func (s *Store) ListActiveTenantIDs(ctx context.Context, accountID int64) ([]int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.account_tenants AS "account_tenant"`).
		ColumnExpr(`"account_tenant".tenant_id`).
		Where(`"account_tenant".account_id = ?`, accountID).
		Where(`"account_tenant".status = ?`, "active").
		OrderExpr(`"account_tenant".created_at ASC`).
		Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(ids))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list active tenant mappings: %w", err)
	}
	return ids, stats, nil
}

// LockActiveTenantMappingShared reports an active mapping and pins its row
// with FOR SHARE, so a revocation that deactivates it blocks until the
// caller's transaction commits.
func (s *Store) LockActiveTenantMappingShared(ctx context.Context, accountID, tenantID int64) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	var ones []int
	started := time.Now()
	err = db.NewSelect().
		ColumnExpr("1").
		TableExpr("auth.account_tenants").
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).
		Where("status = ?", "active").
		For("SHARE").
		Limit(1).
		Scan(ctx, &ones)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(ones))}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: lock active account mapping: %w", err)
	}
	return len(ones) > 0, stats, nil
}

type roleAssignmentRow struct {
	RoleID   int64  `bun:"role_id"`
	Name     string `bun:"name"`
	IsSystem bool   `bun:"is_system"`
	TenantID *int64 `bun:"tenant_id"`
}

// ListAccountRolesAtTenant returns the roles the account holds at the school.
// A tenant of zero falls back to the caller's tenant scope and, without one,
// to every assignment of the account, matching the retained repository's
// defense-in-depth filter. forShare pins the assignment rows with
// `FOR SHARE OF "account_role"`; Postgres refuses a bare FOR SHARE on the
// joined role side.
func (s *Store) ListAccountRolesAtTenant(ctx context.Context, accountID, tenantID int64, forShare bool) ([]domain.RoleAssignment, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []roleAssignmentRow
	query := db.NewSelect().
		TableExpr(`auth.account_roles AS "account_role"`).
		ColumnExpr(`"role".id AS role_id, "role".name, "role".is_system, "role".tenant_id`).
		Join(`JOIN auth.roles AS "role" ON "role".id = "account_role".role_id`).
		Where(`"account_role".account_id = ?`, accountID)
	query = withAssignmentTenant(s.scope(ctx), query, tenantID)
	if forShare {
		query = query.For(`SHARE OF "account_role"`)
	}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list account roles at tenant: %w", err)
	}
	roles := make([]domain.RoleAssignment, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, domain.RoleAssignment{RoleID: row.RoleID, Name: row.Name, IsSystem: row.IsSystem, TenantID: row.TenantID})
	}
	return roles, stats, nil
}

func withAssignmentTenant(scope TenantScope, query *bun.SelectQuery, tenantID int64) *bun.SelectQuery {
	if tenantID > 0 {
		return query.Where(`"account_role".tenant_id = ?`, tenantID)
	}
	if scope.TenantID > 0 {
		return query.Where(`"account_role".tenant_id = ?`, scope.TenantID)
	}
	return query
}

// ListAccountPermissionsAtTenant returns the distinct effective permission
// names granted directly (granted = true) or through the roles the account
// holds at the school. The union is joined as a derived table so the
// evaluator resolves every table this read touches.
func (s *Store) ListAccountPermissionsAtTenant(ctx context.Context, accountID, tenantID int64) ([]string, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	direct := db.NewSelect().
		ColumnExpr(`"account_permission".permission_id`).
		TableExpr(`auth.account_permissions AS "account_permission"`).
		Where(`"account_permission".account_id = ? AND "account_permission".granted = true`, accountID)
	fromRoles := db.NewSelect().
		ColumnExpr(`"role_permission".permission_id`).
		TableExpr(`auth.role_permissions AS "role_permission"`).
		Join(`JOIN auth.account_roles AS "ar" ON "ar".role_id = "role_permission".role_id`).
		Where(`"ar".account_id = ?`, accountID)
	effectiveTenant := tenantID
	if effectiveTenant <= 0 {
		effectiveTenant = s.scope(ctx).TenantID
	}
	if effectiveTenant > 0 {
		direct = direct.Where(`"account_permission".tenant_id = ?`, effectiveTenant)
		fromRoles = fromRoles.Where(`"ar".tenant_id = ?`, effectiveTenant)
	}
	var names []string
	started := time.Now()
	err = db.NewSelect().
		Distinct().
		TableExpr(`auth.permissions AS "permission"`).
		ColumnExpr(`"permission".resource || ':' || "permission".action`).
		Join(`JOIN (?) AS "aap" ON "aap".permission_id = "permission".id`, direct.UnionAll(fromRoles)).
		Scan(ctx, &names)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(names))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list account permissions at tenant: %w", err)
	}
	return names, stats, nil
}

// LockAccountPermissionSources takes FOR SHARE locks on the direct grants and
// on the role permissions of the roles the account holds at the school, in
// the account -> roles -> permissions order every revocation path walks.
func (s *Store) LockAccountPermissionSources(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	var stats domain.OperationStats
	var lockedDirect []int
	started := time.Now()
	err = db.NewSelect().
		ColumnExpr("1").
		TableExpr("auth.account_permissions AS ap").
		Where("ap.account_id = ? AND ap.tenant_id = ?", accountID, tenantID).
		For("SHARE OF ap").
		Scan(ctx, &lockedDirect)
	stats.Add(domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(lockedDirect))})
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: lock direct account permissions: %w", err)
	}
	var lockedFromRoles []int
	started = time.Now()
	err = db.NewSelect().
		ColumnExpr("1").
		TableExpr("auth.role_permissions AS rp").
		Where(`rp.role_id IN (SELECT ar.role_id FROM auth.account_roles AS ar WHERE ar.account_id = ? AND ar.tenant_id = ?)`, accountID, tenantID).
		For("SHARE OF rp").
		Scan(ctx, &lockedFromRoles)
	stats.Add(domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(lockedFromRoles))})
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: lock role permissions: %w", err)
	}
	return stats, nil
}
