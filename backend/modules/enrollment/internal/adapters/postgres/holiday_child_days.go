package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The bookings children hold in a period across their registrations
// (#6446): the bound of care days per child counts them. A child is an
// enrollment.HolidayChildKey. One statement reads the bookings of every
// asked key, so the count costs one roundtrip however many children a
// registration names. It runs in the Träger transaction and names the
// Träger once more in its filter; the joins carry it on to the
// registration and the bookings, so it stays with its Träger beneath RLS as
// well. The lookup by period, name and date of birth uses
// idx_holiday_registration_children_duplicate.

// holidayChildKeyBookingsQuery reads every booking of the asked children in
// the registrations of the period that count, placed, waiting or waiting for
// the office alike, except those of the child row exceptChildID. The keys
// come lowercased and trimmed (enrollment.NewHolidayChildKey). Deduplicate
// lookup keys before joining so each stored booking is returned once.
const holidayChildKeyBookingsQuery = `SELECT wanted.guardian_email, wanted.first_name, wanted.last_name, wanted.birth_date,
		booking.holiday_period_week_id, booking.holiday_week_time_model_id, booking.days::text AS days
	FROM (SELECT DISTINCT * FROM unnest(?::text[], ?::text[], ?::text[], ?::date[])) AS wanted(guardian_email, first_name, last_name, birth_date)
	JOIN enrollment.holiday_registration_children AS child
		ON lower(child.first_name) = lower(wanted.first_name) AND lower(child.last_name) = lower(wanted.last_name)
			AND child.birth_date = wanted.birth_date
	JOIN enrollment.holiday_registrations AS registration
		ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
	JOIN enrollment.holiday_registration_bookings AS booking
		ON booking.organization_id = child.organization_id AND booking.holiday_registration_child_id = child.id
	WHERE child.organization_id = ? AND child.holiday_period_id = ?
		AND lower(btrim(registration.guardian_email)) = lower(wanted.guardian_email)
		AND registration.status IN ` + holidayCountingStatuses + `
		AND child.id <> ?
	ORDER BY booking.id`

type holidayChildBookingRow struct {
	GuardianEmail string        `bun:"guardian_email"`
	FirstName     string        `bun:"first_name"`
	LastName      string        `bun:"last_name"`
	BirthDate     calendar.Date `bun:"birth_date"`
	WeekID        int64         `bun:"holiday_period_week_id"`
	TimeModelID   int64         `bun:"holiday_week_time_model_id"`
	Days          *string       `bun:"days"`
}

// ReadHolidayChildBookings reads the bookings of the children keys names in
// the period, over all their registrations that count, except those of the
// child row exceptChildID (0 for none), by child. No key reads nothing.
func (s *HolidayRegistrationStore) ReadHolidayChildBookings(ctx context.Context, organizationID, periodID int64,
	keys []enrollment.HolidayChildKey, exceptChildID int64) (map[enrollment.HolidayChildKey][]enrollment.HolidayBookingInput, error) {
	bookings := map[enrollment.HolidayChildKey][]enrollment.HolidayBookingInput{}
	if len(keys) == 0 {
		return bookings, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	emails, firsts, lasts, births := make([]string, len(keys)), make([]string, len(keys)), make([]string, len(keys)), make([]string, len(keys))
	for i, key := range keys {
		emails[i], firsts[i], lasts[i], births[i] = key.GuardianEmail, key.FirstName, key.LastName, key.BirthDate.String()
	}
	var rows []holidayChildBookingRow
	if err := db.NewRaw(holidayChildKeyBookingsQuery, pgdialect.Array(emails), pgdialect.Array(firsts), pgdialect.Array(lasts), pgdialect.Array(births),
		organizationID, periodID, exceptChildID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday child bookings: %w", err)
	}
	for _, row := range rows {
		days, err := holidayDaysOf(row.Days)
		if err != nil {
			return nil, err
		}
		key := enrollment.HolidayChildKey{GuardianEmail: row.GuardianEmail, FirstName: row.FirstName, LastName: row.LastName, BirthDate: row.BirthDate}
		bookings[key] = append(bookings[key], enrollment.HolidayBookingInput{WeekID: row.WeekID, TimeModelID: row.TimeModelID, Days: days})
	}
	return bookings, nil
}
