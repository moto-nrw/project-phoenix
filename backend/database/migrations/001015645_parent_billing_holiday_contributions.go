package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingHolidayContributionsVersion = "1.15.645"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingHolidayContributionsVersion,
		Description: "Elternabrechnung: Beitragsregeln je Träger und festgehaltener Ferienbeitrag (#4664)",
		DependsOn: []string{
			holidayRegistrationReviewVersion, // the last migration before this one and the confirmation it freezes
		},
	})
	Migrations.MustRegister(parentBillingHolidayContributionsUp, parentBillingHolidayContributionsDown)
}

// parentBillingHolidayContributionsUpSQL adds the first tables of the owner
// Elternabrechnung (parent-billing, ADR 0088, #4664, ADR 0115):
//
//   - parent_billing.contribution_rules: the contribution rules of a Träger
//     with their parameters and validity. v1 knows the service "holiday" and
//     the rule kinds sibling (a discount in basis points or a fixed amount in
//     cent from the n-th child on) and benefits_exemption (no contribution
//     for a family receiving benefits, with the note it shows). Rules of one
//     kind never overlap in time; no price stands in the code.
//   - parent_billing.holiday_contributions: the contribution of one child of
//     a confirmed holiday registration, frozen at the confirmation, in whole
//     cent.
//   - parent_billing.holiday_contribution_lines: its positions in the fixed
//     order of the rule chain: the base price of every booked week, then the
//     sibling discount, then the exemption. Each line keeps what it shows, so
//     a later change of a price, a time model or a rule changes nothing.
//   - enrollment.holiday_registrations.benefits_declared: the parents state
//     in the registration that they receive benefits; the Träger-Büro decides
//     at the confirmation whether the exemption applies.
//
// Every table carries organization_id and the Träger policy of ADR 0105. The
// request role may insert and read a frozen contribution, never change or
// delete it; it goes with the child of its registration (ON DELETE CASCADE),
// 92 days after the period's last week or with the Träger.
const parentBillingHolidayContributionsUpSQL = `
CREATE SCHEMA IF NOT EXISTS parent_billing;
GRANT USAGE ON SCHEMA parent_billing TO phoenix_tenant, phoenix_admin;

ALTER TABLE enrollment.holiday_registrations
	ADD COLUMN benefits_declared BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE parent_billing.contribution_rules (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	service VARCHAR(16) NOT NULL,
	kind VARCHAR(32) NOT NULL,
	valid_from DATE NOT NULL,
	valid_until DATE,
	sibling_from_child SMALLINT,
	discount_basis_points INT,
	discount_cents BIGINT,
	note TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_contribution_rules_service CHECK (service IN ('holiday')),
	CONSTRAINT chk_contribution_rules_kind CHECK (kind IN ('sibling', 'benefits_exemption')),
	CONSTRAINT chk_contribution_rules_validity CHECK (valid_until IS NULL OR valid_until >= valid_from),
	CONSTRAINT chk_contribution_rules_sibling CHECK (kind <> 'sibling' OR (
		sibling_from_child BETWEEN 2 AND 3 AND BTRIM(note) = '' AND (
			(discount_basis_points BETWEEN 1 AND 10000 AND discount_cents IS NULL)
			OR (discount_cents BETWEEN 1 AND 100000000 AND discount_basis_points IS NULL)))),
	CONSTRAINT chk_contribution_rules_exemption CHECK (kind <> 'benefits_exemption' OR (
		sibling_from_child IS NULL AND discount_basis_points IS NULL AND discount_cents IS NULL
		AND BTRIM(note) <> '' AND char_length(note) <= 300)),
	CONSTRAINT excl_contribution_rules_overlap EXCLUDE USING gist (
		organization_id WITH =, service WITH =, kind WITH =, daterange(valid_from, valid_until, '[]') WITH &&)
);
COMMENT ON TABLE parent_billing.contribution_rules IS
	'Contribution rule of a Träger with its parameters and validity (#4664, ADR 0088, ADR 0115); read and written in the Träger transaction.';

CREATE TABLE parent_billing.holiday_contributions (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	holiday_period_id BIGINT NOT NULL,
	holiday_registration_id BIGINT NOT NULL,
	holiday_registration_child_id BIGINT NOT NULL,
	total_cents BIGINT NOT NULL,
	confirmed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	confirmed_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	CONSTRAINT uq_holiday_contributions_child UNIQUE (holiday_registration_child_id),
	CONSTRAINT uq_holiday_contributions_organization UNIQUE (organization_id, id),
	CONSTRAINT fk_holiday_contributions_child
		FOREIGN KEY (organization_id, holiday_period_id, holiday_registration_child_id)
		REFERENCES enrollment.holiday_registration_children (organization_id, holiday_period_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_holiday_contributions_total CHECK (total_cents >= 0)
);
CREATE INDEX idx_holiday_contributions_registration
	ON parent_billing.holiday_contributions (organization_id, holiday_registration_id);
COMMENT ON TABLE parent_billing.holiday_contributions IS
	'Contribution of one child of a confirmed holiday registration, frozen at the confirmation (#4664, ADR 0115); goes with the child.';

CREATE TABLE parent_billing.holiday_contribution_lines (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	holiday_contribution_id BIGINT NOT NULL,
	position SMALLINT NOT NULL,
	kind VARCHAR(32) NOT NULL,
	amount_cents BIGINT NOT NULL,
	week_starts_on DATE,
	week_ends_on DATE,
	time_model_name TEXT NOT NULL DEFAULT '',
	child_rank SMALLINT,
	discount_basis_points INT,
	discount_cents BIGINT,
	note TEXT NOT NULL DEFAULT '',
	CONSTRAINT fk_holiday_contribution_lines_contribution FOREIGN KEY (organization_id, holiday_contribution_id)
		REFERENCES parent_billing.holiday_contributions (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_holiday_contribution_lines_position UNIQUE (holiday_contribution_id, position),
	CONSTRAINT chk_holiday_contribution_lines_kind CHECK (kind IN ('base', 'sibling', 'benefits_exemption')),
	CONSTRAINT chk_holiday_contribution_lines_base CHECK (kind <> 'base' OR (
		amount_cents >= 0 AND week_starts_on IS NOT NULL AND week_ends_on IS NOT NULL AND BTRIM(time_model_name) <> '')),
	CONSTRAINT chk_holiday_contribution_lines_discount CHECK (kind = 'base' OR amount_cents <= 0)
);
COMMENT ON TABLE parent_billing.holiday_contribution_lines IS
	'Position of a frozen holiday contribution in the order of the rule chain (#4664, ADR 0115); goes with its contribution.';

CREATE TRIGGER update_parent_billing_contribution_rules_updated_at
	BEFORE UPDATE ON parent_billing.contribution_rules
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

ALTER TABLE parent_billing.contribution_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.contribution_rules FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_contribution_rules ON parent_billing.contribution_rules FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.holiday_contributions ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.holiday_contributions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_holiday_contributions ON parent_billing.holiday_contributions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.holiday_contribution_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.holiday_contribution_lines FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_holiday_contribution_lines ON parent_billing.holiday_contribution_lines FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.contribution_rules, parent_billing.holiday_contributions,
	parent_billing.holiday_contribution_lines FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON parent_billing.contribution_rules TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.holiday_contributions, parent_billing.holiday_contribution_lines TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.contribution_rules_id_seq, parent_billing.holiday_contributions_id_seq,
	parent_billing.holiday_contribution_lines_id_seq TO phoenix_tenant;
GRANT ALL ON ALL TABLES IN SCHEMA parent_billing TO phoenix_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA parent_billing TO phoenix_admin;
`

const parentBillingHolidayContributionsDownSQL = `
DROP SCHEMA IF EXISTS parent_billing CASCADE;
ALTER TABLE enrollment.holiday_registrations DROP COLUMN IF EXISTS benefits_declared;
`

func parentBillingHolidayContributionsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingHolidayContributionsUpSQL); err != nil {
		return fmt.Errorf("create parent billing holiday contributions: %w", err)
	}
	return nil
}

func parentBillingHolidayContributionsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingHolidayContributionsDownSQL); err != nil {
		return fmt.Errorf("drop parent billing holiday contributions: %w", err)
	}
	return nil
}
