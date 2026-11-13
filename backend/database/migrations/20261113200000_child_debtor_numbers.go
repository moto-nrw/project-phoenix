package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const childDebtorNumbersVersion = "20261113200000"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     childDebtorNumbersVersion,
		Description: "Elternabrechnung: Debitorennummer je Kind, wenn der Träger sie so führt (#7007)",
		DependsOn: []string{
			familyDebtorNumberVersion,                    // the family numbers and their retention (1.15.948)
			demoVisitorErasureFamilyDebtorNumbersVersion, // the erasure of the demo families this one extends (1.15.951)
			parentBillingPaymentTermsVersion,             // parent_billing.family_billing
			holidayRegistrationRetentionDaysVersion,      // enrollment.holiday_registrations the erasure reads
			careContractDraftsVersion,                    // care_contract.contracts the erasure reads
		},
	})
	Migrations.MustRegister(childDebtorNumbersUp, childDebtorNumbersDown)
}

// childDebtorNumbersUpSQL keeps the debtor number of a child of a Träger
// that keeps one per child (#7007, parent_billing.debtor_number_scope):
//
//   - family_email is the family of the child, the recipient of its
//     invoices (parentbilling.FamilyKey), as parent_billing.family_billing.
//   - child_key is the trimmed first and last name without case and the date
//     of birth (parentbilling.ChildKey); the child of a contract and the same
//     child at a holiday registration share it. child_first_name,
//     child_last_name and child_birth_date keep the names as entered for the
//     list of families.
//   - last_used_on is the latest day a number was saved or an invoice took
//     it. The Beleg retention deletes a row on 1 January of the fourth year
//     after it, unless a document of the family is still kept, like a family
//     row without a contract (#6692): the same instruction and deletion stop
//     of the Träger for its holiday documents, logged in beleg_deletions as
//     a family_debtor_number.
//
// The trail names a number only by its ID. Both go with their Träger. The
// erasure of a retired demo school deletes the numbers of its families with
// the families (platform.erase_demo_visitor_families).
const childDebtorNumbersUpSQL = `
CREATE TABLE parent_billing.child_debtor_numbers (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	family_email TEXT NOT NULL,
	child_key TEXT NOT NULL,
	child_first_name TEXT NOT NULL,
	child_last_name TEXT NOT NULL,
	child_birth_date DATE,
	debtor_number TEXT NOT NULL,
	last_used_on DATE NOT NULL,
	updated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_child_debtor_numbers_organization UNIQUE (organization_id, id),
	CONSTRAINT uq_child_debtor_numbers_child UNIQUE (organization_id, family_email, child_key),
	CONSTRAINT chk_child_debtor_numbers_email CHECK (
		family_email = LOWER(BTRIM(family_email)) AND family_email <> '' AND char_length(family_email) <= 320),
	CONSTRAINT chk_child_debtor_numbers_key CHECK (child_key <> '' AND char_length(child_key) <= 220),
	CONSTRAINT chk_child_debtor_numbers_names CHECK (BTRIM(child_first_name) <> '' AND BTRIM(child_last_name) <> ''
		AND char_length(child_first_name) <= 100 AND char_length(child_last_name) <= 100),
	CONSTRAINT chk_child_debtor_numbers_number CHECK (BTRIM(debtor_number) <> '' AND char_length(debtor_number) <= 35)
);
CREATE INDEX idx_child_debtor_numbers_last_used ON parent_billing.child_debtor_numbers (last_used_on);
COMMENT ON TABLE parent_billing.child_debtor_numbers IS
	'Debtor number of one child of a family of a Träger that keeps one per child (#7007); goes with the Beleg retention like a family number without a contract.';

CREATE TABLE parent_billing.child_debtor_number_events (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	child_debtor_number_id BIGINT NOT NULL,
	action VARCHAR(16) NOT NULL,
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_child_debtor_number_events_action CHECK (action IN ('read', 'save', 'delete'))
);
CREATE INDEX idx_child_debtor_number_events_number
	ON parent_billing.child_debtor_number_events (organization_id, child_debtor_number_id, occurred_at);
CREATE INDEX idx_child_debtor_number_events_occurred ON parent_billing.child_debtor_number_events (occurred_at);
COMMENT ON TABLE parent_billing.child_debtor_number_events IS
	'Trail of every read and change of the debtor number of a child (#7007): which one, who and when; no names. Goes after its row and three years.';

ALTER TABLE parent_billing.child_debtor_numbers ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.child_debtor_numbers FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_child_debtor_numbers ON parent_billing.child_debtor_numbers FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.child_debtor_number_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.child_debtor_number_events FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_child_debtor_number_events ON parent_billing.child_debtor_number_events FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.child_debtor_numbers, parent_billing.child_debtor_number_events FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON parent_billing.child_debtor_numbers TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.child_debtor_number_events TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.child_debtor_numbers_id_seq, parent_billing.child_debtor_number_events_id_seq
	TO phoenix_tenant;
-- The Beleg retention deletes expired numbers across every Träger.
GRANT ALL ON parent_billing.child_debtor_numbers, parent_billing.child_debtor_number_events TO phoenix_admin;
`

