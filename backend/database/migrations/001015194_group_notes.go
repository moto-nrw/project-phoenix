package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	groupNotesVersion     = "1.15.194"
	groupNotesDescription = "Add durable Wochennotiz (notes) to activities.groups (recurring-template series note, issue #1837 follow-up)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     groupNotesVersion,
		Description: groupNotesDescription,
		DependsOn: []string{
			ActivitiesGroupsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return groupNotesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return groupNotesDown(ctx, db)
		},
	)
}

func groupNotesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.194: Adding notes (Wochennotiz) column to activities.groups...")

	// Durable series note lives ONLY on the template. It is joined onto every
	// materialized instance at read time, so it survives ReplanWeek and the
	// series split without any instance-table column (mirrors required_staff).
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
		ADD COLUMN IF NOT EXISTS notes TEXT;

		COMMENT ON COLUMN activities.groups.notes IS 'Durable Wochennotiz for this recurring template; shown on every materialized instance at read time and preserved across ReplanWeek and series splits. Per-occurrence Tagesnotiz stays on schedule.activity_instances.notes.';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding notes column: %w", err)
	}

	return nil
}

func groupNotesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.194: Removing notes column from activities.groups...")

	_, err := db.NewRaw(`
		ALTER TABLE activities.groups DROP COLUMN IF EXISTS notes;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping notes column: %w", err)
	}

	return nil
}
