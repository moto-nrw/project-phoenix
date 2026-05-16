package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	activeVisitsUniqueActivePerStudentVersion     = "1.15.47"
	activeVisitsUniqueActivePerStudentDescription = "Add partial UNIQUE(tenant_id, student_id) WHERE exit_time IS NULL on active.visits to block duplicate-active-visit races"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activeVisitsUniqueActivePerStudentVersion,
		Description: activeVisitsUniqueActivePerStudentDescription,
		DependsOn: []string{
			ActiveVisitsVersion, // 1.4.2 — active.visits
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return activeVisitsUniqueActivePerStudentUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return activeVisitsUniqueActivePerStudentDown(ctx, db)
		},
	)
}

func activeVisitsUniqueActivePerStudentUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.47: Closing duplicate active visits then adding partial unique index...")

	// Pre-step: dedupe. CREATE UNIQUE INDEX would fail outright on any DB
	// where the old race left more than one open visit per
	// (tenant_id, student_id) — the very condition this migration exists to
	// make impossible. Pick the row with the latest entry_time as the
	// survivor; close every older duplicate by setting
	// exit_time = entry_time + 1 second so the close timestamp follows the
	// open one but doesn't claim a supervised checkout.
	//
	// This mirrors the cleanup pattern from 1.15.42
	// (uniq_attendance_open_per_student_day): cleanup + index in one tx so
	// the migration framework rolls everything back together if the index
	// build fails.
	res, err := db.NewRaw(`
		WITH ranked AS (
			SELECT id,
			       ROW_NUMBER() OVER (
			           PARTITION BY tenant_id, student_id
			           ORDER BY entry_time DESC, id DESC
			       ) AS rn,
			       entry_time
			FROM active.visits
			WHERE exit_time IS NULL
		)
		UPDATE active.visits v
		SET exit_time = ranked.entry_time + INTERVAL '1 second',
		    updated_at = NOW()
		FROM ranked
		WHERE v.id = ranked.id
		  AND ranked.rn > 1;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed deduping open visits before unique index: %w", err)
	}
	if affected, raErr := res.RowsAffected(); raErr == nil && affected > 0 {
		fmt.Printf("Migration 1.15.47: closed %d duplicate active visit(s) before applying unique index\n", affected)
	}

	// Two concurrent checkin requests (kiosk + web, retry, race after a
	// silent checkout failure) used to both pass the read-then-write
	// idempotency check in services/active.ensureStudentHasNoActiveVisit
	// and INSERT duplicate active visits. The partial unique index closes
	// the race at the database layer; the repository maps the resulting
	// 23505 to ErrStudentAlreadyActive so the IoT handler returns 409
	// regardless of which path detected the conflict first.
	if _, err := db.NewRaw(`
		CREATE UNIQUE INDEX IF NOT EXISTS uniq_active_visits_open_per_student
		ON active.visits (tenant_id, student_id)
		WHERE exit_time IS NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating uniq_active_visits_open_per_student: %w", err)
	}

	return nil
}

func activeVisitsUniqueActivePerStudentDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.47: dropping uniq_active_visits_open_per_student...")

	// Cleanup of duplicate visits is intentionally NOT reversed — the closed
	// rows were always invalid and re-opening them would corrupt active
	// state. Only the index drops on rollback.
	if _, err := db.NewRaw(`DROP INDEX IF EXISTS active.uniq_active_visits_open_per_student;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping uniq_active_visits_open_per_student: %w", err)
	}

	return nil
}
