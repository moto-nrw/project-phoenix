package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addYardSinceToAttendanceVersion     = "1.15.39"
	addYardSinceToAttendanceDescription = "Add yard_since column to active.attendance for binary-mode yard tracking"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addYardSinceToAttendanceVersion,
		Description: addYardSinceToAttendanceDescription,
		DependsOn: []string{
			AttendanceVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addYardSinceToAttendanceUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addYardSinceToAttendanceDown(ctx, db)
		},
	)
}

func addYardSinceToAttendanceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.39: Adding yard_since column to active.attendance...")

	_, err := db.NewRaw(`
		ALTER TABLE active.attendance
		ADD COLUMN IF NOT EXISTS yard_since TIMESTAMPTZ;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding yard_since column to active.attendance: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_attendance_yard_since
		ON active.attendance (student_id)
		WHERE yard_since IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating partial index on yard_since: %w", err)
	}

	return nil
}

func addYardSinceToAttendanceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.39: Removing yard_since column from active.attendance...")

	_, err := db.NewRaw(`DROP INDEX IF EXISTS active.idx_attendance_yard_since;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping index idx_attendance_yard_since: %w", err)
	}

	_, err = db.NewRaw(`ALTER TABLE active.attendance DROP COLUMN IF EXISTS yard_since;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping yard_since column from active.attendance: %w", err)
	}

	return nil
}
