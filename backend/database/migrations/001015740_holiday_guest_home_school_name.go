package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayGuestHomeSchoolNameVersion = "1.15.740"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayGuestHomeSchoolNameVersion,
		Description: "Feriengast nennt seine Heimatschule in Gruppe, Kinderliste, Aufsicht und Kindakte der ausrichtenden OGS (#5630)",
		DependsOn: []string{
			unusedAccountErasureWithoutWaitVersion, // the last migration before this one
			holidayGuestsVersion,                   // enrollment.holiday_guests
		},
	})
	Migrations.MustRegister(holidayGuestHomeSchoolNameUp, holidayGuestHomeSchoolNameDown)
}

// holidayGuestHomeSchoolNameUpSQL keeps the name of the home school on the
// guest copy (#5630, ADR 0120 Nachtrag zu Punkt 4): the reconciliation writes
// it with the other fields of the copy, so the school's lists read it with
// the child and never ask for the Träger's schools on a request. It goes with
// the copy like home_school_id and guest_school_name. Empty for a child
// without a school of the Träger and for a school that no longer exists.
const holidayGuestHomeSchoolNameUpSQL = `
	ALTER TABLE enrollment.holiday_guests
		ADD COLUMN home_school_name TEXT NOT NULL DEFAULT '';
	COMMENT ON COLUMN enrollment.holiday_guests.home_school_name IS
		'Name of the home school of the Träger the reconciliation copied with the guest; empty without one (#5630).';
`

func holidayGuestHomeSchoolNameUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayGuestHomeSchoolNameUpSQL); err != nil {
		return fmt.Errorf("add the home school name to enrollment.holiday_guests: %w", err)
	}
	return nil
}

func holidayGuestHomeSchoolNameDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `ALTER TABLE enrollment.holiday_guests DROP COLUMN IF EXISTS home_school_name`); err != nil {
		return fmt.Errorf("drop the home school name from enrollment.holiday_guests: %w", err)
	}
	return nil
}
