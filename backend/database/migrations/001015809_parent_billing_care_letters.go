package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingCareLettersVersion = "1.15.809"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCareLettersVersion,
		Description: "Elternabrechnung: Schreiben, Versand, Sammeldruck und Storno der Monatsrechnungen (#4684)",
		DependsOn: []string{
			parentBillingCareMonthsVersion,          // the monthly invoices the letters belong to
			parentBillingLetterMailingsVersion,      // the documents sequence the letters share with the inbox
			demoVisitorErasureContractPushVersion,   // the erasure this one extends
			parentBillingDocumentCorrectionsVersion, // the Storno of the holiday invoices this one follows
		},
	})
	Migrations.MustRegister(parentBillingCareLettersUp, parentBillingCareLettersDown)
}

// parentBillingCareLettersUpSQL gives the monthly invoices of #4684 the
// letters, the mail and the Storno of the holiday invoices (ADR 0119, 0121,
// 0123, 0126, ADR 0165):
//
//   - care_invoice_cancellations: the Storno of an issued monthly invoice, at
//     most one per invoice, numbered from the same number range "care" and
//     the same counter as the invoices. It negates the whole invoice.
//   - care_invoices.replaces_invoice_id: the invoice a later run of the month
//     issued in place of a cancelled one. A contract has one first invoice
//     per month (uq_care_invoices_contract_month, now partial) and a cancelled
//     invoice is replaced at most once (uq_care_invoices_reissue).
//   - care_run_letters: payment date and free text of the letters of a
//     released monthly run.
//   - care_letters: the PDF of the letter of an invoice or of its Storno,
//     rendered once. Its ID comes from the sequence of the holiday documents,
//     so the inbox of the parents names both kinds by one ID without a clash.
//     A Storno of an invoice that was never sent has no letter: the family
//     never saw the invoice (ADR 0119 addendum 2026-11-07).
//   - care_letter_mailings: every send of a letter with the outbox entry and
//     the address; a letter is in the parents inbox once it was sent.
//
// Everything issued never changes, for any role; the one update allowed is
// the ON DELETE SET NULL of the account that created a row.
const parentBillingCareLettersUpSQL = `
CREATE TABLE parent_billing.care_invoice_cancellations (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	invoice_id BIGINT NOT NULL,
	number VARCHAR(64) NOT NULL,
	number_year SMALLINT NOT NULL,
	number_sequence BIGINT NOT NULL,
	issued_on DATE NOT NULL,
	issued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	reason TEXT NOT NULL,
	total_cents BIGINT NOT NULL,
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	CONSTRAINT uq_care_invoice_cancellations_invoice UNIQUE (organization_id, invoice_id),
	CONSTRAINT uq_care_invoice_cancellations_number UNIQUE (organization_id, number),
	CONSTRAINT uq_care_invoice_cancellations_sequence UNIQUE (organization_id, number_year, number_sequence),
	CONSTRAINT fk_care_invoice_cancellations_invoice FOREIGN KEY (organization_id, invoice_id)
		REFERENCES parent_billing.care_invoices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_care_invoice_cancellations_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.care_runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_invoice_cancellations_number CHECK (BTRIM(number) <> '' AND number_sequence >= 1),
	CONSTRAINT chk_care_invoice_cancellations_reason CHECK (BTRIM(reason) <> '' AND char_length(reason) <= 500),
	CONSTRAINT chk_care_invoice_cancellations_total CHECK (total_cents <= 0)
);
CREATE INDEX idx_care_invoice_cancellations_run ON parent_billing.care_invoice_cancellations (organization_id, run_id);
COMMENT ON TABLE parent_billing.care_invoice_cancellations IS
	'Storno of an issued monthly care invoice, numbered from the number range of the invoices (#4684, ADR 0165); never changes.';

ALTER TABLE parent_billing.care_invoices ADD COLUMN replaces_invoice_id BIGINT;
-- The child record the contract names, copied at the release like the name:
-- the parents inbox shows a letter only to an account whose relationship to
-- this child grants the inbox (ADR 0126 point 2, ADR 0165). An invoice of a
-- contract without a record reaches the family by mail only.
ALTER TABLE parent_billing.care_invoices ADD COLUMN student_id BIGINT;
ALTER TABLE parent_billing.care_invoices ADD CONSTRAINT fk_care_invoices_replaces
	FOREIGN KEY (organization_id, replaces_invoice_id) REFERENCES parent_billing.care_invoices (organization_id, id);
ALTER TABLE parent_billing.care_invoices ADD CONSTRAINT chk_care_invoices_replaces
	CHECK (replaces_invoice_id IS NULL OR replaces_invoice_id <> id);
ALTER TABLE parent_billing.care_invoices DROP CONSTRAINT uq_care_invoices_contract_month;
CREATE UNIQUE INDEX uq_care_invoices_contract_month
	ON parent_billing.care_invoices (organization_id, care_contract_id, billing_month) WHERE replaces_invoice_id IS NULL;
CREATE UNIQUE INDEX uq_care_invoices_reissue
	ON parent_billing.care_invoices (organization_id, replaces_invoice_id) WHERE replaces_invoice_id IS NOT NULL;

CREATE TABLE parent_billing.care_run_letters (
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	payment_due_on DATE NOT NULL,
	notes TEXT NOT NULL DEFAULT '',
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (organization_id, run_id),
	CONSTRAINT fk_care_run_letters_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.care_runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_run_letters_notes CHECK (char_length(notes) <= 4000)
);
COMMENT ON TABLE parent_billing.care_run_letters IS
	'Payment date and free text of the letters of a released monthly run (#4684, ADR 0165); never changes.';

CREATE TABLE parent_billing.care_letters (
	id BIGINT PRIMARY KEY DEFAULT nextval('parent_billing.documents_id_seq'),
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	invoice_id BIGINT NOT NULL,
	kind VARCHAR(16) NOT NULL,
	filename TEXT NOT NULL,
	pdf BYTEA NOT NULL,
	sha256 CHAR(64) GENERATED ALWAYS AS (encode(sha256(pdf), 'hex')) STORED,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_care_letters_organization UNIQUE (organization_id, id),
	CONSTRAINT uq_care_letters_invoice UNIQUE (organization_id, invoice_id, kind),
	CONSTRAINT fk_care_letters_invoice FOREIGN KEY (organization_id, invoice_id)
		REFERENCES parent_billing.care_invoices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_care_letters_run_letters FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.care_run_letters (organization_id, run_id) ON DELETE CASCADE,
	CONSTRAINT chk_care_letters_kind CHECK (kind IN ('invoice', 'cancellation')),
	CONSTRAINT chk_care_letters_pdf CHECK (octet_length(pdf) > 0 AND BTRIM(filename) <> '')
);
CREATE INDEX idx_care_letters_run ON parent_billing.care_letters (organization_id, run_id);
COMMENT ON TABLE parent_billing.care_letters IS
	'PDF of the letter of a monthly invoice or of its Storno in the layout of the Träger (#4684, ADR 0165); rendered once, never changes, goes with its invoice.';

CREATE TABLE parent_billing.care_letter_mailings (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	letter_id BIGINT NOT NULL,
	delivery_id BIGINT NOT NULL,
	recipient_email TEXT NOT NULL,
	requested_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_care_letter_mailings_delivery UNIQUE (organization_id, delivery_id),
	CONSTRAINT fk_care_letter_mailings_letter FOREIGN KEY (organization_id, letter_id)
		REFERENCES parent_billing.care_letters (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_care_letter_mailings_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.care_runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_letter_mailings_recipient CHECK (BTRIM(recipient_email) <> '' AND delivery_id > 0)
);
CREATE INDEX idx_care_letter_mailings_letter ON parent_billing.care_letter_mailings (organization_id, letter_id, id);
CREATE INDEX idx_care_letter_mailings_recipient ON parent_billing.care_letter_mailings (organization_id, LOWER(recipient_email));
COMMENT ON TABLE parent_billing.care_letter_mailings IS
	'Every send of the letter of a monthly invoice or Storno with its outbox entry and address (#4684, ADR 0165); the letter is in the parents inbox once sent.';

CREATE TRIGGER refuse_parent_billing_care_invoice_cancellation_change
	BEFORE UPDATE ON parent_billing.care_invoice_cancellations
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_released_care_run_change();
CREATE TRIGGER refuse_parent_billing_care_run_letters_change
	BEFORE UPDATE ON parent_billing.care_run_letters
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_released_care_run_change();
CREATE TRIGGER refuse_parent_billing_care_letter_change
	BEFORE UPDATE ON parent_billing.care_letters
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();
-- A send never changes either; deleting the account that asked for it
-- clears requested_by_account_id.
CREATE FUNCTION parent_billing.refuse_care_letter_mailing_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.requested_by_account_id IS NULL
		AND to_jsonb(NEW) - 'requested_by_account_id' = to_jsonb(OLD) - 'requested_by_account_id' THEN
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'an issued invoice of the Elternabrechnung never changes (%)', TG_TABLE_NAME
		USING ERRCODE = '55000';
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.refuse_care_letter_mailing_change() FROM PUBLIC;
CREATE TRIGGER refuse_parent_billing_care_letter_mailing_change
	BEFORE UPDATE ON parent_billing.care_letter_mailings
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_care_letter_mailing_change();

ALTER TABLE parent_billing.care_run_events DROP CONSTRAINT chk_care_run_events_action;
ALTER TABLE parent_billing.care_run_events ADD CONSTRAINT chk_care_run_events_action
	CHECK (action IN ('create', 'read', 'release', 'letters', 'mail', 'cancel'));

ALTER TABLE parent_billing.care_invoice_cancellations ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_invoice_cancellations FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_invoice_cancellations ON parent_billing.care_invoice_cancellations FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_run_letters ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_run_letters FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_run_letters ON parent_billing.care_run_letters FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_letters ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_letters FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_letters ON parent_billing.care_letters FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_letter_mailings ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_letter_mailings FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_letter_mailings ON parent_billing.care_letter_mailings FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.care_invoice_cancellations, parent_billing.care_run_letters, parent_billing.care_letters,
	parent_billing.care_letter_mailings FROM PUBLIC;
GRANT SELECT, INSERT ON parent_billing.care_invoice_cancellations, parent_billing.care_run_letters,
	parent_billing.care_letters, parent_billing.care_letter_mailings TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.care_invoice_cancellations_id_seq, parent_billing.care_letter_mailings_id_seq,
	parent_billing.documents_id_seq TO phoenix_tenant;
GRANT ALL ON ALL TABLES IN SCHEMA parent_billing TO phoenix_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA parent_billing TO phoenix_admin;
`

