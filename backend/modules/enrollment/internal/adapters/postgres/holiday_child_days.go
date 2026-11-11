package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The bookings one child holds in a period across its registrations
// (#6446): the bound of care days per child counts them. The child is an
// enrollment.HolidayChildKey. The statement runs in the Träger transaction
// and names the Träger once more in its filter; the joins carry it on to the
// registration and the bookings, so it stays with its Träger beneath RLS as
// well. The lookup by period, name and date of
// birth uses idx_holiday_registration_children_duplicate.

// holidayChildKeyBookingsQuery reads every booking of the child in the
// registrations of the period that count, placed, waiting or waiting for
// the office alike, except those of the child row exceptChildID.
const holidayChildKeyBookingsQuery = `SELECT booking.holiday_period_week_id, booking.holiday_week_time_model_id,
		booking.days::text AS days
	FROM enrollment.holiday_registration_children AS child
	JOIN enrollment.holiday_registrations AS registration
		ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
	JOIN enrollment.holiday_registration_bookings AS booking
		ON booking.organization_id = child.organization_id AND booking.holiday_registration_child_id = child.id
	WHERE child.organization_id = ? AND child.holiday_period_id = ?
		AND lower(child.first_name) = lower(?) AND lower(child.last_name) = lower(?) AND child.birth_date = ?
		AND lower(btrim(registration.guardian_email)) = lower(?)
		AND registration.status IN ` + holidayCountingStatuses + `
		AND child.id <> ?
	ORDER BY booking.id`

type holidayChildBookingRow struct {
	WeekID      int64   `bun:"holiday_period_week_id"`
	TimeModelID int64   `bun:"holiday_week_time_model_id"`
	Days        *string `bun:"days"`
}

// ReadHolidayChildBookings reads the bookings of the child key names in the
// period, over all its registrations that count, except those of the child
// row exceptChildID (0 for none).
func (s *HolidayRegistrationStore) ReadHolidayChildBookings(ctx context.Context, organizationID, periodID int64, key enrollment.HolidayChildKey, exceptChildID int64) ([]enrollment.HolidayBookingInput, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []holidayChildBookingRow
	if err := db.NewRaw(holidayChildKeyBookingsQuery, organizationID, periodID,
		key.FirstName, key.LastName, key.BirthDate, key.GuardianEmail, exceptChildID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday child bookings: %w", err)
	}
	bookings := make([]enrollment.HolidayBookingInput, len(rows))
	for i, row := range rows {
		days, err := holidayDaysOf(row.Days)
		if err != nil {
			return nil, err
		}
		bookings[i] = enrollment.HolidayBookingInput{WeekID: row.WeekID, TimeModelID: row.TimeModelID, Days: days}
	}
	return bookings, nil
}
