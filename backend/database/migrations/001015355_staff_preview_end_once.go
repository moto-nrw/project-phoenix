package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffPreviewEndOnceVersion     = "1.15.355"
	staffPreviewEndOnceDescription = "Vorschau-Ende genau einmal pro Vorschau-Instanz (#2893)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: staffPreviewEndOnceVersion, Description: staffPreviewEndOnceDescription,
		DependsOn: []string{additionalSupervisionAuditVersion},
	})
	Migrations.MustRegister(staffPreviewEndOnceUp, staffPreviewEndOnceDown)
}

// staffPreviewEndOnceUp makes "diese Vorschau wurde beendet" a database fact
// instead of a read-then-write check (#2893).
//
// Ending a preview is idempotent: the client may repeat the call, and two
// browser tabs may end the same preview in the same moment. A check-then-insert
// cannot cover the second case — both requests read "not ended yet" before
// either row lands, and the audit trail gets two ends for one preview. The
// partial unique index moves that decision into PostgreSQL, so the INSERT ...
// ON CONFLICT DO NOTHING in the repository is race-free by construction.
//
// The index covers only the end events that carry a preview id; every other
// auth event stays completely unconstrained.
func staffPreviewEndOnceUp(ctx context.Context, db *bun.DB) error {
	// Rows written before this index existed may already contain duplicates
	// (the feature ran with the check-then-insert guard). Keep the first end
	// per preview — it is the one that reflects when the admin actually left.
	if _, err := db.NewRaw(`
		DELETE FROM audit.auth_events AS ae
		USING audit.auth_events AS keep
		WHERE ae.event_type = 'staff_preview_ended'
		  AND keep.event_type = 'staff_preview_ended'
		  AND ae.account_id = keep.account_id
		  AND ae.metadata->>'preview_id' IS NOT NULL
		  AND ae.metadata->>'preview_id' = keep.metadata->>'preview_id'
		  AND (keep.created_at, keep.id) < (ae.created_at, ae.id);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("deduplicate staff preview end events: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_auth_events_staff_preview_end_once
			ON audit.auth_events (account_id, (metadata->>'preview_id'))
			WHERE event_type = 'staff_preview_ended'
			  AND metadata->>'preview_id' IS NOT NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("create staff preview end uniqueness index: %w", err)
	}
	return nil
}

func staffPreviewEndOnceDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		DROP INDEX IF EXISTS audit.idx_auth_events_staff_preview_end_once;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("drop staff preview end uniqueness index: %w", err)
	}
	return nil
}
