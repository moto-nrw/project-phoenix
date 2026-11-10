package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingBookingAccountsVersion = "1.15.903"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingBookingAccountsVersion,
		Description: "Elternabrechnung: Erlöskonto je Leistung und Angebot, Kostenstellen mit Gültigkeit, festgeschrieben je Position (#4821)",
		DependsOn: []string{
			parentBillingDatevBookedDocumentsVersion, // the DATEV bookings (1.15.888) whose export_files it extends
			bankDataPlaintextDropVersion,             // the newest on development (1.15.900)
		},
	})
	Migrations.MustRegister(parentBillingBookingAccountsUp, parentBillingBookingAccountsDown)
}

// parentBillingBookingAccountsUpSQL holds where the Elternabrechnung books
// (#4821):
//
//   - revenue_accounts: the revenue account of a Leistung (care, meals,
//     holiday; scope_id 0) or of an Angebot of the monthly contributions
//     (scope 'offering', scope_id the care offering). Without a row the
//     Leistung, then the standard of the DATEV settings applies.
//   - cost_centers: KOST1 of a school (scope 'school') or of the holiday
//     care of the Träger (scope 'holiday', scope_id 0), KOST2 of an Angebot
//     (scope 'offering'). A row holds from valid_from until the next row of
//     its scope; an empty code ends the cost center from that day on.
//   - the positions of the issued documents get the revenue account and both
//     cost centers the release resolved, and accounts_frozen. The trigger
//     refuse_issued_change keeps them as issued; a later change of the
//     tables above never reaches an issued position. A position issued
//     before #4821 has accounts_frozen = false: the export resolves it
//     at export time and the protocol says so.
const parentBillingBookingAccountsUpSQL = `
CREATE TABLE parent_billing.revenue_accounts (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	scope VARCHAR(16) NOT NULL,
	scope_id BIGINT NOT NULL DEFAULT 0,
	account VARCHAR(8) NOT NULL,
	updated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_revenue_accounts_scope UNIQUE (organization_id, scope, scope_id),
	CONSTRAINT chk_revenue_accounts_scope CHECK (scope IN ('care', 'meals', 'holiday', 'offering')),
	CONSTRAINT chk_revenue_accounts_scope_id CHECK ((scope = 'offering') = (scope_id > 0) AND scope_id >= 0),
	CONSTRAINT chk_revenue_accounts_account CHECK (account ~ '^0*[1-9][0-9]*$')
);
COMMENT ON TABLE parent_billing.revenue_accounts IS
	'Revenue account of a Leistung or an Angebot of the Elternabrechnung (#4821); the release copies it onto each position.';

CREATE TABLE parent_billing.cost_centers (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	scope VARCHAR(16) NOT NULL,
	scope_id BIGINT NOT NULL DEFAULT 0,
	valid_from DATE NOT NULL,
	code VARCHAR(36) NOT NULL,
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_cost_centers_scope UNIQUE (organization_id, scope, scope_id, valid_from),
	CONSTRAINT chk_cost_centers_scope CHECK (scope IN ('school', 'holiday', 'offering')),
	CONSTRAINT chk_cost_centers_scope_id CHECK ((scope = 'holiday') = (scope_id = 0) AND scope_id >= 0),
	CONSTRAINT chk_cost_centers_code CHECK (code ~ '^[A-Za-z0-9_ ]*$' AND code = BTRIM(code))
);
COMMENT ON TABLE parent_billing.cost_centers IS
	'Cost center (KOST1 of a school or the holiday care, KOST2 of an Angebot) from valid_from until the next row of its scope (#4821); an empty code ends it.';

ALTER TABLE parent_billing.care_invoice_positions
	ADD COLUMN revenue_account VARCHAR(8),
	ADD COLUMN cost_center VARCHAR(36),
	ADD COLUMN cost_center_2 VARCHAR(36),
	ADD COLUMN accounts_frozen BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE parent_billing.care_meal_positions
	ADD COLUMN revenue_account VARCHAR(8),
	ADD COLUMN cost_center VARCHAR(36),
	ADD COLUMN cost_center_2 VARCHAR(36),
	ADD COLUMN accounts_frozen BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE parent_billing.document_positions
	ADD COLUMN revenue_account VARCHAR(8),
	ADD COLUMN cost_center VARCHAR(36),
	ADD COLUMN cost_center_2 VARCHAR(36),
	ADD COLUMN accounts_frozen BOOLEAN NOT NULL DEFAULT false;

-- How many positions of a file had no values of their release and booked
-- on the values of the day of the export.
ALTER TABLE parent_billing.export_files
	ADD COLUMN resolved_positions INT NOT NULL DEFAULT 0,
	ADD CONSTRAINT chk_export_files_resolved_positions CHECK (resolved_positions >= 0);

ALTER TABLE parent_billing.revenue_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.revenue_accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_revenue_accounts ON parent_billing.revenue_accounts FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
REVOKE ALL ON parent_billing.revenue_accounts FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON parent_billing.revenue_accounts TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.revenue_accounts_id_seq TO phoenix_tenant;
GRANT ALL ON parent_billing.revenue_accounts TO phoenix_admin;
GRANT USAGE ON SEQUENCE parent_billing.revenue_accounts_id_seq TO phoenix_admin;

ALTER TABLE parent_billing.cost_centers ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.cost_centers FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_cost_centers ON parent_billing.cost_centers FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
REVOKE ALL ON parent_billing.cost_centers FROM PUBLIC;
GRANT SELECT, INSERT, DELETE ON parent_billing.cost_centers TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.cost_centers_id_seq TO phoenix_tenant;
GRANT ALL ON parent_billing.cost_centers TO phoenix_admin;
GRANT USAGE ON SEQUENCE parent_billing.cost_centers_id_seq TO phoenix_admin;
`

const parentBillingBookingAccountsDownSQL = `
ALTER TABLE parent_billing.export_files
	DROP CONSTRAINT IF EXISTS chk_export_files_resolved_positions, DROP COLUMN IF EXISTS resolved_positions;
ALTER TABLE parent_billing.document_positions
	DROP COLUMN IF EXISTS revenue_account, DROP COLUMN IF EXISTS cost_center,
	DROP COLUMN IF EXISTS cost_center_2, DROP COLUMN IF EXISTS accounts_frozen;
ALTER TABLE parent_billing.care_meal_positions
	DROP COLUMN IF EXISTS revenue_account, DROP COLUMN IF EXISTS cost_center,
	DROP COLUMN IF EXISTS cost_center_2, DROP COLUMN IF EXISTS accounts_frozen;
ALTER TABLE parent_billing.care_invoice_positions
	DROP COLUMN IF EXISTS revenue_account, DROP COLUMN IF EXISTS cost_center,
	DROP COLUMN IF EXISTS cost_center_2, DROP COLUMN IF EXISTS accounts_frozen;
DROP TABLE IF EXISTS parent_billing.cost_centers;
DROP TABLE IF EXISTS parent_billing.revenue_accounts;
`

func parentBillingBookingAccountsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingBookingAccountsUpSQL); err != nil {
		return fmt.Errorf("add the revenue accounts and cost centers of the Elternabrechnung: %w", err)
	}
	return nil
}

func parentBillingBookingAccountsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingBookingAccountsDownSQL); err != nil {
		return fmt.Errorf("drop the revenue accounts and cost centers of the Elternabrechnung: %w", err)
	}
	return nil
}
