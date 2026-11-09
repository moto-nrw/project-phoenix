package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Places of the holiday weeks (#4699). Every statement runs in the Träger
// transaction of the Träger it names and names it once more in its filter.
// A placement runs only while the caller holds the week's row, which
// LockHolidayWeeks takes with SELECT ... FOR UPDATE.

// holidayCountingStatuses are the registrations whose bookings take or wait
// for a place: received and confirmed ones.
const holidayCountingStatuses = `('received', 'confirmed')`

const (
	// lockHolidayWeeksQuery locks the weeks in ascending ID: the sort runs
	// below the lock, so two callers never lock the same weeks the other
	// way round.
	lockHolidayWeeksQuery = `SELECT id FROM enrollment.holiday_period_weeks
		WHERE organization_id = ? AND holiday_period_id = ? AND id = ANY(?::bigint[])
		ORDER BY id
		FOR UPDATE`
	// placeHolidayWaitlistQuery gives the free places of a locked week to
	// its waiting list in its order: the time the registration came in, its
	// number, then the position of the child in the form. It counts the
	// placed bookings of received and confirmed registrations. Run in READ
	// COMMITTED after the lock, the statement sees every placement committed
	// before; a NULL capacity places every booking. submitted names the
	// registration being sent or rebooked: its initial bookings (restricted
	// to initialChildID when nonzero) get no promoted_at.
	placeHolidayWaitlistQuery = `WITH week AS (
			SELECT capacity FROM enrollment.holiday_period_weeks
			WHERE organization_id = ? AND id = ?
		), counted AS (
			SELECT booking.id, booking.placement, registration.id AS registration_id, registration.submitted_at,
				child.id AS child_id, child.position
			FROM enrollment.holiday_registration_bookings AS booking
			JOIN enrollment.holiday_registration_children AS child
				ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
			JOIN enrollment.holiday_registrations AS registration
				ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
			WHERE booking.organization_id = ? AND booking.holiday_period_week_id = ?
				AND registration.status IN ` + holidayCountingStatuses + `
		), free AS (
			SELECT week.capacity - (SELECT count(*) FROM counted WHERE counted.placement = 'placed') AS places
			FROM week
		), queue AS (
			SELECT counted.*, row_number() OVER (ORDER BY counted.submitted_at, counted.registration_id, counted.position) AS rank
			FROM counted
			WHERE counted.placement = 'waitlisted'
		)
		UPDATE enrollment.holiday_registration_bookings AS booking
		SET placement = 'placed',
			promoted_at = CASE WHEN queue.registration_id = ? AND (?::bigint = 0 OR queue.child_id = ?) THEN NULL ELSE statement_timestamp() END
		FROM queue, free
		WHERE booking.organization_id = ? AND booking.id = queue.id
			AND (free.places IS NULL OR queue.rank <= free.places)
		RETURNING booking.id AS booking_id, queue.registration_id, queue.child_id, queue.position, booking.holiday_period_week_id`
	setHolidayWeekCapacityQuery = `UPDATE enrollment.holiday_period_weeks SET capacity = ?
		WHERE organization_id = ? AND holiday_period_id = ? AND id = ?`
	holidayWeekPlacesQuery = `SELECT week.id, week.starts_on, week.ends_on, week.host_school_id, week.capacity,
			(SELECT count(*)
				FROM enrollment.holiday_registration_bookings AS booking
				JOIN enrollment.holiday_registration_children AS child
					ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
				JOIN enrollment.holiday_registrations AS registration
					ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
				WHERE booking.organization_id = week.organization_id AND booking.holiday_period_week_id = week.id
					AND booking.placement = 'placed' AND registration.status IN ` + holidayCountingStatuses + `) AS placed
		FROM enrollment.holiday_period_weeks AS week
		WHERE week.organization_id = ? AND week.holiday_period_id = ?
		ORDER BY week.starts_on, week.id`
	holidayWaitlistQuery = `SELECT booking.holiday_period_week_id,
			row_number() OVER (PARTITION BY booking.holiday_period_week_id
				ORDER BY registration.submitted_at, registration.id, child.position) AS position,
			registration.id AS registration_id, registration.status, registration.submitted_at,
			child.id AS child_id, child.first_name, child.last_name, model.name AS time_model_name
		FROM enrollment.holiday_registration_bookings AS booking
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		JOIN enrollment.holiday_week_time_models AS model
			ON model.organization_id = booking.organization_id AND model.id = booking.holiday_week_time_model_id
		WHERE booking.organization_id = ? AND booking.holiday_period_id = ?
			AND booking.placement = 'waitlisted' AND registration.status IN ` + holidayCountingStatuses + `
		ORDER BY booking.holiday_period_week_id, position`
)

// LockHolidayWeeks holds the rows of the weeks of the period weekIDs names,
// in ascending ID, until the transaction ends, and returns the IDs it
// locked: a week of another period or Träger is left out.
func (s *HolidayRegistrationStore) LockHolidayWeeks(ctx context.Context, organizationID, periodID int64, weekIDs []int64) ([]int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var locked []int64
	if err := db.NewRaw(lockHolidayWeeksQuery, organizationID, periodID, pgdialect.Array(weekIDs)).Scan(ctx, &locked); err != nil {
		return nil, fmt.Errorf("enrollment postgres: lock holiday weeks: %w", err)
	}
	return locked, nil
}

// PlaceHolidayWaitlist gives the free places of a week the caller holds to
// its waiting list and returns the bookings that got one. submittedID names
// the registration being sent or rebooked, zero for none. A nonzero
// initialChildID restricts its initial placements to the rebooked child.
func (s *HolidayRegistrationStore) PlaceHolidayWaitlist(ctx context.Context, organizationID, weekID, submittedID, initialChildID int64) ([]enrollment.HolidayPromotion, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var placed []enrollment.HolidayPromotion
	var rows []struct {
		BookingID      int64 `bun:"booking_id"`
		RegistrationID int64 `bun:"registration_id"`
		ChildID        int64 `bun:"child_id"`
		Position       int   `bun:"position"`
		WeekID         int64 `bun:"holiday_period_week_id"`
	}
	if err := db.NewRaw(placeHolidayWaitlistQuery, organizationID, weekID, organizationID, weekID, submittedID, initialChildID, initialChildID, organizationID).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: place holiday waiting list: %w", err)
	}
	for _, row := range rows {
		placed = append(placed, enrollment.HolidayPromotion{
			BookingID: row.BookingID, RegistrationID: row.RegistrationID, ChildID: row.ChildID, ChildPosition: row.Position, WeekID: row.WeekID,
		})
	}
	return placed, nil
}

