package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// Identity-owned reads and writes behind the account lifecycle flows (#3225):
// the school's account listing with role names, the direct permission grants,
// the school mapping deactivation and account e-mails.
// Every statement runs on the connection the caller's context carries.

type tenantAccountRow struct {
	AccountID int64  `bun:"account_id"`
	Email     string `bun:"email"`
	Active    bool   `bun:"active"`
	Status    string `bun:"status"`
	RoleNames string `bun:"role_names"`
}

// ListTenantAccounts returns every mapped account of the school with the
// names of the roles it holds there, aggregated the way the retained account
// listing aggregated them.
func (s *Store) ListTenantAccounts(ctx context.Context, tenantID int64) ([]domain.TenantAccount, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []tenantAccountRow
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.account_tenants AS "at"`).
		ColumnExpr(`"at".account_id`).
		ColumnExpr(`"a".email`).
		ColumnExpr(`"a".active`).
		ColumnExpr(`"at".status`).
		ColumnExpr(`COALESCE(string_agg(DISTINCT "r".name, ', ' ORDER BY "r".name), '') AS role_names`).
		Join(`INNER JOIN auth.accounts AS "a" ON "a".id = "at".account_id`).
		Join(`LEFT JOIN auth.account_roles AS "ar" ON "ar".account_id = "at".account_id AND "ar".tenant_id = ?`, tenantID).
		Join(`LEFT JOIN auth.roles AS "r" ON "r".id = "ar".role_id`).
		Where(`"at".tenant_id = ?`, tenantID).
		GroupExpr(`"at".account_id, "a".email, "a".active, "at".status`).
		OrderExpr(`"at".account_id ASC`).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list tenant accounts: %w", err)
	}
	accounts := make([]domain.TenantAccount, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, domain.TenantAccount{
			AccountID: row.AccountID, Email: row.Email, Active: row.Active, Status: row.Status, RoleNames: splitRoleNames(row.RoleNames),
		})
	}
	return accounts, stats, nil
}

func splitRoleNames(aggregated string) []string {
	if strings.TrimSpace(aggregated) == "" {
		return nil
	}
	parts := strings.Split(aggregated, ",")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		if name := strings.TrimSpace(part); name != "" {
			names = append(names, name)
		}
	}
	return names
}

type permissionGrantRow struct {
	ID           int64 `bun:"id"`
	PermissionID int64 `bun:"permission_id"`
	Granted      bool  `bun:"granted"`
}

func (s *Store) ListAccountPermissionGrants(ctx context.Context, accountID, tenantID int64) ([]domain.PermissionGrant, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []permissionGrantRow
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.account_permissions AS "account_permission"`).
		ColumnExpr(`"account_permission".id, "account_permission".permission_id, "account_permission".granted`).
		Where(`"account_permission".account_id = ?`, accountID).
		Where(`"account_permission".tenant_id = ?`, tenantID).
		OrderExpr(`"account_permission".id ASC`).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list account permission grants: %w", err)
	}
	grants := make([]domain.PermissionGrant, 0, len(rows))
	for _, row := range rows {
		grants = append(grants, domain.PermissionGrant(row))
	}
	return grants, stats, nil
}

func (s *Store) DeleteAccountPermissionGrants(ctx context.Context, accountID, tenantID int64) (int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().
		TableExpr(`auth.account_permissions AS "account_permission"`).
		Where(`"account_permission".account_id = ?`, accountID).
		Where(`"account_permission".tenant_id = ?`, tenantID).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("identity access postgres: delete account permission grants: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows, stats, nil
}

func (s *Store) ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, domain.OperationStats, error) {
	if len(accountIDs) == 0 {
		return map[int64]string{}, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []accountRow
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.accounts AS "account"`).
		ColumnExpr(`"account".id, "account".email`).
		Where(`"account".id IN (?)`, bun.List(accountIDs)).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list account emails: %w", err)
	}
	emails := make(map[int64]string, len(rows))
	for _, row := range rows {
		emails[row.ID] = row.Email
	}
	return emails, stats, nil
}
