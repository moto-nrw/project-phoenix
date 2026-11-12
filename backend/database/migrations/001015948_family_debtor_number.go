package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const familyDebtorNumberVersion = "1.15.948"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     familyDebtorNumberVersion,
		Description: "Elternabrechnung: Debitorennummer an der Familie merken, auch ohne Betreuungsvertrag (#6692)",
		DependsOn: []string{
			holidayRegistrationRetentionDaysVersion, // the newest migration on development (1.15.928)
			parentBillingPaymentTermsVersion,        // the family deviation this one extends
			parentBillingExportFilesVersion,         // its debtor number
			holidayDebtorNumberVersion,              // the debtor number of a registration the backfill reads
		},
	})
	Migrations.MustRegister(familyDebtorNumberUp, familyDebtorNumberDown)
}

// familyDebtorNumberUpSQL lets the Träger remember the debtor number of a
// family that has no care contract (#6692): a family that only registers
// for the holiday care. Such a row hangs on no contract, so it goes with
// its Träger (platform.organizations) and has a retention of its own:
//
//   - holder_first_name, holder_last_name: the parent who registered last,
//     so the list of families names the family before any invoice exists.
//   - last_used_on: the latest day a registration of the family needs the
//     number, the later of the confirmation or change and the last booked
//     day. The Beleg retention deletes a row without a contract on 1 January
//     of the fourth year after it, unless a document of the family is still
//     kept: three years after the end of the year, the regular limitation
//     period of the contribution the number identifies (§§ 195, 199 BGB),
//     which also outlasts every retention a registration can name (at most
//     1095 days after its period, #6691), and only under the instruction of
//     the Träger for its holiday documents and without its deletion stop.
//   - debtor_number_registration_id: the holiday registration the number
//     came from, NULL when the office set it in the list of families. Only
//     that registration corrects the family's number without the office's
//     choice; a registration that took the number as the proposal does not
//     (#6692). An ID without a foreign key: the registration is Enrollment's
//     and goes before the number does.
//
// A row a holiday registration used (last_used_on) outlives the contract it
// was set from: the contract's deletion sets care_contract_id to NULL, and
// trg_family_billing_detach drops the deviation that belonged to the
// contract, so the row keeps the number for its own period. A row no
// registration used still goes with its contract: cascade_contract_id names
// the contract only for such a row and cascades. The retention of a row
// without a contract follows the instruction and the deletion stop of the
// Träger for its holiday documents and logs its count in beleg_deletions
// (type family_debtor_number); audit.data_deletions cannot hold it, because
// it belongs to a school and a family of a Träger does not (like
// enrollment.holiday_registration_deletions, ADR 0118).
//
// The trail of such a row hangs on no contract either; the retention
// deletes its entries once their row is gone and they are as old.
const familyDebtorNumberUpSQL = `
ALTER TABLE parent_billing.family_billing
	ALTER COLUMN care_contract_id DROP NOT NULL,
	ADD COLUMN holder_first_name TEXT NOT NULL DEFAULT '',
	ADD COLUMN holder_last_name TEXT NOT NULL DEFAULT '',
	ADD COLUMN last_used_on DATE,
	ADD COLUMN debtor_number_registration_id BIGINT,
	DROP CONSTRAINT fk_family_billing_contract;
ALTER TABLE parent_billing.family_billing
	ADD COLUMN cascade_contract_id BIGINT GENERATED ALWAYS AS (CASE WHEN last_used_on IS NULL THEN care_contract_id END) STORED,
	ADD CONSTRAINT fk_family_billing_contract_cascade FOREIGN KEY (organization_id, cascade_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	ADD CONSTRAINT fk_family_billing_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE SET NULL (care_contract_id),
	ADD CONSTRAINT fk_family_billing_organization FOREIGN KEY (organization_id)
		REFERENCES platform.organizations (id) ON DELETE CASCADE,
	ADD CONSTRAINT chk_family_billing_holder CHECK (char_length(holder_first_name) <= 100 AND char_length(holder_last_name) <= 100),
	ADD CONSTRAINT chk_family_billing_retention CHECK (care_contract_id IS NOT NULL OR last_used_on IS NOT NULL),
	ADD CONSTRAINT chk_family_billing_without_contract CHECK (care_contract_id IS NOT NULL OR (
		recipient_kind = 'applicant' AND payment_method IS NULL AND due_day IS NULL));
COMMENT ON COLUMN parent_billing.family_billing.care_contract_id IS
	'Contract the deviation was set from; NULL for a family the holiday care knows only (#6692), whose row keeps its debtor number.';
COMMENT ON COLUMN parent_billing.family_billing.holder_first_name IS
	'First name of the parent who registered the family for the holiday care last (#6692); empty when no registration named it.';
COMMENT ON COLUMN parent_billing.family_billing.holder_last_name IS
	'Last name of the parent who registered the family for the holiday care last (#6692).';
COMMENT ON COLUMN parent_billing.family_billing.last_used_on IS
	'Latest day a holiday registration of the family needs the number (#6692); a row without a contract goes on 1 January of the fourth year after it unless a document of the family is still kept.';
COMMENT ON COLUMN parent_billing.family_billing.debtor_number_registration_id IS
	'Holiday registration the debtor number came from (#6692), NULL when set in the list of families; only it corrects the number without a choice. No foreign key: the registration goes first.';
COMMENT ON TABLE parent_billing.family_billing IS
	'Deviation and debtor number of one family (#4689, #6692), keyed by the mail address of the registration; goes with the contract it was set from unless a holiday registration used it (last_used_on), a row without a contract after its own period.';

COMMENT ON COLUMN parent_billing.family_billing.cascade_contract_id IS
	'The contract of a row no holiday registration used (#6692): such a row goes with its contract by cascade; a used row only loses its contract.';

-- family_billing_detach runs when the deletion of a contract sets
-- care_contract_id to NULL (#6692). A row no holiday registration used goes
-- with the contract by the cascade of cascade_contract_id: should the
-- SET NULL reach it first, it keeps its contract, so the cascade still finds
-- it. A used row drops the deviation of the contract and keeps its number
-- without one.
CREATE OR REPLACE FUNCTION parent_billing.family_billing_detach()
RETURNS TRIGGER LANGUAGE plpgsql SET search_path = pg_catalog AS $$
BEGIN
	IF NEW.last_used_on IS NULL THEN
		RETURN OLD;
	END IF;
	NEW.recipient_kind := 'applicant';
	NEW.second_first_name := ''; NEW.second_last_name := '';
	NEW.payer_first_name := ''; NEW.payer_last_name := ''; NEW.payer_email := '';
	NEW.payer_street := ''; NEW.payer_postal_code := ''; NEW.payer_city := '';
	NEW.payment_method := NULL; NEW.due_day := NULL;
	RETURN NEW;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.family_billing_detach() FROM PUBLIC;
CREATE TRIGGER trg_family_billing_detach BEFORE UPDATE OF care_contract_id ON parent_billing.family_billing
	FOR EACH ROW WHEN (OLD.care_contract_id IS NOT NULL AND NEW.care_contract_id IS NULL)
	EXECUTE FUNCTION parent_billing.family_billing_detach();

ALTER TABLE parent_billing.beleg_deletions
	DROP CONSTRAINT chk_beleg_deletions_type,
	ADD CONSTRAINT chk_beleg_deletions_type CHECK (beleg_type IN
		('holiday_document', 'care_invoice', 'care_notice', 'care_contract', 'sepa_mandate', 'sepa_debit', 'family_debtor_number'));

ALTER TABLE parent_billing.family_billing_events
	ALTER COLUMN care_contract_id DROP NOT NULL,
	ADD CONSTRAINT fk_family_billing_events_organization FOREIGN KEY (organization_id)
		REFERENCES platform.organizations (id) ON DELETE CASCADE;
CREATE INDEX idx_family_billing_events_without_contract ON parent_billing.family_billing_events (organization_id, occurred_at)
	WHERE care_contract_id IS NULL;
COMMENT ON TABLE parent_billing.family_billing_events IS
	'Trail of every read and change of a family deviation or debtor number (#4689, #6692): which one, who and when; no names. Goes with its contract, without one after its row and three years.';
`

