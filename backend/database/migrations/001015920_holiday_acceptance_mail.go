package migrations

import (
	"context"
	"fmt"
	"slices"

	"github.com/uptrace/bun"
)

const holidayAcceptanceMailVersion = "1.15.920"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayAcceptanceMailVersion,
		Description: "Ferien: Annahme-Mail an Eltern beim Bestätigen, mit Text des Trägers (#6672)",
		DependsOn: []string{
			dataReturnErasureOrderLockVersion,           // the newest migration on development
			holidayClosureNoticesVersion,                // the outbox CHECK this one extends
			parentBillingDiscardEmptyHolidayRunsVersion, // the erasure this one extends
		},
	})
	Migrations.MustRegister(holidayAcceptanceMailUp, holidayAcceptanceMailDown)
}

// holidayAcceptanceMailUpSQL keeps the text the Träger writes for the mail
// that accepts a registration of the period (#6672). Plain text, empty for
// none; the CHECK bounds it like the application does.
const holidayAcceptanceMailUpSQL = `
ALTER TABLE enrollment.holiday_periods
	ADD COLUMN acceptance_mail_text TEXT NOT NULL DEFAULT ''
		CONSTRAINT chk_holiday_periods_acceptance_mail_text CHECK (char_length(acceptance_mail_text) <= 2000);
COMMENT ON COLUMN enrollment.holiday_periods.acceptance_mail_text IS
	'Text of the Träger in the mail that accepts a registration of the period (#6672); plain text, empty for none.';
`

const holidayAcceptanceMailDownSQL = `ALTER TABLE enrollment.holiday_periods DROP COLUMN IF EXISTS acceptance_mail_text;`

// The parent who registered gets a mail when the Träger-Büro confirms the
// registration (#6672). It is a mail of the Träger, so the CHECK of 1.15.887
// declares its kind, and the demo erasure deletes it with the registration
// it names, like the receipt.
var holidayAcceptanceEmailOutboxTraegerKinds = slices.Sorted(slices.Values(append(slices.Clone(holidayClosureEmailOutboxTraegerKinds), "holiday_registration_accepted")))

const holidayAcceptanceEmailOutboxTraegerKindCheckSQL = `
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_traeger_kind;
ALTER TABLE platform.email_outbox ADD CONSTRAINT chk_email_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract', 'care_contract_notice', 'holiday_closure_notice', 'holiday_place_granted', 'holiday_registration_accepted',
		'holiday_registration_received', 'parent_billing_letter')) NOT VALID;
COMMENT ON CONSTRAINT chk_email_outbox_traeger_kind ON platform.email_outbox IS
	'The mail kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
`

// holidayAcceptanceRestoredKindCheckSQL is the CHECK of 1.15.887, which a
// rollback writes back.
const holidayAcceptanceRestoredKindCheckSQL = `
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_traeger_kind;
ALTER TABLE platform.email_outbox ADD CONSTRAINT chk_email_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract', 'care_contract_notice', 'holiday_closure_notice', 'holiday_place_granted', 'holiday_registration_received',
		'parent_billing_letter')) NOT VALID;
COMMENT ON CONSTRAINT chk_email_outbox_traeger_kind ON platform.email_outbox IS
	'The mail kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
`

const validateHolidayAcceptanceKindCheckSQL = `ALTER TABLE platform.email_outbox VALIDATE CONSTRAINT chk_email_outbox_traeger_kind`

// A rollback deletes the mails the CHECK of 1.15.887 would refuse.
const deleteHolidayAcceptanceMailsSQL = `DELETE FROM platform.email_outbox
	WHERE organization_id IS NOT NULL AND kind = 'holiday_registration_accepted'`

func holidayAcceptanceMailUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayAcceptanceMailUpSQL); err != nil {
		return fmt.Errorf("add the text of the holiday acceptance mail: %w", err)
	}
	// As in 1.15.878: the ADD holds the table only for the catalog change,
	// the VALIDATE scans it in a statement of its own.
	if _, err := db.ExecContext(ctx, holidayAcceptanceEmailOutboxTraegerKindCheckSQL); err != nil {
		return fmt.Errorf("declare the holiday acceptance mail of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, validateHolidayAcceptanceKindCheckSQL); err != nil {
		return fmt.Errorf("validate the mail kinds of a Träger: %w", err)
	}
	return installHolidayAcceptanceErasure(ctx, db)
}

func holidayAcceptanceMailDown(ctx context.Context, db *bun.DB) error {
	if err := installEmptyRunErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, deleteHolidayAcceptanceMailsSQL); err != nil {
		return fmt.Errorf("delete the holiday acceptance mails: %w", err)
	}
	if _, err := db.ExecContext(ctx, holidayAcceptanceRestoredKindCheckSQL); err != nil {
		return fmt.Errorf("restore the mail kinds of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, validateHolidayAcceptanceKindCheckSQL); err != nil {
		return fmt.Errorf("validate the mail kinds of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, holidayAcceptanceMailDownSQL); err != nil {
		return fmt.Errorf("drop the text of the holiday acceptance mail: %w", err)
	}
	return nil
}
