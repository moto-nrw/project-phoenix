package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingCareReductionsVersion = "1.15.807"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCareReductionsVersion,
		Description: "Elternabrechnung: Einkommensstufen, Geschwister, Ermäßigung und Höchstbetrag im Monatsbeitrag (#4685)",
		DependsOn: []string{
			demoVisitorErasureCareInvoicesVersion, // the last migration of the monthly billing before this one
			parentBillingCareMonthsVersion,        // the invoices a reduction belongs to
		},
	})
	Migrations.MustRegister(parentBillingCareReductionsUp, parentBillingCareReductionsDown)
}

// parentBillingCareReductionsUpSQL adds the rule chain of the monthly care
// contribution (#4685, ADR 0164) on the rules the holiday care already has:
//
//   - parent_billing.contribution_rules gets the service "care" with four
//     kinds in the fixed order of ADR 0088 section 4: income_level (a level
//     of the Träger's statute), sibling (from the n-th child on a share, a
//     fixed discount or a fixed end price), single_parent (the reduction for
//     single parents) and cap (the most a family pays per month). A care
//     rule has a name the invoice shows, the schools it applies to, the
//     modules it applies to (none: all) and, for a sibling rule, the schools
//     whose children count (none: the schools of the rule). Care rules may
//     run side by side for different schools, so their overlap is checked
//     by the owner under the lock of the monthly billing; the holiday rules
//     keep their exclusion constraint.
//   - parent_billing.care_evidences: the income level or the single-parent
//     status of a contract, with the kind of evidence the Abrechnung checked,
//     the day it checked it and the validity. No document and no income is
//     stored (data minimisation); it goes with its contract.
//   - parent_billing.care_evidence_events: the trail of every read and
//     change of an evidence, one row per contract; it goes with its contract.
//   - parent_billing.care_invoice_reductions: the reductions of an issued
//     monthly invoice, one row per rule, with its own copy of the rule's
//     name, parameters, modules and the child's rank. It names no person and never
//     changes; it goes with its invoice.
const parentBillingCareReductionsUpSQL = `
-- The amount due after reductions is separate from the net/tax split.
-- NULL denotes a position issued before reductions existed (full amount due).
ALTER TABLE parent_billing.care_invoice_positions
	ADD COLUMN IF NOT EXISTS payable_cents BIGINT,
	DROP CONSTRAINT IF EXISTS chk_care_invoice_positions_payable;
ALTER TABLE parent_billing.care_invoice_positions
	ADD CONSTRAINT chk_care_invoice_positions_payable CHECK (payable_cents BETWEEN 0 AND amount_cents);

ALTER TABLE parent_billing.contribution_rules
	ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '',
	ADD COLUMN IF NOT EXISTS school_ids BIGINT[] NOT NULL DEFAULT '{}',
	ADD COLUMN IF NOT EXISTS care_offering_ids BIGINT[] NOT NULL DEFAULT '{}',
	ADD COLUMN IF NOT EXISTS count_school_ids BIGINT[] NOT NULL DEFAULT '{}',
	ADD COLUMN IF NOT EXISTS price_cents BIGINT,
	ADD COLUMN IF NOT EXISTS cap_cents BIGINT;
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS chk_contribution_rules_service;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_service CHECK (service IN ('holiday', 'care'));
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS chk_contribution_rules_kind;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_kind CHECK (
	(service = 'holiday' AND kind IN ('sibling', 'benefits_exemption'))
	OR (service = 'care' AND kind IN ('income_level', 'sibling', 'single_parent', 'cap')));
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS chk_contribution_rules_sibling;
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS chk_contribution_rules_holiday;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_holiday CHECK (service <> 'holiday' OR (
	BTRIM(name) = '' AND cardinality(school_ids) = 0 AND cardinality(care_offering_ids) = 0
	AND cardinality(count_school_ids) = 0 AND price_cents IS NULL AND cap_cents IS NULL));
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_sibling CHECK (
	service <> 'holiday' OR kind <> 'sibling' OR (
		sibling_from_child BETWEEN 2 AND 3 AND BTRIM(note) = '' AND (
			(discount_basis_points BETWEEN 1 AND 10000 AND discount_cents IS NULL)
			OR (discount_cents BETWEEN 1 AND 100000000 AND discount_basis_points IS NULL))));
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS chk_contribution_rules_care;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_care CHECK (service <> 'care' OR (
	BTRIM(name) <> '' AND char_length(name) <= 120 AND BTRIM(note) = ''
	AND cardinality(school_ids) BETWEEN 1 AND 200 AND cardinality(care_offering_ids) <= 500
	AND EXTRACT(DAY FROM valid_from) = 1
	AND (valid_until IS NULL OR EXTRACT(DAY FROM valid_until + 1) = 1)
	AND (kind = 'sibling' OR cardinality(count_school_ids) = 0) AND cardinality(count_school_ids) <= 200
	AND (kind = 'sibling') = (sibling_from_child IS NOT NULL)
	AND (sibling_from_child IS NULL OR sibling_from_child BETWEEN 2 AND 9)
	AND (discount_basis_points IS NULL OR discount_basis_points BETWEEN 1 AND 10000)
	AND (discount_cents IS NULL OR discount_cents BETWEEN 1 AND 100000000)
	AND (price_cents IS NULL OR price_cents BETWEEN 0 AND 100000000)
	AND (cap_cents IS NULL OR cap_cents BETWEEN 0 AND 100000000)
	AND CASE kind
		WHEN 'cap' THEN cap_cents IS NOT NULL AND num_nonnulls(discount_basis_points, discount_cents, price_cents) = 0
		WHEN 'single_parent' THEN cap_cents IS NULL AND price_cents IS NULL
			AND num_nonnulls(discount_basis_points, discount_cents) = 1
		ELSE cap_cents IS NULL AND num_nonnulls(discount_basis_points, discount_cents, price_cents) = 1
	END));
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS excl_contribution_rules_overlap;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT excl_contribution_rules_overlap EXCLUDE USING gist (
	organization_id WITH =, service WITH =, kind WITH =, daterange(valid_from, valid_until, '[]') WITH &&)
	WHERE (service = 'holiday');
CREATE UNIQUE INDEX IF NOT EXISTS uq_contribution_rules_organization ON parent_billing.contribution_rules (organization_id, id);

CREATE TABLE IF NOT EXISTS parent_billing.care_evidences (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	kind VARCHAR(16) NOT NULL,
	rule_id BIGINT,
	valid_from DATE NOT NULL,
	valid_until DATE,
	evidence TEXT NOT NULL,
	checked_on DATE NOT NULL,
	recorded_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_care_evidences_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	-- A level in use cannot be deleted: the evidence would lose its level.
	CONSTRAINT fk_care_evidences_rule FOREIGN KEY (organization_id, rule_id)
		REFERENCES parent_billing.contribution_rules (organization_id, id) ON DELETE RESTRICT,
	CONSTRAINT chk_care_evidences_kind CHECK (kind IN ('income_level', 'single_parent')),
	CONSTRAINT chk_care_evidences_level CHECK ((kind = 'income_level') = (rule_id IS NOT NULL)),
	CONSTRAINT chk_care_evidences_validity CHECK (valid_until IS NULL OR valid_until >= valid_from),
	CONSTRAINT chk_care_evidences_evidence CHECK (BTRIM(evidence) <> '' AND char_length(evidence) <= 120),
	CONSTRAINT excl_care_evidences_overlap EXCLUDE USING gist (
		organization_id WITH =, care_contract_id WITH =, kind WITH =, daterange(valid_from, valid_until, '[]') WITH &&)
);
CREATE INDEX IF NOT EXISTS idx_care_evidences_contract ON parent_billing.care_evidences (organization_id, care_contract_id, kind, valid_from);
CREATE INDEX IF NOT EXISTS idx_care_evidences_rule ON parent_billing.care_evidences (organization_id, rule_id) WHERE rule_id IS NOT NULL;
COMMENT ON TABLE parent_billing.care_evidences IS
	'Income level or single-parent status of a care contract with the evidence checked and its validity (#4685); no document, no income.';

CREATE TABLE IF NOT EXISTS parent_billing.care_evidence_events (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	evidence_id BIGINT,
	action VARCHAR(16) NOT NULL,
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_care_evidence_events_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_evidence_events_action CHECK (action IN ('read', 'create', 'update', 'delete'))
);
CREATE INDEX IF NOT EXISTS idx_care_evidence_events_contract ON parent_billing.care_evidence_events (organization_id, care_contract_id, occurred_at);
COMMENT ON TABLE parent_billing.care_evidence_events IS
	'Trail of every read and change of the evidences of a care contract (#4685).';

CREATE TABLE IF NOT EXISTS parent_billing.care_invoice_reductions (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	invoice_id BIGINT NOT NULL,
	position SMALLINT NOT NULL,
	kind VARCHAR(16) NOT NULL,
	rule_id BIGINT NOT NULL,
	name TEXT NOT NULL,
	child_rank SMALLINT,
	family_children SMALLINT,
	discount_basis_points INT,
	discount_cents BIGINT,
	price_cents BIGINT,
	cap_cents BIGINT,
	care_offering_ids BIGINT[] NOT NULL DEFAULT '{}',
	amount_cents BIGINT NOT NULL,
	CONSTRAINT fk_care_invoice_reductions_invoice FOREIGN KEY (organization_id, invoice_id)
		REFERENCES parent_billing.care_invoices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_care_invoice_reductions_position UNIQUE (invoice_id, position),
	CONSTRAINT chk_care_invoice_reductions_kind CHECK (kind IN ('income_level', 'sibling', 'single_parent', 'cap')),
	CONSTRAINT chk_care_invoice_reductions_amount CHECK (amount_cents < 0),
	CONSTRAINT chk_care_invoice_reductions_rank CHECK ((kind = 'sibling') = (child_rank IS NOT NULL AND family_children IS NOT NULL))
);
COMMENT ON TABLE parent_billing.care_invoice_reductions IS
	'Reduction of an issued monthly care invoice by one rule of the chain, with its own copy of the rule (#4685); never changes.';

DROP TRIGGER IF EXISTS refuse_parent_billing_care_invoice_reduction_change ON parent_billing.care_invoice_reductions;
CREATE TRIGGER refuse_parent_billing_care_invoice_reduction_change
	BEFORE UPDATE ON parent_billing.care_invoice_reductions
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();

ALTER TABLE parent_billing.care_evidences ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_evidences FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS traeger_isolation_parent_billing_care_evidences ON parent_billing.care_evidences;
CREATE POLICY traeger_isolation_parent_billing_care_evidences ON parent_billing.care_evidences FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_evidence_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_evidence_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS traeger_isolation_parent_billing_care_evidence_events ON parent_billing.care_evidence_events;
CREATE POLICY traeger_isolation_parent_billing_care_evidence_events ON parent_billing.care_evidence_events FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_invoice_reductions ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_invoice_reductions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS traeger_isolation_parent_billing_care_invoice_reductions ON parent_billing.care_invoice_reductions;
CREATE POLICY traeger_isolation_parent_billing_care_invoice_reductions ON parent_billing.care_invoice_reductions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.care_evidences, parent_billing.care_evidence_events,
	parent_billing.care_invoice_reductions FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON parent_billing.care_evidences TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.care_evidence_events, parent_billing.care_invoice_reductions TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.care_evidences_id_seq, parent_billing.care_evidence_events_id_seq,
	parent_billing.care_invoice_reductions_id_seq TO phoenix_tenant;
GRANT ALL ON ALL TABLES IN SCHEMA parent_billing TO phoenix_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA parent_billing TO phoenix_admin;
`

