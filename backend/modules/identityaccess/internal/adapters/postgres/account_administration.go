package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// accountTenantActive is the only mapping status that grants access; an
// offboarded membership keeps its row with status 'inactive'.
const accountTenantActive = "active"

// The account administration predicates, as compile-time SQL so the
// architecture evaluator resolves the tables they read. They are the
// boundary that keeps a school administrator inside their own school: an
// account row itself carries no tenant.
const (
	accountVisibilityDeniedSQL = `FALSE`
	accountVisibilityTenantSQL = `EXISTS (
		SELECT 1 FROM auth.account_tenants AS "account_tenant"
		WHERE "account_tenant".account_id = "account".id
		  AND "account_tenant".tenant_id = ?
		  AND "account_tenant".status = ?
	)`
	accountVisibilityOrganizationSQL = `EXISTS (
		SELECT 1 FROM auth.account_tenants AS "account_tenant"
		WHERE "account_tenant".account_id = "account".id
		  AND "account_tenant".status = ?
		  AND "account_tenant".tenant_id IN (?)
	)`
)

// scopeAccounts applies the caller's visibility to an account read or write.
// The global visibility leaves the statement untouched; it is generic over
// the query kind because selects and updates carry the same predicate.
func scopeAccounts[Q interface{ Where(string, ...any) Q }](query Q, visibility domain.AccountVisibility) Q {
	switch visibility.Kind {
	case domain.AccountVisibilityDenied:
		return query.Where(accountVisibilityDeniedSQL)
	case domain.AccountVisibilityOrganization:
		return query.Where(accountVisibilityOrganizationSQL, accountTenantActive, bun.List(visibility.SchoolIDs))
	case domain.AccountVisibilityTenant:
		return query.Where(accountVisibilityTenantSQL, visibility.TenantID, accountTenantActive)
	default:
		return query
	}
}

type administeredAccountRow struct {
	ID           int64      `bun:"id"`
	Email        string     `bun:"email"`
	Username     *string    `bun:"username"`
	Active       bool       `bun:"active"`
	PasswordHash *string    `bun:"password_hash"`
	CreatedAt    time.Time  `bun:"created_at"`
	UpdatedAt    time.Time  `bun:"updated_at"`
	LastLogin    *time.Time `bun:"last_login"`
}

func (r administeredAccountRow) toDomain() domain.ManagedAccountRecord {
	record := domain.ManagedAccountRecord{
		ID: r.ID, Email: r.Email, Active: r.Active,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, LastLogin: r.LastLogin,
	}
	if r.Username != nil {
		record.Username = *r.Username
	}
	if r.PasswordHash != nil {
		record.PasswordHash = *r.PasswordHash
	}
	return record
}

const administeredAccountColumns = `"account".id, "account".email, "account".username, "account".active,
	"account".password_hash, "account".created_at, "account".updated_at, "account".last_login`

func (s *Store) accountSelect(ctx context.Context) (*bun.SelectQuery, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	return db.NewSelect().TableExpr(`auth.accounts AS "account"`).ColumnExpr(administeredAccountColumns), nil
}

func (s *Store) FindAccountRecord(ctx context.Context, id int64) (domain.ManagedAccountRecord, bool, domain.OperationStats, error) {
	query, err := s.accountSelect(ctx)
	if err != nil {
		return domain.ManagedAccountRecord{}, false, domain.OperationStats{}, err
	}
	return scanAdministeredAccount(ctx, query.Where(`"account".id = ?`, id).Limit(1), "find account record")
}

func (s *Store) FindManageableAccountRecord(ctx context.Context, id int64, visibility domain.AccountVisibility) (domain.ManagedAccountRecord, bool, domain.OperationStats, error) {
	query, err := s.accountSelect(ctx)
	if err != nil {
		return domain.ManagedAccountRecord{}, false, domain.OperationStats{}, err
	}
	query = scopeAccounts(query.Where(`"account".id = ?`, id), visibility)
	return scanAdministeredAccount(ctx, query.Limit(1), "find manageable account record")
}

func scanAdministeredAccount(ctx context.Context, query *bun.SelectQuery, operation string) (domain.ManagedAccountRecord, bool, domain.OperationStats, error) {
	var rows []administeredAccountRow
	started := time.Now()
	err := query.Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.ManagedAccountRecord{}, false, stats, fmt.Errorf("identity access postgres: %s: %w", operation, err)
	}
	if len(rows) == 0 {
		return domain.ManagedAccountRecord{}, false, stats, nil
	}
	return rows[0].toDomain(), true, stats, nil
}

