package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayClosureNoticesVersion = "1.15.887"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayClosureNoticesVersion,
		Description: "Ferien: Eltern erfahren von Schließtagen in gebuchten Wochen, je Tag nur einmal (#6434)",
		DependsOn: []string{
			parentBillingDatevExportFilesVersion,  // the newest migration on development
			holidayWeekCapacityVersion,            // the outbox CHECK this one extends
			parentBillingCareNoticeLettersVersion, // the erasure this one extends
		},
	})
	Migrations.MustRegister(holidayClosureNoticesUp, holidayClosureNoticesDown)
}

// holidayClosureNoticesUpSQL records the closed days of a booked holiday week
// the parents of a registration were told about (#6434): a closing day the
// Träger or the host school planned after the booking. One row per
// registration, week and day, so a day is told once, however often the
// closing day is saved. The rows belong to the Träger (ADR 0105) and go with
// their registration or week; they hold no more than the IDs and the day.
const holidayClosureNoticesUpSQL = `
CREATE TABLE enrollment.holiday_closure_notices (
	organization_id BIGINT NOT NULL,
	holiday_registration_id BIGINT NOT NULL,
	holiday_period_week_id BIGINT NOT NULL,
	closed_on DATE NOT NULL,
	notified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (organization_id, holiday_registration_id, holiday_period_week_id, closed_on),
	CONSTRAINT fk_holiday_closure_notices_registration FOREIGN KEY (organization_id, holiday_registration_id)
		REFERENCES enrollment.holiday_registrations (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_holiday_closure_notices_week FOREIGN KEY (organization_id, holiday_period_week_id)
		REFERENCES enrollment.holiday_period_weeks (organization_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_holiday_closure_notices_week ON enrollment.holiday_closure_notices (organization_id, holiday_period_week_id);
COMMENT ON TABLE enrollment.holiday_closure_notices IS
	'Closed day of a booked holiday week the parents of a registration were told about (#6434); each day is told once.';

ALTER TABLE enrollment.holiday_closure_notices ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_closure_notices FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_closure_notices ON enrollment.holiday_closure_notices FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
GRANT SELECT, INSERT ON enrollment.holiday_closure_notices TO phoenix_tenant;
`

const holidayClosureNoticesDownSQL = `DROP TABLE IF EXISTS enrollment.holiday_closure_notices;`

// The parents get a mail when a closing day falls into a holiday week they
// booked (#6434). It is a mail of the Träger, so the CHECK of 1.15.878
// declares its kind, and the demo erasure deletes it with the registration it
// names, like the receipt and the mail that a booking moved up.
var holidayClosureEmailOutboxTraegerKinds = []string{"care_contract", "care_contract_notice", "holiday_closure_notice", "holiday_place_granted",
	"holiday_registration_received", "parent_billing_letter"}

const holidayClosureEmailOutboxTraegerKindCheckSQL = `
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_traeger_kind;
ALTER TABLE platform.email_outbox ADD CONSTRAINT chk_email_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract', 'care_contract_notice', 'holiday_closure_notice', 'holiday_place_granted', 'holiday_registration_received',
		'parent_billing_letter')) NOT VALID;
COMMENT ON CONSTRAINT chk_email_outbox_traeger_kind ON platform.email_outbox IS
	'The mail kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
`

// A rollback deletes the mails the CHECK of 1.15.878 would refuse.
const deleteHolidayClosureMailsSQL = `DELETE FROM platform.email_outbox
	WHERE organization_id IS NOT NULL AND kind = 'holiday_closure_notice'`

// holidayClosureRestoredKindCheckSQL is the CHECK of 1.15.878, which a
// rollback writes back.
const holidayClosureRestoredKindCheckSQL = `
ALTER TABLE platform.email_outbox DROP CONSTRAINT IF EXISTS chk_email_outbox_traeger_kind;
ALTER TABLE platform.email_outbox ADD CONSTRAINT chk_email_outbox_traeger_kind CHECK (
	organization_id IS NULL OR kind IN (
		'care_contract', 'care_contract_notice', 'holiday_place_granted', 'holiday_registration_received', 'parent_billing_letter')) NOT VALID;
COMMENT ON CONSTRAINT chk_email_outbox_traeger_kind ON platform.email_outbox IS
	'The mail kinds of a Träger. A new kind needs its demo erasure in platform.erase_closed_demo_visitors (#6342).';
`

const validateHolidayClosureKindCheckSQL = `ALTER TABLE platform.email_outbox VALIDATE CONSTRAINT chk_email_outbox_traeger_kind`

func holidayClosureNoticesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayClosureNoticesUpSQL); err != nil {
		return fmt.Errorf("create the holiday closure notices: %w", err)
	}
	// As in 1.15.878: the ADD holds the table only for the catalog change,
	// the VALIDATE scans it in a statement of its own.
	if _, err := db.ExecContext(ctx, holidayClosureEmailOutboxTraegerKindCheckSQL); err != nil {
		return fmt.Errorf("declare the holiday closure mail of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, validateHolidayClosureKindCheckSQL); err != nil {
		return fmt.Errorf("validate the mail kinds of a Träger: %w", err)
	}
	return installHolidayClosureErasure(ctx, db)
}

func holidayClosureNoticesDown(ctx context.Context, db *bun.DB) error {
	if err := installCareNoticeLetterErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, deleteHolidayClosureMailsSQL); err != nil {
		return fmt.Errorf("delete the holiday closure mails: %w", err)
	}
	if _, err := db.ExecContext(ctx, holidayClosureRestoredKindCheckSQL); err != nil {
		return fmt.Errorf("restore the mail kinds of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, validateHolidayClosureKindCheckSQL); err != nil {
		return fmt.Errorf("validate the mail kinds of a Träger: %w", err)
	}
	if _, err := db.ExecContext(ctx, holidayClosureNoticesDownSQL); err != nil {
		return fmt.Errorf("drop the holiday closure notices: %w", err)
	}
	return nil
}
