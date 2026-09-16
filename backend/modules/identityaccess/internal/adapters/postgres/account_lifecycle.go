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
// the PIN columns of auth.accounts, the school's account listing with role
// names, the direct permission grants, the school mapping deactivation,
// account e-mails and auth.accounts_parents. Every statement runs on the
// connection the caller's context carries.

type pinAccountRow struct {
	ID             int64      `bun:"id"`
	Active         bool       `bun:"active"`
	PINHash        *string    `bun:"pin_hash"`
	PINLockedUntil *time.Time `bun:"pin_locked_until"`
	UpdatedAt      time.Time  `bun:"updated_at"`
}

func (r pinAccountRow) toDomain() domain.PINAccount {
	account := domain.PINAccount{ID: r.ID, Active: r.Active, PINLockedUntil: r.PINLockedUntil, UpdatedAt: r.UpdatedAt}
	if r.PINHash != nil {
		account.PINHash = *r.PINHash
	}
	return account
}

func (s *Store) FindPINAccount(ctx context.Context, id int64, forUpdate bool) (domain.PINAccount, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.PINAccount{}, false, domain.OperationStats{}, err
	}
	var rows []pinAccountRow
	query := db.NewSelect().
		TableExpr(`auth.accounts AS "account"`).
		ColumnExpr(`"account".id, "account".active, "account".pin_hash, "account".pin_locked_until, "account".updated_at`).
		Where(`"account".id = ?`, id)
	if forUpdate {
		query = query.For("UPDATE")
	}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.PINAccount{}, false, stats, fmt.Errorf("identity access postgres: find pin account: %w", err)
	}
	if len(rows) == 0 {
		return domain.PINAccount{}, false, stats, nil
	}
	return rows[0].toDomain(), true, stats, nil
}

// IncrementPINAttempts bumps pin_attempts by one and sets the lock deadline
// when the post-increment count reaches the threshold, in one statement so
// concurrent failures count independently (issue #586).
func (s *Store) IncrementPINAttempts(ctx context.Context, id int64, threshold int, lockedUntil time.Time) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().
		TableExpr(`auth.accounts AS "account"`).
		Set("pin_attempts = pin_attempts + 1").
		Set("pin_locked_until = CASE WHEN pin_attempts + 1 >= ? THEN ? ELSE pin_locked_until END", threshold, lockedUntil).
		Where(`"account".id = ?`, id).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: increment pin attempts: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) ResetPINAttempts(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().
		TableExpr(`auth.accounts AS "account"`).
		Set("pin_attempts = 0").
		Set("pin_locked_until = NULL").
		Where(`"account".id = ?`, id).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: reset pin attempts: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) UpdatePINHash(ctx context.Context, id int64, hash string) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().
		TableExpr(`auth.accounts AS "account"`).
		Set("pin_hash = ?", hash).
		Set("updated_at = NOW()").
		Where(`"account".id = ?`, id).
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: update pin hash: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

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

// DeactivateTenantMapping keeps the row (with deactivated_at) so a later
// re-invitation can reactivate it; the staff calendar feed token dies with
// the membership.
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

// --- parent accounts -------------------------------------------------------

