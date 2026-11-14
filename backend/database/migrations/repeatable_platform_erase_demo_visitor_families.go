package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	RegisterRepeatable("platform.erase_demo_visitor_families", func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, repeatablePlatformEraseDemoVisitorFamiliesSQL)
		return err
	})
}

// repeatablePlatformEraseDemoVisitorFamiliesSQL is the current definition of
// platform.erase_demo_visitor_families(hidden_schools) (ADR 0223), which
// platform.erase_closed_demo_visitors calls. It erases the rows of
// parent_billing.family_billing that name the visitor of a retired demo
// school, with their trail (#6812), and the debtor numbers of the children
// of those families (#7007): the children of a contract of the retired
// schools and of a family whose holiday registration goes with them, unless
// the family keeps a registration or a contract elsewhere. A family that
// keeps another registration keeps its row. The trails name a row only by
// its ID and go with it.
//
// The registrations that go are held first, so a confirmation under way
// either finishes before or waits for the erasure. The erasure runs it as
// its owner (SECURITY DEFINER); nobody else may.
// TestDemoErasureCoversEveryTableWithPersonalData reads it as part of the
// erasure.
const repeatablePlatformEraseDemoVisitorFamiliesSQL = `
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
