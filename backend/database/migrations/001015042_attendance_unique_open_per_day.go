package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	// Renumbered to 1.15.42 to avoid colliding with the deleted nullable
	// device_id migration that briefly held 1.15.41 on this branch — dev
	// DBs already have "001015041" in bun_migrations from that earlier
	// version, so the new partial-index migration must be a fresh number.
	attendanceUniqueOpenPerDayVersion     = "1.15.42"
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
	fmt.Println("Migration 1.15.42: Closing duplicate open attendance rows then adding partial unique index...")

	// Pre-step: dedupe. CREATE UNIQUE INDEX would fail outright on any DB
	// where the old race left more than one open row per (student_id, date)
	// — the very condition this migration exists to make impossible. Pick
	// the row with the latest check_in_time as the survivor; close every
	// older duplicate by setting check_out_time = check_in_time + 1 second
	// so the close timestamp follows the open one but doesn't claim a
	// supervised checkout (checked_out_by stays NULL — clearly a system
	// closure rather than a real person's action).
	//
	// Why this query touches only check_out_time: the yard_since column is
	// added by a later migration (1.15.43 after the rename), so referencing
	// it here would crash on a fresh-DB run that applies migrations in
	// version order. The IsOnYard invariant
	// (yard_since != NULL && check_out_time == NULL) already evaluates to
	// false once check_out_time is set, so leaving any stale yard_since
	// timestamp on these dedup-closed rows is functionally harmless.
	res, err := db.NewRaw(`
		WITH ranked AS (
			SELECT id,
			       ROW_NUMBER() OVER (
			           PARTITION BY student_id, date
			           ORDER BY check_in_time DESC, id DESC
			       ) AS rn,
			       check_in_time
			FROM active.attendance
			WHERE check_out_time IS NULL
		)
		UPDATE active.attendance a
		SET check_out_time = ranked.check_in_time + INTERVAL '1 second'
		FROM ranked
		WHERE a.id = ranked.id
		  AND ranked.rn > 1;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed deduping open attendance rows before unique index: %w", err)
	}
	if affected, raErr := res.RowsAffected(); raErr == nil && affected > 0 {
		fmt.Printf("Migration 1.15.42: closed %d duplicate open attendance row(s) before applying unique index\n", affected)
	}

	// Two concurrent "in" calls (tab double-click, web + kiosk, retry) used to
	// both pass the read-then-write idempotency check and INSERT duplicate
	// open attendance rows. The partial unique index closes the race at the
	// database layer, and performCheckIn now uses ON CONFLICT DO NOTHING so
	// the loser quietly re-fetches the open row instead of raising an error.
	if _, err := db.NewRaw(`
		CREATE UNIQUE INDEX IF NOT EXISTS uniq_attendance_open_per_student_day
		ON active.attendance (student_id, date)
		WHERE check_out_time IS NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating uniq_attendance_open_per_student_day: %w", err)
	}

	return nil
}

func attendanceUniqueOpenPerDayDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.42: dropping uniq_attendance_open_per_student_day...")

	_, err := db.NewRaw(`DROP INDEX IF EXISTS active.uniq_attendance_open_per_student_day;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping uniq_attendance_open_per_student_day: %w", err)
	}

	return nil
}
