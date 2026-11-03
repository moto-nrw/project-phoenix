package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingDocumentCorrectionsVersion = "1.15.743"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingDocumentCorrectionsVersion,
		Description: "Elternabrechnung: Storno, Gutschrift und Neuausstellung ausgestellter Rechnungen als eigene Dokumente (#4690)",
		DependsOn: []string{
			holidayDebtorNumberVersion,         // the last migration before this one
			parentBillingRunExportEventVersion, // the actions of the trail this extends
			parentBillingHolidayRunsVersion,    // the invoices this extends
		},
	})
	Migrations.MustRegister(parentBillingDocumentCorrectionsUp, parentBillingDocumentCorrectionsDown)
}

// parentBillingDocumentCorrectionsUpSQL lets an issued invoice be cancelled,
// credited and issued again, each as a document of its own (spec #4611 user
// story 72, #4690, ADR 0119 addendum):
//
//   - parent_billing.documents.kind: 'invoice', 'cancellation' (Storno) or
//     'credit' (Gutschrift). Every existing document is an invoice.
//   - reference_document_id: the invoice a cancellation or credit belongs
//     to, and on an invoice issued again the cancelled invoice it replaces.
//   - reason: why the Abrechnung cancelled or credited; an invoice has none.
//
// A cancellation negates every position of its invoice, a credit lowers it
// by one position 'credit'; both carry a number from the same number range
// as the invoices. An invoice is cancelled and replaced at most once
// (partial unique indexes). The rule "one invoice per registration" of
// ADR 0119 now holds for the first invoice of a registration; a cancelled
// invoice lets the next run of the period issue it again. The trigger
// against changes stays: no document is ever updated. The trail of a run
// records a cancellation as 'cancel' and a credit as 'credit'.
const parentBillingDocumentCorrectionsUpSQL = `
ALTER TABLE parent_billing.documents
	ADD COLUMN kind VARCHAR(16) NOT NULL DEFAULT 'invoice',
	ADD COLUMN reference_document_id BIGINT,
	ADD COLUMN reason TEXT NOT NULL DEFAULT '',
	ADD CONSTRAINT fk_documents_reference FOREIGN KEY (organization_id, reference_document_id)
		REFERENCES parent_billing.documents (organization_id, id) ON DELETE CASCADE,
	ADD CONSTRAINT chk_documents_kind CHECK (kind IN ('invoice', 'cancellation', 'credit')),
	ADD CONSTRAINT chk_documents_reference CHECK (kind = 'invoice' OR reference_document_id IS NOT NULL),
	ADD CONSTRAINT chk_documents_reason CHECK (
		(kind = 'invoice' AND reason = '')
		OR (kind <> 'invoice' AND BTRIM(reason) <> '' AND char_length(reason) <= 500)),
	DROP CONSTRAINT chk_documents_total,
	ADD CONSTRAINT chk_documents_total CHECK (
		(kind = 'invoice' AND total_cents >= 0)
		OR (kind = 'cancellation' AND total_cents <= 0)
		OR (kind = 'credit' AND total_cents < 0)),
	DROP CONSTRAINT uq_documents_registration;
COMMENT ON COLUMN parent_billing.documents.kind IS
	'invoice, cancellation (Storno) or credit (Gutschrift) of an issued invoice (#4690); a document of its own with a number.';
COMMENT ON COLUMN parent_billing.documents.reference_document_id IS
	'Invoice a cancellation or credit belongs to; on an invoice issued again the cancelled invoice it replaces (#4690).';

CREATE UNIQUE INDEX uq_documents_first_invoice ON parent_billing.documents (organization_id, service, holiday_registration_id)
	WHERE kind = 'invoice' AND reference_document_id IS NULL;
CREATE UNIQUE INDEX uq_documents_cancellation ON parent_billing.documents (organization_id, reference_document_id)
	WHERE kind = 'cancellation';
CREATE UNIQUE INDEX uq_documents_reissue ON parent_billing.documents (organization_id, reference_document_id)
	WHERE kind = 'invoice';
CREATE INDEX idx_documents_reference ON parent_billing.documents (organization_id, reference_document_id)
	WHERE reference_document_id IS NOT NULL;
CREATE INDEX idx_documents_registration ON parent_billing.documents (organization_id, service, holiday_registration_id);

ALTER TABLE parent_billing.document_positions
	DROP CONSTRAINT chk_document_positions_kind,
	ADD CONSTRAINT chk_document_positions_kind CHECK (kind IN ('base', 'sibling', 'benefits_exemption', 'correction', 'credit')),
	ADD CONSTRAINT chk_document_positions_credit CHECK (kind <> 'credit' OR (amount_cents < 0 AND BTRIM(note) <> ''));

ALTER TABLE parent_billing.run_events DROP CONSTRAINT chk_run_events_action;
ALTER TABLE parent_billing.run_events ADD CONSTRAINT chk_run_events_action
	CHECK (action IN ('create', 'read', 'correct', 'release', 'letters', 'export', 'cancel', 'credit'));
`

// The way down removes the cancellations, the credits and the invoices
// issued again, so the rule "one invoice per registration" holds again.
const parentBillingDocumentCorrectionsDownSQL = `
DELETE FROM parent_billing.run_events WHERE action IN ('cancel', 'credit');
ALTER TABLE parent_billing.run_events DROP CONSTRAINT chk_run_events_action;
ALTER TABLE parent_billing.run_events ADD CONSTRAINT chk_run_events_action
	CHECK (action IN ('create', 'read', 'correct', 'release', 'letters', 'export'));
DELETE FROM parent_billing.documents WHERE kind <> 'invoice' OR reference_document_id IS NOT NULL;
ALTER TABLE parent_billing.document_positions
	DROP CONSTRAINT chk_document_positions_credit,
	DROP CONSTRAINT chk_document_positions_kind,
	ADD CONSTRAINT chk_document_positions_kind CHECK (kind IN ('base', 'sibling', 'benefits_exemption', 'correction'));
DROP INDEX IF EXISTS parent_billing.idx_documents_registration;
DROP INDEX IF EXISTS parent_billing.idx_documents_reference;
DROP INDEX IF EXISTS parent_billing.uq_documents_reissue;
DROP INDEX IF EXISTS parent_billing.uq_documents_cancellation;
DROP INDEX IF EXISTS parent_billing.uq_documents_first_invoice;
ALTER TABLE parent_billing.documents
	DROP CONSTRAINT chk_documents_total,
	ADD CONSTRAINT chk_documents_total CHECK (total_cents >= 0),
	ADD CONSTRAINT uq_documents_registration UNIQUE (organization_id, service, holiday_registration_id),
	DROP CONSTRAINT chk_documents_reason,
	DROP CONSTRAINT chk_documents_reference,
	DROP CONSTRAINT chk_documents_kind,
	DROP CONSTRAINT fk_documents_reference,
	DROP COLUMN reason,
	DROP COLUMN reference_document_id,
	DROP COLUMN kind;
`

func parentBillingDocumentCorrectionsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDocumentCorrectionsUpSQL); err != nil {
		return fmt.Errorf("add parent billing document corrections: %w", err)
	}
	return nil
}

func parentBillingDocumentCorrectionsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDocumentCorrectionsDownSQL); err != nil {
		return fmt.Errorf("drop parent billing document corrections: %w", err)
	}
	return nil
}
