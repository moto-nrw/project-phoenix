package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	groupTargetGroupConstraintsVersion     = "1.15.184"
	groupTargetGroupConstraintsDescription = "Enforce valid grade and class values for timetable template target groups"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     groupTargetGroupConstraintsVersion,
		Description: groupTargetGroupConstraintsDescription,
		DependsOn:   []string{groupSeriesRootVersion},
	})
	Migrations.MustRegister(groupTargetGroupConstraintsUp, groupTargetGroupConstraintsDown)
}

func groupTargetGroupConstraintsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.184: Strengthening activities.groups target-group constraints...")
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS chk_activities_groups_target_group_values,
			DROP CONSTRAINT IF EXISTS chk_activities_groups_target_school_class,
			DROP CONSTRAINT IF EXISTS chk_activities_groups_target_grade_level;

		ALTER TABLE activities.groups
			ADD CONSTRAINT chk_activities_groups_target_grade_level
			CHECK (target_grade_level IS NULL OR target_grade_level BETWEEN 1 AND 13),
			ADD CONSTRAINT chk_activities_groups_target_school_class
			CHECK (
				target_school_class IS NULL
					OR BTRIM(
						target_school_class,
						U&'\0009\000A\000B\000C\000D\0020\0085\00A0\1680\2000\2001\2002\2003\2004\2005\2006\2007\2008\2009\200A\2028\2029\202F\205F\3000'
					) <> ''
			),
			ADD CONSTRAINT chk_activities_groups_target_group_values
			CHECK (
				(target_group_type = 'jahrgang'
					AND target_grade_level IS NOT NULL
					AND target_school_class IS NULL)
				OR (target_group_type = 'klasse'
					AND target_grade_level IS NULL
					AND target_school_class IS NOT NULL
					AND BTRIM(
						target_school_class,
						U&'\0009\000A\000B\000C\000D\0020\0085\00A0\1680\2000\2001\2002\2003\2004\2005\2006\2007\2008\2009\200A\2028\2029\202F\205F\3000'
					) <> '')
				OR (target_group_type = 'gruppe'
					AND education_group_id IS NOT NULL
					AND target_grade_level IS NULL
					AND target_school_class IS NULL)
				OR (target_group_type IN ('angebot', 'none')
					AND target_grade_level IS NULL
					AND target_school_class IS NULL)
			);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed strengthening activities.groups target-group constraints: %w", err)
	}
	fmt.Println("Migration 1.15.184: Completed successfully")
	return nil
}

func groupTargetGroupConstraintsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.184: Restoring 1.15.182 target-group constraints...")
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS chk_activities_groups_target_group_values,
			DROP CONSTRAINT IF EXISTS chk_activities_groups_target_school_class,
			DROP CONSTRAINT IF EXISTS chk_activities_groups_target_grade_level;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed restoring 1.15.182 activities.groups target-group constraints: %w", err)
	}
	return nil
}
