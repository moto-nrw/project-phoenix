package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const demoVisitorErasureFamilyDebtorNumbersVersion = "1.15.951"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     demoVisitorErasureFamilyDebtorNumbersVersion,
		Description: "Demo-Löschung: Debitorennummern der Familien des Besuchers auch ohne Betreuungsvertrag löschen (#6812)",
		DependsOn: []string{
			holidayClosureRetireWaitingRenewalsVersion, // the newest migration on development (1.15.950)
			holidayRegistrationParentAccountVersion,    // the erasure this one replaces
			familyDebtorNumberVersion,                  // the rows without a contract it erases
		},
	})
	Migrations.MustRegister(demoVisitorErasureFamilyDebtorNumbersUp, demoVisitorErasureFamilyDebtorNumbersDown)
}

// demoVisitorFamiliesErasureSQL erases the rows of parent_billing.family_billing
// that name the visitor of a retired demo school, with their trail (#6812).
// Since #6692 a family the holiday care knows only keeps its debtor number in
// a row without a contract, named after the parent who registered last, and a
// contract row a registration used only loses its contract when the contract
// goes (trg_family_billing_detach). Neither goes with the contracts of the
// school, so platform.erase_closed_demo_visitors calls this before it deletes
// them.
//
// A row goes when it hangs on a contract of the retired schools or, without a
// contract, its family (the mail address of a registration, FamilyKey) has a
// holiday registration that goes with the school. A family that keeps another
// registration keeps its row: the number belongs to that registration too,
// and the deletion of the contract detaches a contract row as before. The
// trail of a row names it only by its ID and goes with it.
//
// The registrations that go are held first, so a confirmation under way
// either finishes before (and its row is seen here) or waits for the
// erasure and finds its registration gone. Like every lock of the erasure
// this one gives up after its lock_timeout and defers the order.
//
// The erasure runs it as its owner (SECURITY DEFINER); nobody else may.
// TestDemoErasureCoversEveryTableWithPersonalData reads the functions the
// erasure performs as part of it.
const demoVisitorFamiliesErasureSQL = `
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

const dropDemoVisitorFamiliesErasureSQL = `DROP FUNCTION IF EXISTS platform.erase_demo_visitor_families(BIGINT[])`

func demoVisitorErasureFamilyDebtorNumbersUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoVisitorFamiliesErasureSQL); err != nil {
		return fmt.Errorf("create the erasure of the demo visitor's families: %w", err)
	}
	return installFamilyDebtorNumberErasure(ctx, db)
}

// demoVisitorErasureFamilyDebtorNumbersDown restores the erasure of 1.15.945,
// which no longer calls the function, and drops it.
func demoVisitorErasureFamilyDebtorNumbersDown(ctx context.Context, db *bun.DB) error {
	if err := installParentAccountOfferErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, dropDemoVisitorFamiliesErasureSQL); err != nil {
		return fmt.Errorf("drop the erasure of the demo visitor's families: %w", err)
	}
	return nil
}
