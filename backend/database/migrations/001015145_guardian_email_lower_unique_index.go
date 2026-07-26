package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	guardianEmailLowerUniqueIndexVersion     = "1.15.145"
	guardianEmailLowerUniqueIndexDescription = "Make guardian email uniqueness case-insensitive: rebuild idx_guardian_profiles_tenant_email on (tenant_id, LOWER(email)) so concurrent case-variant edits cannot create duplicate LOWER(email) rows (#1667)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianEmailLowerUniqueIndexVersion,
		Description: guardianEmailLowerUniqueIndexDescription,
		DependsOn:   []string{guardianChangeAuditVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.145: Rebuilding idx_guardian_profiles_tenant_email as a case-insensitive (LOWER(email)) unique index...")
			// The old index was UNIQUE(tenant_id, email) — case-sensitive — so two
			// rows could hold Oma@x.de and oma@x.de in the same tenant even though
			// every guardian email lookup (FindByEmail, invite/account matching)
			// compares case-insensitively via LOWER(email). Rebuilding the SAME
			// index name on (tenant_id, LOWER(email)) closes that race atomically
			// for every write path (staff, parent, enrollment, import) and keeps the
			// name stable so isGuardianEmailUniqueViolation still recognizes the
			// 23505. NULL emails stay non-unique (LOWER(NULL) = NULL), matching the
			// old behavior; no partial WHERE needed. If pre-existing case-variant
			// duplicates exist this CREATE fails loudly inside the migration tx —
			// the correct outcome, since silently keeping ambiguous rows would break
			// case-insensitive matching.
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS users.idx_guardian_profiles_tenant_email;
				CREATE UNIQUE INDEX idx_guardian_profiles_tenant_email
					ON users.guardian_profiles (tenant_id, LOWER(email));
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed rebuilding idx_guardian_profiles_tenant_email as case-insensitive: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.145: Restoring case-sensitive idx_guardian_profiles_tenant_email...")
			if _, err := db.NewRaw(`
				DROP INDEX IF EXISTS users.idx_guardian_profiles_tenant_email;
				CREATE UNIQUE INDEX idx_guardian_profiles_tenant_email
					ON users.guardian_profiles (tenant_id, email);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed restoring case-sensitive idx_guardian_profiles_tenant_email: %w", err)
			}
			return nil
		},
	)
}
