package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	backfillMensaCategoryVersion     = "1.15.260"
	backfillMensaCategoryDescription = "Provision the 'Mensa' activity category for every existing tenant that lacks it (issue #2131)"

	// mensaCategoryName / mensaCategoryDescription / mensaCategoryColor mirror
	// the values the operator provisioning service seeds for new schools. Keep
	// the three in sync so a backfilled tenant and a freshly provisioned one
	// end up with the same row.
	mensaCategoryName        = "Mensa"
	mensaCategoryDescription = "Aktivitäten rund um das Mittagessen"
	mensaCategoryColor       = "#FF9500"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     backfillMensaCategoryVersion,
		Description: backfillMensaCategoryDescription,
		DependsOn:   []string{activityCategoryArchivalVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error { return backfillMensaCategoryUp(ctx, db) },
		func(ctx context.Context, db *bun.DB) error { return backfillMensaCategoryDown(ctx, db) },
	)
}

// backfillMensaCategoryUp inserts one 'Mensa' category per non-deleted school
// that has no category of that name yet. Schools created before the
// multi-tenant split inherited the pre-1.14 global seed (which included
// Mensa), but every tenant provisioned via the operator portal got the
// hard-coded default list in operator_provisioning_service.go, which never
// contained it — so Essenszeiten had no fitting Pflichtkategorie (#2131).
//
// The name match is case-insensitive and ignores archived rows only in the
// sense that an archived 'Mensa' still counts as present: re-adding a second
// row a school deliberately archived would be worse than leaving it archived,
// and the admin can restore it from the UI.
func backfillMensaCategoryUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.260: Backfilling 'Mensa' activity category per tenant...")

	res, err := db.NewRaw(`
		INSERT INTO activities.categories (tenant_id, name, description, color, is_system, created_at, updated_at)
		SELECT s.id, ?, ?, ?, FALSE, NOW(), NOW()
		FROM platform.schools s
		WHERE s.deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM activities.categories c
			WHERE c.tenant_id = s.id
			  AND LOWER(c.name) = LOWER(?)
		  );
	`, mensaCategoryName, mensaCategoryDescription, mensaCategoryColor, mensaCategoryName).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed backfilling Mensa category: %w", err)
	}

	if affected, rowsErr := res.RowsAffected(); rowsErr == nil {
		fmt.Printf("Migration 1.15.260: Added 'Mensa' category to %d tenant(s)\n", affected)
	}

	return nil
}

// backfillMensaCategoryDown removes 'Mensa' categories that nothing references.
// The insert leaves no marker distinguishing a backfilled row from a
// pre-existing one, so a school that already had an unused 'Mensa' loses it on
// rollback — the same tradeoff the 1.6.4 seed rollback makes. Rows referenced
// by an activity group or timetable template are always kept; deleting them
// would violate the FK and destroy real planning data.
func backfillMensaCategoryDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.260: Removing unused 'Mensa' categories...")

	if _, err := db.NewRaw(`
		DELETE FROM activities.categories c
		WHERE LOWER(c.name) = LOWER(?)
		  AND NOT EXISTS (
			SELECT 1 FROM activities.groups g WHERE g.category_id = c.id
		  );
	`, mensaCategoryName).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing Mensa categories: %w", err)
	}

	return nil
}
