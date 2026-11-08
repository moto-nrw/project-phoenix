package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingPaymentTermsVersion = "1.15.813"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingPaymentTermsVersion,
		Description: "Elternabrechnung: Bündelung, Empfänger, Zahlungsziel und Zahlungsweg (#4689)",
		DependsOn: []string{
			parentBillingCareLettersVersion, // the monthly invoices and their letters this one extends
		},
	})
	Migrations.MustRegister(parentBillingPaymentTermsUp, parentBillingPaymentTermsDown)
}

// parentBillingPaymentTermsUpSQL adds who an invoice goes to, how it is paid
// and by when (#4689, spec #4611 user stories 74, 76, 77, ADR 0167):
//
//   - service_terms: per Träger and service the payment method, the payment
//     term in days and the service whose invoice carries this one's
//     positions (the bundling). Without a row the defaults of the code hold.
//     The holiday care has its own run and is never bundled.
//   - family_billing: the deviation of one family, keyed by the mail address
//     of the registration (the contract party): who gets the invoice (the
//     applicant, both parents or another payer) and its own payment method or
//     day of the month the invoice is due. It hangs on the contract it was
//     set from and goes with it.
//   - family_billing_events: the trail of every read and change of a
//     deviation, without names.
//   - run_due_dates: the due date of a calculated run per payment method; a
//     released run never changes them.
//   - care_invoices and documents copy at the release the family, the
//     recipient kind, the second parent, the payer's address (monthly
//     invoices; the holiday documents already copy an address), the payment
//     method, the due date and where the date came from. Invoices issued
//     before stay as they are (ADR 0119 point 7): the columns are NULL there.
const parentBillingPaymentTermsUpSQL = `
CREATE TABLE parent_billing.service_terms (
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	service VARCHAR(16) NOT NULL,
	payment_method VARCHAR(16) NOT NULL,
	due_days SMALLINT NOT NULL,
	bundle_with VARCHAR(16),
	updated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT pk_service_terms PRIMARY KEY (organization_id, service),
	CONSTRAINT chk_service_terms_service CHECK (service IN ('care', 'meal', 'holiday')),
	CONSTRAINT chk_service_terms_method CHECK (payment_method IN ('transfer', 'direct_debit')),
	CONSTRAINT chk_service_terms_due_days CHECK (due_days BETWEEN 0 AND 120),
	CONSTRAINT chk_service_terms_bundle CHECK (bundle_with IS NULL
		OR (bundle_with IN ('care', 'meal') AND bundle_with <> service AND service <> 'holiday'))
);
COMMENT ON TABLE parent_billing.service_terms IS
	'Payment method, payment term and bundling of one service of a Träger (#4689); the holiday care is never bundled.';

CREATE TABLE parent_billing.family_billing (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	family_email TEXT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	recipient_kind VARCHAR(16) NOT NULL,
	second_first_name TEXT NOT NULL DEFAULT '',
	second_last_name TEXT NOT NULL DEFAULT '',
	payer_first_name TEXT NOT NULL DEFAULT '',
	payer_last_name TEXT NOT NULL DEFAULT '',
	payer_email TEXT NOT NULL DEFAULT '',
	payer_street VARCHAR(100) NOT NULL DEFAULT '',
	payer_postal_code VARCHAR(5) NOT NULL DEFAULT '',
	payer_city VARCHAR(100) NOT NULL DEFAULT '',
	payment_method VARCHAR(16),
	due_day SMALLINT,
	updated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_family_billing_organization UNIQUE (organization_id, id),
	CONSTRAINT uq_family_billing_family UNIQUE (organization_id, family_email),
	CONSTRAINT fk_family_billing_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_family_billing_email CHECK (
		family_email = LOWER(BTRIM(family_email)) AND family_email <> '' AND char_length(family_email) <= 320),
	CONSTRAINT chk_family_billing_kind CHECK (recipient_kind IN ('applicant', 'both', 'payer')),
	CONSTRAINT chk_family_billing_both CHECK ((recipient_kind = 'both')
		= (BTRIM(second_first_name) <> '' AND BTRIM(second_last_name) <> '')),
	CONSTRAINT chk_family_billing_payer CHECK ((recipient_kind = 'payer') = (
		BTRIM(payer_first_name) <> '' AND BTRIM(payer_last_name) <> '' AND BTRIM(payer_email) <> ''
		AND BTRIM(payer_street) <> '' AND payer_postal_code ~ '^[0-9]{5}$' AND BTRIM(payer_city) <> '')),
	CONSTRAINT chk_family_billing_lengths CHECK (char_length(second_first_name) <= 100 AND char_length(second_last_name) <= 100
		AND char_length(payer_first_name) <= 100 AND char_length(payer_last_name) <= 100 AND char_length(payer_email) <= 320),
	CONSTRAINT chk_family_billing_method CHECK (payment_method IS NULL OR payment_method IN ('transfer', 'direct_debit')),
	CONSTRAINT chk_family_billing_due_day CHECK (due_day IS NULL OR due_day BETWEEN 1 AND 28)
);
CREATE INDEX idx_family_billing_contract ON parent_billing.family_billing (organization_id, care_contract_id);
COMMENT ON TABLE parent_billing.family_billing IS
	'Deviation of one family (#4689): who gets its invoices and its own payment method or due day; keyed by the mail address of the registration, goes with the contract it was set from.';

CREATE TABLE parent_billing.family_billing_events (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	family_billing_id BIGINT NOT NULL,
	action VARCHAR(16) NOT NULL,
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_family_billing_events_action CHECK (action IN ('read', 'save', 'delete'))
);
CREATE INDEX idx_family_billing_events_family ON parent_billing.family_billing_events (organization_id, family_billing_id, occurred_at);
COMMENT ON TABLE parent_billing.family_billing_events IS
	'Trail of every read and change of a family deviation (#4689): which one, who and when; no names.';

CREATE TABLE parent_billing.run_due_dates (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	care_run_id BIGINT,
	holiday_run_id BIGINT,
	payment_method VARCHAR(16) NOT NULL,
	due_on DATE NOT NULL,
	set_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	set_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_run_due_dates_care_run FOREIGN KEY (organization_id, care_run_id)
		REFERENCES parent_billing.care_runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_run_due_dates_holiday_run FOREIGN KEY (organization_id, holiday_run_id)
		REFERENCES parent_billing.runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_run_due_dates_run CHECK (num_nonnulls(care_run_id, holiday_run_id) = 1),
	CONSTRAINT chk_run_due_dates_method CHECK (payment_method IN ('transfer', 'direct_debit'))
);
CREATE UNIQUE INDEX uq_run_due_dates_care ON parent_billing.run_due_dates (care_run_id, payment_method) WHERE care_run_id IS NOT NULL;
CREATE UNIQUE INDEX uq_run_due_dates_holiday ON parent_billing.run_due_dates (holiday_run_id, payment_method) WHERE holiday_run_id IS NOT NULL;
COMMENT ON TABLE parent_billing.run_due_dates IS
	'Due date of a monthly or holiday run per payment method (#4689); every invoice of the run with that method is due then unless its family deviates.';

-- A due date belongs to a calculated run: once the run is released, its
-- invoices copied the dates and nothing may change them.
CREATE FUNCTION parent_billing.refuse_released_due_date() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
	row_care BIGINT := CASE WHEN TG_OP = 'DELETE' THEN OLD.care_run_id ELSE NEW.care_run_id END;
	row_holiday BIGINT := CASE WHEN TG_OP = 'DELETE' THEN OLD.holiday_run_id ELSE NEW.holiday_run_id END;
	row_organization BIGINT := CASE WHEN TG_OP = 'DELETE' THEN OLD.organization_id ELSE NEW.organization_id END;
BEGIN
	IF TG_OP = 'UPDATE' AND NEW.set_by_account_id IS NULL
		AND to_jsonb(NEW) - 'set_by_account_id' = to_jsonb(OLD) - 'set_by_account_id' THEN
		RETURN NEW;
	END IF;
	IF EXISTS (SELECT 1 FROM parent_billing.care_runs WHERE organization_id = row_organization AND id = row_care AND status = 'released')
		OR EXISTS (SELECT 1 FROM parent_billing.runs WHERE organization_id = row_organization AND id = row_holiday AND status = 'released') THEN
		RAISE EXCEPTION 'the due dates of a released run never change' USING ERRCODE = '55000';
	END IF;
	RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.refuse_released_due_date() FROM PUBLIC;
CREATE TRIGGER refuse_parent_billing_released_due_date
	BEFORE INSERT OR UPDATE OR DELETE ON parent_billing.run_due_dates
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_released_due_date();

ALTER TABLE parent_billing.care_invoices
	ADD COLUMN family_email TEXT,
	ADD COLUMN recipient_kind VARCHAR(16),
	ADD COLUMN recipient_second_name TEXT,
	ADD COLUMN recipient_address_street VARCHAR(100),
	ADD COLUMN recipient_address_postal_code VARCHAR(5),
	ADD COLUMN recipient_address_city VARCHAR(100),
	ADD COLUMN payment_method VARCHAR(16),
	ADD COLUMN due_on DATE,
	ADD COLUMN due_source VARCHAR(8),
	ADD CONSTRAINT chk_care_invoices_recipient_kind CHECK (recipient_kind IS NULL OR recipient_kind IN ('applicant', 'both', 'payer')),
	ADD CONSTRAINT chk_care_invoices_recipient_address CHECK (
		(recipient_address_street IS NULL AND recipient_address_postal_code IS NULL AND recipient_address_city IS NULL)
		OR (BTRIM(recipient_address_street) <> '' AND recipient_address_postal_code ~ '^[0-9]{5}$' AND BTRIM(recipient_address_city) <> '')),
	ADD CONSTRAINT chk_care_invoices_payment CHECK (num_nulls(payment_method, due_on, due_source) IN (0, 3)
		AND (payment_method IS NULL OR payment_method IN ('transfer', 'direct_debit'))
		AND (due_source IS NULL OR due_source IN ('family', 'run', 'term')));

ALTER TABLE parent_billing.documents
	ADD COLUMN family_email TEXT,
	ADD COLUMN recipient_kind VARCHAR(16),
	ADD COLUMN recipient_second_name TEXT,
	ADD COLUMN payment_method VARCHAR(16),
	ADD COLUMN due_on DATE,
	ADD COLUMN due_source VARCHAR(8),
	ADD CONSTRAINT chk_documents_recipient_kind CHECK (recipient_kind IS NULL OR recipient_kind IN ('applicant', 'both', 'payer')),
	ADD CONSTRAINT chk_documents_payment CHECK (num_nulls(payment_method, due_on, due_source) IN (0, 3)
		AND (payment_method IS NULL OR payment_method IN ('transfer', 'direct_debit'))
		AND (due_source IS NULL OR due_source IN ('family', 'run', 'term')));

ALTER TABLE parent_billing.service_terms ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.service_terms FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_service_terms ON parent_billing.service_terms FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.family_billing ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.family_billing FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_family_billing ON parent_billing.family_billing FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.family_billing_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.family_billing_events FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_family_billing_events ON parent_billing.family_billing_events FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.run_due_dates ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.run_due_dates FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_run_due_dates ON parent_billing.run_due_dates FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.service_terms, parent_billing.family_billing, parent_billing.family_billing_events,
	parent_billing.run_due_dates FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON parent_billing.service_terms TO phoenix_tenant;
GRANT SELECT, INSERT, UPDATE, DELETE ON parent_billing.family_billing, parent_billing.run_due_dates TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.family_billing_events TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.family_billing_id_seq, parent_billing.family_billing_events_id_seq,
	parent_billing.run_due_dates_id_seq TO phoenix_tenant;
`

