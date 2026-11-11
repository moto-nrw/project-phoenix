package migrations

import (
	"context"
	"fmt"
	"slices"

	"github.com/uptrace/bun"
)

const holidayRegistrationParentAccountVersion = "1.15.939"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayRegistrationParentAccountVersion,
		Description: "Ferienanmeldung: Elternkonto an der Anmeldung und Angebot eines Kontos per Mail (#6673, ADR 0216)",
		DependsOn: []string{
			traegerParentAccessVersion,   // the newest migration on this branch (1.15.938)
			holidayAcceptanceMailVersion, // the outbox CHECK and the erasure this one extends (1.15.927)
		},
	})
	Migrations.MustRegister(holidayRegistrationParentAccountUp, holidayRegistrationParentAccountDown)
}

// holidayRegistrationParentAccountUpSQL links a holiday registration to the
// parent account it belongs to (ADR 0216 points 1 to 4 and 10):
//
//   - parent_account_id names the account; ON DELETE SET NULL keeps the
//     registration when the account goes. The link lives in the registration
//     row and goes with it when the retention deletes it (ADR 0118).
//   - parent_account_requested records that the parent asked for an account
//     at the end of the form; the offer mail went out with the registration.
//   - idx_holiday_registrations_parent_account serves the read of the
//     account's registrations, idx_holiday_registrations_unclaimed_email the
//     claim by the account's address of the registrations without an
//     account.
const holidayRegistrationParentAccountUpSQL = `
ALTER TABLE enrollment.holiday_registrations
	ADD COLUMN parent_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	ADD COLUMN parent_account_requested BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX idx_holiday_registrations_parent_account ON enrollment.holiday_registrations (parent_account_id, submitted_at DESC)
	WHERE parent_account_id IS NOT NULL;
CREATE INDEX idx_holiday_registrations_unclaimed_email ON enrollment.holiday_registrations (LOWER(guardian_email))
	WHERE parent_account_id IS NULL;
COMMENT ON COLUMN enrollment.holiday_registrations.parent_account_id IS
	'Parent account the registration belongs to (ADR 0216): from the sealed proof of the prefill, or claimed by the account''s address; goes with the registration.';
COMMENT ON COLUMN enrollment.holiday_registrations.parent_account_requested IS
	'The parent asked for a parent account at the end of the form (ADR 0216); the offer mail went out with the registration.';
`

const holidayRegistrationParentAccountDownSQL = `
ALTER TABLE enrollment.holiday_registrations
	DROP COLUMN parent_account_requested,
	DROP COLUMN parent_account_id;
`

// The offer of a parent account (ADR 0216 point 4) is a mail of the Träger,
// so the CHECK of 1.15.927 declares its kind on top of the acceptance mail,
// and the demo erasure deletes it with the registration it names, like the
// receipt.
var holidayParentAccountOfferTraegerKinds = slices.Sorted(slices.Values(append(slices.Clone(holidayAcceptanceEmailOutboxTraegerKinds), "holiday_parent_account_offer")))

const holidayParentAccountOfferKindCheckSQL = `
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_traeger_kind;
ALTER TABLE platform.email_outbox ADD CONSTRAINT chk_email_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract', 'care_contract_notice', 'holiday_closure_notice', 'holiday_parent_account_offer', 'holiday_place_granted',
		'holiday_registration_accepted', 'holiday_registration_received', 'parent_billing_letter')) NOT VALID;
COMMENT ON CONSTRAINT chk_email_outbox_traeger_kind ON platform.email_outbox IS
	'The mail kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
`

// A rollback deletes the offers, which the CHECK of 1.15.927 refuses, and
// writes that CHECK back.
const deleteHolidayParentAccountOffersSQL = `DELETE FROM platform.email_outbox
	WHERE organization_id IS NOT NULL AND kind = 'holiday_parent_account_offer'`

// holidayParentAccountRestoredKindCheckSQL is the CHECK of 1.15.927, which a
// rollback writes back. The literals stay in this file so the architecture
// check reads the migration's SQL.
const holidayParentAccountRestoredKindCheckSQL = `
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_traeger_kind;
ALTER TABLE platform.email_outbox ADD CONSTRAINT chk_email_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract', 'care_contract_notice', 'holiday_closure_notice', 'holiday_place_granted', 'holiday_registration_accepted',
		'holiday_registration_received', 'parent_billing_letter')) NOT VALID;
COMMENT ON CONSTRAINT chk_email_outbox_traeger_kind ON platform.email_outbox IS
	'The mail kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
`

const validateHolidayParentAccountKindCheckSQL = `ALTER TABLE platform.email_outbox VALIDATE CONSTRAINT chk_email_outbox_traeger_kind`

func holidayRegistrationParentAccountUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationParentAccountUpSQL); err != nil {
		return fmt.Errorf("link holiday registrations to parent accounts: %w", err)
	}
	// As in 1.15.887: the ADD holds the table only for the catalog change,
	// the VALIDATE scans it in a statement of its own.
	if _, err := db.ExecContext(ctx, holidayParentAccountOfferKindCheckSQL); err != nil {
		return fmt.Errorf("declare the parent account offer of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, validateHolidayParentAccountKindCheckSQL); err != nil {
		return fmt.Errorf("validate the mail kinds of a Träger: %w", err)
	}
	return installParentAccountOfferErasure(ctx, db)
}

func holidayRegistrationParentAccountDown(ctx context.Context, db *bun.DB) error {
	if err := installHolidayAcceptanceErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, deleteHolidayParentAccountOffersSQL); err != nil {
		return fmt.Errorf("delete the parent account offers: %w", err)
	}
	if _, err := db.ExecContext(ctx, holidayParentAccountRestoredKindCheckSQL); err != nil {
		return fmt.Errorf("restore the mail kinds of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, validateHolidayParentAccountKindCheckSQL); err != nil {
		return fmt.Errorf("validate the mail kinds of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, holidayRegistrationParentAccountDownSQL); err != nil {
		return fmt.Errorf("unlink holiday registrations from parent accounts: %w", err)
	}
	return nil
}
