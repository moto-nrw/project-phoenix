package migrations

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
)

const staffOwnerCutoverVersion = "1.15.409"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffOwnerCutoverVersion,
		Description: "Cut over School Membership and Workforce staff storage with previous-image compatibility (#2753)",
		DependsOn: []string{
			staffOwnerBackfillVersion,
			"1.15.408", // preserves ladder order
		},
		// A staff row naming another school's work-time model can never be
		// copied, so it is reported before the deployment stops the
		// application instead of rolling the release back mid-migration.
		Precondition: staffOwnerCutoverPrecondition,
	})
	Migrations.MustRegister(staffOwnerCutoverUp, staffOwnerCutoverDown)
}

// staffOwnerCutoverUp switches the authority for staff data from users.staff
// to users.staff_school_memberships (School Membership) and
// users.staff_employment_profiles (Workforce).
//
// The final delta, the per-tenant verification and the schema switch share one
// transaction and one write lock, so no write can land between "the targets
// reproduce the old table" and "the old table is no longer written". The
// dependent foreign keys move onto the membership in the same transaction;
// only their validation scan runs afterwards, where it no longer blocks writes.
func staffOwnerCutoverUp(ctx context.Context, db *bun.DB) error {
	kind, err := staffOwnerRelationKind(ctx, db)
	if err != nil {
		return err
	}
	// VALIDATE CONSTRAINT runs after the switch commits. A timeout there
	// leaves the view in place and 1.15.409 unrecorded; the next migrate
	// must resume validation instead of refusing on the base-table guard.
	if kind == "v" {
		return ValidateStaffOwnerForeignKeys(ctx, db)
	}
	if err := finalizeStaffOwnerStorage(ctx, db, installStaffOwnerCompatibility); err != nil {
		return err
	}
	return ValidateStaffOwnerForeignKeys(ctx, db)
}

func staffOwnerCutoverDown(context.Context, *bun.DB) error {
	return errors.New("staff owner cutover rollback deploys the previous application image against the retained compatibility view; the old storage is not restored here")
}
