package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	phaseEligibleGradeLevelsVersion     = "1.15.230"
	phaseEligibleGradeLevelsDescription = "Add enrollment.phases.eligible_grade_levels so a phase can be restricted to whole grades for issue #1663"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     phaseEligibleGradeLevelsVersion,
		Description: phaseEligibleGradeLevelsDescription,
		DependsOn: []string{
			phaseEligibilityColumnsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return phaseEligibleGradeLevelsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return phaseEligibleGradeLevelsDown(ctx, db)
		},
	)
}

func phaseEligibleGradeLevelsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.230: Adding eligible_grade_levels to enrollment.phases...")

	_, err := db.NewRaw(`
		ALTER TABLE enrollment.phases
		ADD COLUMN IF NOT EXISTS eligible_grade_levels JSONB NOT NULL DEFAULT '[]'::jsonb;

		COMMENT ON COLUMN enrollment.phases.eligible_grade_levels IS 'When non-empty, every child in a submission must declare a grade level contained in this list (e.g. [3]); empty means no grade restriction. Independent of eligible_school_classes: a phase for a whole grade needs no concrete classes.';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding eligible_grade_levels to enrollment.phases: %w", err)
	}

	return nil
}

func phaseEligibleGradeLevelsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.230: Removing eligible_grade_levels from enrollment.phases...")

	_, err := db.NewRaw(`
		ALTER TABLE enrollment.phases
		DROP COLUMN IF EXISTS eligible_grade_levels;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping eligible_grade_levels from enrollment.phases: %w", err)
	}

	return nil
}
