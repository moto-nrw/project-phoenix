package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidaySecondGuardianVersion = "1.15.694"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidaySecondGuardianVersion,
		Description: "Ferienanmeldung: freiwillige zweite sorgeberechtigte Person (#5197)",
		DependsOn:   []string{holidayRegistrationReviewVersion},
	})
	Migrations.MustRegister(holidaySecondGuardianUp, holidaySecondGuardianDown)
}

// Nullable columns preserve existing registrations and the period's public
// link. The data stays on the registration, under its Träger RLS and retention.
const holidaySecondGuardianUpSQL = `
ALTER TABLE enrollment.holiday_registrations
 ADD COLUMN second_guardian_first_name VARCHAR(100),
 ADD COLUMN second_guardian_last_name VARCHAR(100),
 ADD COLUMN second_guardian_email VARCHAR(254),
 ADD COLUMN second_guardian_phone VARCHAR(40),
 ADD CONSTRAINT chk_holiday_registrations_second_guardian CHECK (
  (second_guardian_first_name IS NULL AND second_guardian_last_name IS NULL
   AND second_guardian_email IS NULL AND second_guardian_phone IS NULL)
  OR (second_guardian_first_name IS NOT NULL AND btrim(second_guardian_first_name) <> ''
   AND second_guardian_last_name IS NOT NULL AND btrim(second_guardian_last_name) <> '')
 );
`

const holidaySecondGuardianDownSQL = `
ALTER TABLE enrollment.holiday_registrations
 DROP CONSTRAINT chk_holiday_registrations_second_guardian,
 DROP COLUMN second_guardian_first_name,
 DROP COLUMN second_guardian_last_name,
 DROP COLUMN second_guardian_email,
 DROP COLUMN second_guardian_phone;
`

func holidaySecondGuardianUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidaySecondGuardianUpSQL); err != nil {
		return fmt.Errorf("add holiday second guardian: %w", err)
	}
	return nil
}

func holidaySecondGuardianDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidaySecondGuardianDownSQL); err != nil {
		return fmt.Errorf("drop holiday second guardian: %w", err)
	}
	return nil
}
