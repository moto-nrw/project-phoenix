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
	err = db.NewRaw(`INSERT INTO auth.demo_accesses (email, person_name, school_name, source, contact_opt_in, token_hash, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		access.Email, access.PersonName, access.SchoolName, access.Source, access.ContactOptIn, access.TokenHash, access.ExpiresAt).Scan(ctx, &id)
	if err != nil {
		return 0, fmt.Errorf("identity access postgres: insert demo access: %w", err)
	}
	return id, nil
}

func (s *Store) FindDemoAccessByTokenHash(ctx context.Context, tokenHash string) (domain.DemoAccess, bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.DemoAccess{}, false, err
	}
	var row struct {
		ID        int64     `bun:"id"`
		ExpiresAt time.Time `bun:"expires_at"`
	}
	err = db.NewRaw(`SELECT id, expires_at FROM auth.demo_accesses WHERE token_hash = ?`, tokenHash).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DemoAccess{}, false, nil
	}
	if err != nil {
		return domain.DemoAccess{}, false, fmt.Errorf("identity access postgres: find demo access: %w", err)
	}
	return domain.DemoAccess{ID: row.ID, TokenHash: tokenHash, ExpiresAt: row.ExpiresAt}, true, nil
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
