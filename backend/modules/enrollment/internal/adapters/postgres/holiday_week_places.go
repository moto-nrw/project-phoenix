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
	// holidayWeekQueueQuery reads a locked week and every booking of a
	// received or confirmed registration in it, in the order of the waiting
	// list: the time the registration came in, its number, then the position
	// of the child in the form (#4699), after the criteria in a period by
	// criteria (#4700). Run in READ COMMITTED after the lock,
	// it sees every placement committed before. Days are those of a booking
	// of days, NULL for a whole week (#4701).
	holidayWeekQueueQuery = `SELECT week.capacity, week.starts_on, week.ends_on,
			booking.id AS booking_id, booking.placement, booking.days::text AS days,
			registration.id AS registration_id, child.id AS child_id, child.position
		FROM enrollment.holiday_period_weeks AS week
		LEFT JOIN enrollment.holiday_registration_bookings AS booking
			ON booking.organization_id = week.organization_id AND booking.holiday_period_week_id = week.id
			AND EXISTS (
				SELECT 1 FROM enrollment.holiday_registration_children AS counted_child
				JOIN enrollment.holiday_registrations AS counted
					ON counted.organization_id = counted_child.organization_id AND counted.id = counted_child.holiday_registration_id
				WHERE counted_child.organization_id = booking.organization_id AND counted_child.id = booking.holiday_registration_child_id
					AND counted.status IN ` + holidayCountingStatuses + `)
		LEFT JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
		LEFT JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		WHERE week.organization_id = ? AND week.id = ?
		ORDER BY ` + holidayCriteriaOrderKey + `, registration.submitted_at, registration.id, child.position`
	// placeHolidayBookingsQuery gives the named waiting bookings of a locked
	// week their place; initial ones get no promoted_at.
	placeHolidayBookingsQuery = `UPDATE enrollment.holiday_registration_bookings AS booking
		SET placement = 'placed',
			promoted_at = CASE WHEN booking.id = ANY(?::bigint[]) THEN NULL ELSE statement_timestamp() END
		FROM enrollment.holiday_registration_children AS child
		WHERE booking.organization_id = ? AND booking.holiday_period_week_id = ? AND booking.id = ANY(?::bigint[])
			AND booking.placement = 'waitlisted'
			AND child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
		RETURNING booking.id AS booking_id, child.holiday_registration_id AS registration_id, child.id AS child_id,
			child.position, booking.holiday_period_week_id`
	setHolidayWeekCapacityQuery = `UPDATE enrollment.holiday_period_weeks SET capacity = ?
		WHERE organization_id = ? AND holiday_period_id = ? AND id = ?`
	holidayWeekPlacesQuery = `SELECT week.id, week.starts_on, week.ends_on, week.host_school_id, week.capacity, week.booking_unit,
			(SELECT count(*)
				FROM enrollment.holiday_registration_bookings AS booking
				JOIN enrollment.holiday_registration_children AS child
					ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
				JOIN enrollment.holiday_registrations AS registration
					ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
				WHERE booking.organization_id = week.organization_id AND booking.holiday_period_week_id = week.id
					AND booking.placement = 'placed' AND registration.status IN ` + holidayCountingStatuses + `) AS placed,
			(SELECT COALESCE(jsonb_object_agg(counted.day, counted.placed), '{}'::jsonb)
				FROM (SELECT booked.day::text AS day, count(*) AS placed
					FROM enrollment.holiday_registration_bookings AS booking
					CROSS JOIN LATERAL unnest(booking.days) AS booked(day)
					JOIN enrollment.holiday_registration_children AS child
						ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
					JOIN enrollment.holiday_registrations AS registration
						ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
					WHERE booking.organization_id = week.organization_id AND booking.holiday_period_week_id = week.id
						AND booking.placement = 'placed' AND registration.status IN ` + holidayCountingStatuses + `
					GROUP BY booked.day) AS counted) AS day_placed,
			(SELECT count(*)
				FROM enrollment.holiday_registration_bookings AS booking
				JOIN enrollment.holiday_registration_children AS child
					ON child.organization_id = booking.organization_id AND child.id = booking.holiday_registration_child_id
				JOIN enrollment.holiday_registrations AS registration
					ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
				WHERE booking.organization_id = week.organization_id AND booking.holiday_period_week_id = week.id
					AND booking.placement = 'pending' AND registration.status IN ` + holidayCountingStatuses + `) AS pending
		FROM enrollment.holiday_period_weeks AS week
		WHERE week.organization_id = ? AND week.holiday_period_id = ?
		ORDER BY week.starts_on, week.id`
	holidayWaitlistQuery = `SELECT booking.holiday_period_week_id,
			row_number() OVER (PARTITION BY booking.holiday_period_week_id
				ORDER BY ` + holidayCriteriaOrderKey + `, registration.submitted_at, registration.id, child.position) AS position,
			registration.id AS registration_id, registration.status, registration.submitted_at,
			child.id AS child_id, child.first_name, child.last_name, model.name AS time_model_name, booking.days::text AS days
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

type holidayWeekQueueRow struct {
	Capacity       *int          `bun:"capacity"`
	StartsOn       calendar.Date `bun:"starts_on"`
	EndsOn         calendar.Date `bun:"ends_on"`
	BookingID      *int64        `bun:"booking_id"`
	Placement      *string       `bun:"placement"`
	Days           *string       `bun:"days"`
	RegistrationID *int64        `bun:"registration_id"`
	ChildID        *int64        `bun:"child_id"`
	Position       *int          `bun:"position"`
}

// ReadHolidayWeekQueue reads a week the caller holds with the bookings that
// take or wait for its places, in the order of the waiting list: one
// statement. found is false for a week of another Träger.
func (s *HolidayRegistrationStore) ReadHolidayWeekQueue(ctx context.Context, organizationID, weekID int64) (enrollment.HolidayWeekQueue, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayWeekQueue{}, false, err
	}
	var rows []holidayWeekQueueRow
	if err := db.NewRaw(holidayWeekQueueQuery, organizationID, weekID).Scan(ctx, &rows); err != nil {
		return enrollment.HolidayWeekQueue{}, false, fmt.Errorf("enrollment postgres: read holiday week queue: %w", err)
	}
	if len(rows) == 0 {
		return enrollment.HolidayWeekQueue{}, false, nil
	}
	queue := enrollment.HolidayWeekQueue{Capacity: rows[0].Capacity, StartsOn: rows[0].StartsOn, EndsOn: rows[0].EndsOn}
	for _, row := range rows {
		if row.BookingID == nil || row.ChildID == nil || row.RegistrationID == nil || row.Placement == nil || row.Position == nil {
			continue
		}
		days, err := holidayDaysOf(row.Days)
		if err != nil {
			return enrollment.HolidayWeekQueue{}, false, err
		}
		queue.Bookings = append(queue.Bookings, enrollment.HolidayQueuedBooking{
			ID: *row.BookingID, RegistrationID: *row.RegistrationID, ChildID: *row.ChildID, ChildPosition: *row.Position,
			Placed: *row.Placement == enrollment.HolidayPlacementPlaced, Pending: *row.Placement == enrollment.HolidayPlacementPending, Days: days,
		})
	}
	return queue, true, nil
}

// PlaceHolidayBookings gives the waiting bookings bookingIDs names in a week
// the caller holds their place and returns them; those of initialIDs are
// initial placements without promoted_at.
func (s *HolidayRegistrationStore) PlaceHolidayBookings(ctx context.Context, organizationID, weekID int64, bookingIDs, initialIDs []int64) ([]enrollment.HolidayPromotion, error) {
	if len(bookingIDs) == 0 {
		return nil, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		BookingID      int64 `bun:"booking_id"`
		RegistrationID int64 `bun:"registration_id"`
		ChildID        int64 `bun:"child_id"`
		Position       int   `bun:"position"`
		WeekID         int64 `bun:"holiday_period_week_id"`
	}
	if initialIDs == nil {
		initialIDs = []int64{}
	}
	if err := db.NewRaw(placeHolidayBookingsQuery, pgdialect.Array(initialIDs), organizationID, weekID, pgdialect.Array(bookingIDs)).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: place holiday bookings: %w", err)
	}
	placed := make([]enrollment.HolidayPromotion, len(rows))
	for i, row := range rows {
		placed[i] = enrollment.HolidayPromotion{
			BookingID: row.BookingID, RegistrationID: row.RegistrationID, ChildID: row.ChildID, ChildPosition: row.Position, WeekID: row.WeekID,
		}
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
	BookingUnit  string        `bun:"booking_unit"`
	Placed       int           `bun:"placed"`
	Pending      int           `bun:"pending"`
	// DayPlaced counts the placed bookings of each booked day (#4701).
	DayPlaced map[string]int `bun:"day_placed,type:jsonb"`
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
	Days           *string   `bun:"days"`
}

// ReadHolidayWeekPlaces reads the places of every week of the period with
// its waiting list in its order and the places of each day of a week by
// days: two statements for any number of weeks.
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
		days, err := holidayDaysOf(row.Days)
		if err != nil {
			return nil, err
		}
		byWeek[row.WeekID] = append(byWeek[row.WeekID], enrollment.HolidayWaitlistEntry{
			Position: row.Position, RegistrationID: row.RegistrationID, RegistrationStatus: row.Status,
			SubmittedAt: row.SubmittedAt, ChildID: row.ChildID, ChildFirstName: row.FirstName, ChildLastName: row.LastName,
			TimeModelName: row.TimeModelName, Days: days,
		})
	}
	places := make([]enrollment.HolidayWeekPlaces, len(weeks))
	for i, week := range weeks {
		waitlist := byWeek[week.ID]
		if waitlist == nil {
			waitlist = []enrollment.HolidayWaitlistEntry{}
		}
		days := []enrollment.HolidayDayPlaces{}
		if week.BookingUnit == enrollment.HolidayBookingUnitDay {
			for _, day := range enrollment.HolidayWeekCareDays(week.StartsOn, week.EndsOn) {
				days = append(days, enrollment.HolidayDayPlaces{Date: day, Placed: week.DayPlaced[day.String()]})
			}
		}
		places[i] = enrollment.HolidayWeekPlaces{
			WeekID: week.ID, StartsOn: week.StartsOn, EndsOn: week.EndsOn, HostSchoolID: week.HostSchoolID,
			Capacity: week.Capacity, Placed: week.Placed, Waitlist: waitlist, Pending: week.Pending,
			BookingUnit: week.BookingUnit, Days: days,
		}
	}
	return places, nil
}
