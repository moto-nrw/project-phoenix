package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	FixGroupSupervisorUniqueConstraintVersion     = "1.7.3"
	FixGroupSupervisorUniqueConstraintDescription = "Fix group supervisor unique constraint to allow re-claiming after ending supervision"
)

func init() {
	MigrationRegistry[FixGroupSupervisorUniqueConstraintVersion] = &Migration{
		Version:     FixGroupSupervisorUniqueConstraintVersion,
		Description: FixGroupSupervisorUniqueConstraintDescription,
		DependsOn:   []string{"1.7.2"},
	}

	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Drop the old unique constraint that prevents re-claiming after ending supervision
		_, err := db.ExecContext(ctx, `
			ALTER TABLE active.group_supervisors
			DROP CONSTRAINT IF EXISTS unique_staff_group_role;
		`)
		if err != nil {
			return err
		}

		// Create a partial unique index that only applies to active supervisions (end_date IS NULL)
		// This allows a staff member to have multiple historical records for the same group/role,
		// but only ONE active supervision at a time
		_, err = db.ExecContext(ctx, `
			CREATE UNIQUE INDEX IF NOT EXISTS unique_active_staff_group_role
			ON active.group_supervisors (staff_id, group_id, role)
			WHERE end_date IS NULL;
		`)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		// Rollback: drop the partial index and recreate the old constraint
		_, err := db.ExecContext(ctx, `
			DROP INDEX IF EXISTS active.unique_active_staff_group_role;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			ALTER TABLE active.group_supervisors
			ADD CONSTRAINT unique_staff_group_role UNIQUE (staff_id, group_id, role);
		`)
		return err
	})
}