const parentBillingPaymentTermsDownSQL = `
ALTER TABLE parent_billing.documents
	DROP CONSTRAINT IF EXISTS chk_documents_payment,
	DROP CONSTRAINT IF EXISTS chk_documents_recipient_kind,
	DROP COLUMN IF EXISTS due_source,
	DROP COLUMN IF EXISTS due_on,
	DROP COLUMN IF EXISTS payment_method,
	DROP COLUMN IF EXISTS recipient_second_name,
	DROP COLUMN IF EXISTS recipient_kind,
	DROP COLUMN IF EXISTS family_email;
ALTER TABLE parent_billing.care_invoices
	DROP CONSTRAINT IF EXISTS chk_care_invoices_payment,
	DROP CONSTRAINT IF EXISTS chk_care_invoices_recipient_address,
	DROP CONSTRAINT IF EXISTS chk_care_invoices_recipient_kind,
	DROP COLUMN IF EXISTS due_source,
	DROP COLUMN IF EXISTS due_on,
	DROP COLUMN IF EXISTS payment_method,
	DROP COLUMN IF EXISTS recipient_address_city,
	DROP COLUMN IF EXISTS recipient_address_postal_code,
	DROP COLUMN IF EXISTS recipient_address_street,
	DROP COLUMN IF EXISTS recipient_second_name,
	DROP COLUMN IF EXISTS recipient_kind,
	DROP COLUMN IF EXISTS family_email;
DROP TABLE IF EXISTS parent_billing.run_due_dates;
DROP FUNCTION IF EXISTS parent_billing.refuse_released_due_date();
DROP TABLE IF EXISTS parent_billing.family_billing_events;
DROP TABLE IF EXISTS parent_billing.family_billing;
DROP TABLE IF EXISTS parent_billing.service_terms;
`

func parentBillingPaymentTermsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingPaymentTermsUpSQL); err != nil {
		return fmt.Errorf("create parent billing payment terms: %w", err)
	}
	return nil
}

func parentBillingPaymentTermsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingPaymentTermsDownSQL); err != nil {
		return fmt.Errorf("drop parent billing payment terms: %w", err)
	}
	return nil
}
