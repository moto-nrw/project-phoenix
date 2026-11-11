package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const familyDebtorNumberVersion = "1.15.937"

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
//     1095 days after its period, #6691).
//
// The trail of such a row hangs on no contract either; the retention
// deletes its entries once their row is gone and they are as old.
const familyDebtorNumberUpSQL = `
ALTER TABLE parent_billing.family_billing
	ALTER COLUMN care_contract_id DROP NOT NULL,
	ADD COLUMN holder_first_name TEXT NOT NULL DEFAULT '',
	ADD COLUMN holder_last_name TEXT NOT NULL DEFAULT '',
	ADD COLUMN last_used_on DATE,
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
COMMENT ON TABLE parent_billing.family_billing IS
	'Deviation and debtor number of one family (#4689, #6692), keyed by the mail address of the registration; goes with the contract it was set from, a row without a contract after its own period (last_used_on).';

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
// another number keeps it. Each new or changed row gets a "save" in its
// trail without an account.
const familyDebtorNumberBackfillSQL = `
WITH registration AS (
	SELECT organization_id, LOWER(BTRIM(guardian_email)) AS family_email, debtor_number,
		guardian_first_name, guardian_last_name, submitted_at, id
	FROM enrollment.holiday_registrations
	WHERE status = 'confirmed' AND debtor_number IS NOT NULL AND BTRIM(guardian_email) <> ''
),
single AS (
	SELECT organization_id, family_email, MIN(debtor_number) AS debtor_number
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
		debtor_number, holder_first_name, holder_last_name, last_used_on)
	SELECT single.organization_id, single.family_email, NULL, 'applicant', single.debtor_number,
		holder.first_name, holder.last_name, CURRENT_DATE
	FROM single
	JOIN holder ON holder.organization_id = single.organization_id AND holder.family_email = single.family_email
	ON CONFLICT (organization_id, family_email) DO UPDATE
		SET debtor_number = EXCLUDED.debtor_number,
			holder_first_name = EXCLUDED.holder_first_name, holder_last_name = EXCLUDED.holder_last_name,
			last_used_on = EXCLUDED.last_used_on, updated_at = CURRENT_TIMESTAMP
		WHERE family.debtor_number IS NULL
	RETURNING family.id, family.organization_id, family.care_contract_id
)
INSERT INTO parent_billing.family_billing_events (organization_id, family_billing_id, care_contract_id, action)
SELECT organization_id, id, care_contract_id, 'save' FROM saved`

// A rollback deletes what has no contract and restores the columns.
const familyDebtorNumberDownSQL = `
DELETE FROM parent_billing.family_billing_events WHERE care_contract_id IS NULL;
DELETE FROM parent_billing.family_billing WHERE care_contract_id IS NULL;
DROP INDEX IF EXISTS parent_billing.idx_family_billing_events_without_contract;
ALTER TABLE parent_billing.family_billing_events
	DROP CONSTRAINT IF EXISTS fk_family_billing_events_organization,
	ALTER COLUMN care_contract_id SET NOT NULL;
ALTER TABLE parent_billing.family_billing
	DROP CONSTRAINT IF EXISTS chk_family_billing_without_contract,
	DROP CONSTRAINT IF EXISTS chk_family_billing_retention,
	DROP CONSTRAINT IF EXISTS chk_family_billing_holder,
	DROP CONSTRAINT IF EXISTS fk_family_billing_organization,
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

func familyDebtorNumberDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, familyDebtorNumberDownSQL); err != nil {
		return fmt.Errorf("drop the family debtor numbers without a contract: %w", err)
	}
	return nil
}
