package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	guardianProfileAccountFKVersion     = "1.15.46"
	guardianProfileAccountFKDescription = "Repoint users.guardian_profiles.account_id FK from auth.accounts_parents to auth.accounts (parent-enrollment PR 3)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianProfileAccountFKVersion,
		Description: guardianProfileAccountFKDescription,
		DependsOn: []string{
			"1.3.5.1", // users.guardian_profiles
			"1.0.1",   // auth.accounts
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return guardianProfileAccountFKUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return guardianProfileAccountFKDown(ctx, db)
		},
	)
}

func guardianProfileAccountFKUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.46: Repointing users.guardian_profiles.account_id FK to auth.accounts...")

	// Step 1: NULL out any account_id values that point at auth.accounts_parents.
	// The orphaned accounts_parents table is being abandoned by the parent-enrollment
	// flow; only seeder fixtures populated this column. Production tenants have no
	// real data here. Pair with has_account=false so the consistency invariant holds.
	if _, err := db.NewRaw(`
		UPDATE users.guardian_profiles
		SET account_id = NULL,
		    has_account = FALSE
		WHERE account_id IS NOT NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed to clear legacy account_id values: %w", err)
	}

	// Step 2: Drop the legacy single-column FK pointing at auth.accounts_parents.
	if _, err := db.NewRaw(`
		ALTER TABLE users.guardian_profiles
		DROP CONSTRAINT IF EXISTS fk_guardian_profile_account;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed to drop old FK: %w", err)
	}

	// Step 3: Drop the legacy composite (tenant_id, account_id) FK pointing
	// at auth.accounts_parents. The composite was added by the tenant-FK
	// migration sweep (001014007) to enforce tenant isolation, but with
	// auth.accounts as the new target it can't be reproduced — auth.accounts
	// has no tenant_id column (accounts are cross-tenant via
	// auth.account_tenants). Tenant isolation moves to RLS on
	// users.guardian_profiles itself.
	if _, err := db.NewRaw(`
		ALTER TABLE users.guardian_profiles
		DROP CONSTRAINT IF EXISTS fk_guardian_profiles_account_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed to drop composite FK: %w", err)
	}

	// Step 4: Add the new single-column FK pointing at auth.accounts.
	if _, err := db.NewRaw(`
		ALTER TABLE users.guardian_profiles
		ADD CONSTRAINT fk_guardian_profile_account
			FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE SET NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed to add new FK to auth.accounts: %w", err)
	}

	return nil
}

func guardianProfileAccountFKDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.46: Restoring guardian_profiles.account_id FK to auth.accounts_parents...")

	// We cannot recover the legacy account_id values that were NULL'd out by
	// the up migration — those mappings are lost. The down migration only
	// restores the FK target, leaving cleared rows as NULL.

	if _, err := db.NewRaw(`
		ALTER TABLE users.guardian_profiles
		DROP CONSTRAINT IF EXISTS fk_guardian_profile_account;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed to drop new FK: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE users.guardian_profiles
		ADD CONSTRAINT fk_guardian_profile_account
			FOREIGN KEY (account_id) REFERENCES auth.accounts_parents(id) ON DELETE SET NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed to restore FK to auth.accounts_parents: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE users.guardian_profiles
		ADD CONSTRAINT fk_guardian_profiles_account_tenant
			FOREIGN KEY (tenant_id, account_id) REFERENCES auth.accounts_parents(tenant_id, id) ON DELETE SET NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed to restore composite FK to auth.accounts_parents: %w", err)
	}

	return nil
}
