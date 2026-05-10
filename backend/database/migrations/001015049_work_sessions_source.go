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
			workSessionsVersion,     // 1.10.1 — active.work_sessions
			GroupSupervisorsVersion, // 1.4.3 — active.group_supervisors (heuristic backfill reads it)
			addTenantIDVersion,      // 1.14.2 — tenant_id columns on both tables (heuristic scopes by tenant)
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
// Historical rows pre-date the column. They were created by one of two paths:
//
//  1. App / Web check-in handler -> WorkSessionService.CheckIn — no
//     active.group_supervisors row written in the same transaction.
//  2. IoT kiosk activity start -> assignSupervisorNonCritical, which (a)
//     inserts a group_supervisors row and then (b) calls EnsureCheckedIn,
//     which calls CheckIn milliseconds later for the SAME staff member.
//
// The heuristic below labels a row 'nfc' when an active.group_supervisors
// row exists for the same staff in the same tenant with created_at in the
// 2-second window immediately preceding the work_session.check_in_time.
// This catches path (2) reliably because the two inserts happen in the
// same function call, but does not false-positive path (1) because the
// App handler creates no supervisor row. The window is intentionally
// asymmetric (gs.created_at <= ws.check_in_time) — supervisor row is
// always written first.
//
// Caveats (acceptable for a one-shot backfill of pre-existing rows):
//
//   - Other call sites also create active.group_supervisors rows (e.g.
//     the activity-supervisor management flow elsewhere in
//     services/active/session_service.go). If one of those happens to
//     land within 2s of an App check-in for the same staff in the same
//     tenant, this heuristic would mislabel an App row as 'nfc'.
//     Operationally unlikely — those flows are admin-triggered and not
//     co-scheduled with App check-ins — but not zero.
//
//   - Timestamp sources differ: group_supervisors.created_at is set by
//     the Postgres clock (DEFAULT current_timestamp); work_sessions.check_in_time
//     is set by Go's time.Now() in WorkSessionService.CheckIn. The
//     2-second window absorbs typical app-server-to-DB clock skew, but
//     if the app server clock drifts more than ~2s ahead of the DB at
//     the time the original rows were written, NFC pairs miss the
//     window and fall through to 'app'. Sanity-check `SELECT now()` vs
//     the app server clock before running this in prod.
//
//   - Supervisor rows have shorter lifetimes than work_sessions:
//     active.group_supervisors rows are deleted when the activity ends
//     (no soft-delete column). Any historical NFC stamp whose paired
//     supervisor row is already gone at migration time falls through to
//     'app'. Acceptable for a one-shot best-effort backfill — labels are
//     advisory for pre-existing rows, and forward writes set source
//     explicitly.
//
// Any rows the heuristic does not match — and all future inserts that do
// not specify source — fall back to 'app', matching the DB DEFAULT below.
// The DEFAULT is correct going forward because new code paths must pass
// source explicitly (validated in WorkSessionService.CheckIn); the
// DEFAULT only catches the in-flight transaction window between this
// migration running and the new server binary booting.
func workSessionsSourceUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.49: Adding source column to active.work_sessions...")

	// Step 1: add column nullable so the heuristic UPDATE can run before
	// the NOT NULL + DEFAULT lock-in.
	if _, err := db.NewRaw(`
		ALTER TABLE active.work_sessions
		ADD COLUMN IF NOT EXISTS source VARCHAR(10);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding source column: %w", err)
	}

	// Step 2: best-effort NFC backfill for historical rows. Tenant-scoped
	// to avoid cross-tenant matches even though staff_id is global-unique
	// today — defence in depth.
	nfcRes, err := db.NewRaw(`
		UPDATE active.work_sessions ws
		SET source = 'nfc'
		WHERE source IS NULL
		  AND EXISTS (
		    SELECT 1
		      FROM active.group_supervisors gs
		     WHERE gs.staff_id  = ws.staff_id
		       AND gs.tenant_id = ws.tenant_id
		       AND ws.check_in_time
		           BETWEEN gs.created_at
		               AND gs.created_at + INTERVAL '2 seconds'
		  );
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed backfilling source='nfc': %w", err)
	}
	nfcAffected, _ := nfcRes.RowsAffected()
	fmt.Printf("Migration 1.15.49: labelled %d historical row(s) as 'nfc' via group_supervisors timing heuristic\n", nfcAffected)

	// Step 3: everything else is App-originated. Includes legacy rows
	// from the App flow and any odd cases the heuristic missed.
	appRes, err := db.NewRaw(`
		UPDATE active.work_sessions
		   SET source = 'app'
		 WHERE source IS NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed backfilling source='app': %w", err)
	}
	appAffected, _ := appRes.RowsAffected()
	fmt.Printf("Migration 1.15.49: labelled %d historical row(s) as 'app'\n", appAffected)

	// Step 4: lock the column down. DEFAULT 'app' is for the brief window
	// between this migration completing and the new server (which always
	// passes source explicitly) being live; CHECK is the safety net for
	// any future code path that bypasses WorkSessionService.CheckIn.
	if _, err := db.NewRaw(`
		ALTER TABLE active.work_sessions
		  ALTER COLUMN source SET NOT NULL,
		  ALTER COLUMN source SET DEFAULT 'app';
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed locking source column: %w", err)
	}

	// Adding the CHECK constraint as a separate statement because
	// ALTER COLUMN ... ADD CONSTRAINT can't be combined with the NOT NULL
	// flip on every Postgres version we support.
	if _, err := db.NewRaw(`
		ALTER TABLE active.work_sessions
		ADD CONSTRAINT chk_work_sessions_source CHECK (source IN ('app','nfc'));
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding source CHECK constraint: %w", err)
	}

	return nil
}

func workSessionsSourceDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.49: Dropping source column from active.work_sessions...")

	// Drop the constraint first so the down is idempotent even if a
	// partial up left the constraint behind without the column rename.
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
