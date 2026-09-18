package migrations

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
)

const studentOwnerCutoverVersion = "1.15.397"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentOwnerCutoverVersion,
		Description: "Cut over People, School Membership and Care Plan student storage with previous-image compatibility (#2759)",
		DependsOn: []string{
			studentOwnerBackfillVersion,
			absenceRebookingAuditVersion, // 1.15.396 — preserves ladder order
		},
	})
	Migrations.MustRegister(studentOwnerCutoverUp, studentOwnerCutoverDown)
}

// studentOwnerCutoverUp switches the authority for student data from
// users.students to users.student_profiles, users.student_school_memberships
// and users.student_care_profiles.
//
// The final delta, the per-tenant verification and the schema switch share one
// transaction and one write lock, so no write can land between "the targets
// reproduce the old table" and "the old table is no longer written". The
// dependent foreign keys move onto the People profile in the same transaction;
// only their validation scan runs afterwards, where it no longer blocks writes.
func studentOwnerCutoverUp(ctx context.Context, db *bun.DB) error {
	kind, err := studentOwnerRelationKind(ctx, db)
	if err != nil {
		return err
	}
	// VALIDATE CONSTRAINT runs after the switch commits. A timeout there
	// leaves the view in place and 1.15.397 unrecorded; the next migrate
	// must resume validation instead of refusing on the base-table guard.
	if kind == "v" {
		return ValidateStudentOwnerForeignKeys(ctx, db)
	}
	if err := finalizeStudentOwnerStorage(ctx, db, installStudentOwnerCompatibility); err != nil {
		return err
	}
	return ValidateStudentOwnerForeignKeys(ctx, db)
}

func studentOwnerCutoverDown(context.Context, *bun.DB) error {
	return errors.New("student owner cutover rollback deploys the previous application image against the retained compatibility view; the old storage is not restored here")
}
