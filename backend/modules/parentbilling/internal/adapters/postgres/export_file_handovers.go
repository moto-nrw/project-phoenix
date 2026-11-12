package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The handovers of protocol entries to the recipient of the Träger (#6824,
// ADR 0217). Every statement names the Träger in its own filter, also in
// its joins: below row level security the filter alone keeps the Träger to
// its rows.

// handoverUnknownAfter is when a pending handover counts as unknown: past
// the transport's intent and dispatch fences (two minutes each) and its
// one-minute upload bound.
const handoverUnknownAfter = "5 minutes"

// SameContentHandedOver tells whether another file of the same run and
// format with the same content (the comparison of the protocol's repeat_of)
// was handed over, is under way or has an unknown outcome.
func (s *Store) SameContentHandedOver(ctx context.Context, organizationID, exportFileID int64) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	var exists bool
	if err := db.NewRaw(`SELECT EXISTS (
		SELECT 1 FROM parent_billing.export_files AS file
		JOIN parent_billing.export_files AS earlier ON earlier.organization_id = file.organization_id
			AND earlier.run_kind = file.run_kind AND earlier.run_id = file.run_id AND earlier.format = file.format
			AND earlier.id <> file.id
			AND CASE WHEN file.format = 'pain008' AND NULLIF(file.reference, '') IS NOT NULL
				THEN earlier.reference = file.reference
				WHEN file.format = 'datev' AND file.booking_sha256 IS NOT NULL
				THEN earlier.booking_sha256 = file.booking_sha256
				ELSE earlier.content_sha256 = file.content_sha256 END
		JOIN parent_billing.export_file_transfers AS handover ON handover.organization_id = earlier.organization_id
			AND handover.export_file_id = earlier.id
		WHERE file.organization_id = ? AND earlier.organization_id = ? AND handover.organization_id = ?
			AND file.id = ? AND handover.state IN ('pending', 'transferred'))`,
		organizationID, organizationID, organizationID, exportFileID).Scan(ctx, &exists); err != nil {
		return false, fmt.Errorf("check earlier handovers: %w", err)
	}
	return exists, nil
}

// InsertExportFileHandover records the pending handover of a file.
func (s *Store) InsertExportFileHandover(ctx context.Context, organizationID, exportFileID int64, host, directory string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO parent_billing.export_file_transfers
		(organization_id, export_file_id, target_host, target_directory) VALUES (?, ?, ?, ?)`,
		organizationID, exportFileID, host, directory); err != nil {
		return fmt.Errorf("insert the handover: %w", err)
	}
	return nil
}

// SettleExportFileHandover settles a pending handover once.
func (s *Store) SettleExportFileHandover(ctx context.Context, organizationID, exportFileID int64, state, reason string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx, `UPDATE parent_billing.export_file_transfers
		SET state = ?, failure_reason = ?, settled_at = CURRENT_TIMESTAMP
		WHERE organization_id = ? AND export_file_id = ? AND state = 'pending'`, state, reason, organizationID, exportFileID)
	if err != nil {
		return fmt.Errorf("settle the handover: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return errors.New("settle the handover: no pending handover of this file")
	}
	return nil
}

// exportFileHandoverColumns are the columns of the protocol list's join.
type exportFileHandoverColumns struct {
	HandoverState     *string    `bun:"handover_state,scanonly"`
	HandoverReason    *string    `bun:"handover_reason,scanonly"`
	HandoverHost      *string    `bun:"handover_host,scanonly"`
	HandoverDirectory *string    `bun:"handover_directory,scanonly"`
	HandoverStartedAt *time.Time `bun:"handover_started_at,scanonly"`
	HandoverSettledAt *time.Time `bun:"handover_settled_at,scanonly"`
}

func (c exportFileHandoverColumns) handover() *parentbilling.ExportFileHandover {
	if c.HandoverState == nil || c.HandoverStartedAt == nil {
		return nil
	}
	return &parentbilling.ExportFileHandover{State: *c.HandoverState, Reason: textOf(c.HandoverReason),
		TargetHost: textOf(c.HandoverHost), TargetDirectory: textOf(c.HandoverDirectory),
		StartedAt: *c.HandoverStartedAt, SettledAt: c.HandoverSettledAt}
}
