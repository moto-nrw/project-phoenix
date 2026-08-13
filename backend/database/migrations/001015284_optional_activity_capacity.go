package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	optionalActivityCapacityVersion     = "1.15.284"
	optionalActivityCapacityDescription = "Allow activities and timetable templates without a participant limit (#2236)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     optionalActivityCapacityVersion,
		Description: optionalActivityCapacityDescription,
		DependsOn:   []string{templateMultiOfferingSourceVersion},
	})
	Migrations.MustRegister(optionalActivityCapacityUp, optionalActivityCapacityDown)
}

func optionalActivityCapacityUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.284: Making activity participant limits optional...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE activities.groups
			ALTER COLUMN max_participants DROP NOT NULL;

		UPDATE activities.groups
		SET max_participants = NULL
		WHERE is_template = TRUE
			AND max_participants = 999;

		UPDATE activities.groups AS activity_group
		SET max_participants = NULL
		FROM activities.categories AS category
		WHERE activity_group.tenant_id = category.tenant_id
			AND activity_group.category_id = category.id
			AND activity_group.is_template = FALSE
			AND activity_group.max_participants = 999
			AND LOWER(TRIM(category.name)) = 'spontan';

		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS chk_activities_groups_max_participants,
			ADD CONSTRAINT chk_activities_groups_max_participants
			CHECK (max_participants IS NULL OR max_participants > 0);
	`)
	if err != nil {
		return fmt.Errorf("make activity participant limits optional: %w", err)
	}
	return nil
}

func optionalActivityCapacityDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.284: Restoring required activity participant limits...")
	_, err := db.ExecContext(ctx, `
		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS chk_activities_groups_max_participants;

		UPDATE activities.groups
		SET max_participants = 999
		WHERE max_participants IS NULL;

		ALTER TABLE activities.groups
			ALTER COLUMN max_participants SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("restore required activity participant limits: %w", err)
	}
	return nil
}
