package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingCarriedCorrectionsVersion = "1.15.748"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCarriedCorrectionsVersion,
		Description: "Elternabrechnung: neu ausgestellte Rechnung übernimmt die Korrekturen der stornierten (#5663)",
		DependsOn: []string{
			holidaySiblingGroupsVersion,             // the last migration before this one
			parentBillingDocumentCorrectionsVersion, // the cancelled invoices this reads
		},
	})
	Migrations.MustRegister(parentBillingCarriedCorrectionsUp, parentBillingCarriedCorrectionsDown)
}

// parentBillingCarriedCorrectionsUpSQL lets the next run of a period propose
// the corrections of a cancelled invoice (#5663, ADR 0119 addendum). A
// calculated run stores no drafts, so a proposal is no row: the preview reads
// it from the correction positions of the invoice it replaces.
//
//   - parent_billing.run_correction_dismissals: a proposal the Abrechnung
//     removed or changed in a calculated run; only added.
//   - parent_billing.run_corrections.carried_from_number: the number of the
//     cancelled invoice on the history of a proposal the release issued.
const parentBillingCarriedCorrectionsUpSQL = `
ALTER TABLE parent_billing.run_corrections
	ADD COLUMN carried_from_number VARCHAR(64) NOT NULL DEFAULT '';
COMMENT ON COLUMN parent_billing.run_corrections.carried_from_number IS
	'Number of the cancelled invoice this correction was carried over from (#5663); empty for a correction of the run itself.';

CREATE TABLE parent_billing.run_correction_dismissals (
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	document_position_id BIGINT NOT NULL,
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (organization_id, run_id, document_position_id),
	CONSTRAINT fk_run_correction_dismissals_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_run_correction_dismissals_position FOREIGN KEY (document_position_id)
		REFERENCES parent_billing.document_positions (id) ON DELETE CASCADE
);
CREATE INDEX idx_run_correction_dismissals_position ON parent_billing.run_correction_dismissals (document_position_id);
COMMENT ON TABLE parent_billing.run_correction_dismissals IS
	'Correction of a cancelled invoice the Abrechnung removed or changed in a calculated run, so the preview no longer proposes it (#5663); only added.';

ALTER TABLE parent_billing.run_correction_dismissals ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.run_correction_dismissals FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_run_correction_dismissals ON parent_billing.run_correction_dismissals FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.run_correction_dismissals FROM PUBLIC;
GRANT SELECT, INSERT ON parent_billing.run_correction_dismissals TO phoenix_tenant;
GRANT ALL ON parent_billing.run_correction_dismissals TO phoenix_admin;
`

const parentBillingCarriedCorrectionsDownSQL = `
DROP TABLE IF EXISTS parent_billing.run_correction_dismissals;
ALTER TABLE parent_billing.run_corrections DROP COLUMN carried_from_number;
`

func parentBillingCarriedCorrectionsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCarriedCorrectionsUpSQL); err != nil {
		return fmt.Errorf("add parent billing carried corrections: %w", err)
	}
	return nil
}

func parentBillingCarriedCorrectionsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCarriedCorrectionsDownSQL); err != nil {
		return fmt.Errorf("drop parent billing carried corrections: %w", err)
	}
	return nil
}
