// Package postgres persists the Identity & Access guardian-access facts over
// Bun. Every statement runs on the connection the composition resolves from
// the caller's context, so tenant transactions and row-level security apply
// exactly as they did for the legacy repositories.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// Database resolves the connection of the current request.
type Database func(context.Context) (bun.IDB, error)

type Store struct{ database Database }

func New(database Database) *Store {
	if database == nil {
		panic("identity access postgres: database runtime is required")
	}
	return &Store{database: database}
}

type accountRow struct {
	ID    int64  `bun:"id"`
	Email string `bun:"email"`
}

type accountTenantRow struct {
	bun.BaseModel `bun:"table:auth.account_tenants,alias:account_tenant"`
	AccountID     int64     `bun:"account_id,notnull"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	Status        string    `bun:"status,notnull"`
	ActivatedAt   time.Time `bun:"activated_at"`
	// invited_at is written as an explicit NULL by the insert: an approval
	// grants access without an invitation, matching the legacy model that
	// never set it on this path, and the column default would stamp NOW().
	InvitedAt *time.Time `bun:"invited_at"`
	CreatedAt time.Time  `bun:"created_at"`
	UpdatedAt time.Time  `bun:"updated_at"`
}

func (s *Store) FindAccount(ctx context.Context, id int64) (domain.Account, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Account{}, false, domain.OperationStats{}, err
	}
	var rows []accountRow
	started := time.Now()
	err = db.NewRaw(`SELECT id, email FROM auth.accounts WHERE id = ?`, id).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.Account{}, false, stats, fmt.Errorf("identity access postgres: find account: %w", err)
	}
	if len(rows) == 0 {
		return domain.Account{}, false, stats, nil
	}
	return domain.Account{ID: rows[0].ID, Email: rows[0].Email}, true, stats, nil
}

// FindAccountByEmail matches case-insensitively; the caller passes a
// lower-cased address. E-mails are unique platform-wide.
func (s *Store) FindAccountByEmail(ctx context.Context, email string) (domain.Account, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Account{}, false, domain.OperationStats{}, err
	}
	var rows []accountRow
	started := time.Now()
	err = db.NewRaw(`SELECT id, email FROM auth.accounts WHERE LOWER(email) = ? ORDER BY id LIMIT 1`, email).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.Account{}, false, stats, fmt.Errorf("identity access postgres: find account by email: %w", err)
	}
	if len(rows) == 0 {
		return domain.Account{}, false, stats, nil
	}
	return domain.Account{ID: rows[0].ID, Email: rows[0].Email}, true, stats, nil
}

// EnsureActiveTenantMapping is an upsert on the (account, tenant) key: a
// missing mapping is inserted active, an inactive one is reactivated and its
// deactivation cleared.
func (s *Store) EnsureActiveTenantMapping(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	now := time.Now()
	row := accountTenantRow{AccountID: accountID, TenantID: tenantID, Status: "active", ActivatedAt: now, CreatedAt: now, UpdatedAt: now}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).ModelTableExpr(`auth.account_tenants`).
		Value("invited_at", "NULL").
		On("CONFLICT (account_id, tenant_id) DO UPDATE").
		Set("status = EXCLUDED.status").
		Set("activated_at = EXCLUDED.activated_at").
		Set("deactivated_at = NULL").
		Set("updated_at = NOW()").
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: ensure active tenant mapping: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

// FindRoleByName prefers the tenant's own role over the system role of the
// same name, matching the legacy role lookup.
func (s *Store) FindRoleByName(ctx context.Context, name string, tenantID int64) (int64, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, false, domain.OperationStats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewRaw(`SELECT id FROM auth.roles
		WHERE LOWER(name) = LOWER(?) AND (tenant_id = ? OR tenant_id IS NULL)
		ORDER BY tenant_id IS NULL ASC, id ASC LIMIT 1`, name, tenantID).Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, false, stats, fmt.Errorf("identity access postgres: find role by name: %w", err)
	}
	if len(ids) == 0 {
		return 0, false, stats, nil
	}
	return ids[0], true, stats, nil
}

type accountRoleRow struct {
	bun.BaseModel `bun:"table:auth.account_roles,alias:account_role"`
	AccountID     int64     `bun:"account_id,notnull"`
	RoleID        int64     `bun:"role_id,notnull"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

// AssignAccountRole is an insert-if-absent on the unique
// (account_id, role_id, tenant_id) index; the reported flag is the driver's
// affected-row count, zero when the assignment already existed.
func (s *Store) AssignAccountRole(ctx context.Context, accountID, roleID, tenantID int64) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	now := time.Now()
	row := accountRoleRow{AccountID: accountID, RoleID: roleID, TenantID: tenantID, CreatedAt: now, UpdatedAt: now}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).ModelTableExpr(`auth.account_roles`).
		On("CONFLICT (account_id, role_id, tenant_id) DO NOTHING").
		Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: assign account role: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows > 0, stats, nil
}