type parentAccountRow struct {
	bun.BaseModel `bun:"table:auth.accounts_parents,alias:account_parent"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id"`
	Email         string    `bun:"email"`
	Username      *string   `bun:"username"`
	Active        bool      `bun:"active"`
	PasswordHash  *string   `bun:"password_hash"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

func (r parentAccountRow) toDomain() domain.ParentAccount {
	account := domain.ParentAccount{
		ID: r.ID, TenantID: r.TenantID, Email: r.Email, Active: r.Active, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if r.Username != nil {
		account.Username = *r.Username
	}
	if r.PasswordHash != nil {
		account.PasswordHash = *r.PasswordHash
	}
	return account
}

const parentAccountColumns = `"account_parent".id, "account_parent".tenant_id, "account_parent".email, "account_parent".username, "account_parent".active, "account_parent".password_hash, "account_parent".created_at, "account_parent".updated_at`

func (s *Store) parentAccountSelect(ctx context.Context, db bun.IDB) *bun.SelectQuery {
	query := db.NewSelect().
		TableExpr(`auth.accounts_parents AS "account_parent"`).
		ColumnExpr(parentAccountColumns)
	if scope := s.scope(ctx); scope.TenantID > 0 {
		query = query.Where(`"account_parent".tenant_id = ?`, scope.TenantID)
	}
	return query
}

func (s *Store) findParentAccount(ctx context.Context, operation string, where func(*bun.SelectQuery) *bun.SelectQuery) (domain.ParentAccount, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.ParentAccount{}, false, domain.OperationStats{}, err
	}
	var rows []parentAccountRow
	started := time.Now()
	err = where(s.parentAccountSelect(ctx, db)).Limit(1).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.ParentAccount{}, false, stats, fmt.Errorf("identity access postgres: %s: %w", operation, err)
	}
	if len(rows) == 0 {
		return domain.ParentAccount{}, false, stats, nil
	}
	return rows[0].toDomain(), true, stats, nil
}

func (s *Store) FindParentAccount(ctx context.Context, id int64) (domain.ParentAccount, bool, domain.OperationStats, error) {
	return s.findParentAccount(ctx, "find parent account", func(query *bun.SelectQuery) *bun.SelectQuery {
		return query.Where(`"account_parent".id = ?`, id)
	})
}

func (s *Store) FindParentAccountByEmail(ctx context.Context, email string) (domain.ParentAccount, bool, domain.OperationStats, error) {
	return s.findParentAccount(ctx, "find parent account by email", func(query *bun.SelectQuery) *bun.SelectQuery {
		return query.Where(`LOWER("account_parent".email) = LOWER(?)`, email)
	})
}

func (s *Store) FindParentAccountByUsername(ctx context.Context, username string) (domain.ParentAccount, bool, domain.OperationStats, error) {
	return s.findParentAccount(ctx, "find parent account by username", func(query *bun.SelectQuery) *bun.SelectQuery {
		return query.Where(`LOWER("account_parent".username) = LOWER(?)`, username)
	})
}

func (s *Store) InsertParentAccount(ctx context.Context, account domain.ParentAccount) (domain.ParentAccount, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.ParentAccount{}, domain.OperationStats{}, err
	}
	now := time.Now()
	row := parentAccountRow{TenantID: account.TenantID, Email: account.Email, Active: account.Active, CreatedAt: now, UpdatedAt: now}
	if account.Username != "" {
		username := account.Username
		row.Username = &username
	}
	if account.PasswordHash != "" {
		hash := account.PasswordHash
		row.PasswordHash = &hash
	}
	started := time.Now()
	_, err = db.NewInsert().Model(&row).ModelTableExpr(`auth.accounts_parents`).Returning("id, created_at, updated_at").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: 1}
	if err != nil {
		return domain.ParentAccount{}, stats, fmt.Errorf("identity access postgres: insert parent account: %w", err)
	}
	return row.toDomain(), stats, nil
}

func (s *Store) UpdateParentAccount(ctx context.Context, account domain.ParentAccount) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	var username, hash *string
	if account.Username != "" {
		value := account.Username
		username = &value
	}
	if account.PasswordHash != "" {
		value := account.PasswordHash
		hash = &value
	}
	query := db.NewUpdate().
		TableExpr(`auth.accounts_parents AS "account_parent"`).
		Set("email = ?", account.Email).
		Set("username = ?", username).
		Set("active = ?", account.Active).
		Set("password_hash = ?", hash).
		Set("updated_at = NOW()").
		Where(`"account_parent".id = ?`, account.ID)
	if scope := s.scope(ctx); scope.TenantID > 0 {
		query = query.Where(`"account_parent".tenant_id = ?`, scope.TenantID)
	}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: update parent account: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows > 0, stats, nil
}

func (s *Store) ListParentAccounts(ctx context.Context, filter domain.ParentAccountFilter) ([]domain.ParentAccount, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	query := s.parentAccountSelect(ctx, db)
	if filter.Email != "" {
		query = query.Where(`LOWER("account_parent".email) = LOWER(?)`, filter.Email)
	}
	if filter.Active != nil {
		query = query.Where(`"account_parent".active = ?`, *filter.Active)
	}
	var rows []parentAccountRow
	started := time.Now()
	err = query.OrderExpr(`"account_parent".id ASC`).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list parent accounts: %w", err)
	}
	accounts := make([]domain.ParentAccount, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, row.toDomain())
	}
	return accounts, stats, nil
}
