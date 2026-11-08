package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const parentBillingCareAmendmentsVersion = "1.15.820"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCareAmendmentsVersion,
		Description: "Elternabrechnung: Rechnung in der Prüfphase ändern (Betrag, Empfänger, Anschrift, Positionen) mit angefügter Historie (#6197)",
		DependsOn: []string{
			parentBillingCareReviewVersion,     // the review whose invoices change
			parentBillingCostShareScopeVersion, // the last parent billing migration before this one
			mealBillingMonthsVersion,           // the erasure of 1.15.811 this one extends
		},
	})
	Migrations.MustRegister(parentBillingCareAmendmentsUp, parentBillingCareAmendmentsDown)
}

// parentBillingCareAmendmentsUpSQL keeps the changes of single invoices in
// the review of a monthly run (#6197, ADR 0119 addendum 2026-11-08
// "Änderungen je Rechnung"):
//
//   - care_run_amendments: the entries of a calculated run, only appended.
//     A change names its invoice (contract and service), its kind, the old
//     and the new value, the correction line it adds, the reason, the
//     account and the time. A revocation is another entry naming the change
//     of the run or the carried correction it takes back. The request role
//     reads and inserts; a trigger refuses every update and an entry of a
//     run that is no longer calculated, for every role.
//   - care_invoice_corrections: what a release applied, copied onto the
//     issued invoice and never changed: the correction lines with label and
//     amount, the recipient and address changes, and the number of the
//     cancelled invoice a carried change came from. The next invoice of the
//     month after a Storno takes them over as proposals (ADR 0119 addendum
//     2026-11-04).
const parentBillingCareAmendmentsUpSQL = `
CREATE TABLE parent_billing.care_run_amendments (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	service TEXT NOT NULL,
	kind TEXT NOT NULL,
	label TEXT NOT NULL DEFAULT '',
	amount_cents BIGINT NOT NULL DEFAULT 0,
	position_key TEXT NOT NULL DEFAULT '',
	before_value JSONB NOT NULL DEFAULT '{}'::jsonb,
	after_value JSONB NOT NULL DEFAULT '{}'::jsonb,
	reason TEXT NOT NULL,
	revokes_amendment_id BIGINT,
	revokes_correction_id BIGINT,
	account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	amended_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_care_run_amendments_organization UNIQUE (organization_id, id),
	CONSTRAINT fk_care_run_amendments_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.care_runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_care_run_amendments_revokes FOREIGN KEY (organization_id, revokes_amendment_id)
		REFERENCES parent_billing.care_run_amendments (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_run_amendments_service CHECK (service IN ('care', 'meal')),
	CONSTRAINT chk_care_run_amendments_kind CHECK (kind IN ('amount', 'add_position', 'remove_position', 'recipient', 'address', 'revoke')),
	CONSTRAINT chk_care_run_amendments_reason CHECK (BTRIM(reason) <> '' AND char_length(reason) <= 500),
	CONSTRAINT chk_care_run_amendments_label CHECK (char_length(label) <= 200),
	CONSTRAINT chk_care_run_amendments_revoke CHECK ((kind = 'revoke') = (num_nonnulls(revokes_amendment_id, revokes_correction_id) = 1)),
	CONSTRAINT chk_care_run_amendments_position CHECK ((kind = 'remove_position') = (position_key <> '')),
	CONSTRAINT chk_care_run_amendments_line CHECK (kind IN ('amount', 'add_position', 'remove_position') OR amount_cents = 0),
	CONSTRAINT chk_care_run_amendments_added CHECK (kind <> 'add_position' OR (amount_cents > 0 AND BTRIM(label) <> '')),
	CONSTRAINT chk_care_run_amendments_values CHECK (jsonb_typeof(before_value) = 'object' AND jsonb_typeof(after_value) = 'object')
);
CREATE INDEX idx_care_run_amendments_run ON parent_billing.care_run_amendments (organization_id, run_id, id);
CREATE INDEX idx_care_run_amendments_contract ON parent_billing.care_run_amendments (organization_id, care_contract_id);
COMMENT ON TABLE parent_billing.care_run_amendments IS
	'Changes of single invoices in the review of a monthly run (#6197); only appended, a revocation is another entry.';

-- An entry is never changed, and a run takes entries only while it is
-- calculated: after the release a correction is a Storno (#4690).
CREATE FUNCTION parent_billing.refuse_care_run_amendment() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF TG_OP = 'UPDATE' THEN
		RAISE EXCEPTION 'a change of an invoice in the review is only ever appended (%)', TG_TABLE_NAME
			USING ERRCODE = '55000';
	END IF;
	IF NOT EXISTS (SELECT 1 FROM parent_billing.care_runs AS run
		WHERE run.organization_id = NEW.organization_id AND run.id = NEW.run_id AND run.status = 'calculated') THEN
		RAISE EXCEPTION 'an invoice of a released monthly run never changes (%)', TG_TABLE_NAME
			USING ERRCODE = '55000';
	END IF;
	RETURN NEW;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.refuse_care_run_amendment() FROM PUBLIC;
CREATE TRIGGER refuse_parent_billing_care_run_amendment
	BEFORE INSERT OR UPDATE ON parent_billing.care_run_amendments
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_care_run_amendment();

CREATE TABLE parent_billing.care_invoice_corrections (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	invoice_id BIGINT NOT NULL,
	position SMALLINT NOT NULL,
	kind TEXT NOT NULL,
	label TEXT NOT NULL DEFAULT '',
	amount_cents BIGINT NOT NULL DEFAULT 0,
	position_key TEXT NOT NULL DEFAULT '',
	before_value JSONB NOT NULL DEFAULT '{}'::jsonb,
	after_value JSONB NOT NULL DEFAULT '{}'::jsonb,
	reason TEXT NOT NULL,
	account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	amended_at TIMESTAMPTZ NOT NULL,
	carried_from_number TEXT NOT NULL DEFAULT '',
	-- The entry of the run the release applied; empty for a carried one.
	-- No foreign key: an issued row never changes, not even to NULL.
	amendment_id BIGINT,
	CONSTRAINT fk_care_invoice_corrections_invoice FOREIGN KEY (organization_id, invoice_id)
		REFERENCES parent_billing.care_invoices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_care_invoice_corrections_position UNIQUE (invoice_id, position),
	CONSTRAINT chk_care_invoice_corrections_kind CHECK (kind IN ('amount', 'add_position', 'remove_position', 'recipient', 'address')),
	CONSTRAINT chk_care_invoice_corrections_reason CHECK (BTRIM(reason) <> '' AND char_length(reason) <= 500),
	CONSTRAINT chk_care_invoice_corrections_line CHECK (kind IN ('amount', 'add_position', 'remove_position') OR amount_cents = 0),
	CONSTRAINT chk_care_invoice_corrections_origin CHECK ((carried_from_number = '') = (amendment_id IS NOT NULL))
);
COMMENT ON TABLE parent_billing.care_invoice_corrections IS
	'Changes the review applied to an issued monthly invoice (#6197): correction lines, recipient and address; never changes.';
CREATE TRIGGER refuse_parent_billing_care_invoice_correction_change
	BEFORE UPDATE ON parent_billing.care_invoice_corrections
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();

ALTER TABLE parent_billing.care_run_amendments
	ADD CONSTRAINT fk_care_run_amendments_correction FOREIGN KEY (revokes_correction_id)
		REFERENCES parent_billing.care_invoice_corrections (id) ON DELETE CASCADE;

ALTER TABLE parent_billing.care_run_amendments ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_run_amendments FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_run_amendments ON parent_billing.care_run_amendments FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_invoice_corrections ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_invoice_corrections FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_invoice_corrections ON parent_billing.care_invoice_corrections FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.care_run_amendments, parent_billing.care_invoice_corrections FROM PUBLIC;
GRANT SELECT, INSERT ON parent_billing.care_run_amendments, parent_billing.care_invoice_corrections TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.care_run_amendments_id_seq, parent_billing.care_invoice_corrections_id_seq TO phoenix_tenant;
GRANT ALL ON ALL TABLES IN SCHEMA parent_billing TO phoenix_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA parent_billing TO phoenix_admin;
`