// demoVisitorChildrenErasureSQL is platform.erase_demo_visitor_families of
// 1.15.951 that also erases the numbers of the children of the families it
// erases (#7007): the children of a contract of the retired schools and of a
// family whose holiday registration goes with them, unless the family keeps
// a registration or a contract elsewhere. The trail of a number names it
// only by its ID and goes with it.
const demoVisitorChildrenErasureSQL = `
CREATE OR REPLACE FUNCTION platform.erase_demo_visitor_families(hidden_schools BIGINT[])
RETURNS VOID
LANGUAGE plpgsql
VOLATILE
SET search_path = pg_catalog
AS $$
BEGIN
	PERFORM 1 FROM enrollment.holiday_registrations AS registration
	WHERE EXISTS (SELECT 1 FROM enrollment.holiday_registration_children AS child
			WHERE child.organization_id = registration.organization_id
				AND child.holiday_registration_id = registration.id
				AND child.home_school_id = ANY (hidden_schools))
	ORDER BY registration.organization_id, registration.id
	FOR UPDATE;
	WITH going AS (
		SELECT DISTINCT registration.organization_id, LOWER(BTRIM(registration.guardian_email)) AS family_email
		FROM enrollment.holiday_registrations AS registration
		WHERE EXISTS (SELECT 1 FROM enrollment.holiday_registration_children AS child
				WHERE child.organization_id = registration.organization_id
					AND child.holiday_registration_id = registration.id
					AND child.home_school_id = ANY (hidden_schools))
	),
	going_children AS (
		SELECT organization_id, family_email FROM going
		UNION
		SELECT DISTINCT party.organization_id, LOWER(BTRIM(party.email)) AS family_email
		FROM care_contract.contract_parties AS party
		JOIN care_contract.contracts AS contract
			ON contract.organization_id = party.organization_id AND contract.id = party.contract_id
		WHERE contract.school_id = ANY (hidden_schools) AND BTRIM(party.email) <> ''
	),
	child AS (
		DELETE FROM parent_billing.child_debtor_numbers AS child
		USING going_children AS going
		WHERE going.organization_id = child.organization_id AND going.family_email = child.family_email
			AND NOT EXISTS (SELECT 1 FROM enrollment.holiday_registrations AS kept
				WHERE kept.organization_id = child.organization_id
					AND LOWER(BTRIM(kept.guardian_email)) = child.family_email
					AND NOT EXISTS (SELECT 1 FROM enrollment.holiday_registration_children AS kept_child
						WHERE kept_child.organization_id = kept.organization_id
							AND kept_child.holiday_registration_id = kept.id
							AND kept_child.home_school_id = ANY (hidden_schools)))
			AND NOT EXISTS (SELECT 1 FROM care_contract.contract_parties AS kept_party
				JOIN care_contract.contracts AS kept_contract
					ON kept_contract.organization_id = kept_party.organization_id AND kept_contract.id = kept_party.contract_id
				WHERE kept_party.organization_id = child.organization_id
					AND LOWER(BTRIM(kept_party.email)) = child.family_email
					AND NOT (kept_contract.school_id = ANY (hidden_schools)))
		RETURNING child.organization_id, child.id
	),
	child_trail AS (
		DELETE FROM parent_billing.child_debtor_number_events AS event
		USING child
		WHERE event.organization_id = child.organization_id AND event.child_debtor_number_id = child.id
	),
	family AS (
		DELETE FROM parent_billing.family_billing AS family
		WHERE (EXISTS (SELECT 1 FROM care_contract.contracts AS contract
					WHERE contract.organization_id = family.organization_id
						AND contract.id = family.care_contract_id
						AND contract.school_id = ANY (hidden_schools))
				OR (family.care_contract_id IS NULL AND EXISTS (SELECT 1 FROM going
					WHERE going.organization_id = family.organization_id AND going.family_email = family.family_email)))
			AND NOT EXISTS (SELECT 1 FROM enrollment.holiday_registrations AS kept
				WHERE kept.organization_id = family.organization_id
					AND LOWER(BTRIM(kept.guardian_email)) = family.family_email
					AND NOT EXISTS (SELECT 1 FROM enrollment.holiday_registration_children AS child
						WHERE child.organization_id = kept.organization_id
							AND child.holiday_registration_id = kept.id
							AND child.home_school_id = ANY (hidden_schools)))
		RETURNING family.organization_id, family.id
	)
	DELETE FROM parent_billing.family_billing_events AS event
	USING family
	WHERE event.organization_id = family.organization_id AND event.family_billing_id = family.id;
END;
$$;
REVOKE ALL ON FUNCTION platform.erase_demo_visitor_families(BIGINT[]) FROM PUBLIC;
`

const childDebtorNumbersDownSQL = `
DROP TABLE IF EXISTS parent_billing.child_debtor_number_events;
DROP TABLE IF EXISTS parent_billing.child_debtor_numbers;
`

func childDebtorNumbersUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, childDebtorNumbersUpSQL); err != nil {
		return fmt.Errorf("create the debtor numbers of the children: %w", err)
	}
	if _, err := db.ExecContext(ctx, demoVisitorChildrenErasureSQL); err != nil {
		return fmt.Errorf("erase the children's debtor numbers of retired demo schools: %w", err)
	}
	return nil
}

// childDebtorNumbersDown restores the erasure of 1.15.951 first, which no
// longer reads the tables, and drops them.
func childDebtorNumbersDown(ctx context.Context, db *bun.DB) error {
	if err := restoreDemoVisitorFamiliesErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, childDebtorNumbersDownSQL); err != nil {
		return fmt.Errorf("drop the debtor numbers of the children: %w", err)
	}
	return nil
}
