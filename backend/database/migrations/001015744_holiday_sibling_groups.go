package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidaySiblingGroupsVersion = "1.15.744"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidaySiblingGroupsVersion,
		Description: "Elternabrechnung: Geschwister aus getrennten Ferienanmeldungen und Fassungen des festgehaltenen Beitrags (#5628)",
		DependsOn: []string{
			holidayGuestHomeSchoolNameVersion,        // the last migration before this one
			holidayDebtorNumberVersion,               // the audit actions this extends
			parentBillingHolidayContributionsVersion, // the frozen contributions this versions
		},
	})
	Migrations.MustRegister(holidaySiblingGroupsUp, holidaySiblingGroupsDown)
}

// holidaySiblingGroupsUpSQL lets the Träger-Büro merge holiday registrations
// of one period as siblings (#5628, ADR 0115 addendum):
//
//   - parent_billing.holiday_sibling_groups: a group of registrations of one
//     period whose children the sibling rule ranks together.
//   - parent_billing.holiday_sibling_group_members: its registrations. A
//     registration belongs to at most one group. The member goes with its
//     registration (ON DELETE CASCADE, ADR 0118); a group with fewer than two
//     members goes with its last member (trigger).
//   - parent_billing.holiday_contributions.version: a merge or a split
//     computes the frozen contribution of a confirmed registration again and
//     stores it as a new version; the old one stays. The request role still
//     only inserts and reads a contribution (ADR 0115 point 5). Existing
//     contributions are version 1.
//   - enrollment.holiday_registration_audit takes the actions sibling_merge
//     and sibling_split.
const holidaySiblingGroupsUpSQL = `
ALTER TABLE parent_billing.holiday_contributions
	ADD COLUMN version INT NOT NULL DEFAULT 1,
	ADD CONSTRAINT chk_holiday_contributions_version CHECK (version >= 1),
	DROP CONSTRAINT uq_holiday_contributions_child,
	ADD CONSTRAINT uq_holiday_contributions_child_version UNIQUE (holiday_registration_child_id, version);
COMMENT ON COLUMN parent_billing.holiday_contributions.version IS
	'Version of the frozen contribution of the child; a merge or split of siblings adds a new one, the highest is valid (#5628).';

CREATE TABLE parent_billing.holiday_sibling_groups (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	holiday_period_id BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	CONSTRAINT uq_holiday_sibling_groups_period UNIQUE (organization_id, holiday_period_id, id)
);
COMMENT ON TABLE parent_billing.holiday_sibling_groups IS
	'Holiday registrations of one period the Träger-Büro merged as siblings (#5628); goes with its last but one member.';

CREATE TABLE parent_billing.holiday_sibling_group_members (
	organization_id BIGINT NOT NULL,
	holiday_period_id BIGINT NOT NULL,
	sibling_group_id BIGINT NOT NULL,
	holiday_registration_id BIGINT NOT NULL,
	CONSTRAINT pk_holiday_sibling_group_members PRIMARY KEY (holiday_registration_id),
	CONSTRAINT fk_holiday_sibling_group_members_group FOREIGN KEY (organization_id, holiday_period_id, sibling_group_id)
		REFERENCES parent_billing.holiday_sibling_groups (organization_id, holiday_period_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_holiday_sibling_group_members_registration FOREIGN KEY (organization_id, holiday_period_id, holiday_registration_id)
		REFERENCES enrollment.holiday_registrations (organization_id, holiday_period_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_holiday_sibling_group_members_group
	ON parent_billing.holiday_sibling_group_members (organization_id, holiday_period_id, sibling_group_id);
COMMENT ON TABLE parent_billing.holiday_sibling_group_members IS
	'Registration of a sibling group (#5628); at most one group per registration, goes with the registration.';

CREATE FUNCTION parent_billing.drop_lone_holiday_sibling_group() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	DELETE FROM parent_billing.holiday_sibling_groups AS sibling_group
	WHERE sibling_group.id = OLD.sibling_group_id
		AND (SELECT count(*) FROM parent_billing.holiday_sibling_group_members AS member
			WHERE member.sibling_group_id = OLD.sibling_group_id) < 2;
	RETURN NULL;
END;
$$;
CREATE TRIGGER drop_lone_holiday_sibling_group
	AFTER DELETE ON parent_billing.holiday_sibling_group_members
	FOR EACH ROW EXECUTE FUNCTION parent_billing.drop_lone_holiday_sibling_group();

ALTER TABLE parent_billing.holiday_sibling_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.holiday_sibling_groups FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_holiday_sibling_groups ON parent_billing.holiday_sibling_groups FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.holiday_sibling_group_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.holiday_sibling_group_members FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_holiday_sibling_group_members ON parent_billing.holiday_sibling_group_members FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.holiday_sibling_groups, parent_billing.holiday_sibling_group_members FROM PUBLIC;
GRANT SELECT, INSERT, DELETE ON parent_billing.holiday_sibling_groups, parent_billing.holiday_sibling_group_members TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.holiday_sibling_groups_id_seq TO phoenix_tenant;
GRANT ALL ON parent_billing.holiday_sibling_groups, parent_billing.holiday_sibling_group_members TO phoenix_admin;
GRANT ALL ON SEQUENCE parent_billing.holiday_sibling_groups_id_seq TO phoenix_admin;

ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'debtor_number',
			'sibling_merge', 'sibling_split'));
`

// holidaySiblingGroupsDownSQL drops the groups and every version but the
// first; a child keeps its first frozen contribution.
const holidaySiblingGroupsDownSQL = `
DELETE FROM enrollment.holiday_registration_audit WHERE action IN ('sibling_merge', 'sibling_split');
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'debtor_number'));
DROP TABLE IF EXISTS parent_billing.holiday_sibling_group_members;
DROP TABLE IF EXISTS parent_billing.holiday_sibling_groups;
DROP FUNCTION IF EXISTS parent_billing.drop_lone_holiday_sibling_group();
DELETE FROM parent_billing.holiday_contributions WHERE version > 1;
ALTER TABLE parent_billing.holiday_contributions
	DROP CONSTRAINT uq_holiday_contributions_child_version,
	DROP CONSTRAINT chk_holiday_contributions_version,
	DROP COLUMN version,
	ADD CONSTRAINT uq_holiday_contributions_child UNIQUE (holiday_registration_child_id);
`

func holidaySiblingGroupsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidaySiblingGroupsUpSQL); err != nil {
		return fmt.Errorf("add holiday sibling groups: %w", err)
	}
	return nil
}

func holidaySiblingGroupsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidaySiblingGroupsDownSQL); err != nil {
		return fmt.Errorf("drop holiday sibling groups: %w", err)
	}
	return nil
}
