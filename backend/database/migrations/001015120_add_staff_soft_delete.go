package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	addStaffSoftDeleteVersion     = "1.15.120"
	addStaffSoftDeleteDescription = "Add deleted_at columns to users.staff and users.teachers for soft delete support"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addStaffSoftDeleteVersion,
		Description: addStaffSoftDeleteDescription,
		DependsOn:   []string{studentsDropBusColumnVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() { _ = tx.Rollback() }()

			// Add deleted_at columns for BUN soft delete
			_, err = tx.ExecContext(ctx, `
				ALTER TABLE users.staff
				ADD COLUMN deleted_at TIMESTAMPTZ;

				ALTER TABLE users.teachers
				ADD COLUMN deleted_at TIMESTAMPTZ;
			`)
			if err != nil {
				return err
			}

			// Replace tenant-scoped unique indexes with partial unique indexes
			// that exclude soft-deleted rows. This allows the same person to be
			// re-added as staff (and a staff member to regain a teacher record)
			// after offboarding.
			_, err = tx.ExecContext(ctx, `
				DROP INDEX IF EXISTS users.idx_staff_tenant_person;
				CREATE UNIQUE INDEX idx_staff_tenant_person
					ON users.staff(tenant_id, person_id)
					WHERE deleted_at IS NULL;

				DROP INDEX IF EXISTS users.idx_teachers_tenant_staff;
				CREATE UNIQUE INDEX idx_teachers_tenant_staff
					ON users.teachers(tenant_id, staff_id)
					WHERE deleted_at IS NULL;
			`)
			if err != nil {
				return err
			}

			return tx.Commit()
		},
		func(ctx context.Context, db *bun.DB) error {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() { _ = tx.Rollback() }()

			// Restore original tenant-scoped unique indexes (without partial filter).
			// Note: fails if live + soft-deleted duplicates exist (same caveat as 1.15.20).
			_, err = tx.ExecContext(ctx, `
				DROP INDEX IF EXISTS users.idx_staff_tenant_person;
				CREATE UNIQUE INDEX idx_staff_tenant_person
					ON users.staff(tenant_id, person_id);

				DROP INDEX IF EXISTS users.idx_teachers_tenant_staff;
				CREATE UNIQUE INDEX idx_teachers_tenant_staff
					ON users.teachers(tenant_id, staff_id);
			`)
			if err != nil {
				return err
			}

			_, err = tx.ExecContext(ctx, `
				ALTER TABLE users.staff
				DROP COLUMN IF EXISTS deleted_at;

				ALTER TABLE users.teachers
				DROP COLUMN IF EXISTS deleted_at;
			`)
			if err != nil {
				return err
			}

			return tx.Commit()
		},
	)
}
