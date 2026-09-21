package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Identity-owned rows behind the demo access of the public demo (#3462). The
// table carries no tenant and is reachable for the administrative role only,
// so every statement here runs inside an administrative transaction.

func (s *Store) InsertDemoAccess(ctx context.Context, access domain.DemoAccess) (int64, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	var id int64
	err = db.NewRaw(`INSERT INTO auth.demo_accesses (email, person_name, school_name, source, contact_opt_in, token_hash, expires_at, school_slug)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		access.Email, access.PersonName, access.SchoolName, access.Source, access.ContactOptIn, access.TokenHash, access.ExpiresAt, access.SchoolSlug).Scan(ctx, &id)
	if err != nil {
		return 0, fmt.Errorf("identity access postgres: insert demo access: %w", err)
	}
	return id, nil
}

// FindActiveDemoAccessByEmail returns the newest unexpired access of the
// address. The advisory lock makes two requests of one address take turns,
// so the second one finds the access the first one stored.
func (s *Store) FindActiveDemoAccessByEmail(ctx context.Context, email string, now time.Time) (domain.DemoAccess, bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.DemoAccess{}, false, err
	}
	if _, err := db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended('auth.demo_accesses:' || ?, 0))`, email).Exec(ctx); err != nil {
		return domain.DemoAccess{}, false, fmt.Errorf("identity access postgres: lock demo access address: %w", err)
	}
	var row struct {
		ID           int64     `bun:"id"`
		PersonName   string    `bun:"person_name"`
		SchoolName   string    `bun:"school_name"`
		Source       string    `bun:"source"`
		ContactOptIn bool      `bun:"contact_opt_in"`
		CreatedAt    time.Time `bun:"created_at"`
		SchoolSlug   string    `bun:"school_slug"`
	}
	err = db.NewRaw(`SELECT id, person_name, school_name, source, contact_opt_in, created_at, school_slug FROM auth.demo_accesses
		WHERE email = ? AND expires_at > ? ORDER BY id DESC LIMIT 1`, email, now).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DemoAccess{}, false, nil
	}
	if err != nil {
		return domain.DemoAccess{}, false, fmt.Errorf("identity access postgres: find demo access by address: %w", err)
	}
	return domain.DemoAccess{
		ID: row.ID, Email: email, PersonName: row.PersonName, SchoolName: row.SchoolName,
		Source: row.Source, ContactOptIn: row.ContactOptIn, CreatedAt: row.CreatedAt, SchoolSlug: row.SchoolSlug,
	}, true, nil
}

func (s *Store) FindDemoAccessByTokenHash(ctx context.Context, tokenHash string) (domain.DemoAccess, bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.DemoAccess{}, false, err
	}
	var row struct {
		ID         int64     `bun:"id"`
		ExpiresAt  time.Time `bun:"expires_at"`
		SchoolSlug string    `bun:"school_slug"`
		SchoolName string    `bun:"school_name"`
		Source     string    `bun:"source"`
	}
	err = db.NewRaw(`SELECT id, expires_at, school_slug, school_name, source FROM auth.demo_accesses WHERE token_hash = ?`, tokenHash).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DemoAccess{}, false, nil
	}
	if err != nil {
		return domain.DemoAccess{}, false, fmt.Errorf("identity access postgres: find demo access: %w", err)
	}
	return domain.DemoAccess{ID: row.ID, TokenHash: tokenHash, ExpiresAt: row.ExpiresAt, SchoolSlug: row.SchoolSlug, SchoolName: row.SchoolName, Source: row.Source}, true, nil
}

// RecordDemoAccessUse notes one redemption and the account it signed in.
func (s *Store) RecordDemoAccessUse(ctx context.Context, id, accountID int64, usedAt time.Time) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`UPDATE auth.demo_accesses SET use_count = use_count + 1, last_used_at = ?, account_id = ? WHERE id = ?`,
		usedAt, accountID, id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("identity access postgres: record demo access use: %w", err)
	}
	return nil
}

// ReplaceDemoAccountRole swaps the system staff roles of the visitor's
// account in its demo school for role. School-defined roles and the guardian
// role stay as they are.
func (s *Store) ReplaceDemoAccountRole(ctx context.Context, accountID, tenantID int64, role string) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`DELETE FROM auth.account_roles
		WHERE account_id = ? AND tenant_id = ? AND role_id IN (
			SELECT id FROM auth.roles WHERE tenant_id IS NULL AND name IN ('admin', 'user', 'guest', 'lehrkraft') AND name <> ?)`,
		accountID, tenantID, role).Exec(ctx)
	if err != nil {
		return fmt.Errorf("identity access postgres: drop demo account role: %w", err)
	}
	_, err = db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		SELECT ?, id, ? FROM auth.roles WHERE tenant_id IS NULL AND name = ?
		ON CONFLICT (account_id, role_id, tenant_id) DO NOTHING`,
		accountID, tenantID, role).Exec(ctx)
	if err != nil {
		return fmt.Errorf("identity access postgres: grant demo account role: %w", err)
	}
	return nil
}

// FindSchoolAdministrator returns the oldest active administrator of the
// school. A school the demo process has not provisioned yet has none.
func (s *Store) FindSchoolAdministrator(ctx context.Context, tenantID int64) (int64, bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, false, err
	}
	var accountID int64
	err = db.NewRaw(`SELECT account.id
		FROM auth.account_tenants AS account_tenant
		JOIN auth.accounts AS account ON account.id = account_tenant.account_id AND account.active
		JOIN auth.account_roles AS account_role ON account_role.account_id = account.id AND account_role.tenant_id = account_tenant.tenant_id
		JOIN auth.roles AS role ON role.id = account_role.role_id AND role.name = 'admin'
		WHERE account_tenant.tenant_id = ? AND account_tenant.status = 'active'
		ORDER BY account.id LIMIT 1`, tenantID).Scan(ctx, &accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("identity access postgres: find school administrator: %w", err)
	}
	return accountID, true, nil
}

// DemoAccountExists reports whether a demo access ever signed the account in.
func (s *Store) DemoAccountExists(ctx context.Context, accountID int64) (bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	var exists bool
	err = db.NewRaw(`SELECT EXISTS (SELECT 1 FROM auth.demo_accesses WHERE account_id = ?)`, accountID).Scan(ctx, &exists)
	if err != nil {
		return false, fmt.Errorf("identity access postgres: check demo account: %w", err)
	}
	return exists, nil
}