// familyDebtorNumberBackfillSQL remembers the number of a family from its
// confirmed holiday registrations (#6692, superseding the suggestion of
// #5623): only where every confirmed registration of the family key that
// names a number names the same one. A family with two numbers stays
// without, so the Träger-Büro decides. A family whose row already names
// another number keeps it. A matching existing number gets holiday retention
// metadata but keeps its independent provenance. Each changed row gets a "save" in its
// trail without an account. The number is kept from the last week the
// registrations naming it booked (their submission without a booking), the
// day the runtime keeps it from too, and came from the first of them.
const familyDebtorNumberBackfillSQL = `
WITH registration AS (
	SELECT registration.organization_id, LOWER(BTRIM(registration.guardian_email)) AS family_email, registration.debtor_number,
		registration.guardian_first_name, registration.guardian_last_name, registration.submitted_at, registration.id,
		GREATEST((registration.submitted_at AT TIME ZONE 'Europe/Berlin')::date,
			(SELECT max(week.ends_on) FROM enrollment.holiday_registration_children AS child
				JOIN enrollment.holiday_registration_bookings AS booking
					ON booking.organization_id = child.organization_id AND booking.holiday_registration_child_id = child.id
				JOIN enrollment.holiday_period_weeks AS week
					ON week.organization_id = booking.organization_id AND week.id = booking.holiday_period_week_id
				WHERE child.organization_id = registration.organization_id AND child.holiday_registration_id = registration.id)) AS used_on
	FROM enrollment.holiday_registrations AS registration
	WHERE registration.status = 'confirmed' AND registration.debtor_number IS NOT NULL AND BTRIM(registration.guardian_email) <> ''
),
single AS (
	SELECT organization_id, family_email, MIN(debtor_number) AS debtor_number, MAX(used_on) AS used_on, MIN(id) AS origin
	FROM registration
	GROUP BY organization_id, family_email
	HAVING COUNT(DISTINCT debtor_number) = 1
),
holder AS (
	SELECT DISTINCT ON (organization_id, family_email) organization_id, family_email,
		LEFT(BTRIM(guardian_first_name), 100) AS first_name, LEFT(BTRIM(guardian_last_name), 100) AS last_name
	FROM registration
	ORDER BY organization_id, family_email, submitted_at DESC, id DESC
),
saved AS (
	INSERT INTO parent_billing.family_billing AS family (organization_id, family_email, care_contract_id, recipient_kind,
		debtor_number, holder_first_name, holder_last_name, last_used_on, debtor_number_registration_id)
	SELECT single.organization_id, single.family_email, NULL, 'applicant', single.debtor_number,
		holder.first_name, holder.last_name, single.used_on, single.origin
	FROM single
	JOIN holder ON holder.organization_id = single.organization_id AND holder.family_email = single.family_email
	ON CONFLICT (organization_id, family_email) DO UPDATE
		SET debtor_number = EXCLUDED.debtor_number,
			holder_first_name = EXCLUDED.holder_first_name, holder_last_name = EXCLUDED.holder_last_name,
			last_used_on = GREATEST(family.last_used_on, EXCLUDED.last_used_on),
			debtor_number_registration_id = CASE WHEN family.debtor_number IS NULL
				THEN EXCLUDED.debtor_number_registration_id ELSE family.debtor_number_registration_id END,
			updated_at = CURRENT_TIMESTAMP
		WHERE family.debtor_number IS NULL OR family.debtor_number = EXCLUDED.debtor_number
	RETURNING family.id, family.organization_id, family.care_contract_id
)
INSERT INTO parent_billing.family_billing_events (organization_id, family_billing_id, care_contract_id, action)
SELECT organization_id, id, care_contract_id, 'save' FROM saved`

