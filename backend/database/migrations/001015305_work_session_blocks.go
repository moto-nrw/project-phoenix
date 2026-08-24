package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	workSessionBlocksVersion     = "1.15.305"
	workSessionBlocksDescription = "Allow multiple work session blocks per staff and day (#2402)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     workSessionBlocksVersion,
		Description: workSessionBlocksDescription,
		DependsOn:   []string{reclassifyPlannedSpontaneousInstancesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return workSessionBlocksUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return workSessionBlocksDown(ctx, db)
		},
	)
}

// workSessionBlocksUp allows several closed blocks on the same day while the
// partial unique index permits only one open block for a staff member across
// all days. The latter closes the concurrent-check-in race even at midnight.
func workSessionBlocksUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.305: Allowing multiple work session blocks per day...")

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var duplicateOpenSessions bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM active.work_sessions
				WHERE check_out_time IS NULL
				GROUP BY staff_id
				HAVING count(*) > 1
			);`).Scan(&duplicateOpenSessions); err != nil {
			return fmt.Errorf("failed checking for multiple open work sessions: %w", err)
		}
		if duplicateOpenSessions {
			return fmt.Errorf("cannot create open work-session guard: a staff member has multiple open sessions")
		}

		if _, err := tx.ExecContext(ctx, `
			CREATE UNIQUE INDEX IF NOT EXISTS uq_work_sessions_staff_date_open
			ON active.work_sessions (staff_id)
			WHERE check_out_time IS NULL;
		`); err != nil {
			return fmt.Errorf("failed creating uq_work_sessions_staff_date_open: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE active.work_sessions
			DROP CONSTRAINT IF EXISTS uq_work_sessions_staff_date;
		`); err != nil {
			return fmt.Errorf("failed dropping uq_work_sessions_staff_date: %w", err)
		}

		return nil
	})
}

// workSessionBlocksDown restores the strict one-per-day constraint. This can
// only succeed while no staff member has recorded more than one block on a
// single day; otherwise the transaction rolls back and leaves the open-block
// guard in place.
func workSessionBlocksDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.305: Restoring one work session per day...")

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE active.work_sessions
			ADD CONSTRAINT uq_work_sessions_staff_date UNIQUE (staff_id, date);
		`); err != nil {
			return fmt.Errorf("failed restoring uq_work_sessions_staff_date: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			DROP INDEX IF EXISTS active.uq_work_sessions_staff_date_open;
		`); err != nil {
			return fmt.Errorf("failed dropping uq_work_sessions_staff_date_open: %w", err)
		}

		return nil
	})
}
