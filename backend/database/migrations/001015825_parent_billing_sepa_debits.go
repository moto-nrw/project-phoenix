package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingSEPADebitsVersion = "1.15.825"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingSEPADebitsVersion,
		Description: "Elternabrechnung: SEPA-Lastschriftdatei PAIN.008 je Lauf und Einzugshistorie (#4697)",
		DependsOn: []string{
			parentBillingSEPAMandatesVersion, // the mandates and the mandate copied onto the invoices
		},
	})
	Migrations.MustRegister(parentBillingSEPADebitsUp, parentBillingSEPADebitsDown)
}

// parentBillingSEPADebitsUpSQL adds the debits of the SEPA direct debit file
// (#4697, ADR 0181):
//
//   - sepa_debits: one row per issued invoice that is collected by direct
//     debit (monthly invoice or holiday invoice). The release writes it with
//     the account of the mandate as it is then: holder and IBAN stay sealed
//     (copied, never opened), so a later change of the mandate does not
//     reach a file of an issued run. The amount is what the family pays
//     after the cost bearers. A row is pending until the office confirms
//     that it submitted the file to its bank; the confirmation marks the
//     rows submitted, sets the last collection of the mandate and turns its
//     next sequence from FRST to RCUR. That is the collection history per
//     mandate and invoice.
//   - care_invoices and documents: a direct debit whose mandate expired
//     (36 months without a collection) falls back with the gap
//     mandate_expired.
//
// Invoices issued before by direct debit get their row from the mandate as
// it is now; no file was ever made for them.
const parentBillingSEPADebitsUpSQL = `
CREATE TABLE parent_billing.sepa_debits (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	mandate_id BIGINT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	run_kind VARCHAR(16) NOT NULL,
	run_id BIGINT NOT NULL,
	care_invoice_id BIGINT,
	document_id BIGINT,
	invoice_number VARCHAR(64) NOT NULL,
	amount_cents BIGINT NOT NULL,
	due_on DATE NOT NULL,
	sequence_type VARCHAR(4) NOT NULL,
	creditor_id VARCHAR(35) NOT NULL,
	mandate_reference VARCHAR(35) NOT NULL,
	mandate_signed_on DATE NOT NULL,
	account_holder_sealed BYTEA NOT NULL,
	iban_sealed BYTEA NOT NULL,
	bic VARCHAR(11) NOT NULL DEFAULT '',
	status VARCHAR(16) NOT NULL DEFAULT 'pending',
	message_id VARCHAR(35),
	submitted_at TIMESTAMPTZ,
	submitted_amount_cents BIGINT,
	collect_on DATE,
	submitted_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_sepa_debits_organization UNIQUE (organization_id, id),
	CONSTRAINT fk_sepa_debits_mandate FOREIGN KEY (organization_id, mandate_id)
		REFERENCES parent_billing.sepa_mandates (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_sepa_debits_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_sepa_debits_care_invoice FOREIGN KEY (organization_id, care_invoice_id)
		REFERENCES parent_billing.care_invoices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_sepa_debits_document FOREIGN KEY (organization_id, document_id)
		REFERENCES parent_billing.documents (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_sepa_debits_invoice CHECK (
		(run_kind = 'care' AND care_invoice_id IS NOT NULL AND document_id IS NULL)
		OR (run_kind = 'holiday' AND document_id IS NOT NULL AND care_invoice_id IS NULL)),
	CONSTRAINT chk_sepa_debits_amount CHECK (amount_cents > 0),
	CONSTRAINT chk_sepa_debits_sequence CHECK (sequence_type IN ('FRST', 'RCUR', 'OOFF')),
	CONSTRAINT chk_sepa_debits_status CHECK (
		(status = 'pending' AND message_id IS NULL AND submitted_at IS NULL AND submitted_amount_cents IS NULL AND collect_on IS NULL)
		OR (status = 'submitted' AND message_id IS NOT NULL AND submitted_at IS NOT NULL
			AND submitted_amount_cents IS NOT NULL AND submitted_amount_cents > 0 AND submitted_amount_cents <= amount_cents
			AND collect_on IS NOT NULL AND collect_on >= due_on))
);
CREATE UNIQUE INDEX uq_sepa_debits_care_invoice ON parent_billing.sepa_debits (organization_id, care_invoice_id)
	WHERE care_invoice_id IS NOT NULL;
CREATE UNIQUE INDEX uq_sepa_debits_document ON parent_billing.sepa_debits (organization_id, document_id)
	WHERE document_id IS NOT NULL;
CREATE INDEX idx_sepa_debits_run ON parent_billing.sepa_debits (organization_id, run_kind, run_id);
CREATE INDEX idx_sepa_debits_mandate ON parent_billing.sepa_debits (organization_id, mandate_id, status);
CREATE INDEX idx_sepa_debits_contract ON parent_billing.sepa_debits (organization_id, care_contract_id);
COMMENT ON TABLE parent_billing.sepa_debits IS
	'Debit of an issued invoice by SEPA direct debit (#4697): amount, due date, sequence type and the sealed account of the mandate at the release; pending until the file is submitted.';
COMMENT ON COLUMN parent_billing.sepa_debits.iban_sealed IS
	'IBAN of the mandate at the release, sealed with the field cipher, bound to the Träger and the mandate reference.';
COMMENT ON COLUMN parent_billing.sepa_debits.account_holder_sealed IS
	'Account holder of the mandate at the release, sealed like iban_sealed.';

-- A debit keeps what the release wrote. Only a pending one may change: it
-- becomes submitted, or its FRST becomes RCUR when another file of the
-- mandate was submitted first. Account erasure may clear the submitter.
CREATE FUNCTION parent_billing.refuse_sepa_debit_rewrite() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF (to_jsonb(NEW) - ARRAY['status', 'message_id', 'submitted_at', 'submitted_by_account_id', 'sequence_type', 'submitted_amount_cents', 'collect_on'])
		IS DISTINCT FROM
		(to_jsonb(OLD) - ARRAY['status', 'message_id', 'submitted_at', 'submitted_by_account_id', 'sequence_type', 'submitted_amount_cents', 'collect_on']) THEN
		RAISE EXCEPTION 'a SEPA debit keeps its invoice, amount and account' USING ERRCODE = '55000';
	END IF;
	IF OLD.status = 'submitted' AND (NEW.status IS DISTINCT FROM OLD.status OR NEW.message_id IS DISTINCT FROM OLD.message_id
		OR NEW.submitted_amount_cents IS DISTINCT FROM OLD.submitted_amount_cents
		OR NEW.collect_on IS DISTINCT FROM OLD.collect_on
		OR NEW.submitted_at IS DISTINCT FROM OLD.submitted_at OR NEW.sequence_type IS DISTINCT FROM OLD.sequence_type
		OR (NEW.submitted_by_account_id IS NOT NULL AND NEW.submitted_by_account_id IS DISTINCT FROM OLD.submitted_by_account_id)) THEN
		RAISE EXCEPTION 'a submitted SEPA debit never changes' USING ERRCODE = '55000';
	END IF;
	IF NEW.sequence_type IS DISTINCT FROM OLD.sequence_type AND NOT (OLD.sequence_type = 'FRST' AND NEW.sequence_type = 'RCUR') THEN
		RAISE EXCEPTION 'a SEPA debit only turns from FRST to RCUR' USING ERRCODE = '55000';
	END IF;
	RETURN NEW;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.refuse_sepa_debit_rewrite() FROM PUBLIC;
CREATE TRIGGER refuse_parent_billing_sepa_debit_rewrite
	BEFORE UPDATE ON parent_billing.sepa_debits
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_sepa_debit_rewrite();

ALTER TABLE parent_billing.sepa_debits ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.sepa_debits FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_sepa_debits ON parent_billing.sepa_debits FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
REVOKE ALL ON parent_billing.sepa_debits FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON parent_billing.sepa_debits TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.sepa_debits_id_seq TO phoenix_tenant;

ALTER TABLE parent_billing.care_invoices DROP CONSTRAINT chk_care_invoices_mandate,
	ADD CONSTRAINT chk_care_invoices_mandate CHECK (
		num_nulls(mandate_id, mandate_reference, mandate_signed_on, creditor_id, debtor_iban_masked, sequence_type) IN (0, 6)
		AND (mandate_reference IS NULL OR payment_method = 'direct_debit')
		AND (sequence_type IS NULL OR sequence_type IN ('FRST', 'RCUR', 'OOFF'))
		AND (debit_gap IS NULL OR (debit_gap IN ('mandate_missing', 'creditor_missing', 'creditor_changed', 'mandate_expired')
			AND mandate_reference IS NULL)));
ALTER TABLE parent_billing.documents DROP CONSTRAINT chk_documents_mandate,
	ADD CONSTRAINT chk_documents_mandate CHECK (
		num_nulls(mandate_id, mandate_reference, mandate_signed_on, creditor_id, debtor_iban_masked, sequence_type) IN (0, 6)
		AND (mandate_reference IS NULL OR payment_method = 'direct_debit')
		AND (sequence_type IS NULL OR sequence_type IN ('FRST', 'RCUR', 'OOFF'))
		AND (debit_gap IS NULL OR (debit_gap IN ('mandate_missing', 'creditor_missing', 'creditor_changed', 'mandate_expired')
			AND mandate_reference IS NULL)));

INSERT INTO parent_billing.sepa_debits (organization_id, mandate_id, care_contract_id, run_kind, run_id, care_invoice_id,
	invoice_number, amount_cents, due_on, sequence_type, creditor_id, mandate_reference, mandate_signed_on,
	account_holder_sealed, iban_sealed, bic)
SELECT invoice.organization_id, mandate.id, mandate.care_contract_id, 'care', invoice.run_id, invoice.id,
	invoice.number, invoice.total_cents, invoice.due_on, invoice.sequence_type, invoice.creditor_id, invoice.mandate_reference,
	invoice.mandate_signed_on, mandate.account_holder_sealed, mandate.iban_sealed, mandate.bic
FROM parent_billing.care_invoices AS invoice
JOIN parent_billing.sepa_mandates AS mandate
	ON mandate.organization_id = invoice.organization_id AND mandate.id = invoice.mandate_id
WHERE invoice.payment_method = 'direct_debit' AND invoice.total_cents > 0 AND invoice.due_on IS NOT NULL;

INSERT INTO parent_billing.sepa_debits (organization_id, mandate_id, care_contract_id, run_kind, run_id, document_id,
	invoice_number, amount_cents, due_on, sequence_type, creditor_id, mandate_reference, mandate_signed_on,
	account_holder_sealed, iban_sealed, bic)
SELECT document.organization_id, mandate.id, mandate.care_contract_id, 'holiday', document.run_id, document.id,
	document.number, document.total_cents, document.due_on, document.sequence_type, document.creditor_id,
	document.mandate_reference, document.mandate_signed_on, mandate.account_holder_sealed, mandate.iban_sealed, mandate.bic
FROM parent_billing.documents AS document
JOIN parent_billing.sepa_mandates AS mandate
	ON mandate.organization_id = document.organization_id AND mandate.id = document.mandate_id
WHERE document.kind = 'invoice' AND document.payment_method = 'direct_debit' AND document.total_cents > 0
	AND document.due_on IS NOT NULL;
`

