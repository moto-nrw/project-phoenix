// Package postgres persists the Identity & Access guardian-access facts over
// Bun. Every statement runs on the connection the composition resolves from
// the caller's context, so tenant transactions and row-level security apply
// exactly as they did for the legacy repositories.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// Database resolves the connection of the current request.
type Database func(context.Context) (bun.IDB, error)

// TenantScope names a required school or explicit administrative cross-school
// scope. AdminTransaction records privilege independently of the school ID.
type TenantScope struct {
	TenantID         int64
	AdminTransaction bool
	AcrossSchools    bool
}

// Scope resolves the tenant scope of the current request.
type Scope func(context.Context) TenantScope

type Store struct {
	database Database
	scope    Scope
}

func New(database Database, scope Scope) *Store {
	if database == nil || scope == nil {
		panic("identity access postgres: database runtime and tenant scope are required")
	}
	return &Store{database: database, scope: scope}
}

type accountRow struct {
	ID    int64  `bun:"id"`
	Email string `bun:"email"`
}

func (s *Store) FindAccount(ctx context.Context, id int64) (domain.Account, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Account{}, false, domain.OperationStats{}, err
	}
	var rows []accountRow
	started := time.Now()
	err = db.NewRaw(`SELECT id, email FROM auth.accounts
		WHERE id = ? AND personal_data_erased_at IS NULL`, id).Scan(ctx, &rows)
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
	err = db.NewRaw(`SELECT id, email FROM auth.accounts
		WHERE LOWER(email) = ? AND personal_data_erased_at IS NULL
		ORDER BY id LIMIT 1`, email).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.Account{}, false, stats, fmt.Errorf("identity access postgres: find account by email: %w", err)
	}
	if len(rows) == 0 {
		return domain.Account{}, false, stats, nil
	}
	return domain.Account{ID: rows[0].ID, Email: rows[0].Email}, true, stats, nil
}

// FindAccountsByEmails resolves normalized e-mail addresses in one query.
func (s *Store) FindAccountsByEmails(ctx context.Context, emails []string) (map[string]domain.Account, domain.OperationStats, error) {
	accounts := make(map[string]domain.Account)
	if len(emails) == 0 {
		return accounts, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []accountRow
	started := time.Now()
	err = db.NewRaw(`SELECT id, email FROM auth.accounts
		WHERE LOWER(email) IN (?) AND personal_data_erased_at IS NULL`, bun.List(emails)).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: find accounts by email: %w", err)
	}
	for _, row := range rows {
		accounts[strings.ToLower(row.Email)] = domain.Account{ID: row.ID, Email: row.Email}
	}
	return accounts, stats, nil
}

// LockLiveGuardianAccount takes a shared row lock so guardian-profile linking
// serializes with account retention's exclusive account lock.
func (s *Store) LockLiveGuardianAccount(ctx context.Context, accountID int64) (bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	var accountIDs []int64
	err = db.NewRaw(`SELECT id FROM auth.accounts
		WHERE id = ? AND personal_data_erased_at IS NULL FOR KEY SHARE`, accountID).Scan(ctx, &accountIDs)
	if err != nil {
		return false, fmt.Errorf("identity access postgres: lock live guardian account: %w", err)
	}
	return len(accountIDs) == 1, nil
}

// EnsureActiveTenantMapping is an upsert on the (account, tenant) key: a
// missing mapping is inserted active, an inactive one is reactivated and its
// deactivation cleared.
func (s *Store) EnsureActiveTenantMapping(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	// Account lifecycle commands lock account before mapping and roles. Take
	// that same lock before the upsert, otherwise its mapping lock can cycle
	// with offboarding and the role insert's account foreign-key lock.
	started := time.Now()
	var lockedIDs []int64
	err = db.NewRaw(`SELECT id FROM auth.accounts
		WHERE id = ? AND personal_data_erased_at IS NULL
		FOR UPDATE`, accountID).Scan(ctx, &lockedIDs)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: lock account for guardian grant: %w", err)
	}
	if len(lockedIDs) == 0 {
		return stats, domain.ErrAccountNotFound
	}
	// invited_at is written as an explicit NULL: an approval grants access
	// without an invitation, matching the legacy model that never set it on
	// this path, and the column default would stamp NOW().
	mappingStats, err := s.bindAccountToSchool(ctx, "ensure active tenant mapping", accountID,
		`INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at, invited_at, created_at, updated_at)
			SELECT ?, ?, 'active', NOW(), NULL::timestamptz, NOW(), NOW() FROM caterer WHERE NOT caterer.refused
			ON CONFLICT (account_id, tenant_id) DO UPDATE
			SET status = EXCLUDED.status, activated_at = EXCLUDED.activated_at, deactivated_at = NULL, updated_at = NOW()
			RETURNING 1`, accountID, tenantID)
	stats.Add(mappingStats)
	return stats, err
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

// Validate rejects an absent school unless the caller explicitly selected
// a cross-school administrative unit of work. Zero is never a query scope.
func (s TenantScope) Validate() error {
	if s.AcrossSchools {
		if !s.AdminTransaction {
			return domain.ErrTenantRequired
		}
		return nil
	}
	if s.TenantID <= 0 {
		return domain.ErrTenantRequired
	}
	return nil
}

func scopeTenantQuery[Q interface {
	Where(string, ...any) Q
	Err(error) Q
}](scope TenantScope, query Q, predicate string) Q {
	if err := scope.Validate(); err != nil {
		return query.Err(err)
	}
	if scope.AcrossSchools {
		return query
	}
	return query.Where(predicate, scope.TenantID)
}

// ResolveDatabase validates explicit scope and requires the caller's SQL unit.
func ResolveDatabase(raw any, present bool, scope TenantScope) (bun.IDB, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if !present {
		return nil, errors.New("identity access postgres: transaction is required")
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx, nil
	case *bun.Tx:
		if tx != nil {
			return *tx, nil
		}
	}
	return nil, fmt.Errorf("identity access postgres: unsupported transaction %T", raw)
}

// ScopeFor preserves privilege independently of the validated context school.
// Only an actual administrative transaction can name a cross-school scope.
func ScopeFor(schoolID int64, admin bool) TenantScope {
	return TenantScope{TenantID: schoolID, AdminTransaction: admin, AcrossSchools: admin && schoolID == 0}
}
