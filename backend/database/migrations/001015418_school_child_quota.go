package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.418",
		Description: "Store the Kinderkontingent of every school (#3567)",
		DependsOn:   []string{"1.13.1"},
	})
	Migrations.MustRegister(schoolChildQuotaUp, schoolChildQuotaDown)
}

// schoolChildQuotaUp adds the contracted child quota to platform.schools. The
// Kinderkontingent is child_quota_bundles * child_quota_bundle_size; a school
// without bundles has no limit, so every existing school keeps working after
// the deploy. Only the operator sets it; the tenant role keeps its plain read
// grant on the table.
func schoolChildQuotaUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE platform.schools
			ADD COLUMN child_quota_bundles INTEGER CHECK (child_quota_bundles > 0),
			ADD COLUMN child_quota_bundle_size INTEGER NOT NULL DEFAULT 50 CHECK (child_quota_bundle_size > 0);
		COMMENT ON COLUMN platform.schools.child_quota_bundles IS
			'Booked bundles of the Kinderkontingent (#3567). NULL means the school has no limit.';
		COMMENT ON COLUMN platform.schools.child_quota_bundle_size IS
			'Children per booked bundle (#3567). The Kinderkontingent is bundles times this size.';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("add school child quota: %w", err)
	}
	return nil
}

func schoolChildQuotaDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE platform.schools
			DROP COLUMN IF EXISTS child_quota_bundle_size,
			DROP COLUMN IF EXISTS child_quota_bundles;
	`).Exec(ctx)
	return err
}