const parentBillingCareLettersDownSQL = `
DROP TABLE IF EXISTS parent_billing.care_letter_mailings, parent_billing.care_letters, parent_billing.care_run_letters,
	parent_billing.care_invoice_cancellations;
DROP FUNCTION IF EXISTS parent_billing.refuse_care_letter_mailing_change();
DELETE FROM parent_billing.care_run_events WHERE action IN ('letters', 'mail', 'cancel');
ALTER TABLE parent_billing.care_run_events DROP CONSTRAINT chk_care_run_events_action;
ALTER TABLE parent_billing.care_run_events ADD CONSTRAINT chk_care_run_events_action
	CHECK (action IN ('create', 'read', 'release'));
DROP INDEX IF EXISTS parent_billing.uq_care_invoices_reissue;
DROP INDEX IF EXISTS parent_billing.uq_care_invoices_contract_month;
DELETE FROM parent_billing.care_invoices WHERE replaces_invoice_id IS NOT NULL;
ALTER TABLE parent_billing.care_invoices DROP CONSTRAINT chk_care_invoices_replaces;
ALTER TABLE parent_billing.care_invoices DROP CONSTRAINT fk_care_invoices_replaces;
ALTER TABLE parent_billing.care_invoices DROP COLUMN replaces_invoice_id;
ALTER TABLE parent_billing.care_invoices DROP COLUMN student_id;
ALTER TABLE parent_billing.care_invoices ADD CONSTRAINT uq_care_invoices_contract_month
	UNIQUE (organization_id, care_contract_id, billing_month);
`

