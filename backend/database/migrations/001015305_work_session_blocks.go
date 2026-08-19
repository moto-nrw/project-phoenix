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

// workSessionBlocksUp replaces the one-session-per-day constraint with a
// partial unique index that only guards OPEN sessions. A staff member can now
// record several closed work blocks on the same calendar day (Homeoffice
// morning, OGS afternoon), while at most one block per day can be open at a
// time — which also keeps the kiosk's concurrent-scan race detection working:
// two simultaneous check-ins both insert an open row, and the loser still
// hits a unique violation.
//
// Both directions swap the two guards inside ONE transaction: the table must
// never be left without any uniqueness protection, which is exactly what a
// failing second statement would cause if the first had already committed.
func workSessionBlocksUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.305: Allowing multiple work session blocks per day...")

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			CREATE UNIQUE INDEX IF NOT EXISTS uq_work_sessions_staff_date_open
			ON active.work_sessions (staff_id, date)
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
// single day; days with multiple blocks must be merged manually first. When
// that is not the case the ADD CONSTRAINT fails, the transaction rolls back,
// and the partial index stays in place — the table keeps the guard it had.
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
