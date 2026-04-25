package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	attendanceUniqueOpenPerDayVersion     = "1.15.41"
	attendanceUniqueOpenPerDayDescription = "Add partial UNIQUE(student_id, date) WHERE check_out_time IS NULL on active.attendance to block double check-in races"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     attendanceUniqueOpenPerDayVersion,
		Description: attendanceUniqueOpenPerDayDescription,
		DependsOn: []string{
			AttendanceVersion, // 1.6.5 — active.attendance
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return attendanceUniqueOpenPerDayUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return attendanceUniqueOpenPerDayDown(ctx, db)
		},
	)
}

func attendanceUniqueOpenPerDayUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.41: Adding partial unique index on active.attendance(student_id, date) WHERE check_out_time IS NULL...")

	// Two concurrent "in" calls (tab double-click, web + kiosk, retry) used to
	// both pass the read-then-write idempotency check and INSERT duplicate
	// open attendance rows. The partial unique index closes the race at the
	// database layer, and performCheckIn now uses ON CONFLICT DO NOTHING so
	// the loser quietly re-fetches the open row instead of raising an error.
	_, err := db.NewRaw(`
		CREATE UNIQUE INDEX IF NOT EXISTS uniq_attendance_open_per_student_day
		ON active.attendance (student_id, date)
		WHERE check_out_time IS NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating uniq_attendance_open_per_student_day: %w", err)
	}

	return nil
}

func attendanceUniqueOpenPerDayDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.41: dropping uniq_attendance_open_per_student_day...")

	_, err := db.NewRaw(`DROP INDEX IF EXISTS active.uniq_attendance_open_per_student_day;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping uniq_attendance_open_per_student_day: %w", err)
	}

	return nil
}
