package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingSEPAMandatesVersion = "1.15.821"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingSEPAMandatesVersion,
		Description: "Elternabrechnung: SEPA-Mandate, Gläubiger-ID und Vorabankündigung (#4695)",
		DependsOn: []string{
			parentBillingPaymentTermsVersion, // the payment method and due date the invoices copy
		},
	})
	Migrations.MustRegister(parentBillingSEPAMandatesUp, parentBillingSEPAMandatesDown)
}

// parentBillingSEPAMandatesUpSQL adds the SEPA direct debit of the families
// (#4695, spec #4611 user stories 82, 84, 85, ADR 0171):
//
//   - sepa_creditors: per Träger the creditor identifier, the days the
//     pre-notification goes out before the debit and what an invoice of a
//     family that pays by direct debit without a valid mandate does: it is
//     paid by transfer with a hint in the review (the default), or the
//     release refuses it.
//   - sepa_mandates: the mandate of the payer of a family, keyed like the
//     family deviation by the mail address of the contract party and hung on
//     the contract it was given for. The IBAN is sealed with the field cipher
//     (AES-256-GCM, bound to the Träger and the reference); only its mask is
//     readable. At most one mandate of a family is in force; a revoked one
//     stays as history and never changes again.
//   - sepa_mandate_events: the trail of every read and change of a mandate,
//     without names or bank data. Only ever appended.
//   - care_invoices and documents copy at the release the mandate an invoice
//     is collected with (reference, signing date, creditor identifier,
//     masked IBAN, sequence type) and why a direct debit fell back to a
//     transfer. Invoices issued before keep NULL (ADR 0119 point 7).
const parentBillingSEPAMandatesUpSQL = `
CREATE TABLE parent_billing.sepa_creditors (
	organization_id BIGINT PRIMARY KEY REFERENCES platform.organizations(id) ON DELETE CASCADE,
	creditor_id VARCHAR(35) NOT NULL DEFAULT '',
	prenotification_days SMALLINT NOT NULL DEFAULT 14,
	missing_mandate VARCHAR(16) NOT NULL DEFAULT 'transfer',
	updated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_sepa_creditors_id CHECK (creditor_id = '' OR creditor_id ~ '^[A-Z]{2}[0-9]{2}[A-Z0-9]{3}[A-Z0-9]{1,28}$'),
	CONSTRAINT chk_sepa_creditors_days CHECK (prenotification_days BETWEEN 1 AND 30),
	CONSTRAINT chk_sepa_creditors_missing CHECK (missing_mandate IN ('transfer', 'block'))
);
COMMENT ON TABLE parent_billing.sepa_creditors IS
	'SEPA creditor identifier of a Träger, its pre-notification days and what a direct debit without a valid mandate does (#4695).';

CREATE TABLE parent_billing.sepa_mandates (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	family_email TEXT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	reference VARCHAR(35) NOT NULL,
	signed_on DATE NOT NULL,
	kind VARCHAR(16) NOT NULL,
	sequence VARCHAR(16) NOT NULL,
	account_holder VARCHAR(70) NOT NULL,
	iban_sealed BYTEA NOT NULL,
	iban_masked VARCHAR(48) NOT NULL,
	bic VARCHAR(11) NOT NULL DEFAULT '',
	source VARCHAR(16) NOT NULL,
	revoked_on DATE,
	revoked_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	revoked_at TIMESTAMPTZ,
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_sepa_mandates_organization UNIQUE (organization_id, id),
	CONSTRAINT uq_sepa_mandates_reference UNIQUE (organization_id, reference),
	CONSTRAINT fk_sepa_mandates_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_sepa_mandates_email CHECK (
		family_email = LOWER(BTRIM(family_email)) AND family_email <> '' AND char_length(family_email) <= 320),
	CONSTRAINT chk_sepa_mandates_reference CHECK (reference ~ '^[A-Za-z0-9+?/:().,'' -]{1,35}$'
		AND reference NOT LIKE '/%' AND position('//' IN reference) = 0),
	CONSTRAINT chk_sepa_mandates_kind CHECK (kind IN ('recurring', 'one_off')),
	CONSTRAINT chk_sepa_mandates_sequence CHECK (sequence IN ('first', 'recurring') AND (kind = 'recurring' OR sequence = 'first')),
	CONSTRAINT chk_sepa_mandates_holder CHECK (BTRIM(account_holder) <> ''),
	CONSTRAINT chk_sepa_mandates_bic CHECK (bic = '' OR bic ~ '^[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?$'),
	CONSTRAINT chk_sepa_mandates_source CHECK (source IN ('office', 'import', 'parent')),
	CONSTRAINT chk_sepa_mandates_revoked CHECK (num_nulls(revoked_on, revoked_at) IN (0, 2)
		AND (revoked_on IS NULL OR revoked_on >= signed_on))
);
CREATE UNIQUE INDEX uq_sepa_mandates_in_force ON parent_billing.sepa_mandates (organization_id, family_email)
	WHERE revoked_on IS NULL;
CREATE INDEX idx_sepa_mandates_contract ON parent_billing.sepa_mandates (organization_id, care_contract_id);
COMMENT ON TABLE parent_billing.sepa_mandates IS
	'SEPA mandate of the payer of a family (#4695); the IBAN is sealed, a revoked mandate stays as history and never changes.';
COMMENT ON COLUMN parent_billing.sepa_mandates.iban_sealed IS
	'IBAN sealed with the field cipher (AES-256-GCM), bound to the Träger and the reference.';

-- A mandate keeps its family, contract, reference and signing date; a
-- revoked mandate keeps its history; account erasure may clear actor references.
-- A delete stays possible,
-- so the erasure of a contract or a Träger cascades.
CREATE FUNCTION parent_billing.refuse_sepa_mandate_rewrite() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	-- Account erasure may only clear actor references. Compare every other
	-- column so even a combined actor/data update cannot rewrite history.
	IF OLD.revoked_on IS NOT NULL AND (
		(to_jsonb(NEW) - ARRAY['created_by_account_id', 'updated_by_account_id', 'revoked_by_account_id'])
			IS DISTINCT FROM
		(to_jsonb(OLD) - ARRAY['created_by_account_id', 'updated_by_account_id', 'revoked_by_account_id'])
		OR (NEW.created_by_account_id IS NOT NULL AND NEW.created_by_account_id IS DISTINCT FROM OLD.created_by_account_id)
		OR (NEW.updated_by_account_id IS NOT NULL AND NEW.updated_by_account_id IS DISTINCT FROM OLD.updated_by_account_id)
		OR (NEW.revoked_by_account_id IS NOT NULL AND NEW.revoked_by_account_id IS DISTINCT FROM OLD.revoked_by_account_id)
	) THEN
		RAISE EXCEPTION 'a revoked SEPA mandate never changes' USING ERRCODE = '55000';
	END IF;
	IF NEW.organization_id <> OLD.organization_id OR NEW.family_email <> OLD.family_email
		OR NEW.care_contract_id <> OLD.care_contract_id OR NEW.reference <> OLD.reference
		OR NEW.signed_on <> OLD.signed_on OR NEW.source <> OLD.source OR NEW.created_at <> OLD.created_at THEN
		RAISE EXCEPTION 'a SEPA mandate keeps its family, reference and signing date' USING ERRCODE = '55000';
	END IF;
	RETURN NEW;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.refuse_sepa_mandate_rewrite() FROM PUBLIC;
CREATE TRIGGER refuse_parent_billing_sepa_mandate_rewrite
	BEFORE UPDATE ON parent_billing.sepa_mandates
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_sepa_mandate_rewrite();

CREATE TABLE parent_billing.sepa_mandate_events (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	mandate_id BIGINT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	action VARCHAR(16) NOT NULL,
	source VARCHAR(16) NOT NULL,
	fields TEXT[] NOT NULL DEFAULT '{}',
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_sepa_mandate_events_mandate FOREIGN KEY (organization_id, mandate_id)
		REFERENCES parent_billing.sepa_mandates (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_sepa_mandate_events_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_sepa_mandate_events_action CHECK (action IN ('create', 'update', 'revoke', 'read', 'reveal')),
	CONSTRAINT chk_sepa_mandate_events_source CHECK (source IN ('office', 'import', 'parent', 'run'))
);
CREATE INDEX idx_sepa_mandate_events_mandate ON parent_billing.sepa_mandate_events (organization_id, mandate_id, occurred_at);
COMMENT ON TABLE parent_billing.sepa_mandate_events IS
	'Trail of every read and change of a SEPA mandate (#4695): which one, what, who and when; no names or bank data, only appended.';

ALTER TABLE parent_billing.care_invoices
	ADD COLUMN mandate_id BIGINT,
	ADD COLUMN mandate_reference VARCHAR(35),
	ADD COLUMN mandate_signed_on DATE,
	ADD COLUMN creditor_id VARCHAR(35),
	ADD COLUMN debtor_iban_masked VARCHAR(48),
	ADD COLUMN sequence_type VARCHAR(4),
	ADD COLUMN debit_gap VARCHAR(16),
	ADD CONSTRAINT chk_care_invoices_mandate CHECK (
		num_nulls(mandate_id, mandate_reference, mandate_signed_on, creditor_id, debtor_iban_masked, sequence_type) IN (0, 6)
		AND (mandate_reference IS NULL OR payment_method = 'direct_debit')
		AND (sequence_type IS NULL OR sequence_type IN ('FRST', 'RCUR', 'OOFF'))
		AND (debit_gap IS NULL OR (debit_gap IN ('mandate_missing', 'creditor_missing') AND mandate_reference IS NULL)));

ALTER TABLE parent_billing.documents
	ADD COLUMN mandate_id BIGINT,
	ADD COLUMN mandate_reference VARCHAR(35),
	ADD COLUMN mandate_signed_on DATE,
	ADD COLUMN creditor_id VARCHAR(35),
	ADD COLUMN debtor_iban_masked VARCHAR(48),
	ADD COLUMN sequence_type VARCHAR(4),
	ADD COLUMN debit_gap VARCHAR(16),
	ADD CONSTRAINT chk_documents_mandate CHECK (
		num_nulls(mandate_id, mandate_reference, mandate_signed_on, creditor_id, debtor_iban_masked, sequence_type) IN (0, 6)
		AND (mandate_reference IS NULL OR payment_method = 'direct_debit')
		AND (sequence_type IS NULL OR sequence_type IN ('FRST', 'RCUR', 'OOFF'))
		AND (debit_gap IS NULL OR (debit_gap IN ('mandate_missing', 'creditor_missing') AND mandate_reference IS NULL)));

ALTER TABLE parent_billing.sepa_creditors ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.sepa_creditors FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_sepa_creditors ON parent_billing.sepa_creditors FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.sepa_mandates ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.sepa_mandates FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_sepa_mandates ON parent_billing.sepa_mandates FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.sepa_mandate_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.sepa_mandate_events FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_sepa_mandate_events ON parent_billing.sepa_mandate_events FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.sepa_creditors, parent_billing.sepa_mandates, parent_billing.sepa_mandate_events FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON parent_billing.sepa_creditors, parent_billing.sepa_mandates TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.sepa_mandate_events TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.sepa_mandates_id_seq, parent_billing.sepa_mandate_events_id_seq TO phoenix_tenant;
`

