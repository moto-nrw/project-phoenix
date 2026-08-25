package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	careCancellationNoticeVersion     = "1.15.333"
	careCancellationNoticeDescription = "Mark system-authored parent announcements (care cancellation notice, #2601)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careCancellationNoticeVersion,
		Description: careCancellationNoticeDescription,
		DependsOn: []string{
			parentAnnouncementsVersion,
		},
	})

	Migrations.MustRegister(careCancellationNoticeUp, careCancellationNoticeDown)
}

// A cancelled care block informs families through a regular parent
// announcement, so it inherits feed, read state, e-mail and push without a
// second delivery system. system_kind marks such rows: the team list badges
// them, the parent feed labels them, and the feed shows them even when the
// school keeps the optional news feature off.
func careCancellationNoticeUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.333: Adding system_kind to parent announcements...")

	_, err := db.NewRaw(`
		ALTER TABLE users.parent_announcements
			ADD COLUMN system_kind TEXT,
			ADD CONSTRAINT chk_parent_announcements_system_kind
				CHECK (system_kind IS NULL OR system_kind IN ('care_cancellation'));
		COMMENT ON COLUMN users.parent_announcements.system_kind IS
			'NULL for hand-written announcements; care_cancellation for the automatic notice sent when a care block is cancelled';
		CREATE INDEX idx_parent_announcements_system_kind
			ON users.parent_announcements (tenant_id, system_kind)
			WHERE system_kind IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("add parent announcement system_kind: %w", err)
	}
	return nil
}

func careCancellationNoticeDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.333: Removing system_kind from parent announcements...")

	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS users.idx_parent_announcements_system_kind;
		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_system_kind,
			DROP COLUMN IF EXISTS system_kind;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("remove parent announcement system_kind: %w", err)
	}
	return nil
}
