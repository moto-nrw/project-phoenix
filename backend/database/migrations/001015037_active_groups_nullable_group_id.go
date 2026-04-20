package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	activeGroupsNullableGroupIDVersion     = "1.15.37"
	activeGroupsNullableGroupIDDescription = "Make active.groups.group_id nullable so spontaneous activity instances can bridge to a live session without a template (WP-B6)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activeGroupsNullableGroupIDVersion,
		Description: activeGroupsNullableGroupIDDescription,
		DependsOn: []string{
			ActiveGroupsVersion, // 1.4.1 — original NOT NULL definition of group_id
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return activeGroupsNullableGroupIDUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return activeGroupsNullableGroupIDDown(ctx, db)
		},
	)
}

func activeGroupsNullableGroupIDUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.37: Dropping NOT NULL on active.groups.group_id...")

	// Dropping NOT NULL on a column in Postgres is an O(1) catalog update —
	// no table rewrite, only an AccessExclusiveLock held briefly. Safe on
	// production-sized active.groups tables.
	//
	// Semantics: a NULL group_id marks a spontaneous activity instance — one
	// that was started ad-hoc and has no parent template in activities.groups.
	// All existing rows keep their non-null group_id; only new code paths
	// (materialization service in WP-B9) ever write NULL.
	_, err := db.NewRaw(`ALTER TABLE active.groups ALTER COLUMN group_id DROP NOT NULL;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping NOT NULL on active.groups.group_id: %w", err)
	}

	return nil
}

func activeGroupsNullableGroupIDDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.37: Restoring NOT NULL on active.groups.group_id...")

	// Re-imposing NOT NULL requires every existing row to have a value. If a
	// future deployment already created spontaneous rows (group_id IS NULL),
	// the caller must decide how to treat them before rolling back: either
	// delete them, backfill a sentinel, or abort the rollback. We refuse to
	// silently drop or mutate data here.
	var nullCount int
	err := db.NewRaw(`SELECT COUNT(*) FROM active.groups WHERE group_id IS NULL`).Scan(ctx, &nullCount)
	if err != nil {
		return fmt.Errorf("failed counting NULL group_id rows: %w", err)
	}
	if nullCount > 0 {
		return fmt.Errorf(
			"cannot restore NOT NULL on active.groups.group_id: %d rows have NULL group_id (spontaneous sessions). Backfill or delete them before rolling back migration 1.15.37",
			nullCount,
		)
	}

	_, err = db.NewRaw(`ALTER TABLE active.groups ALTER COLUMN group_id SET NOT NULL;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed restoring NOT NULL on active.groups.group_id: %w", err)
	}

	return nil
}
