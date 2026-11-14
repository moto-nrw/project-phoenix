package postgres

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Children of the hosting school itself in the weeks it hosts (#6678, ADR
// 0120 addendum). Every statement runs in the tenant transaction of the
// hosting school (hostDB). The children come only through
// enrollment.holiday_host_participants and their contacts only through
// enrollment.holiday_host_own_contacts, which answer for confirmed, placed
// bookings of the weeks the school hosts until their last day. Each
// statement names the school once more. Nothing is stored.

const (
	// ownChildrenQuery reads, for every hosted week of a published period
	// that ends on today or later, the confirmed children whose registration
	// names the school as their home school, with their booked time model and
	// days: one statement for any number of weeks and children.
	ownChildrenQuery = `SELECT week.id AS week_id, week.organization_id, week.starts_on, week.ends_on, period.name AS period_name,
			participant.child_id, participant.first_name, participant.last_name, participant.birth_date,
			model.name AS time_model_name, to_char(model.starts_at, 'HH24:MI') AS time_model_starts,
			to_char(model.ends_at, 'HH24:MI') AS time_model_ends, booking.days::text AS days,
			booking.lunch AND week.lunch_price_cents IS NOT NULL AS lunch
		FROM enrollment.holiday_period_weeks AS week
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
		CROSS JOIN LATERAL enrollment.holiday_host_participants(week.organization_id, week.id) AS participant
		JOIN enrollment.holiday_week_time_models AS model
			ON model.organization_id = week.organization_id AND model.holiday_period_week_id = week.id
				AND model.id = participant.time_model_id
		JOIN enrollment.holiday_registration_bookings AS booking
			ON booking.organization_id = week.organization_id AND booking.holiday_period_week_id = week.id
				AND booking.holiday_registration_child_id = participant.child_id
		WHERE week.host_school_id = ? AND week.ends_on >= ? AND period.status = 'published'
			AND participant.home_school_id = ?
		ORDER BY week.starts_on, week.id, participant.child_id`
	// ownChildContactsQuery names the week with each contact: a child booked
	// in several hosted weeks has one booking and one set of contacts a week.
	ownChildContactsQuery = `SELECT week.id AS week_id, contact.child_id, contact.kind, contact.name, contact.phone, contact.relationship
		FROM enrollment.holiday_period_weeks AS week
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
		CROSS JOIN LATERAL enrollment.holiday_host_own_contacts(week.organization_id, week.id) AS contact
		WHERE week.host_school_id = ? AND week.ends_on >= ? AND period.status = 'published'
		ORDER BY week.id, contact.child_id, contact.kind, contact.position`
)

type ownChildRow struct {
	WeekID          int64         `bun:"week_id"`
	OrganizationID  int64         `bun:"organization_id"`
	StartsOn        calendar.Date `bun:"starts_on"`
	EndsOn          calendar.Date `bun:"ends_on"`
	PeriodName      string        `bun:"period_name"`
	ChildID         int64         `bun:"child_id"`
	FirstName       string        `bun:"first_name"`
	LastName        string        `bun:"last_name"`
	BirthDate       calendar.Date `bun:"birth_date"`
	TimeModelName   string        `bun:"time_model_name"`
	TimeModelStarts string        `bun:"time_model_starts"`
	TimeModelEnds   string        `bun:"time_model_ends"`
	Days            *string       `bun:"days"`
	Lunch           bool          `bun:"lunch"`
}

// ListOwnChildren reads the confirmed children of the school itself in
// every hosted week that ends on today or later, with their contacts: two
// statements for any number of weeks and children.
func (s *HolidayGuestStore) ListOwnChildren(ctx context.Context, schoolID int64, today calendar.Date) ([]enrollment.HolidayOwnChildBooking, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var rows []ownChildRow
	if err := db.NewRaw(ownChildrenQuery, schoolID, today, schoolID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read the school's own holiday children: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	var contacts []struct {
		WeekID int64 `bun:"week_id"`
		holidayGuestContactRow
	}
	if err := db.NewRaw(ownChildContactsQuery, schoolID, today).Scan(ctx, &contacts); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read the contacts of the school's own holiday children: %w", err)
	}
	type weekChild struct{ weekID, childID int64 }
	byBooking := make(map[weekChild]*enrollment.HolidayOwnChildBooking, len(rows))
	bookings := make([]enrollment.HolidayOwnChildBooking, len(rows))
	for i, row := range rows {
		days, err := holidayDaysOf(row.Days)
		if err != nil {
			return nil, err
		}
		bookings[i] = enrollment.HolidayOwnChildBooking{
			Week: enrollment.HolidayOwnChildWeek{
				HolidayGuestWeek: enrollment.HolidayGuestWeek{ID: row.WeekID, OrganizationID: row.OrganizationID, StartsOn: row.StartsOn, EndsOn: row.EndsOn},
				PeriodName:       row.PeriodName,
			},
			HolidayOwnChildSource: enrollment.HolidayOwnChildSource{
				ChildID:       row.ChildID,
				Identity:      enrollment.HolidayStudentIdentity{FirstName: row.FirstName, LastName: row.LastName, BirthDate: row.BirthDate},
				TimeModelName: row.TimeModelName, TimeModelStarts: row.TimeModelStarts, TimeModelEnds: row.TimeModelEnds, Days: days,
				Lunch: row.Lunch,
			},
		}
		byBooking[weekChild{row.WeekID, row.ChildID}] = &bookings[i]
	}
	for _, contact := range contacts {
		if booking, ok := byBooking[weekChild{contact.WeekID, contact.ChildID}]; ok {
			appendContact(&booking.Emergency, &booking.Pickup, contact.holidayGuestContactRow)
		}
	}
	return bookings, nil
}

