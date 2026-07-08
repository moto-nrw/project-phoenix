package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	phaseSchoolClassColumnsVersion     = "1.15.171"
	phaseSchoolClassColumnsDescription = "Add per-phase concrete-class config to enrollment.phases (available_school_classes + require_school_class) for issue #1833"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     phaseSchoolClassColumnsVersion,
		Description: phaseSchoolClassColumnsDescription,
		DependsOn: []string{
			createEnrollmentPhasesVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return phaseSchoolClassColumnsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return phaseSchoolClassColumnsDown(ctx, db)
		},
	)
}

func phaseSchoolClassColumnsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.171: Adding concrete-class config columns to enrollment.phases...")

	_, err := db.NewRaw(`
		ALTER TABLE enrollment.phases
		ADD COLUMN IF NOT EXISTS available_school_classes JSONB NOT NULL DEFAULT '[]'::jsonb,
		ADD COLUMN IF NOT EXISTS require_school_class BOOLEAN NOT NULL DEFAULT false;

		COMMENT ON COLUMN enrollment.phases.available_school_classes IS 'Admin-managed list of concrete school classes offered for this phase (e.g. ["2a","2b","3a"]); parents pick from it for grade >= 2';
		COMMENT ON COLUMN enrollment.phases.require_school_class IS 'When true, a concrete class is mandatory for children in grade >= 2; grade 1 always exempt';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding concrete-class columns to enrollment.phases: %w", err)
	}

	return nil
}

func phaseSchoolClassColumnsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.171: Removing concrete-class config columns from enrollment.phases...")

	_, err := db.NewRaw(`
		ALTER TABLE enrollment.phases
		DROP COLUMN IF EXISTS require_school_class,
		DROP COLUMN IF EXISTS available_school_classes;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping concrete-class columns from enrollment.phases: %w", err)
	}

	return nil
}