// careLetterErasureAnchor is the statement of 1.15.805, kept by 1.15.808, that deletes the
// monthly invoices of the retired demo schools. The letters, their sends and
// the copies on Delivery's outbox go right before it.
const careLetterErasureAnchor = `					DELETE FROM parent_billing.care_invoices
					WHERE school_id = ANY (hidden_schools);`

// careLetterErasureSQL deletes what the letters of a monthly invoice left on
// Delivery's outbox: the mail with the PDF and the address, its delivery
// result, and the push that announced the letter in the parents inbox. The
// letters and their sends go with the invoice by cascade. A send holds the
// run of its invoice; the erasure takes the runs first, so a send under way
// is seen and a later one finds no letter.
const careLetterErasureSQL = `					-- The letter of a monthly invoice or its Storno travels on
					-- Delivery's outbox with the PDF and the address, and a push
					-- announces it in the parents inbox (#4684, ADR 0165). A send
					-- holds the run of its invoice: take the runs first, so a send
					-- under way is seen before its copies are cleaned up.
					PERFORM 1 FROM parent_billing.care_runs AS run
					WHERE (run.organization_id, run.id) IN (
						SELECT invoice.organization_id, invoice.run_id
						FROM parent_billing.care_invoices AS invoice
						WHERE invoice.school_id = ANY (hidden_schools))
					ORDER BY run.organization_id, run.id
					FOR UPDATE;
					DELETE FROM platform.email_delivery AS delivery
					USING parent_billing.care_letter_mailings AS mailing, parent_billing.care_letters AS letter,
						parent_billing.care_invoices AS invoice
					WHERE invoice.school_id = ANY (hidden_schools)
						AND letter.organization_id = invoice.organization_id
						AND letter.invoice_id = invoice.id
						AND mailing.organization_id = letter.organization_id
						AND mailing.letter_id = letter.id
						AND delivery.outbox_id = mailing.delivery_id;
					DELETE FROM platform.email_outbox AS outbox
					USING parent_billing.care_letter_mailings AS mailing, parent_billing.care_letters AS letter,
						parent_billing.care_invoices AS invoice
					WHERE invoice.school_id = ANY (hidden_schools)
						AND letter.organization_id = invoice.organization_id
						AND letter.invoice_id = invoice.id
						AND mailing.organization_id = letter.organization_id
						AND mailing.letter_id = letter.id
						AND outbox.organization_id = mailing.organization_id
						AND outbox.id = mailing.delivery_id;
					DELETE FROM platform.push_outbox AS push
					USING parent_billing.care_letters AS letter, parent_billing.care_invoices AS invoice
					WHERE invoice.school_id = ANY (hidden_schools)
						AND letter.organization_id = invoice.organization_id
						AND letter.invoice_id = invoice.id
						AND push.organization_id = letter.organization_id
						AND push.related_entity_type = 'parent_billing_document'
						AND push.related_entity_id = letter.id;
`

func parentBillingCareLettersUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareLettersUpSQL); err != nil {
		return fmt.Errorf("create parent billing care letters: %w", err)
	}
	if err := installDemoVisitorErasureOf809(ctx, db); err != nil {
		return err
	}
	return nil
}

// parentBillingCareLettersDown reinstalls the erasure of 1.15.808 before it
// drops the tables that erasure no longer names.
func parentBillingCareLettersDown(ctx context.Context, db *bun.DB) error {
	if err := demoVisitorErasureContractPushUp(ctx, db); err != nil {
		return fmt.Errorf("restore the demo visitor erasure of 1.15.808: %w", err)
	}
	if _, err := db.ExecContext(ctx, parentBillingCareLettersDownSQL); err != nil {
		return fmt.Errorf("drop parent billing care letters: %w", err)
	}
	return nil
}
