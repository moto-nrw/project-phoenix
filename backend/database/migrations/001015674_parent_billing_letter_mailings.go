package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingLetterMailingsVersion = "1.15.674"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingLetterMailingsVersion,
		Description: "Elternabrechnung: Bestätigungsschreiben per Mail senden, einzeln und als Serie (#4670)",
		DependsOn: []string{
			parentBillingLettersVersion, // the stored letters a mail carries
		},
	})
	Migrations.MustRegister(parentBillingLetterMailingsUp, parentBillingLetterMailingsDown)
}

// parentBillingLetterMailingsUpSQL adds the sends of the confirmation
// letters of the Elternabrechnung (parent-billing, #4670, ADR 0123):
//
//   - parent_billing.document_mailings: every send of the letter of an
//     issued invoice: the entry on Delivery's outbox, the address it went
//     to, the account that sent it and when. The state of the mail stays
//     with Delivery; the send stays here after Delivery forgets the mail.
//
// Like the letter, a send never changes: the request role may only read and
// add them. They go with their letter, so with the invoice and the Träger.
// The outbox entry has no foreign key: it belongs to Delivery, which removes
// ended mails after its retention period.
const parentBillingLetterMailingsUpSQL = `
CREATE TABLE parent_billing.document_mailings (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	document_id BIGINT NOT NULL,
	delivery_id BIGINT NOT NULL,
	recipient_email TEXT NOT NULL,
	requested_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_document_mailings_delivery UNIQUE (organization_id, delivery_id),
	CONSTRAINT fk_document_mailings_letter FOREIGN KEY (organization_id, document_id)
		REFERENCES parent_billing.document_letters (organization_id, document_id) ON DELETE CASCADE,
	CONSTRAINT fk_document_mailings_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_document_mailings_recipient CHECK (BTRIM(recipient_email) <> '' AND delivery_id > 0)
);

CREATE INDEX idx_document_mailings_run ON parent_billing.document_mailings (organization_id, run_id, document_id);

COMMENT ON TABLE parent_billing.document_mailings IS
	'Every send of the confirmation letter of an issued invoice by mail (#4670, ADR 0123): outbox entry, address, account and time; never changes, goes with its letter.';

ALTER TABLE parent_billing.document_mailings ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.document_mailings FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_document_mailings ON parent_billing.document_mailings FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.document_mailings FROM PUBLIC;
GRANT SELECT, INSERT ON parent_billing.document_mailings TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.document_mailings_id_seq TO phoenix_tenant;
GRANT ALL ON parent_billing.document_mailings TO phoenix_admin;
GRANT ALL ON SEQUENCE parent_billing.document_mailings_id_seq TO phoenix_admin;
`

const parentBillingLetterMailingsDownSQL = `
DROP TABLE IF EXISTS parent_billing.document_mailings;
`

func parentBillingLetterMailingsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingLetterMailingsUpSQL); err != nil {
		return fmt.Errorf("create parent billing letter mailings: %w", err)
	}
	return nil
}

func parentBillingLetterMailingsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingLetterMailingsDownSQL); err != nil {
		return fmt.Errorf("drop parent billing letter mailings: %w", err)
	}
	return nil
}
