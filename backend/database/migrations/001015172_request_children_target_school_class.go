package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	requestChildrenTargetSchoolClassVersion     = "1.15.172"
	requestChildrenTargetSchoolClassDescription = "Add target_school_class to enrollment.request_children (concrete future class, nullable = 'Klasse offen') for issue #1833"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     requestChildrenTargetSchoolClassVersion,
		Description: requestChildrenTargetSchoolClassDescription,
		DependsOn: []string{
			createEnrollmentRequestChildrenVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return requestChildrenTargetSchoolClassUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return requestChildrenTargetSchoolClassDown(ctx, db)
		},
	)
}

func requestChildrenTargetSchoolClassUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.172: Adding target_school_class to enrollment.request_children...")

	_, err := db.NewRaw(`
		ALTER TABLE enrollment.request_children
		ADD COLUMN IF NOT EXISTS target_school_class TEXT;

		COMMENT ON COLUMN enrollment.request_children.target_school_class IS 'Concrete future class chosen at enrollment (e.g. "2a"); NULL means grade-only ("Klasse offen"). On approval it lands verbatim in users.students.school_class.';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding target_school_class to enrollment.request_children: %w", err)
	}

	return nil
}

func requestChildrenTargetSchoolClassDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.172: Removing target_school_class from enrollment.request_children...")

	_, err := db.NewRaw(`
		ALTER TABLE enrollment.request_children
		DROP COLUMN IF EXISTS target_school_class;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping target_school_class from enrollment.request_children: %w", err)
	}

	return nil
}
