package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	schoolPortalArrivalExceptionsVersion     = "1.15.366"
	schoolPortalArrivalExceptionsDescription = "class_day:arrival_exception_write for the lehrkraft role and the origin of a class arrival exception (#2970)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schoolPortalArrivalExceptionsVersion,
		Description: schoolPortalArrivalExceptionsDescription,
		DependsOn:   []string{lehrkraftRoleVersion, classArrivalExceptionsVersion},
	})

	Migrations.MustRegister(schoolPortalArrivalExceptionsUp, schoolPortalArrivalExceptionsDown)
}

// schoolPortalArrivalExceptionsUp lets a Lehrkraft enter the class-wide
// arrival day exception of #2962 through "moto schule" (#2970):
//
//   - class_day:arrival_exception_write goes to the lehrkraft system role. Like
//     supervision:own it opens a door without widening a scope: the routes
//     behind it re-check the education.class_teachers assignment per class,
//     and the school's setting operations.school_portal_write_scope (default
//     "none") decides whether the door is unlocked at all. No users:update,
//     no users:read. Admins write the same rows through /api/students and
//     need nothing here.
//   - education.class_arrival_exceptions.origin records which portal entered
//     a row ('ogs' or 'school'), so the OGS dialog can say "eingetragen von
//     der Schule". created_by alone cannot tell the two apart: a Lehrkraft
//     has a users.staff row like everybody else.
func schoolPortalArrivalExceptionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.366: School portal writes class arrival exceptions...")

	if err := grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        "class_day:arrival_exception_write",
		Description: "Andere Ankunftszeit für eine zugewiesene Klasse an einem Tag eintragen (moto schule)",
		Resource:    "class_day",
		Action:      "arrival_exception_write",
	}, "lehrkraft"); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `
		ALTER TABLE education.class_arrival_exceptions
			ADD COLUMN IF NOT EXISTS origin VARCHAR(16) NOT NULL DEFAULT 'ogs';

		ALTER TABLE education.class_arrival_exceptions
			DROP CONSTRAINT IF EXISTS chk_class_arrival_exceptions_origin;
		ALTER TABLE education.class_arrival_exceptions
			ADD CONSTRAINT chk_class_arrival_exceptions_origin CHECK (origin IN ('ogs', 'school'));

		COMMENT ON COLUMN education.class_arrival_exceptions.origin IS
			'Portal that entered the row: ogs (tenant portal) or school (moto schule, #2970)';
	`); err != nil {
		return fmt.Errorf("error adding education.class_arrival_exceptions.origin: %w", err)
	}

	return nil
}

func schoolPortalArrivalExceptionsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.366: Removing class_day:arrival_exception_write and class_arrival_exceptions.origin...")

	if _, err := db.ExecContext(ctx, `
		ALTER TABLE education.class_arrival_exceptions
			DROP CONSTRAINT IF EXISTS chk_class_arrival_exceptions_origin;
		ALTER TABLE education.class_arrival_exceptions
			DROP COLUMN IF EXISTS origin;
	`); err != nil {
		return fmt.Errorf("error dropping education.class_arrival_exceptions.origin: %w", err)
	}

	return dropPermission(ctx, db, "class_day:arrival_exception_write")
}
