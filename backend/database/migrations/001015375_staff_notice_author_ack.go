package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffNoticeAuthorAckVersion     = "1.15.375"
	staffNoticeAuthorAckDescription = "Record the author's own acknowledgement for existing staff notices (#2180)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: staffNoticeAuthorAckVersion, Description: staffNoticeAuthorAckDescription,
	})
	Migrations.MustRegister(staffNoticeAuthorAckUp, staffNoticeAuthorAckDown)
}

// staffNoticeAuthorAckUp stamps the author's acknowledgement on every staff
// notice that asks for one.
//
// Whoever writes a notice knows it. Without this row the leader's own
// Tagesinformation asks HER for an acknowledgement and keeps counting in the
// sidebar badge until she confirms what she wrote herself — a task nobody can
// complete because it is not one. New notices get the row from the service;
// this migration does the same for the ones that already exist.
//
// ON CONFLICT DO NOTHING: an author who already confirmed keeps the original
// timestamp. Notices without a required acknowledgement get no row — that
// matches the service, which only stamps when the notice asks.
func staffNoticeAuthorAckUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		INSERT INTO users.staff_notice_acks (tenant_id, notice_id, account_id, acknowledged_at)
		SELECT n.tenant_id, n.id, n.created_by, n.created_at
		FROM users.staff_notices n
		WHERE n.requires_acknowledgement
		ON CONFLICT (notice_id, account_id) DO NOTHING;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("stamp author acknowledgements on staff notices: %w", err)
	}
	return nil
}

// staffNoticeAuthorAckDown is deliberately a no-op: the rows are indis-
// tinguishable from an acknowledgement the author clicked, and deleting those
// would remove real confirmations.
func staffNoticeAuthorAckDown(_ context.Context, _ *bun.DB) error {
	return nil
}