// homeSchoolBookingsQuery reads the confirmed, placed bookings of the
// school's own children in the open weeks other schools of its Träger host
// (#6679) through the database's own narrow function: one statement for any
// number of weeks and children. The function checks the home school, the
// Träger and the end of the week by the Berlin day itself; the statement
// names the end by the service's calendar day once more.
const homeSchoolBookingsQuery = `SELECT child_id, first_name, last_name, birth_date, week_id, period_name, starts_on, ends_on,
		host_school_id, time_model_name, starts_at AS time_model_starts, ends_at AS time_model_ends, days::text AS days, lunch
	FROM enrollment.holiday_home_school_bookings()
	WHERE ends_on >= ?
	ORDER BY starts_on, week_id, lower(last_name), lower(first_name), child_id`

type homeSchoolBookingRow struct {
	ChildID         int64         `bun:"child_id"`
	FirstName       string        `bun:"first_name"`
	LastName        string        `bun:"last_name"`
	BirthDate       calendar.Date `bun:"birth_date"`
	WeekID          int64         `bun:"week_id"`
	PeriodName      string        `bun:"period_name"`
	StartsOn        calendar.Date `bun:"starts_on"`
	EndsOn          calendar.Date `bun:"ends_on"`
	HostSchoolID    int64         `bun:"host_school_id"`
	TimeModelName   string        `bun:"time_model_name"`
	TimeModelStarts string        `bun:"time_model_starts"`
	TimeModelEnds   string        `bun:"time_model_ends"`
	Days            *string       `bun:"days"`
	Lunch           bool          `bun:"lunch"`
}

// ListHomeSchoolBookings reads the confirmed bookings of the school's own
// children in open weeks other schools of its Träger host, in the tenant
// transaction of the home school.
func (s *HolidayGuestStore) ListHomeSchoolBookings(ctx context.Context, schoolID int64, today calendar.Date) ([]enrollment.HolidayHomeSchoolBooking, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var rows []homeSchoolBookingRow
	if err := db.NewRaw(homeSchoolBookingsQuery, today).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read the holiday bookings of the school's children at other schools: %w", err)
	}
	bookings := make([]enrollment.HolidayHomeSchoolBooking, len(rows))
	for i, row := range rows {
		days, err := holidayDaysOf(row.Days)
		if err != nil {
			return nil, err
		}
		if days == nil {
			days = []calendar.Date{}
		}
		bookings[i] = enrollment.HolidayHomeSchoolBooking{
			ChildID: row.ChildID, FirstName: row.FirstName, LastName: row.LastName, BirthDate: row.BirthDate,
			WeekID: row.WeekID, PeriodName: row.PeriodName, StartsOn: row.StartsOn, EndsOn: row.EndsOn,
			HostSchoolID: row.HostSchoolID, TimeModelName: row.TimeModelName,
			TimeModelStarts: row.TimeModelStarts, TimeModelEnds: row.TimeModelEnds, Days: days, Lunch: row.Lunch,
		}
	}
	return bookings, nil
}
