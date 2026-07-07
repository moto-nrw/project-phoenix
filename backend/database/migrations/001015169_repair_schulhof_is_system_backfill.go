package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	repairSchulhofIsSystemVersion     = "1.15.169"
	repairSchulhofIsSystemDescription = "Repair the 1.15.168 is_system backfill: flag Schulhof activities provisioned via ToggleSupervision, which carry a staff created_by and were skipped by the created_by IS NULL predicate (issue #923 review)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     repairSchulhofIsSystemVersion,
		Description: repairSchulhofIsSystemDescription,
		DependsOn: []string{
			addIsSystemFlagsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error { return repairSchulhofIsSystemBackfillUp(ctx, db) },
		func(ctx context.Context, db *bun.DB) error { return repairSchulhofIsSystemBackfillDown(ctx, db) },
	)
}

func repairSchulhofIsSystemBackfillUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.169: Repairing is_system backfill for staff-provisioned Schulhof activities...")

	// 1.15.168 flagged activity groups only where created_by IS NULL, but
	// created_by does not cleanly separate system from staff rows for
	// Schulhof: ToggleSupervision calls EnsureInfrastructure(ctx, staffID),
	// so an auto-provisioned 'Schulhof Freispiel' can carry a staff
	// created_by. Provisioning always links the activity to the tenant's
	// 'Schulhof' category (UNIQUE(tenant_id, name)), so flag rows that are
	// either system-created (settings-toggle path, already covered by
	// 1.15.168) or linked to that category — without catching unrelated
	// staff activities that merely share the name.
	//
	// WC needs no repair: its provisioning (wc_service.go and the IoT
	// checkin paths) never sets created_by, so 1.15.168 already flagged
	// every system WC row.
	if _, err := db.NewRaw(`
		UPDATE activities.groups g
		SET is_system = TRUE
		WHERE g.name = 'Schulhof Freispiel'
		  AND g.is_system = FALSE
		  AND (
			g.created_by IS NULL
			OR EXISTS (
				SELECT 1 FROM activities.categories c
				WHERE c.id = g.category_id
				  AND c.tenant_id = g.tenant_id
				  AND c.name = 'Schulhof'
			)
		  );
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed repairing Schulhof system activity flags: %w", err)
	}
	return nil
}

func repairSchulhofIsSystemBackfillDown(ctx context.Context, db *bun.DB) error {
	// Intentionally a no-op: the repaired rows are indistinguishable from
	// rows 1.15.168 flagged, and the 1.15.168 down migration drops the
	// is_system columns entirely.
	fmt.Println("Rolling back migration 1.15.169 (no-op)...")
	return nil
}
