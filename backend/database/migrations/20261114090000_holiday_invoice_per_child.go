package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayInvoicePerChildVersion = "20261114090000"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayInvoicePerChildVersion,
		Description: "Elternabrechnung: Ferienrechnung je Kind, wenn der Träger die Debitorennummer je Kind führt (#7007)",
		DependsOn: []string{
			parentBillingDocumentCorrectionsVersion, // uq_documents_first_invoice this one replaces
			childDebtorNumbersVersion,               // the debtor numbers of the children the invoices take
		},
	})
	Migrations.MustRegister(holidayInvoicePerChildUp, holidayInvoicePerChildDown)
}

// holidayInvoicePerChildUpSQL lets a holiday invoice bill one child of its
// registration (#7007, parent_billing.debtor_number_scope "child"): each
// child gets an invoice of its own with its own debtor number.
//
//   - holiday_registration_child_id is NULL on an invoice of the whole
//     registration, as every invoice before, and names the child otherwise.
//     Like document_positions it keeps no foreign key: an issued document
//     outlives the registration.
//   - The first invoice is unique per registration and child; a
//     registration billed as a whole has one, one billed per child one per
//     child. The release keeps one shape per registration: the shape of its
//     first invoice. A cancellation, a credit and the invoice issued again
//     copy the child of the invoice they follow.
const holidayInvoicePerChildUpSQL = `
ALTER TABLE parent_billing.documents
	ADD COLUMN holiday_registration_child_id BIGINT,
	ADD CONSTRAINT chk_documents_registration_child CHECK (holiday_registration_child_id IS NULL OR holiday_registration_child_id > 0);
COMMENT ON COLUMN parent_billing.documents.holiday_registration_child_id IS
	'Child of the registration a holiday invoice bills when the Träger keeps the debtor number per child (#7007); NULL bills the whole registration.';

DROP INDEX parent_billing.uq_documents_first_invoice;
CREATE UNIQUE INDEX uq_documents_first_invoice ON parent_billing.documents
	(organization_id, service, holiday_registration_id, COALESCE(holiday_registration_child_id, 0))
	WHERE kind = 'invoice' AND reference_document_id IS NULL;
`

const holidayInvoicePerChildDownSQL = `
DO $$
BEGIN
	IF EXISTS (SELECT 1 FROM parent_billing.documents WHERE holiday_registration_child_id IS NOT NULL) THEN
		RAISE EXCEPTION 'holiday invoices per child exist; they cannot become invoices of their registration';
	END IF;
END $$;
DROP INDEX parent_billing.uq_documents_first_invoice;
CREATE UNIQUE INDEX uq_documents_first_invoice ON parent_billing.documents (organization_id, service, holiday_registration_id)
	WHERE kind = 'invoice' AND reference_document_id IS NULL;
ALTER TABLE parent_billing.documents
	DROP CONSTRAINT chk_documents_registration_child,
	DROP COLUMN holiday_registration_child_id;
`

func holidayInvoicePerChildUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayInvoicePerChildUpSQL); err != nil {
		return fmt.Errorf("let a holiday invoice bill one child: %w", err)
	}
	return nil
}

func holidayInvoicePerChildDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayInvoicePerChildDownSQL); err != nil {
		return fmt.Errorf("bill every holiday invoice per registration again: %w", err)
	}
	return nil
}
