package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	// days and the account that picked the week's holiday group: one
	// statement for any number of weeks and children.
	ownChildrenQuery = `SELECT week.id AS week_id, week.organization_id, week.starts_on, week.ends_on, period.name AS period_name,
			COALESCE(host_group.assigned_by_account_id, 0) AS assigned_by_account_id,
			participant.child_id, participant.first_name, participant.last_name, participant.birth_date,
			model.name AS time_model_name, to_char(model.starts_at, 'HH24:MI') AS time_model_starts,
			to_char(model.ends_at, 'HH24:MI') AS time_model_ends, booking.days::text AS days
		FROM enrollment.holiday_period_weeks AS week
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
		LEFT JOIN enrollment.holiday_host_groups AS host_group
			ON host_group.tenant_id = week.host_school_id AND host_group.holiday_period_week_id = week.id
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
	ownChildContactsQuery = `SELECT contact.child_id, contact.kind, contact.name, contact.phone, contact.relationship
		FROM enrollment.holiday_period_weeks AS week
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
		CROSS JOIN LATERAL enrollment.holiday_host_own_contacts(week.organization_id, week.id) AS contact
		WHERE week.host_school_id = ? AND week.ends_on >= ? AND period.status = 'published'
		ORDER BY contact.child_id, contact.kind, contact.position`
	// guestGroupHintsQuery reads the holiday groups of hosted weeks that end
	// on today or later with their guests counted. It reads no personal
	// data.
	guestGroupHintsQuery = `SELECT host_group.group_id, week.id AS week_id, week.starts_on, week.ends_on,
			(SELECT COUNT(*) FROM enrollment.holiday_guests AS guest
				WHERE guest.tenant_id = host_group.tenant_id AND guest.holiday_period_week_id = week.id
					AND guest.ended_on IS NULL) AS guests
		FROM enrollment.holiday_host_groups AS host_group
		JOIN enrollment.holiday_period_weeks AS week
			ON week.organization_id = host_group.organization_id AND week.id = host_group.holiday_period_week_id
				AND week.host_school_id = host_group.tenant_id
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
		WHERE host_group.tenant_id = ? AND week.ends_on >= ? AND period.status = 'published'
		ORDER BY week.starts_on, week.id`
)

type ownChildRow struct {
	WeekID              int64         `bun:"week_id"`
	OrganizationID      int64         `bun:"organization_id"`
	StartsOn            calendar.Date `bun:"starts_on"`
	EndsOn              calendar.Date `bun:"ends_on"`
	PeriodName          string        `bun:"period_name"`
	AssignedByAccountID int64         `bun:"assigned_by_account_id"`
	ChildID             int64         `bun:"child_id"`
	FirstName           string        `bun:"first_name"`
	LastName            string        `bun:"last_name"`
	BirthDate           calendar.Date `bun:"birth_date"`
	TimeModelName       string        `bun:"time_model_name"`
	TimeModelStarts     string        `bun:"time_model_starts"`
	TimeModelEnds       string        `bun:"time_model_ends"`
	Days                *string       `bun:"days"`
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
	var contacts []holidayGuestContactRow
	if err := db.NewRaw(ownChildContactsQuery, schoolID, today).Scan(ctx, &contacts); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read the contacts of the school's own holiday children: %w", err)
	}
	byChild := make(map[int64]*enrollment.HolidayOwnChildBooking, len(rows))
	bookings := make([]enrollment.HolidayOwnChildBooking, len(rows))
	for i, row := range rows {
		days, err := holidayDaysOf(row.Days)
		if err != nil {
			return nil, err
		}
		bookings[i] = enrollment.HolidayOwnChildBooking{
			Week: enrollment.HolidayOwnChildWeek{
				HolidayGuestWeek: enrollment.HolidayGuestWeek{ID: row.WeekID, OrganizationID: row.OrganizationID, StartsOn: row.StartsOn, EndsOn: row.EndsOn},
				PeriodName:       row.PeriodName, AssignedByAccountID: row.AssignedByAccountID,
			},
			HolidayOwnChildSource: enrollment.HolidayOwnChildSource{
				ChildID:       row.ChildID,
				Identity:      enrollment.HolidayStudentIdentity{FirstName: row.FirstName, LastName: row.LastName, BirthDate: row.BirthDate},
				TimeModelName: row.TimeModelName, TimeModelStarts: row.TimeModelStarts, TimeModelEnds: row.TimeModelEnds, Days: days,
			},
		}
		byChild[row.ChildID] = &bookings[i]
	}
	for _, contact := range contacts {
		if booking, ok := byChild[contact.ChildID]; ok {
			appendContact(&booking.Emergency, &booking.Pickup, contact)
		}
	}
	return bookings, nil
}

// ListGuestGroupHints lists the holiday groups of hosted weeks that end on
// today or later with the number of their guests: one statement.
func (s *HolidayGuestStore) ListGuestGroupHints(ctx context.Context, schoolID int64, today calendar.Date) ([]enrollment.HolidayGuestGroupHint, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		GroupID  int64         `bun:"group_id"`
		WeekID   int64         `bun:"week_id"`
		StartsOn calendar.Date `bun:"starts_on"`
		EndsOn   calendar.Date `bun:"ends_on"`
		Guests   int           `bun:"guests"`
	}
	if err := db.NewRaw(guestGroupHintsQuery, schoolID, today).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: list holiday group hints: %w", err)
	}
	hints := make([]enrollment.HolidayGuestGroupHint, len(rows))
	for i, row := range rows {
		hints[i] = enrollment.HolidayGuestGroupHint(row)
	}
	return hints, nil
}
