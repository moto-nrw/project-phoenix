package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayHostSchoolCostCenterVersion = "1.15.915"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayHostSchoolCostCenterVersion,
		Description: "Elternabrechnung: Ferienpositionen auf KOST1 der ausrichtenden Schule, Rückfall im Exportprotokoll (#6624)",
		DependsOn: []string{
			parentBillingBookingAccountsVersion,  // the frozen accounts (1.15.906) of document_positions and export_files
			parentBillingDiscardEmptyRunsVersion, // the newest on development (1.15.914)
		},
	})
	Migrations.MustRegister(holidayHostSchoolCostCenterUp, holidayHostSchoolCostCenterDown)
}

// holidayHostSchoolCostCenterUpSQL keeps why a holiday position books on
// the KOST1 of the holiday care instead of its host school's (#6624):
//
//   - document_positions.cost_center_fallback: 'no_host_school' when the
//     release found no host school of the week, 'no_school_cost_center'
//     when the host school had no KOST1 on the first day of the week; NULL
//     for a position on its host school's KOST1 and for every position
//     issued before. The release writes it with the position; the trigger
//     refuse_issued_change keeps it.
//   - export_files.cost_center_fallbacks: the positions of the file that
//     booked on the fallback, with document, week, reason and the KOST1
//     used, for the protocol. Files before have none.
const holidayHostSchoolCostCenterUpSQL = `
ALTER TABLE parent_billing.document_positions
	ADD COLUMN cost_center_fallback VARCHAR(24),
	ADD CONSTRAINT chk_document_positions_cost_center_fallback
		CHECK (cost_center_fallback IN ('no_host_school', 'no_school_cost_center'));
ALTER TABLE parent_billing.export_files
	ADD COLUMN cost_center_fallbacks JSONB NOT NULL DEFAULT '[]'::jsonb,
	ADD CONSTRAINT chk_export_files_cost_center_fallbacks CHECK (jsonb_typeof(cost_center_fallbacks) = 'array');
`

const holidayHostSchoolCostCenterDownSQL = `
ALTER TABLE parent_billing.export_files
	DROP CONSTRAINT IF EXISTS chk_export_files_cost_center_fallbacks, DROP COLUMN IF EXISTS cost_center_fallbacks;
ALTER TABLE parent_billing.document_positions
	DROP CONSTRAINT IF EXISTS chk_document_positions_cost_center_fallback, DROP COLUMN IF EXISTS cost_center_fallback;
`

func holidayHostSchoolCostCenterUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayHostSchoolCostCenterUpSQL); err != nil {
		return fmt.Errorf("add the cost center fallback of the holiday positions: %w", err)
	}
	return nil
}

func holidayHostSchoolCostCenterDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayHostSchoolCostCenterDownSQL); err != nil {
		return fmt.Errorf("drop the cost center fallback of the holiday positions: %w", err)
	}
	return nil
}
