package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayHealthRequirementVersion = "1.15.958"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayHealthRequirementVersion,
		Description: "Ferienanmeldung: Allergien und Medikamente je Formularversion freiwillig oder Pflicht (#6686)",
		DependsOn: []string{
			holidayHomeSchoolBookingsVersion, // the newest migration on development (1.15.957)
			holidayPeriodFormsVersion,        // the versions of the form this extends
			holidayGuardianAddressVersion,    // the building block "Anschrift" this one sits beside
		},
	})
	Migrations.MustRegister(holidayHealthRequirementUp, holidayHealthRequirementDown)
}

// holidayHealthRequirementUpSQL adds the building block "Gesundheit" to each
// version of a holiday form. Existing versions keep allergies and
// medication optional, as they were; a registration keeps the version it
// was submitted with, so no submitted registration changes.
const holidayHealthRequirementUpSQL = `
ALTER TABLE enrollment.holiday_period_forms
	ADD COLUMN health_requirement TEXT NOT NULL DEFAULT 'optional',
	ADD CONSTRAINT chk_holiday_period_forms_health_requirement
		CHECK (health_requirement IN ('optional', 'required'));
COMMENT ON COLUMN enrollment.holiday_period_forms.health_requirement IS
	'Building block "Gesundheit" of the version (#6686): optional or required allergies and medication of each child; required takes "Keine" as an answer.';
`

const holidayHealthRequirementDownSQL = `
ALTER TABLE enrollment.holiday_period_forms
	DROP CONSTRAINT chk_holiday_period_forms_health_requirement,
	DROP COLUMN health_requirement;
`

func holidayHealthRequirementUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayHealthRequirementUpSQL); err != nil {
		return fmt.Errorf("add holiday health requirement: %w", err)
	}
	return nil
}

func holidayHealthRequirementDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayHealthRequirementDownSQL); err != nil {
		return fmt.Errorf("drop holiday health requirement: %w", err)
	}
	return nil
}
