package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingFamilyChildrenVersion = "1.15.815"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingFamilyChildrenVersion,
		Description: "Elternabrechnung: Kinder in der Familie für die Geschwisterregel (#5918)",
		DependsOn: []string{
			studentMembershipFirstCareFromVersion, // the last migration before this one
			parentBillingCareLettersVersion,       // the last migration of the parent billing before this one
			parentBillingCareReductionsVersion,    // the sibling rule that counts the family
		},
	})
	Migrations.MustRegister(parentBillingFamilyChildrenUp, parentBillingFamilyChildrenDown)
}

// parentBillingFamilyChildrenUpSQL adds the children of a family to the
// sibling rule of the monthly contribution (#5918, ADR 0088 section 4
// addendum 2026-11-08, E2 option A):
//
//   - contribution_rules.counts_family_children: whether a sibling rule of
//     the care counts the family's number. Off by default, so no rule
//     changes by itself.
//   - parent_billing.family_child_counts: how many children a family has,
//     those without a care contract included. The family is the address of
//     the invoice's recipient, compared without case, as in the rule chain
//     of #4685. The parents state a number, the Abrechnung confirms or
//     changes it with a validity. Only the number is stored, no names, no
//     dates of birth (data minimisation). The row hangs on the contract the
//     number was stated or confirmed for and goes with it.
//   - parent_billing.family_child_count_events: the trail of every read and
//     change, with the value and validity of a change, the account and
//     whether it was a parent or the Abrechnung. It goes with its contract.
const parentBillingFamilyChildrenUpSQL = `
ALTER TABLE parent_billing.contribution_rules
	ADD COLUMN IF NOT EXISTS counts_family_children BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS chk_contribution_rules_family_children;
ALTER TABLE parent_billing.contribution_rules ADD CONSTRAINT chk_contribution_rules_family_children
	CHECK (NOT counts_family_children OR (service = 'care' AND kind = 'sibling'));

CREATE TABLE IF NOT EXISTS parent_billing.family_child_counts (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	family_email TEXT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	stated_children SMALLINT,
	stated_at TIMESTAMPTZ,
	stated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	confirmed_children SMALLINT,
	valid_from DATE,
	valid_until DATE,
	confirmed_at TIMESTAMPTZ,
	confirmed_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_family_child_counts_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_family_child_counts_family UNIQUE (organization_id, family_email),
	CONSTRAINT chk_family_child_counts_email CHECK (
		family_email = LOWER(BTRIM(family_email)) AND family_email <> '' AND char_length(family_email) <= 320),
	CONSTRAINT chk_family_child_counts_stated CHECK (
		(stated_children IS NULL) = (stated_at IS NULL) AND (stated_children IS NULL OR stated_children BETWEEN 1 AND 20)),
	CONSTRAINT chk_family_child_counts_confirmed CHECK (
		num_nulls(confirmed_children, valid_from, confirmed_at) IN (0, 3)
		AND (confirmed_children IS NULL OR confirmed_children BETWEEN 1 AND 20)
		AND (valid_until IS NULL OR (valid_from IS NOT NULL AND valid_until >= valid_from))),
	CONSTRAINT chk_family_child_counts_value CHECK (stated_children IS NOT NULL OR confirmed_children IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_family_child_counts_contract ON parent_billing.family_child_counts (organization_id, care_contract_id);
COMMENT ON TABLE parent_billing.family_child_counts IS
	'Children of a family for the sibling rule (#5918), siblings without a contract included: the number the parents stated and the one the Abrechnung confirmed with its validity; no names, no dates of birth.';

CREATE TABLE IF NOT EXISTS parent_billing.family_child_count_events (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	family_child_count_id BIGINT,
	action VARCHAR(16) NOT NULL,
	actor VARCHAR(8) NOT NULL,
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	children SMALLINT,
	valid_from DATE,
	valid_until DATE,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_family_child_count_events_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_family_child_count_events_action CHECK (action IN ('read', 'state', 'confirm', 'delete')),
	CONSTRAINT chk_family_child_count_events_actor CHECK (actor IN ('parent', 'office')),
	CONSTRAINT chk_family_child_count_events_value CHECK ((action IN ('state', 'confirm')) = (children IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_family_child_count_events_contract
	ON parent_billing.family_child_count_events (organization_id, care_contract_id, occurred_at);
COMMENT ON TABLE parent_billing.family_child_count_events IS
	'Trail of every read and change of the children of a family (#5918): value, validity, account and whether a parent or the Abrechnung acted.';

ALTER TABLE parent_billing.family_child_counts ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.family_child_counts FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS traeger_isolation_parent_billing_family_child_counts ON parent_billing.family_child_counts;
CREATE POLICY traeger_isolation_parent_billing_family_child_counts ON parent_billing.family_child_counts FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.family_child_count_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.family_child_count_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS traeger_isolation_parent_billing_family_child_count_events ON parent_billing.family_child_count_events;
CREATE POLICY traeger_isolation_parent_billing_family_child_count_events ON parent_billing.family_child_count_events FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.family_child_counts, parent_billing.family_child_count_events FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON parent_billing.family_child_counts TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.family_child_count_events TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.family_child_counts_id_seq, parent_billing.family_child_count_events_id_seq TO phoenix_tenant;
GRANT ALL ON ALL TABLES IN SCHEMA parent_billing TO phoenix_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA parent_billing TO phoenix_admin;
`

const parentBillingFamilyChildrenDownSQL = `
DROP TABLE IF EXISTS parent_billing.family_child_count_events, parent_billing.family_child_counts;
ALTER TABLE parent_billing.contribution_rules DROP CONSTRAINT IF EXISTS chk_contribution_rules_family_children;
ALTER TABLE parent_billing.contribution_rules DROP COLUMN IF EXISTS counts_family_children;
`

func parentBillingFamilyChildrenUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingFamilyChildrenUpSQL); err != nil {
		return fmt.Errorf("add the children of a family: %w", err)
	}
	return nil
}

func parentBillingFamilyChildrenDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingFamilyChildrenDownSQL); err != nil {
		return fmt.Errorf("drop the children of a family: %w", err)
	}
	return nil
}
