package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	guardianHasAccountCheckVersion     = "1.15.146"
	guardianHasAccountCheckDescription = "Add a CHECK invariant so users.guardian_profiles.has_account always equals (account_id IS NOT NULL); backfill any drifted rows first so authorization checks reading either column can never disagree (#1667)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianHasAccountCheckVersion,
		Description: guardianHasAccountCheckDescription,
		DependsOn:   []string{guardianEmailLowerUniqueIndexVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.146: Enforcing has_account = (account_id IS NOT NULL) on users.guardian_profiles...")
			// has_account and account_id encode the same fact redundantly. The two
			// columns are kept in sync by hand (LinkAccount/UnlinkAccount set both in
			// one UPDATE), but nothing enforced it, so a bad import, a manual fix, or
			// a future code path could leave a drifted row (account_id set,
			// has_account=false, or vice versa). Authorization checks across the
			// codebase read one column or the other, so drift is a real
			// privilege-escalation / data-exposure risk (#1667 review).
			//
			// Step 1: backfill so no existing row violates the invariant. Runs as the
			// migration superuser, which bypasses RLS, so this corrects every tenant's
			// rows in one statement. IS DISTINCT FROM is NULL-safe, but has_account is
			// NOT NULL anyway.
			if _, err := db.NewRaw(`
				UPDATE users.guardian_profiles
				SET has_account = (account_id IS NOT NULL)
				WHERE has_account IS DISTINCT FROM (account_id IS NOT NULL);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed backfilling has_account to match account_id: %w", err)
			}

			// Step 2: add the invariant. After the backfill no row violates it, so the
			// ADD CONSTRAINT validation passes. From here on every write path (staff
			// LinkAccount/UnlinkAccount, parent contact edits, enrollment attach,
			// import) that tries to set the two columns inconsistently fails loudly
			// with a 23514 instead of silently drifting — which is exactly the
			// fail-fast we want for a safety-relevant flag.
			if _, err := db.NewRaw(`
				ALTER TABLE users.guardian_profiles
				ADD CONSTRAINT chk_guardian_has_account_matches_account_id
					CHECK (has_account = (account_id IS NOT NULL));
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding has_account/account_id consistency check: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.146: Dropping has_account/account_id consistency check...")
			if _, err := db.NewRaw(`
				ALTER TABLE users.guardian_profiles
				DROP CONSTRAINT IF EXISTS chk_guardian_has_account_matches_account_id;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping has_account/account_id consistency check: %w", err)
			}
			return nil
		},
	)
}
