package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	keptWithoutBookingVersion     = "1.15.722"
	keptWithoutBookingDescription = "Regeltermin ohne Buchung: Kind bleibt von Hand eingeplant, Hinweis ausblenden (#5102)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     keptWithoutBookingVersion,
		Description: keptWithoutBookingDescription,
		DependsOn: []string{
			schoolVacationsVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(keptWithoutBookingUp, keptWithoutBookingDown)
}

// keptWithoutBookingUpSQL records on a hand roster row that the school keeps
// the child planned although a booking of that weekday ended (ADR 0128,
// addendum for #5102). The hint "Ohne passende Buchung eingeplant" names a
// hand row whose child lost a booking on a weekday the row plans. The
// decision is kept per ISO weekday, {"4": "2026-11-02"}: a date on or after
// the day that weekday's booking ended hides it; a later booking end shows
// the hint again. Booking-created rows never carry it.
const keptWithoutBookingUpSQL = `
		ALTER TABLE activities.student_enrollments
			ADD COLUMN IF NOT EXISTS kept_without_booking JSONB;

		COMMENT ON COLUMN activities.student_enrollments.kept_without_booking IS
			'Hand rows only: per ISO weekday ("1".."7") the day up to which ended bookings no longer carrying the Regeltermin are accepted; the school keeps the child planned (#5102).';
`

func keptWithoutBookingUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, keptWithoutBookingUpSQL); err != nil {
		return fmt.Errorf("add activities.student_enrollments.kept_without_booking: %w", err)
	}
	return nil
}

func keptWithoutBookingDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `ALTER TABLE activities.student_enrollments DROP COLUMN IF EXISTS kept_without_booking;`); err != nil {
		return fmt.Errorf("drop activities.student_enrollments.kept_without_booking: %w", err)
	}
	return nil
}