// A rollback deletes what has no contract: every number a family without
// a care contract kept, and every row a deleted contract left behind, is
// lost with its trail; so is the count of the family deletions in the
// deletion log. It restores the columns and the cascade of the contract.
const familyDebtorNumberDownSQL = `
DELETE FROM parent_billing.family_billing_events WHERE care_contract_id IS NULL;
DELETE FROM parent_billing.family_billing WHERE care_contract_id IS NULL;
DELETE FROM parent_billing.beleg_deletions WHERE beleg_type = 'family_debtor_number';
ALTER TABLE parent_billing.beleg_deletions
	DROP CONSTRAINT chk_beleg_deletions_type,
	ADD CONSTRAINT chk_beleg_deletions_type CHECK (beleg_type IN
		('holiday_document', 'care_invoice', 'care_notice', 'care_contract', 'sepa_mandate', 'sepa_debit'));
DROP TRIGGER IF EXISTS trg_family_billing_detach ON parent_billing.family_billing;
DROP FUNCTION IF EXISTS parent_billing.family_billing_detach();
ALTER TABLE parent_billing.family_billing
	DROP CONSTRAINT IF EXISTS fk_family_billing_contract_cascade,
	DROP COLUMN IF EXISTS cascade_contract_id;
DROP INDEX IF EXISTS parent_billing.idx_family_billing_events_without_contract;
ALTER TABLE parent_billing.family_billing_events
	DROP CONSTRAINT IF EXISTS fk_family_billing_events_organization,
	ALTER COLUMN care_contract_id SET NOT NULL;
ALTER TABLE parent_billing.family_billing
	DROP CONSTRAINT IF EXISTS chk_family_billing_without_contract,
	DROP CONSTRAINT IF EXISTS chk_family_billing_retention,
	DROP CONSTRAINT IF EXISTS chk_family_billing_holder,
	DROP CONSTRAINT IF EXISTS fk_family_billing_organization,
	DROP CONSTRAINT IF EXISTS fk_family_billing_contract,
	ADD CONSTRAINT fk_family_billing_contract FOREIGN KEY (organization_id, care_contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	DROP COLUMN IF EXISTS debtor_number_registration_id,
	DROP COLUMN IF EXISTS last_used_on,
	DROP COLUMN IF EXISTS holder_last_name,
	DROP COLUMN IF EXISTS holder_first_name,
	ALTER COLUMN care_contract_id SET NOT NULL;
COMMENT ON TABLE parent_billing.family_billing IS
	'Deviation of one family (#4689): who gets its invoices and its own payment method or due day; keyed by the mail address of the registration, goes with the contract it was set from.';
COMMENT ON TABLE parent_billing.family_billing_events IS
	'Trail of every read and change of a family deviation (#4689): which one, who and when; no names.';
`

func familyDebtorNumberUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, familyDebtorNumberUpSQL); err != nil {
			return fmt.Errorf("let the family billing remember a debtor number without a contract: %w", err)
		}
		if _, err := tx.ExecContext(ctx, familyDebtorNumberBackfillSQL); err != nil {
			return fmt.Errorf("remember the debtor numbers of the confirmed holiday registrations: %w", err)
		}
		return nil
	})
}

// familyDebtorNumberDown rolls back in one transaction, like the way up. It
// drops every row without a contract (familyDebtorNumberDownSQL): the
// numbers of the families the holiday care knows only are lost.
func familyDebtorNumberDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, familyDebtorNumberDownSQL); err != nil {
			return fmt.Errorf("drop the family debtor numbers without a contract: %w", err)
		}
		return nil
	})
}
