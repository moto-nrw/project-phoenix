package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingOnlineSEPAMandatesVersion = "1.15.842"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingOnlineSEPAMandatesVersion,
		Description: "Elternabrechnung: Nachweis des SEPA-Mandats, das Eltern in der Eltern-App erteilen (#4696)",
		DependsOn: []string{
			accountCredentialGenerationBindingVersion, // the newest migration on development
			parentBillingSEPAMandatesVersion,          // the mandates the proof belongs to
		},
	})
	Migrations.MustRegister(parentBillingOnlineSEPAMandatesUp, parentBillingOnlineSEPAMandatesDown)
}

// parentBillingOnlineSEPAMandatesUpSQL adds the proof of a SEPA mandate the
// parents give in the parents portal (#4696, ADR 0171 addendum 2026-11-09).
// The creditor bears the burden of proof for a mandate in text form, so the
// proof keeps the version and SHA-256 of the mandate text the parents
// confirmed, the channel, the account and the time, their confirmation to
// hold the account, and what the text named besides the mandate (creditor
// name and address, debtor address), so the text can be rendered again and
// checked. The account holder and the IBAN stay sealed in the mandate.
//
// A proof is written once: the trigger refuses every change but the
// clearing of the account on its erasure, and the request role may only
// read and insert. It goes with its mandate and contract.
const parentBillingOnlineSEPAMandatesUpSQL = `
CREATE TABLE parent_billing.sepa_mandate_consents (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	mandate_id BIGINT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	channel VARCHAR(16) NOT NULL,
	text_version VARCHAR(32) NOT NULL,
	text_sha256 CHAR(64) NOT NULL,
	holder_confirmed BOOLEAN NOT NULL,
	creditor_name TEXT NOT NULL,
	creditor_address TEXT NOT NULL,
	debtor_street TEXT NOT NULL,
	debtor_postal_code CHAR(5) NOT NULL,
	debtor_city TEXT NOT NULL,
	confirmed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_sepa_mandate_consents_mandate UNIQUE (organization_id, mandate_id),
	CONSTRAINT fk_sepa_mandate_consents_mandate FOREIGN KEY (organization_id, mandate_id)
		REFERENCES parent_billing.sepa_mandates (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_sepa_mandate_consents_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_sepa_mandate_consents_channel CHECK (channel IN ('parent_account')),
	CONSTRAINT chk_sepa_mandate_consents_holder CHECK (holder_confirmed),
	CONSTRAINT chk_sepa_mandate_consents_checksum CHECK (text_sha256 ~ '^[0-9a-f]{64}$'),
	CONSTRAINT chk_sepa_mandate_consents_text CHECK (text_version <> '' AND creditor_name <> '' AND creditor_address <> ''
		AND debtor_street <> '' AND debtor_city <> '' AND debtor_postal_code ~ '^[0-9]{5}$' AND char_length(debtor_street) <= 120 AND char_length(debtor_city) <= 80
		AND char_length(creditor_name) <= 300 AND char_length(creditor_address) <= 1000)
);
COMMENT ON TABLE parent_billing.sepa_mandate_consents IS
	'Proof of a SEPA mandate the parents gave in the parents portal (#4696): version and SHA-256 of the confirmed text, channel, account, time, holder confirmation and what the text named. Never changed.';

CREATE FUNCTION parent_billing.refuse_sepa_mandate_consent_rewrite() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	-- Account erasure may only clear the account; every other column stays.
	IF (to_jsonb(NEW) - 'account_id') IS DISTINCT FROM (to_jsonb(OLD) - 'account_id')
		OR (NEW.account_id IS NOT NULL AND NEW.account_id IS DISTINCT FROM OLD.account_id) THEN
		RAISE EXCEPTION 'the proof of a SEPA mandate never changes' USING ERRCODE = '55000';
	END IF;
	RETURN NEW;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.refuse_sepa_mandate_consent_rewrite() FROM PUBLIC;
CREATE TRIGGER refuse_parent_billing_sepa_mandate_consent_rewrite
	BEFORE UPDATE ON parent_billing.sepa_mandate_consents
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_sepa_mandate_consent_rewrite();

ALTER TABLE parent_billing.sepa_mandate_consents ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.sepa_mandate_consents FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_sepa_mandate_consents ON parent_billing.sepa_mandate_consents FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.sepa_mandate_consents FROM PUBLIC;
GRANT SELECT, INSERT ON parent_billing.sepa_mandate_consents TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.sepa_mandate_consents_id_seq TO phoenix_tenant;
`

const parentBillingOnlineSEPAMandatesDownSQL = `
DROP TABLE IF EXISTS parent_billing.sepa_mandate_consents;
DROP FUNCTION IF EXISTS parent_billing.refuse_sepa_mandate_consent_rewrite();
`

func parentBillingOnlineSEPAMandatesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingOnlineSEPAMandatesUpSQL); err != nil {
		return fmt.Errorf("create parent billing SEPA mandate consents: %w", err)
	}
	return nil
}

func parentBillingOnlineSEPAMandatesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingOnlineSEPAMandatesDownSQL); err != nil {
		return fmt.Errorf("drop parent billing SEPA mandate consents: %w", err)
	}
	return nil
}
