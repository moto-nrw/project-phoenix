package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffBirthdayOptOutVersion     = "1.15.268"
	staffBirthdayOptOutDescription = "Add users.staff.birthday_display_opt_out: per-person opt-out for the birthday display (#1542)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffBirthdayOptOutVersion,
		Description: staffBirthdayOptOutDescription,
		DependsOn: []string{
			enrollmentRestorationsAuditVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return staffBirthdayOptOutUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return staffBirthdayOptOutDown(ctx, db)
		},
	)
}

// staffBirthdayOptOutUp backs the birthday display on the dashboard (#1542).
// A child's birthday is school business, but a colleague's is that person's
// own data: whoever does not want their name on a screen every staff member
// sees must be able to say so, without an admin acting on their behalf. The
// flag is therefore set by the staff member themselves on their profile page
// and read by the dashboard query, never by an export a payroll admin runs.
//
// Default FALSE means "visible" — the school-level switch that decides whether
// staff birthdays appear at all is the tenant setting
// operations.birthday_display_include_staff, which is off by default. Both
// have to be on for a name to show.
func staffBirthdayOptOutUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.268: Adding birthday_display_opt_out to users.staff...")

	if _, err := db.ExecContext(ctx, `
		ALTER TABLE users.staff
			ADD COLUMN IF NOT EXISTS birthday_display_opt_out BOOLEAN NOT NULL DEFAULT FALSE;
	`); err != nil {
		return fmt.Errorf("add users.staff.birthday_display_opt_out: %w", err)
	}

	fmt.Println("Migration 1.15.268: Completed successfully")
	return nil
}

func staffBirthdayOptOutDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.268: Dropping users.staff.birthday_display_opt_out...")

	if _, err := db.ExecContext(ctx, `
		ALTER TABLE users.staff
			DROP COLUMN IF EXISTS birthday_display_opt_out;
	`); err != nil {
		return fmt.Errorf("drop users.staff.birthday_display_opt_out: %w", err)
	}

	return nil
}
