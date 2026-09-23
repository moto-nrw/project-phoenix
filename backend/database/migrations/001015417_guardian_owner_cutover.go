package migrations

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
)

const guardianOwnerCutoverVersion = "1.15.417"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianOwnerCutoverVersion,
		Description: "Cut over People, Care Plan and Identity guardian storage with previous-image compatibility (#2756)",
		DependsOn: []string{
			guardianOwnerBackfillVersion,
			presenceCutoverVersion,
			"1.15.416", // preserves ladder order
		},
		// A relationship the copy rejects (a child with two primaries, a
		// guardian of another school) can never reach the targets: more passes
		// cannot close that gap, only a data correction can. Report it before
		// the deployment stops the application instead of rolling the release
		// back mid-migration.
		Precondition: guardianOwnerCutoverPrecondition,
	})
	Migrations.MustRegister(guardianOwnerCutoverUp, guardianOwnerCutoverDown)
}

// guardianOwnerCutoverUp switches the authority for the student-guardian
// relationship from users.students_guardians to its three owners:
//
//   - users.student_guardian_relationships (People Directory): student,
//     guardian, relationship type, role, primary, emergency contact and
//     priority, payer;
//   - users.student_guardian_pickup_permissions (Care Plan): can_pickup and
//     pickup_notes;
//   - auth.guardian_student_access (Identity & Access): the guardian account
//     binding and the parents-portal permissions.
//
// users.students_guardians stays a base table under its old name. The previous
// image links guardians with INSERT ... ON CONFLICT, which PostgreSQL refuses
// through a trigger-updatable view, so the rollback shape cannot be a view
// here (the same reason as #2762). Instead the old table becomes a
// rollback-only mirror: triggers on the three owner tables keep it equal, and
// a routing trigger sends the previous image's writes into the owners.
//
// The final delta, the per-tenant verification and the compatibility install
// share one transaction and one write lock, so no write can land between "the
// targets reproduce the old table" and "the old table is only a mirror".
// Running it again after the switch only resumes the foreign-key validation.
func guardianOwnerCutoverUp(ctx context.Context, db *bun.DB) error {
	installed, err := guardianCompatibilityInstalled(ctx, db)
	if err != nil {
		return err
	}
	if !installed {
		if err := finalizeGuardianOwnerStorage(ctx, db, installGuardianOwnerCompatibility); err != nil {
			return err
		}
	}
	return ValidateGuardianOwnerForeignKeys(ctx, db)
}

func guardianOwnerCutoverDown(context.Context, *bun.DB) error {
	return errors.New("guardian owner cutover rollback deploys the previous application image against the retained compatibility mirror; the old authority is not restored here")
}