const parentBillingCareAmendmentsDownSQL = `
DROP TABLE IF EXISTS parent_billing.care_run_amendments;
DROP TABLE IF EXISTS parent_billing.care_invoice_corrections;
DROP FUNCTION IF EXISTS parent_billing.refuse_care_run_amendment();
`

// careAmendmentsErasureAnchor is the statement of the erasure of 1.15.811
// the deletion of the changes goes before: the contracts still exist there.
const careAmendmentsErasureAnchor = "\t\t\t\t\tDELETE FROM care_contract.contract_events AS event\n"

// careAmendmentsErasureStatement deletes the changes of the review that name
// a contract of a retired demo school (#6197). A recipient or address change
// copies a name, a mail address and a postal address; an amount or position
// change names the contract with its reason, which may name the family. The
// run stays, so nothing cascades: the entries go with their contract. The
// corrections an issued invoice copied go with the invoice by cascade.
const careAmendmentsErasureStatement = "\t\t\t\t\t-- A change of an invoice in the review (#6197) names the\n" +
	"\t\t\t\t\t-- contract and may copy a recipient; it goes with the contract.\n" +
	"\t\t\t\t\tDELETE FROM parent_billing.care_run_amendments AS amendment\n" +
	"\t\t\t\t\tUSING care_contract.contracts AS contract\n" +
	"\t\t\t\t\tWHERE contract.school_id = ANY (hidden_schools)\n" +
	"\t\t\t\t\t\tAND amendment.organization_id = contract.organization_id\n" +
	"\t\t\t\t\t\tAND amendment.care_contract_id = contract.id;\n"

// demoVisitorErasureCareAmendmentsSQL is platform.erase_closed_demo_visitors
// of 1.15.811 with the deletion of the changes of the review; the rest is
// unchanged.
func demoVisitorErasureCareAmendmentsSQL() (string, error) {
	if count := strings.Count(demoVisitorErasureMealBillingSQL, careAmendmentsErasureAnchor); count != 1 {
		return "", fmt.Errorf("the erasure of 1.15.811 has the anchor %d times, want once", count)
	}
	return strings.Replace(demoVisitorErasureMealBillingSQL, careAmendmentsErasureAnchor,
		careAmendmentsErasureStatement+careAmendmentsErasureAnchor, 1), nil
}

func parentBillingCareAmendmentsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareAmendmentsUpSQL); err != nil {
		return fmt.Errorf("add the changes of the review: %w", err)
	}
	erasure, err := demoVisitorErasureCareAmendmentsSQL()
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, erasure); err != nil {
		return fmt.Errorf("erase the changes of the review of retired demo schools: %w", err)
	}
	return nil
}

func parentBillingCareAmendmentsDown(ctx context.Context, db *bun.DB) error {
	if err := installMealBillingErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, parentBillingCareAmendmentsDownSQL); err != nil {
		return fmt.Errorf("drop the changes of the review: %w", err)
	}
	return nil
}
