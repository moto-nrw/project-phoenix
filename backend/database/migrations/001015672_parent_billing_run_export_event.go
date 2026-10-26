package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingRunExportEventVersion = "1.15.672"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingRunExportEventVersion,
		Description: "Elternabrechnung: Download der Übersicht eines Laufs für die Kämmerei steht in der Spur des Laufs (#4672)",
		DependsOn: []string{
			parentBillingHolidayRunsVersion, // the trail of a run
			parentBillingLettersVersion,     // the letters action of the trail
		},
	})
	Migrations.MustRegister(parentBillingRunExportEventUp, parentBillingRunExportEventDown)
}

// parentBillingRunExportEventUpSQL lets the trail of a billing run record
// the download of its overview for the Kämmerei (#4672, ADR 0122).
const parentBillingRunExportEventUpSQL = `
ALTER TABLE parent_billing.run_events DROP CONSTRAINT chk_run_events_action;
ALTER TABLE parent_billing.run_events ADD CONSTRAINT chk_run_events_action
	CHECK (action IN ('create', 'read', 'correct', 'release', 'letters', 'export'));
`

// parentBillingRunExportEventDownSQL drops the recorded downloads with the
// action, since the trail of a run is otherwise append-only.
const parentBillingRunExportEventDownSQL = `
DELETE FROM parent_billing.run_events WHERE action = 'export';
ALTER TABLE parent_billing.run_events DROP CONSTRAINT chk_run_events_action;
ALTER TABLE parent_billing.run_events ADD CONSTRAINT chk_run_events_action
	CHECK (action IN ('create', 'read', 'correct', 'release', 'letters'));
`

func parentBillingRunExportEventUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingRunExportEventUpSQL); err != nil {
		return fmt.Errorf("allow the export event of a billing run: %w", err)
	}
	return nil
}

func parentBillingRunExportEventDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingRunExportEventDownSQL); err != nil {
		return fmt.Errorf("drop the export event of a billing run: %w", err)
	}
	return nil
}
