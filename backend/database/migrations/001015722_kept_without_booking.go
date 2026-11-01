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
// hand row whose child lost a booking on a weekday the row plans; a date on
// or after the day that booking ended hides it. A later booking end shows
// the hint again. Booking-created rows never carry it.
const keptWithoutBookingUpSQL = `
		ALTER TABLE activities.student_enrollments
			ADD COLUMN IF NOT EXISTS kept_without_booking_through DATE;

		COMMENT ON COLUMN activities.student_enrollments.kept_without_booking_through IS
			'Hand rows only: the school keeps the child planned although bookings ending up to this day no longer carry the Regeltermin (#5102).';
`

func keptWithoutBookingUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, keptWithoutBookingUpSQL); err != nil {
		return fmt.Errorf("add activities.student_enrollments.kept_without_booking_through: %w", err)
	}
	return nil
}

func keptWithoutBookingDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `ALTER TABLE activities.student_enrollments DROP COLUMN IF EXISTS kept_without_booking_through;`); err != nil {
		return fmt.Errorf("drop activities.student_enrollments.kept_without_booking_through: %w", err)
	}
	return nil
}
