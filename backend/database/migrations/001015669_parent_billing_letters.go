package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingLettersVersion = "1.15.669"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingLettersVersion,
		Description: "Elternabrechnung: Bestätigungsschreiben als Rechnung im Layout des Trägers (#4669)",
		DependsOn: []string{
			parentBillingHolidayRunsVersion, // the released runs and issued invoices a letter confirms
		},
	})
	Migrations.MustRegister(parentBillingLettersUp, parentBillingLettersDown)
}

// parentBillingLettersUpSQL adds the confirmation letter of the
// Elternabrechnung (parent-billing, #4669, ADR 0121), the letter that
// confirms a holiday registration and is its invoice:
//
//   - parent_billing.run_letters: what the Abrechnung wrote for the letters
//     of a released run: the payment date, a free text for the parents
//     (packing list, rule on illness) and the emergency numbers of the
//     hosting OGS. A run has its letters once.
//   - parent_billing.document_letters: the PDF of the letter of each issued
//     invoice with its SHA-256, which the database computes, rendered once by Document Rendering in the
//     layout of the Träger.
//
// Like an issued invoice, neither ever changes: the request role may only
// read and add them, and the trigger of ADR 0119 refuses every update for
// every role. They go with their run and invoice, so with the Träger. The
// trail of a run records the creation as the action 'letters'.
const parentBillingLettersUpSQL = `
CREATE TABLE parent_billing.run_letters (
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	payment_due_on DATE NOT NULL,
	notes TEXT NOT NULL DEFAULT '',
	emergency_contacts JSONB NOT NULL DEFAULT '[]'::jsonb,
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (organization_id, run_id),
	CONSTRAINT fk_run_letters_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_run_letters_notes CHECK (char_length(notes) <= 4000),
	CONSTRAINT chk_run_letters_contacts CHECK (jsonb_typeof(emergency_contacts) = 'array' AND jsonb_array_length(emergency_contacts) <= 10)
);

COMMENT ON TABLE parent_billing.run_letters IS
	'Payment date, free text and emergency numbers of the confirmation letters of a released billing run (#4669, ADR 0121); never changes.';

CREATE TABLE parent_billing.document_letters (
	organization_id BIGINT NOT NULL,
	document_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	filename TEXT NOT NULL,
	pdf BYTEA NOT NULL,
	sha256 CHAR(64) GENERATED ALWAYS AS (encode(sha256(pdf), 'hex')) STORED,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (organization_id, document_id),
	CONSTRAINT fk_document_letters_document FOREIGN KEY (organization_id, document_id)
		REFERENCES parent_billing.documents (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_document_letters_run_letters FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.run_letters (organization_id, run_id) ON DELETE CASCADE,
	CONSTRAINT chk_document_letters_pdf CHECK (octet_length(pdf) > 0 AND BTRIM(filename) <> '')
);

CREATE INDEX idx_document_letters_run ON parent_billing.document_letters (organization_id, run_id);

COMMENT ON TABLE parent_billing.document_letters IS
	'PDF of the confirmation letter of an issued invoice in the layout of the Träger (#4669, ADR 0121); rendered once, never changes, goes with its invoice.';

CREATE TRIGGER refuse_parent_billing_run_letters_change
	BEFORE UPDATE ON parent_billing.run_letters
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();

CREATE TRIGGER refuse_parent_billing_document_letter_change
	BEFORE UPDATE ON parent_billing.document_letters
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();

ALTER TABLE parent_billing.run_events DROP CONSTRAINT chk_run_events_action;
ALTER TABLE parent_billing.run_events ADD CONSTRAINT chk_run_events_action
	CHECK (action IN ('create', 'read', 'correct', 'release', 'letters'));

ALTER TABLE parent_billing.run_letters ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.run_letters FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_run_letters ON parent_billing.run_letters FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

ALTER TABLE parent_billing.document_letters ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.document_letters FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_document_letters ON parent_billing.document_letters FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.run_letters, parent_billing.document_letters FROM PUBLIC;
GRANT SELECT, INSERT ON parent_billing.run_letters, parent_billing.document_letters TO phoenix_tenant;
GRANT ALL ON parent_billing.run_letters, parent_billing.document_letters TO phoenix_admin;
`

const parentBillingLettersDownSQL = `
DROP TABLE IF EXISTS parent_billing.document_letters, parent_billing.run_letters;
DELETE FROM parent_billing.run_events WHERE action = 'letters';
ALTER TABLE parent_billing.run_events DROP CONSTRAINT chk_run_events_action;
ALTER TABLE parent_billing.run_events ADD CONSTRAINT chk_run_events_action
	CHECK (action IN ('create', 'read', 'correct', 'release'));
`

func parentBillingLettersUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingLettersUpSQL); err != nil {
		return fmt.Errorf("create parent billing letters: %w", err)
	}
	return nil
}

func parentBillingLettersDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingLettersDownSQL); err != nil {
		return fmt.Errorf("drop parent billing letters: %w", err)
	}
	return nil
}
