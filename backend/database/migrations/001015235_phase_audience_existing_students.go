package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	phaseAudienceExistingStudentsVersion     = "1.15.235"
	phaseAudienceExistingStudentsDescription = "Allow the existing_students phase audience (only already-enrolled students may apply) for issue #1663"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     phaseAudienceExistingStudentsVersion,
		Description: phaseAudienceExistingStudentsDescription,
		DependsOn: []string{
			phaseEligibilityColumnsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return phaseAudienceExistingStudentsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return phaseAudienceExistingStudentsDown(ctx, db)
		},
	)
}

func phaseAudienceExistingStudentsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.235: Adding existing_students to the enrollment.phases audience CHECK...")

	_, err := db.NewRaw(`
		ALTER TABLE enrollment.phases
		DROP CONSTRAINT IF EXISTS phases_audience_check;

		ALTER TABLE enrollment.phases
		ADD CONSTRAINT phases_audience_check
		CHECK (audience IN ('open', 'new_students', 'existing_students', 'linked_parents'));

		COMMENT ON COLUMN enrollment.phases.audience IS 'Who may apply to this phase: open (everyone incl. anonymous), new_students (anonymous allowed, children already enrolled at the school are rejected), existing_students (anonymous allowed, every submitted child must already be enrolled at the school), linked_parents (only parent accounts with an active guardian link at the school; hidden from the public listing)';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed widening enrollment.phases audience CHECK: %w", err)
	}

	return nil
}

func phaseAudienceExistingStudentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.235: Removing existing_students from the enrollment.phases audience CHECK...")

	// Fold any existing_students phases back to 'open' so the narrower
	// CHECK below can be re-created without a constraint violation.
	_, err := db.NewRaw(`
		UPDATE enrollment.phases SET audience = 'open' WHERE audience = 'existing_students';

		ALTER TABLE enrollment.phases
		DROP CONSTRAINT IF EXISTS phases_audience_check;

		ALTER TABLE enrollment.phases
		ADD CONSTRAINT phases_audience_check
		CHECK (audience IN ('open', 'new_students', 'linked_parents'));

		COMMENT ON COLUMN enrollment.phases.audience IS 'Who may apply to this phase: open (everyone incl. anonymous), new_students (anonymous allowed, children already enrolled at the school are rejected), linked_parents (only parent accounts with an active guardian link at the school; hidden from the public listing)';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed narrowing enrollment.phases audience CHECK: %w", err)
	}

	return nil
}