const parentBillingSEPADebitsDownSQL = `
ALTER TABLE parent_billing.documents DROP CONSTRAINT IF EXISTS chk_documents_mandate,
	ADD CONSTRAINT chk_documents_mandate CHECK (
		num_nulls(mandate_id, mandate_reference, mandate_signed_on, creditor_id, debtor_iban_masked, sequence_type) IN (0, 6)
		AND (mandate_reference IS NULL OR payment_method = 'direct_debit')
		AND (sequence_type IS NULL OR sequence_type IN ('FRST', 'RCUR', 'OOFF'))
		AND (debit_gap IS NULL OR (debit_gap IN ('mandate_missing', 'creditor_missing', 'creditor_changed') AND mandate_reference IS NULL)));
ALTER TABLE parent_billing.care_invoices DROP CONSTRAINT IF EXISTS chk_care_invoices_mandate,
	ADD CONSTRAINT chk_care_invoices_mandate CHECK (
		num_nulls(mandate_id, mandate_reference, mandate_signed_on, creditor_id, debtor_iban_masked, sequence_type) IN (0, 6)
		AND (mandate_reference IS NULL OR payment_method = 'direct_debit')
		AND (sequence_type IS NULL OR sequence_type IN ('FRST', 'RCUR', 'OOFF'))
		AND (debit_gap IS NULL OR (debit_gap IN ('mandate_missing', 'creditor_missing', 'creditor_changed') AND mandate_reference IS NULL)));
DROP TABLE IF EXISTS parent_billing.sepa_debits;
DROP FUNCTION IF EXISTS parent_billing.refuse_sepa_debit_rewrite();
`

func parentBillingSEPADebitsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingSEPADebitsUpSQL); err != nil {
		return fmt.Errorf("create parent billing SEPA debits: %w", err)
	}
	return nil
}

func parentBillingSEPADebitsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingSEPADebitsDownSQL); err != nil {
		return fmt.Errorf("drop parent billing SEPA debits: %w", err)
	}
	return nil
}
