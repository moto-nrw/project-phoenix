package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	requestChildrenMatchedByOfficeVersion     = "1.15.809"
	requestChildrenMatchedByOfficeDescription = "Record that the office, not the submitting family's relationship, chose the enrolled child an approval renews, so the renewal links no guardian (issue #6053)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     requestChildrenMatchedByOfficeVersion,
		Description: requestChildrenMatchedByOfficeDescription,
		DependsOn: []string{
			requestChildrenMatchedStudentVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return requestChildrenMatchedByOfficeUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return requestChildrenMatchedByOfficeDown(ctx, db)
		},
	)
}

func requestChildrenMatchedByOfficeUp(ctx context.Context, db *bun.DB) error {
	// A constant default adds the column without rewriting the table; every
	// existing pin came from a submission and keeps false.
	_, err := db.NewRaw(`
		ALTER TABLE enrollment.request_children
		ADD COLUMN IF NOT EXISTS matched_by_office BOOLEAN NOT NULL DEFAULT FALSE;

		COMMENT ON COLUMN enrollment.request_children.matched_by_office IS 'TRUE when the office chose matched_student_id at approval because the child may already be enrolled (#6053). The submitting family holds no authorized relationship to that child, so neither the approval nor a later sync links their guardians or applies their form data to it.';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding matched_by_office to enrollment.request_children: %w", err)
	}
	return nil
}

func requestChildrenMatchedByOfficeDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE enrollment.request_children
		DROP COLUMN IF EXISTS matched_by_office;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping matched_by_office from enrollment.request_children: %w", err)
	}
	return nil
}
