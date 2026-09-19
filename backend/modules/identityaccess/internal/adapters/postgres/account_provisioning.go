package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// registeredAccountRow is the account a registration inserted.
type registeredAccountRow struct {
	ID        int64      `bun:"id"`
	Email     string     `bun:"email"`
	Username  *string    `bun:"username"`
	Active    bool       `bun:"active"`
	CreatedAt time.Time  `bun:"created_at"`
	UpdatedAt time.Time  `bun:"updated_at"`
	LastLogin *time.Time `bun:"last_login"`
}

func (r registeredAccountRow) toDomain() domain.RegisteredAccount {
	account := domain.RegisteredAccount{
		ID: r.ID, Email: r.Email, Active: r.Active,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, LastLogin: r.LastLogin,
	}
	if r.Username != nil {
		account.Username = *r.Username
	}
	return account
}

// InsertSchoolAccount writes the account row of a registration. The username
// is stored as given — the empty string included, because the unique index is
// what the flow's own uniqueness check reads it back through. last_login is
// stamped at creation, as the retained registration did.
func (s *Store) InsertSchoolAccount(ctx context.Context, account domain.NewSchoolAccount) (domain.RegisteredAccount, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.RegisteredAccount{}, domain.OperationStats{}, err
	}
	var rows []registeredAccountRow
	started := time.Now()
	err = db.NewRaw(`INSERT INTO auth.accounts (email, username, password_hash, active, last_login, created_at, updated_at)
		VALUES (?, ?, NULLIF(?, ''), TRUE, NOW(), NOW(), NOW())
		RETURNING id, email, username, active, created_at, updated_at, last_login`,
		account.Email, account.Username, account.PasswordHash).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.RegisteredAccount{}, stats, fmt.Errorf("identity access postgres: insert school account: %w", err)
	}
	if len(rows) == 0 {
		return domain.RegisteredAccount{}, stats, fmt.Errorf("identity access postgres: insert school account: no row returned")
	}
	return rows[0].toDomain(), stats, nil
}

// FindLoginAccountByUsername matches case-insensitively and across every
// school, as the retained repository did.
func (s *Store) FindLoginAccountByUsername(ctx context.Context, username string) (domain.LoginAccount, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.LoginAccount{}, false, domain.OperationStats{}, err
	}
	var rows []loginAccountRow
	started := time.Now()
	err = db.NewSelect().
		TableExpr(`auth.accounts AS "account"`).
		ColumnExpr(loginAccountColumns).
		Where(`LOWER("account".username) = LOWER(?)`, username).
		Limit(1).
		Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.LoginAccount{}, false, stats, fmt.Errorf("identity access postgres: find login account by username: %w", err)
	}
	if len(rows) == 0 {
		return domain.LoginAccount{}, false, stats, nil
	}
	return rows[0].toDomain(), true, stats, nil
}

// InsertTenantMappingIfAbsent leaves an existing mapping exactly as it is.
// DO NOTHING rather than the invitation's DO UPDATE: a registration and a
// link must not resurrect a membership an offboarding deactivated.
func (s *Store) InsertTenantMappingIfAbsent(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error) {
	_, stats, err := s.exec(ctx, "insert tenant mapping", func(db bun.IDB) executor {
		return db.NewRaw(`INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at, created_at, updated_at)
			VALUES (?, ?, 'active', NOW(), NOW(), NOW())
			ON CONFLICT (account_id, tenant_id) DO NOTHING`, accountID, tenantID)
	})
	return stats, err
}
