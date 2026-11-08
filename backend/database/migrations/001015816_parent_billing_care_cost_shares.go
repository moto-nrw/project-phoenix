package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingCareCostSharesVersion = "1.15.816"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCareCostSharesVersion,
		Description: "Elternabrechnung: Kostenträger-Anteile am Monatsbeitrag (BuT, Jugendamt, Zuschüsse) (#4687)",
		DependsOn: []string{
			parentBillingFamilyChildrenVersion, // the last migration of the monthly billing before this one
			parentBillingCareReviewVersion,     // the review the fingerprint covers
			parentBillingCareReductionsVersion, // the rule chain the shares follow
			parentBillingCostBearersVersion,    // the cost bearers a share names
		},
	})
	Migrations.MustRegister(parentBillingCareCostSharesUp, parentBillingCareCostSharesDown)
}

// parentBillingCareCostSharesUpSQL adds the cost bearer shares of the
// monthly care contribution (#4687, ADR 0169):
//
//   - parent_billing.care_cost_shares: the part a cost bearer of the Träger
//     pays of the contribution of a contract (Bildung und Teilhabe, Jugendamt
//     or another subsidy), as basis points or a monthly amount, with its
//     validity, the reference to the proof and the day the Abrechnung checked
//     it. No document and nothing of the decision is stored (data
//     minimisation). It goes with its contract. Like the lines and positions
//     of #5627 it names its cost bearer without a foreign key: the erasure of
//     a Träger removes both in one cascade, and the owner checks the cost
//     bearer of the Träger under its row lock, which a deletion takes too.
//   - parent_billing.care_cost_share_events: the trail of every read and
//     change of the shares, one row per contract; it goes with its contract.
//   - parent_billing.care_invoice_cost_shares: the shares of an issued
//     monthly invoice with their own copy of the cost bearer's name and of
//     the share. It names no person and never changes; it goes with its
//     invoice.
const parentBillingCareCostSharesUpSQL = `
CREATE TABLE IF NOT EXISTS parent_billing.care_cost_shares (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	kind VARCHAR(24) NOT NULL,
	cost_bearer_id BIGINT NOT NULL,
	share_basis_points INT,
	share_cents BIGINT,
	valid_from DATE NOT NULL,
	valid_until DATE NOT NULL,
	reference TEXT NOT NULL,
	checked_on DATE NOT NULL,
	recorded_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_care_cost_shares_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_cost_shares_kind CHECK (kind IN ('education_participation', 'youth_office', 'other_subsidy')),
	CONSTRAINT chk_care_cost_shares_share CHECK (
		(share_basis_points BETWEEN 1 AND 10000 AND share_cents IS NULL)
		OR (share_cents BETWEEN 1 AND 100000000 AND share_basis_points IS NULL)),
	CONSTRAINT chk_care_cost_shares_validity CHECK (valid_until >= valid_from),
	CONSTRAINT chk_care_cost_shares_reference CHECK (BTRIM(reference) <> '' AND char_length(reference) <= 120),
	CONSTRAINT excl_care_cost_shares_overlap EXCLUDE USING gist (
		organization_id WITH =, care_contract_id WITH =, kind WITH =, daterange(valid_from, valid_until, '[]') WITH &&)
);
CREATE INDEX IF NOT EXISTS idx_care_cost_shares_contract ON parent_billing.care_cost_shares (organization_id, care_contract_id, kind, valid_from);
CREATE INDEX IF NOT EXISTS idx_care_cost_shares_bearer ON parent_billing.care_cost_shares (organization_id, cost_bearer_id);
COMMENT ON TABLE parent_billing.care_cost_shares IS
	'Part a cost bearer pays of the monthly contribution of a care contract with its validity and a reference to the proof (#4687); no document, no content of the decision.';

CREATE TABLE IF NOT EXISTS parent_billing.care_cost_share_events (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	share_id BIGINT,
	action VARCHAR(16) NOT NULL,
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_care_cost_share_events_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_cost_share_events_action CHECK (action IN ('read', 'create', 'update', 'delete'))
);
CREATE INDEX IF NOT EXISTS idx_care_cost_share_events_contract ON parent_billing.care_cost_share_events (organization_id, care_contract_id, occurred_at);
COMMENT ON TABLE parent_billing.care_cost_share_events IS
	'Trail of every read and change of the cost bearer shares of a care contract (#4687).';

CREATE TABLE IF NOT EXISTS parent_billing.care_invoice_cost_shares (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	invoice_id BIGINT NOT NULL,
	position SMALLINT NOT NULL,
	kind VARCHAR(24) NOT NULL,
	cost_bearer_id BIGINT NOT NULL,
	cost_bearer_name TEXT NOT NULL,
	share_basis_points INT,
	share_cents BIGINT,
	amount_cents BIGINT NOT NULL,
	CONSTRAINT fk_care_invoice_cost_shares_invoice FOREIGN KEY (organization_id, invoice_id)
		REFERENCES parent_billing.care_invoices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_care_invoice_cost_shares_position UNIQUE (invoice_id, position),
	CONSTRAINT chk_care_invoice_cost_shares_kind CHECK (kind IN ('education_participation', 'youth_office', 'other_subsidy')),
	CONSTRAINT chk_care_invoice_cost_shares_share CHECK (num_nonnulls(share_basis_points, share_cents) = 1),
	CONSTRAINT chk_care_invoice_cost_shares_amount CHECK (amount_cents > 0)
);
CREATE INDEX IF NOT EXISTS idx_care_invoice_cost_shares_bearer ON parent_billing.care_invoice_cost_shares (organization_id, cost_bearer_id);
COMMENT ON TABLE parent_billing.care_invoice_cost_shares IS
	'Part a cost bearer pays of an issued monthly care invoice, with its own copy of the share and the name (#4687); never changes.';

DROP TRIGGER IF EXISTS refuse_parent_billing_care_invoice_cost_share_change ON parent_billing.care_invoice_cost_shares;
CREATE TRIGGER refuse_parent_billing_care_invoice_cost_share_change
	BEFORE UPDATE ON parent_billing.care_invoice_cost_shares
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();

ALTER TABLE parent_billing.care_cost_shares ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_cost_shares FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS traeger_isolation_parent_billing_care_cost_shares ON parent_billing.care_cost_shares;
CREATE POLICY traeger_isolation_parent_billing_care_cost_shares ON parent_billing.care_cost_shares FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_cost_share_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_cost_share_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS traeger_isolation_parent_billing_care_cost_share_events ON parent_billing.care_cost_share_events;
CREATE POLICY traeger_isolation_parent_billing_care_cost_share_events ON parent_billing.care_cost_share_events FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_invoice_cost_shares ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_invoice_cost_shares FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS traeger_isolation_parent_billing_care_invoice_cost_shares ON parent_billing.care_invoice_cost_shares;
CREATE POLICY traeger_isolation_parent_billing_care_invoice_cost_shares ON parent_billing.care_invoice_cost_shares FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.care_cost_shares, parent_billing.care_cost_share_events,
	parent_billing.care_invoice_cost_shares FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON parent_billing.care_cost_shares TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.care_cost_share_events, parent_billing.care_invoice_cost_shares TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.care_cost_shares_id_seq, parent_billing.care_cost_share_events_id_seq,
	parent_billing.care_invoice_cost_shares_id_seq TO phoenix_tenant;
GRANT ALL ON ALL TABLES IN SCHEMA parent_billing TO phoenix_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA parent_billing TO phoenix_admin;
`

const parentBillingCareCostSharesDownSQL = `
DROP TABLE IF EXISTS parent_billing.care_invoice_cost_shares, parent_billing.care_cost_share_events,
	parent_billing.care_cost_shares;
`

func parentBillingCareCostSharesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareCostSharesUpSQL); err != nil {
		return fmt.Errorf("add the cost bearer shares of the monthly care contribution: %w", err)
	}
	return nil
}

func parentBillingCareCostSharesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareCostSharesDownSQL); err != nil {
		return fmt.Errorf("drop the cost bearer shares of the monthly care contribution: %w", err)
	}
	return nil
}
