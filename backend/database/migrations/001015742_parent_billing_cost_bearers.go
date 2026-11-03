package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingCostBearersVersion = "1.15.742"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCostBearersVersion,
		Description: "Elternabrechnung: Kostenübernahme durch einen Kostenträger in der Ferienabrechnung (#5627)",
		DependsOn: []string{
			holidayGuestHomeSchoolNameVersion, // the last migration before this one
			holidayDebtorNumberVersion,        // the audit actions this extends
			parentBillingHolidayRunsVersion,   // the invoice positions this extends
			holidayRegistrationProofsVersion,  // the proofs the Träger-Büro now uploads too
		},
	})
	Migrations.MustRegister(parentBillingCostBearersUp, parentBillingCostBearersDown)
}

// parentBillingCostBearersUpSQL adds the cost takeover of the holiday
// contribution (#5627, ADR 0115 addendum):
//
//   - parent_billing.cost_bearers: the bodies of a Träger that pay the
//     contribution of a family in its place, for example the youth office
//     of a district (§ 90 Abs. 4 SGB VIII), with their address, the file
//     reference of the Träger at the body and whether the Träger-Büro may
//     still choose them. Träger policy of ADR 0105.
//   - the line kind cost_takeover of a frozen contribution and of an issued
//     invoice: what the body pays, with the ID and a copy of the name of the
//     body, so a later rename changes neither. A line or a position keeps
//     the ID without a foreign key: the erasure of a Träger deletes the
//     bodies and the invoices in one cascade, which a foreign key would
//     refuse; the Elternabrechnung refuses to delete a referenced body.
//   - the audit action proof_upload: the Träger-Büro uploads a proof of
//     benefits the family brought on paper.
const parentBillingCostBearersUpSQL = `
CREATE TABLE parent_billing.cost_bearers (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	name VARCHAR(200) NOT NULL,
	address TEXT NOT NULL DEFAULT '',
	file_reference VARCHAR(100) NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_cost_bearers_name CHECK (BTRIM(name) <> '' AND name = BTRIM(name) AND name !~ '[\r\n]'),
	CONSTRAINT chk_cost_bearers_address CHECK (char_length(address) <= 500),
	CONSTRAINT chk_cost_bearers_file_reference CHECK (file_reference = BTRIM(file_reference) AND file_reference !~ '[\r\n]')
);
CREATE INDEX idx_cost_bearers_organization ON parent_billing.cost_bearers (organization_id, name);
COMMENT ON TABLE parent_billing.cost_bearers IS
	'Body that pays the holiday contribution of a family in its place (#5627, ADR 0115 addendum); read and written in the Träger transaction.';

CREATE TRIGGER update_parent_billing_cost_bearers_updated_at
	BEFORE UPDATE ON parent_billing.cost_bearers
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

ALTER TABLE parent_billing.cost_bearers ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.cost_bearers FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_cost_bearers ON parent_billing.cost_bearers FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
REVOKE ALL ON parent_billing.cost_bearers FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON parent_billing.cost_bearers TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.cost_bearers_id_seq TO phoenix_tenant;
GRANT ALL ON parent_billing.cost_bearers TO phoenix_admin;
GRANT ALL ON SEQUENCE parent_billing.cost_bearers_id_seq TO phoenix_admin;

ALTER TABLE parent_billing.holiday_contribution_lines
	ADD COLUMN cost_bearer_id BIGINT,
	ADD COLUMN cost_bearer_name TEXT NOT NULL DEFAULT '',
	DROP CONSTRAINT chk_holiday_contribution_lines_kind,
	ADD CONSTRAINT chk_holiday_contribution_lines_kind CHECK (kind IN ('base', 'sibling', 'benefits_exemption', 'cost_takeover')),
	ADD CONSTRAINT chk_holiday_contribution_lines_cost_bearer CHECK (
		(kind = 'cost_takeover' AND cost_bearer_id IS NOT NULL AND BTRIM(cost_bearer_name) <> '')
		OR (kind <> 'cost_takeover' AND cost_bearer_id IS NULL AND cost_bearer_name = ''));
CREATE INDEX idx_holiday_contribution_lines_cost_bearer
	ON parent_billing.holiday_contribution_lines (organization_id, cost_bearer_id) WHERE cost_bearer_id IS NOT NULL;

ALTER TABLE parent_billing.document_positions
	ADD COLUMN cost_bearer_id BIGINT,
	ADD COLUMN cost_bearer_name TEXT NOT NULL DEFAULT '',
	DROP CONSTRAINT chk_document_positions_kind,
	ADD CONSTRAINT chk_document_positions_kind CHECK (kind IN ('base', 'sibling', 'benefits_exemption', 'cost_takeover', 'correction')),
	ADD CONSTRAINT chk_document_positions_cost_bearer CHECK (
		(kind = 'cost_takeover' AND cost_bearer_id IS NOT NULL AND BTRIM(cost_bearer_name) <> '')
		OR (kind <> 'cost_takeover' AND cost_bearer_id IS NULL AND cost_bearer_name = ''));
CREATE INDEX idx_document_positions_cost_bearer
	ON parent_billing.document_positions (organization_id, cost_bearer_id) WHERE cost_bearer_id IS NOT NULL;

ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'debtor_number',
			'proof_upload'));
`

// parentBillingCostBearersDownSQL drops the cost takeover. The lines and
// positions of a takeover go first: the restored checks would reject them.
const parentBillingCostBearersDownSQL = `
DELETE FROM enrollment.holiday_registration_audit WHERE action = 'proof_upload';
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'debtor_number'));

DELETE FROM parent_billing.document_positions WHERE kind = 'cost_takeover';
ALTER TABLE parent_billing.document_positions
	DROP CONSTRAINT chk_document_positions_cost_bearer,
	DROP CONSTRAINT chk_document_positions_kind,
	ADD CONSTRAINT chk_document_positions_kind CHECK (kind IN ('base', 'sibling', 'benefits_exemption', 'correction')),
	DROP COLUMN cost_bearer_name,
	DROP COLUMN cost_bearer_id;

DELETE FROM parent_billing.holiday_contribution_lines WHERE kind = 'cost_takeover';
ALTER TABLE parent_billing.holiday_contribution_lines
	DROP CONSTRAINT chk_holiday_contribution_lines_cost_bearer,
	DROP CONSTRAINT chk_holiday_contribution_lines_kind,
	ADD CONSTRAINT chk_holiday_contribution_lines_kind CHECK (kind IN ('base', 'sibling', 'benefits_exemption')),
	DROP COLUMN cost_bearer_name,
	DROP COLUMN cost_bearer_id;

DROP TABLE IF EXISTS parent_billing.cost_bearers;
`

func parentBillingCostBearersUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCostBearersUpSQL); err != nil {
		return fmt.Errorf("add parent billing cost bearers: %w", err)
	}
	return nil
}

func parentBillingCostBearersDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCostBearersDownSQL); err != nil {
		return fmt.Errorf("drop parent billing cost bearers: %w", err)
	}
	return nil
}
