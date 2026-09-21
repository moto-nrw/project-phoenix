package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// These batch reads supply identity facts to the staff membership projection.
// They use the caller's ambient transaction and leave staff/person rows to
// their respective owners.

func lowerTrimmed(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

// ListActiveAccountIDsForTenant narrows accountIDs to the accounts that can
// actually act at tenantID: the account itself is active (the global switch
// account management flips) AND it holds an ACTIVE mapping to that tenant.
func (s *Store) ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, domain.OperationStats, error) {
	if tenantID <= 0 || len(accountIDs) == 0 {
		return []int64{}, domain.OperationStats{}, nil
	}
	var rows []struct {
		AccountID int64 `bun:"account_id"`
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.account_tenants AS "account_tenant"`).
		ColumnExpr(`"account_tenant".account_id AS account_id`).
		Join(`JOIN auth.accounts AS "account" ON "account".id = "account_tenant".account_id AND "account".active = TRUE`).
		Where(`"account_tenant".tenant_id = ?`, tenantID).
		Where(`"account_tenant".status = ?`, "active").
		Where(`"account_tenant".account_id IN (?)`, bun.List(accountIDs)).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	result := make([]int64, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.AccountID)
	}
	return result, stats, nil
}

// FindEffectivePermissionNamesByAccountIDsForTenant resolves the effective
// permission names (resource:action) of every given account at one tenant:
// role-granted UNION directly-granted. Direct denials do not remove role
// grants in this projection. Names stay raw for the caller's wildcard matcher.
func (s *Store) FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, domain.OperationStats, error) {
	result := make(map[int64][]string, len(accountIDs))
	if tenantID <= 0 || len(accountIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	started := time.Now()
	effective := db.NewSelect().
		ColumnExpr(`"account_role".account_id AS account_id`).
		ColumnExpr(`"role_permission".permission_id AS permission_id`).
		TableExpr(`auth.account_roles AS "account_role"`).
		Join(`JOIN auth.role_permissions AS "role_permission" ON "role_permission".role_id = "account_role".role_id`).
		Where(`"account_role".tenant_id = ?`, tenantID).
		Where(`"account_role".account_id IN (?)`, bun.List(accountIDs)).
		UnionAll(
			db.NewSelect().
				ColumnExpr(`"account_permission".account_id AS account_id`).
				ColumnExpr(`"account_permission".permission_id AS permission_id`).
				TableExpr(`auth.account_permissions AS "account_permission"`).
				Where(`"account_permission".granted = ?`, true).
				Where(`"account_permission".tenant_id = ?`, tenantID).
				Where(`"account_permission".account_id IN (?)`, bun.List(accountIDs)),
		)

	var rows []struct {
		AccountID      int64  `bun:"account_id"`
		PermissionName string `bun:"permission_name"`
	}
	// The union is joined as a derived table so the evaluator resolves every
	// table this read touches.
	err = db.NewSelect().
		ColumnExpr(`DISTINCT "effective_permission".account_id AS account_id`).
		ColumnExpr(`("permission".resource || ':' || "permission".action) AS permission_name`).
		TableExpr(`auth.permissions AS "permission"`).
		Join(`JOIN (?) AS "effective_permission" ON "effective_permission".permission_id = "permission".id`, effective).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	for _, row := range rows {
		result[row.AccountID] = append(result[row.AccountID], row.PermissionName)
	}
	return result, stats, nil
}

// CountRoleNameMatchesByAccountIDs counts, per account, how many of its role
// assignments carry one of roleNames (case-insensitively). The count — not a
// boolean — because the staff-by-roles listing historically emitted one row
// per matching assignment, and dropping the duplicates here would silently
// change that wire shape.
//
// Deliberately WITHOUT a tenant predicate on auth.account_roles: the legacy
// join had none either, and row-level security is what scopes it.
func (s *Store) CountRoleNameMatchesByAccountIDs(ctx context.Context, accountIDs []int64, roleNames []string) (map[int64]int, domain.OperationStats, error) {
	result := make(map[int64]int, len(accountIDs))
	if len(accountIDs) == 0 || len(roleNames) == 0 {
		return result, domain.OperationStats{}, nil
	}
	lowered := make([]string, 0, len(roleNames))
	for _, name := range roleNames {
		lowered = append(lowered, lowerTrimmed(name))
	}
	var rows []struct {
		AccountID int64 `bun:"account_id"`
		Matches   int   `bun:"matches"`
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.account_roles AS "account_role"`).
		ColumnExpr(`"account_role".account_id AS account_id`).
		ColumnExpr(`COUNT(*) AS matches`).
		Join(`JOIN auth.roles AS "role" ON "role".id = "account_role".role_id`).
		Where(`LOWER("role".name) IN (?)`, bun.List(lowered)).
		Where(`"account_role".account_id IN (?)`, bun.List(accountIDs)).
		GroupExpr(`"account_role".account_id`).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	for _, row := range rows {
		result[row.AccountID] = row.Matches
	}
	return result, stats, nil
}

// ListAccountIDsWithSystemRoleNames narrows accountIDs to those holding one of
// the platform SYSTEM roles named in roleNames at tenantID. System means
// is_system AND tenant_id IS NULL, so a school's own custom role that happens
// to share the label does not count.
func (s *Store) ListAccountIDsWithSystemRoleNames(ctx context.Context, accountIDs []int64, roleNames []string, tenantID int64) ([]int64, domain.OperationStats, error) {
	if tenantID <= 0 || len(accountIDs) == 0 || len(roleNames) == 0 {
		return []int64{}, domain.OperationStats{}, nil
	}
	lowered := make([]string, 0, len(roleNames))
	for _, name := range roleNames {
		lowered = append(lowered, lowerTrimmed(name))
	}
	var rows []struct {
		AccountID int64 `bun:"account_id"`
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.account_roles AS "account_role"`).
		ColumnExpr(`DISTINCT "account_role".account_id AS account_id`).
		Join(`JOIN auth.roles AS "role" ON "role".id = "account_role".role_id`).
		Where(`"role".is_system = TRUE`).
		Where(`"role".tenant_id IS NULL`).
		Where(`LOWER("role".name) IN (?)`, bun.List(lowered)).
		Where(`"account_role".tenant_id = ?`, tenantID).
		Where(`"account_role".account_id IN (?)`, bun.List(accountIDs)).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	result := make([]int64, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.AccountID)
	}
	return result, stats, nil
}
