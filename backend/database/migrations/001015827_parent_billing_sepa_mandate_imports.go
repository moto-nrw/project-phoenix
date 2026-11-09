package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingSEPAMandateImportsVersion = "1.15.827"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingSEPAMandateImportsVersion,
		Description: "Elternabrechnung: Import bestehender SEPA-Mandate mit Herkunft im Protokoll (#6198)",
		DependsOn: []string{
			studentMembershipCareGapVersion,  // the newest migration on development
			parentBillingSEPAMandatesVersion, // the trail it extends
		},
	})
	Migrations.MustRegister(parentBillingSEPAMandateImportsUp, parentBillingSEPAMandateImportsDown)
}

// parentBillingSEPAMandateImportsUpSQL records where an imported SEPA
// mandate came from (#6198): the creation entry of its trail keeps the
// name of the file and the line. The file itself is never stored, so no
// bank data stays outside the sealed mandate. Only the creation of an
// imported mandate carries both; the trail stays only appended.
const parentBillingSEPAMandateImportsUpSQL = `
ALTER TABLE parent_billing.sepa_mandate_events
	ADD COLUMN import_file VARCHAR(255),
	ADD COLUMN import_line INTEGER,
	ADD CONSTRAINT chk_sepa_mandate_events_import CHECK (
		num_nulls(import_file, import_line) IN (0, 2)
		AND (import_file IS NULL OR (action = 'create' AND source = 'import'
			AND BTRIM(import_file) <> '' AND import_line > 1)));

CREATE INDEX idx_sepa_mandate_events_import ON parent_billing.sepa_mandate_events (organization_id, occurred_at DESC)
	WHERE import_file IS NOT NULL;

COMMENT ON COLUMN parent_billing.sepa_mandate_events.import_file IS
	'Name of the CSV file an imported mandate came from (#6198); the file itself is not stored.';
COMMENT ON COLUMN parent_billing.sepa_mandate_events.import_line IS
	'Line of the CSV file an imported mandate came from (#6198).';
`

const parentBillingSEPAMandateImportsDownSQL = `
DROP INDEX IF EXISTS parent_billing.idx_sepa_mandate_events_import;
ALTER TABLE parent_billing.sepa_mandate_events
	DROP CONSTRAINT IF EXISTS chk_sepa_mandate_events_import,
	DROP COLUMN IF EXISTS import_line,
	DROP COLUMN IF EXISTS import_file;
`

func parentBillingSEPAMandateImportsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingSEPAMandateImportsUpSQL); err != nil {
		return fmt.Errorf("add parent billing SEPA mandate import trail: %w", err)
	}
	return nil
}

func parentBillingSEPAMandateImportsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingSEPAMandateImportsDownSQL); err != nil {
		return fmt.Errorf("drop parent billing SEPA mandate import trail: %w", err)
	}
	return nil
}