// ListManageableAccountRecords orders by account id. The retained listing
// left the order to the planner; the endpoint's page is stable now.
func (s *Store) ListManageableAccountRecords(ctx context.Context, filter domain.AccountListFilter, visibility domain.AccountVisibility) ([]domain.ManagedAccountRecord, domain.OperationStats, error) {
	query, err := s.accountSelect(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	query = scopeAccounts(query, visibility)
	if filter.Email != "" {
		query = query.Where(`LOWER("account".email) = LOWER(?)`, filter.Email)
	}
	if filter.Active != nil {
		query = query.Where(`"account".active = ?`, *filter.Active)
	}
	return scanAdministeredAccounts(ctx, query.OrderExpr(`"account".id`), "list manageable account records")
}

// ListManageableAccountRecordsByRole joins the role assignment and keeps it
// inside the caller's visibility as well: an organisation administrator sees
// the role only where it was granted at one of their schools, a school
// administrator only where it was granted at theirs.
func (s *Store) ListManageableAccountRecordsByRole(ctx context.Context, roleName string, visibility domain.AccountVisibility) ([]domain.ManagedAccountRecord, domain.OperationStats, error) {
	query, err := s.accountSelect(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	query = scopeAccounts(query, visibility).
		Join(`JOIN auth.account_roles AS "account_role" ON "account_role".account_id = "account".id`).
		Join(`JOIN auth.roles AS "role" ON "account_role".role_id = "role".id`).
		Where(`LOWER("role".name) = LOWER(?)`, roleName).
		Distinct()
	switch visibility.Kind {
	case domain.AccountVisibilityOrganization:
		query = query.
			Join(`INNER JOIN auth.account_tenants AS "role_account_tenant" ON "role_account_tenant".account_id = "account_role".account_id AND "role_account_tenant".tenant_id = "account_role".tenant_id`).
			Where(`"account_role".tenant_id IN (?)`, bun.List(visibility.SchoolIDs)).
			Where(`"role_account_tenant".status = ?`, accountTenantActive)
	case domain.AccountVisibilityTenant:
		query = query.Where(`"account_role".tenant_id = ?`, visibility.TenantID)
	case domain.AccountVisibilityDenied:
		query = query.Where(accountVisibilityDeniedSQL)
	case domain.AccountVisibilityGlobal:
	}
	return scanAdministeredAccounts(ctx, query.OrderExpr(`"account".id`), "list manageable account records by role")
}

func scanAdministeredAccounts(ctx context.Context, query *bun.SelectQuery, operation string) ([]domain.ManagedAccountRecord, domain.OperationStats, error) {
	var rows []administeredAccountRow
	started := time.Now()
	err := query.Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: %s: %w", operation, err)
	}
	records := make([]domain.ManagedAccountRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, row.toDomain())
	}
	return records, stats, nil
}

func (s *Store) UpdateManageableAccountIdentity(ctx context.Context, update domain.AccountIdentityUpdate, visibility domain.AccountVisibility) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := db.NewUpdate().
		Model((*administeredAccountRow)(nil)).
		ModelTableExpr(`auth.accounts AS "account"`).
		Set(`email = ?`, update.Email).
		Set(`updated_at = NOW()`).
		Where(`"account".id = ?`, update.AccountID)
	if update.Username != nil {
		query = query.Set(`username = ?`, *update.Username)
	}
	return execManageableAccountWrite(ctx, scopeAccounts(query, visibility), "update manageable account identity")
}

func (s *Store) SetManageableAccountActive(ctx context.Context, id int64, active bool, visibility domain.AccountVisibility) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := db.NewUpdate().
		Model((*administeredAccountRow)(nil)).
		ModelTableExpr(`auth.accounts AS "account"`).
		Set(`active = ?`, active).
		Set(`updated_at = NOW()`).
		Where(`"account".id = ?`, id)
	return execManageableAccountWrite(ctx, scopeAccounts(query, visibility), "set manageable account active")
}

func (s *Store) UpdateAccountPasswordHash(ctx context.Context, id int64, hash string) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := db.NewUpdate().
		Model((*administeredAccountRow)(nil)).
		ModelTableExpr(`auth.accounts AS "account"`).
		Set(`password_hash = ?`, hash).
		Set(`updated_at = NOW()`).
		Where(`"account".id = ?`, id)
	return execManageableAccountWrite(ctx, query, "update account password hash")
}

func execManageableAccountWrite(ctx context.Context, query *bun.UpdateQuery, operation string) (bool, domain.OperationStats, error) {
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: %s: %w", operation, err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows > 0, stats, nil
}
