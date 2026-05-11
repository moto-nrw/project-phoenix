package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	workSessionsSourceVersion     = "1.15.49"
	workSessionsSourceDescription = "Add source column to active.work_sessions to distinguish App vs NFC stamps (Issue #1368)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     workSessionsSourceVersion,
		Description: workSessionsSourceDescription,
		DependsOn: []string{
			workSessionsVersion, // 1.10.1 — active.work_sessions
			addTenantIDVersion,  // 1.14.2 — tenant_id columns
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return workSessionsSourceUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return workSessionsSourceDown(ctx, db)
		},
	)
}

// workSessionsSourceUp adds a `source` column that records which channel
// produced a work session: 'app' (POST /api/time-tracking/check-in, used by
// the App and Web flow) or 'nfc' (auto-stamp triggered by a kiosk scan
// flowing through services/active session_service.assignSupervisorNonCritical).
//
// Pre-existing rows are labelled 'unknown' — the channel was never recorded
// for them and we do not guess. The export renders 'unknown' as "—" so the
// audit trail stays honest about what is and isn't known. Going forward,
// WorkSessionService.CheckIn rejects 'unknown' as a write value, so only
// 'app' and 'nfc' ever land on new rows.
//
// The DB DEFAULT is 'app' as a safety net for the brief window between this
// migration completing and the new server (which always passes source
// explicitly) being live. The CHECK constraint is the safety net for any
// future code path that bypasses the service.
func workSessionsSourceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.49: Adding source column to active.work_sessions...")

	if _, err := db.NewRaw(`
		ALTER TABLE active.work_sessions
		ADD COLUMN IF NOT EXISTS source VARCHAR(10);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding source column: %w", err)
	}

	res, err := db.NewRaw(`
		UPDATE active.work_sessions
		   SET source = 'unknown'
		 WHERE source IS NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed backfilling source='unknown': %w", err)
	}
	affected, _ := res.RowsAffected()
	fmt.Printf("Migration 1.15.49: labelled %d historical row(s) as 'unknown'\n", affected)

	if _, err := db.NewRaw(`
		ALTER TABLE active.work_sessions
		  ALTER COLUMN source SET NOT NULL,
		  ALTER COLUMN source SET DEFAULT 'app';
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed locking source column: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE active.work_sessions
		ADD CONSTRAINT chk_work_sessions_source CHECK (source IN ('app','nfc','unknown'));
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding source CHECK constraint: %w", err)
	}

	return nil
}

func workSessionsSourceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.49: Dropping source column from active.work_sessions...")

	// Drop the constraint first so the down is idempotent even if a partial
	// up left the constraint behind without the column.
	if _, err := db.NewRaw(`
		ALTER TABLE active.work_sessions
		DROP CONSTRAINT IF EXISTS chk_work_sessions_source;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping source CHECK constraint: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE active.work_sessions DROP COLUMN IF EXISTS source;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping source column: %w", err)
	}

	return nil
}
