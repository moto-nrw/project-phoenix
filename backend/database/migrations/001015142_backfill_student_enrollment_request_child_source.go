package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	backfillStudentEnrollmentRequestChildSourceVersion     = "1.15.142"
	backfillStudentEnrollmentRequestChildSourceDescription = "Backfill unambiguous request child provenance on legacy materialized student enrollments."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     backfillStudentEnrollmentRequestChildSourceVersion,
		Description: backfillStudentEnrollmentRequestChildSourceDescription,
		DependsOn:   []string{studentEnrollmentRequestChildSourceVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.142: Backfilling activities.student_enrollments enrollment_request_child_id...")
			return backfillStudentEnrollmentRequestChildSource(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.142: no-op; provenance backfill is not safely reversible.")
			return nil
		},
	)
}