const parentBillingCareReductionsDownSQL = `
ALTER TABLE parent_billing.care_invoice_positions DROP COLUMN IF EXISTS payable_cents;

DROP TABLE IF EXISTS parent_billing.care_invoice_reductions, parent_billing.care_evidence_events,
	parent_billing.care_evidences;
DELETE FROM parent_billing.contribution_rules WHERE service = 'care';
DROP INDEX IF EXISTS parent_billing.uq_contribution_rules_organization;
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS excl_contribution_rules_overlap;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT excl_contribution_rules_overlap EXCLUDE USING gist (
	organization_id WITH =, service WITH =, kind WITH =, daterange(valid_from, valid_until, '[]') WITH &&);
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS chk_contribution_rules_care;
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS chk_contribution_rules_holiday;
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS chk_contribution_rules_sibling;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_sibling CHECK (kind <> 'sibling' OR (
	sibling_from_child BETWEEN 2 AND 3 AND BTRIM(note) = '' AND (
		(discount_basis_points BETWEEN 1 AND 10000 AND discount_cents IS NULL)
		OR (discount_cents BETWEEN 1 AND 100000000 AND discount_basis_points IS NULL))));
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS chk_contribution_rules_kind;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_kind CHECK (kind IN ('sibling', 'benefits_exemption'));
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS chk_contribution_rules_service;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_service CHECK (service IN ('holiday'));
ALTER TABLE parent_billing.contribution_rules
	DROP COLUMN IF EXISTS name, DROP COLUMN IF EXISTS school_ids, DROP COLUMN IF EXISTS care_offering_ids,
	DROP COLUMN IF EXISTS count_school_ids, DROP COLUMN IF EXISTS price_cents, DROP COLUMN IF EXISTS cap_cents;
`

func parentBillingCareReductionsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareReductionsUpSQL); err != nil {
		return fmt.Errorf("add the rule chain of the monthly care contribution: %w", err)
	}
	return nil
}

func parentBillingCareReductionsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareReductionsDownSQL); err != nil {
		return fmt.Errorf("drop the rule chain of the monthly care contribution: %w", err)
	}
	return nil
}