// SetHolidayWeekCapacity writes the capacity of a week of the period; nil
// is unlimited.
func (s *HolidayRegistrationStore) SetHolidayWeekCapacity(ctx context.Context, organizationID, periodID, weekID int64, capacity *int) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(setHolidayWeekCapacityQuery, capacity, organizationID, periodID, weekID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: set holiday week capacity: %w", err)
	}
	return nil
}

type holidayWeekPlacesRow struct {
	ID           int64         `bun:"id"`
	StartsOn     calendar.Date `bun:"starts_on"`
	EndsOn       calendar.Date `bun:"ends_on"`
	HostSchoolID int64         `bun:"host_school_id"`
	Capacity     *int          `bun:"capacity"`
	Placed       int           `bun:"placed"`
}

type holidayWaitlistRow struct {
	WeekID         int64     `bun:"holiday_period_week_id"`
	Position       int       `bun:"position"`
	RegistrationID int64     `bun:"registration_id"`
	Status         string    `bun:"status"`
	SubmittedAt    time.Time `bun:"submitted_at"`
	ChildID        int64     `bun:"child_id"`
	FirstName      string    `bun:"first_name"`
	LastName       string    `bun:"last_name"`
	TimeModelName  string    `bun:"time_model_name"`
}

// ReadHolidayWeekPlaces reads the places of every week of the period with
// its waiting list in its order: two statements for any number of weeks.
func (s *HolidayRegistrationStore) ReadHolidayWeekPlaces(ctx context.Context, organizationID, periodID int64) ([]enrollment.HolidayWeekPlaces, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var weeks []holidayWeekPlacesRow
	if err := db.NewRaw(holidayWeekPlacesQuery, organizationID, periodID).Scan(ctx, &weeks); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday week places: %w", err)
	}
	var waiting []holidayWaitlistRow
	if err := db.NewRaw(holidayWaitlistQuery, organizationID, periodID).Scan(ctx, &waiting); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday waiting lists: %w", err)
	}
	byWeek := map[int64][]enrollment.HolidayWaitlistEntry{}
	for _, row := range waiting {
		byWeek[row.WeekID] = append(byWeek[row.WeekID], enrollment.HolidayWaitlistEntry{
			Position: row.Position, RegistrationID: row.RegistrationID, RegistrationStatus: row.Status,
			SubmittedAt: row.SubmittedAt, ChildID: row.ChildID, ChildFirstName: row.FirstName, ChildLastName: row.LastName,
			TimeModelName: row.TimeModelName,
		})
	}
	places := make([]enrollment.HolidayWeekPlaces, len(weeks))
	for i, week := range weeks {
		waitlist := byWeek[week.ID]
		if waitlist == nil {
			waitlist = []enrollment.HolidayWaitlistEntry{}
		}
		places[i] = enrollment.HolidayWeekPlaces{
			WeekID: week.ID, StartsOn: week.StartsOn, EndsOn: week.EndsOn, HostSchoolID: week.HostSchoolID,
			Capacity: week.Capacity, Placed: week.Placed, Waitlist: waitlist,
		}
	}
	return places, nil
}