const parentBillingSEPAMandatesDownSQL = `
ALTER TABLE parent_billing.documents
	DROP CONSTRAINT IF EXISTS chk_documents_mandate,
	DROP COLUMN IF EXISTS debit_gap,
	DROP COLUMN IF EXISTS sequence_type,
	DROP COLUMN IF EXISTS debtor_iban_masked,
	DROP COLUMN IF EXISTS creditor_id,
	DROP COLUMN IF EXISTS mandate_signed_on,
	DROP COLUMN IF EXISTS mandate_reference,
	DROP COLUMN IF EXISTS mandate_id;
ALTER TABLE parent_billing.care_invoices
	DROP CONSTRAINT IF EXISTS chk_care_invoices_mandate,
	DROP COLUMN IF EXISTS debit_gap,
	DROP COLUMN IF EXISTS sequence_type,
	DROP COLUMN IF EXISTS debtor_iban_masked,
	DROP COLUMN IF EXISTS creditor_id,
	DROP COLUMN IF EXISTS mandate_signed_on,
	DROP COLUMN IF EXISTS mandate_reference,
	DROP COLUMN IF EXISTS mandate_id;
DROP TABLE IF EXISTS parent_billing.sepa_mandate_events;
DROP TABLE IF EXISTS parent_billing.sepa_mandates;
DROP FUNCTION IF EXISTS parent_billing.refuse_sepa_mandate_rewrite();
DROP TABLE IF EXISTS parent_billing.sepa_creditors;
`

func parentBillingSEPAMandatesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingSEPAMandatesUpSQL); err != nil {
		return fmt.Errorf("create parent billing SEPA mandates: %w", err)
	}
	return nil
}

func parentBillingSEPAMandatesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingSEPAMandatesDownSQL); err != nil {
		return fmt.Errorf("drop parent billing SEPA mandates: %w", err)
	}
	return nil
}
