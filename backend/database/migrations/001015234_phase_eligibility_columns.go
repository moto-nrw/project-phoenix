package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	phaseEligibilityColumnsVersion     = "1.15.234"
	phaseEligibilityColumnsDescription = "Add per-phase eligibility config to enrollment.phases (audience + eligible_school_classes) for issue #1663"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     phaseEligibilityColumnsVersion,
		Description: phaseEligibilityColumnsDescription,
		DependsOn: []string{
			phaseSchoolClassColumnsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return phaseEligibilityColumnsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return phaseEligibilityColumnsDown(ctx, db)
		},
	)
}

func phaseEligibilityColumnsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.234: Adding eligibility config columns to enrollment.phases...")

	_, err := db.NewRaw(`
		ALTER TABLE enrollment.phases
		ADD COLUMN IF NOT EXISTS audience TEXT NOT NULL DEFAULT 'open',
		ADD COLUMN IF NOT EXISTS eligible_school_classes JSONB NOT NULL DEFAULT '[]'::jsonb;

		ALTER TABLE enrollment.phases
		DROP CONSTRAINT IF EXISTS phases_audience_check;

		ALTER TABLE enrollment.phases
		ADD CONSTRAINT phases_audience_check
		CHECK (audience IN ('open', 'new_students', 'existing_students', 'linked_parents'));

		COMMENT ON COLUMN enrollment.phases.audience IS 'Who may apply to this phase: open (everyone incl. anonymous), new_students (anonymous allowed, children already enrolled at the school are rejected), existing_students (anonymous allowed, every submitted child must already be enrolled at the school), linked_parents (only parent accounts with an active guardian link at the school; hidden from the public listing)';
		COMMENT ON COLUMN enrollment.phases.eligible_school_classes IS 'When non-empty, every child in a submission must declare a school class contained in this list (e.g. ["1a","2b"]); empty means no class restriction';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding eligibility columns to enrollment.phases: %w", err)
	}

	return nil
}

func phaseEligibilityColumnsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.234: Removing eligibility config columns from enrollment.phases...")

	_, err := db.NewRaw(`
		ALTER TABLE enrollment.phases
		DROP CONSTRAINT IF EXISTS phases_audience_check;

		ALTER TABLE enrollment.phases
		DROP COLUMN IF EXISTS eligible_school_classes,
		DROP COLUMN IF EXISTS audience;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping eligibility columns from enrollment.phases: %w", err)
	}

	return nil
}
