package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayDebtorNumberVersion = "1.15.739"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayDebtorNumberVersion,
		Description: "Ferienanmeldung: Debitorennummer der Familie beim Bestätigen, Kopie in der Rechnung (#5621)",
		DependsOn: []string{
			holidayGuardianAddressVersion,   // the last migration before this one
			holidayHealthDataVersion,        // the audit actions this extends
			parentBillingHolidayRunsVersion, // the invoices this extends
		},
	})
	Migrations.MustRegister(holidayDebtorNumberUp, holidayDebtorNumberDown)
}

// holidayDebtorNumberUpSQL adds the debtor number of the family to the
// registration and its copy to the invoice (ADR 0121 and ADR 0122 addenda,
// #5621). The Träger-Büro enters it when it confirms the registration; the
// letter names it as the reference to pay with. Existing registrations and
// invoices have none. The number stays on the registration, under its Träger
// RLS and its deletion after 92 days (ADR 0118); the invoice keeps its own
// copy and its trigger against changes. Each change of the number is an
// entry "debtor_number" in the registration's audit trail.
const holidayDebtorNumberUpSQL = `
ALTER TABLE enrollment.holiday_registrations
	ADD COLUMN debtor_number VARCHAR(35),
	ADD CONSTRAINT chk_holiday_registrations_debtor_number CHECK (
		debtor_number IS NULL OR (debtor_number <> '' AND debtor_number = btrim(debtor_number) AND debtor_number !~ '[\r\n]')
	);
COMMENT ON COLUMN enrollment.holiday_registrations.debtor_number IS
	'Debtor number of the family in the finance system of the Träger, entered at the confirmation (#5621); the reference to pay with.';

ALTER TABLE parent_billing.documents
	ADD COLUMN debtor_number VARCHAR(35),
	ADD CONSTRAINT chk_documents_debtor_number CHECK (
		debtor_number IS NULL OR (debtor_number <> '' AND debtor_number = btrim(debtor_number) AND debtor_number !~ '[\r\n]')
	);
COMMENT ON COLUMN parent_billing.documents.debtor_number IS
	'Copy of the debtor number of the registration at the release (#5621); stays when the registration goes.';

ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'debtor_number'));
`

const holidayDebtorNumberDownSQL = `
DELETE FROM enrollment.holiday_registration_audit WHERE action = 'debtor_number';
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health'));
ALTER TABLE parent_billing.documents
	DROP CONSTRAINT chk_documents_debtor_number,
	DROP COLUMN debtor_number;
ALTER TABLE enrollment.holiday_registrations
	DROP CONSTRAINT chk_holiday_registrations_debtor_number,
	DROP COLUMN debtor_number;
`

func holidayDebtorNumberUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayDebtorNumberUpSQL); err != nil {
		return fmt.Errorf("add holiday debtor number: %w", err)
	}
	return nil
}

func holidayDebtorNumberDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayDebtorNumberDownSQL); err != nil {
		return fmt.Errorf("drop holiday debtor number: %w", err)
	}
	return nil
}
