package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayGuardianAddressVersion = "1.15.738"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayGuardianAddressVersion,
		Description: "Ferienanmeldung: schaltbare Anschrift des Elternteils, Kopie in der Rechnung (#5625)",
		DependsOn: []string{
			holidayRegistrationDeclarationVersion, // the last migration before this one
			holidayPeriodFormsVersion,             // the versions of the form this extends
			holidaySecondGuardianVersion,          // the registration columns this extends
			parentBillingHolidayRunsVersion,       // the invoices this extends
		},
	})
	Migrations.MustRegister(holidayGuardianAddressUp, holidayGuardianAddressDown)
}

// holidayGuardianAddressUpSQL adds the building block "Anschrift" to each
// version of a holiday form, the postal address to the registration and
// its copy to the invoice (ADR 0121 and ADR 0122 addenda). Existing
// versions ask for no address; existing registrations and invoices have
// none. The address stays on the registration, under its Träger RLS and
// its deletion after 92 days (ADR 0118); the invoice keeps its own copy
// and its trigger against changes.
const holidayGuardianAddressUpSQL = `
ALTER TABLE enrollment.holiday_period_forms
	ADD COLUMN address_requirement TEXT NOT NULL DEFAULT 'off',
	ADD CONSTRAINT chk_holiday_period_forms_address_requirement
		CHECK (address_requirement IN ('off', 'optional', 'required'));
COMMENT ON COLUMN enrollment.holiday_period_forms.address_requirement IS
	'Building block "Anschrift" of the version (#5625): off asks for no postal address, optional or required ask for it.';

ALTER TABLE enrollment.holiday_registrations
	ADD COLUMN guardian_address_street VARCHAR(100),
	ADD COLUMN guardian_address_postal_code VARCHAR(5),
	ADD COLUMN guardian_address_city VARCHAR(100),
	ADD CONSTRAINT chk_holiday_registrations_guardian_address CHECK (
		(guardian_address_street IS NULL AND guardian_address_postal_code IS NULL AND guardian_address_city IS NULL)
		OR (guardian_address_street IS NOT NULL AND guardian_address_postal_code IS NOT NULL AND guardian_address_city IS NOT NULL
			AND btrim(guardian_address_street) <> '' AND guardian_address_postal_code ~ '^[0-9]{5}$' AND btrim(guardian_address_city) <> '')
	);
COMMENT ON COLUMN enrollment.holiday_registrations.guardian_address_street IS
	'Street and house number of the parent who registered, when the form asked for it (#5625); all three parts or none.';

ALTER TABLE parent_billing.documents
	ADD COLUMN recipient_address_street VARCHAR(100),
	ADD COLUMN recipient_address_postal_code VARCHAR(5),
	ADD COLUMN recipient_address_city VARCHAR(100),
	ADD CONSTRAINT chk_documents_recipient_address CHECK (
		(recipient_address_street IS NULL AND recipient_address_postal_code IS NULL AND recipient_address_city IS NULL)
		OR (recipient_address_street IS NOT NULL AND recipient_address_postal_code IS NOT NULL AND recipient_address_city IS NOT NULL
			AND btrim(recipient_address_street) <> '' AND recipient_address_postal_code ~ '^[0-9]{5}$' AND btrim(recipient_address_city) <> '')
	);
COMMENT ON COLUMN parent_billing.documents.recipient_address_street IS
	'Copy of the postal address of the registration at the release (#5625); stays when the registration goes.';
`

const holidayGuardianAddressDownSQL = `
ALTER TABLE parent_billing.documents
	DROP CONSTRAINT chk_documents_recipient_address,
	DROP COLUMN recipient_address_street,
	DROP COLUMN recipient_address_postal_code,
	DROP COLUMN recipient_address_city;
ALTER TABLE enrollment.holiday_registrations
	DROP CONSTRAINT chk_holiday_registrations_guardian_address,
	DROP COLUMN guardian_address_street,
	DROP COLUMN guardian_address_postal_code,
	DROP COLUMN guardian_address_city;
ALTER TABLE enrollment.holiday_period_forms
	DROP CONSTRAINT chk_holiday_period_forms_address_requirement,
	DROP COLUMN address_requirement;
`

func holidayGuardianAddressUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayGuardianAddressUpSQL); err != nil {
		return fmt.Errorf("add holiday guardian address: %w", err)
	}
	return nil
}

func holidayGuardianAddressDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayGuardianAddressDownSQL); err != nil {
		return fmt.Errorf("drop holiday guardian address: %w", err)
	}
	return nil
}
