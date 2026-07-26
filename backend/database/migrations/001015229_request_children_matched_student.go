package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	requestChildrenMatchedStudentVersion     = "1.15.229"
	requestChildrenMatchedStudentDescription = "Record the already-enrolled student matched at submission for existing_students phases so approval renews it instead of creating a duplicate (issue #1663)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     requestChildrenMatchedStudentVersion,
		Description: requestChildrenMatchedStudentDescription,
		DependsOn: []string{
			phaseAudienceExistingStudentsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return requestChildrenMatchedStudentUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return requestChildrenMatchedStudentDown(ctx, db)
		},
	)
}

func requestChildrenMatchedStudentUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.229: Adding matched_student_id to enrollment.request_children...")

	// ON DELETE SET NULL mirrors created_student_id: if the matched student
	// is removed before the request is approved, the reference clears and
	// approval falls back to the fresh-create path rather than dangling.
	_, err := db.NewRaw(`
		ALTER TABLE enrollment.request_children
		ADD COLUMN IF NOT EXISTS matched_student_id BIGINT
			REFERENCES users.students(id) ON DELETE SET NULL;

		COMMENT ON COLUMN enrollment.request_children.matched_student_id IS 'For existing_students phases: the already-enrolled student this child was matched to at submission (name+birthday). On approval the decision service renews that student instead of creating a duplicate Person/Student. NULL for open/new_students/linked_parents submissions and when the match was ambiguous.';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding matched_student_id to enrollment.request_children: %w", err)
	}

	// PostgreSQL does not index foreign-key columns automatically. Two hot
	// paths need this one: the matched-student uniqueness guard
	// (HasActiveRequestForMatchedStudent, run per child on every
	// existing-student submission/edit/change-request approval) and the
	// ON DELETE SET NULL scan PostgreSQL performs when a student row is
	// deleted. Partial, mirroring idx_enrollment_request_children_created_student:
	// only existing_students children carry a non-NULL value.
	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_enrollment_request_children_matched_student
		ON enrollment.request_children (matched_student_id)
		WHERE matched_student_id IS NOT NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating matched_student_id index on enrollment.request_children: %w", err)
	}

	return nil
}

func requestChildrenMatchedStudentDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.229: Removing matched_student_id from enrollment.request_children...")

	// DROP COLUMN removes the dependent index implicitly; the explicit DROP
	// keeps the rollback correct if the column was already gone.
	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS enrollment.idx_enrollment_request_children_matched_student;

		ALTER TABLE enrollment.request_children
		DROP COLUMN IF EXISTS matched_student_id;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping matched_student_id from enrollment.request_children: %w", err)
	}

	return nil
}
