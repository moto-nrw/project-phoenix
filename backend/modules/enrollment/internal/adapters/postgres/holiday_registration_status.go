package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// The status link of a holiday registration (#6676). The lookup by token
// runs in the administrative transaction, like the link of a period, and
// reads only the Träger and the ID of the registration; the read of the
// registration runs in the Träger transaction of that Träger and names it
// once more in its filter. Two statements, whatever the number of children
// and weeks.

const (
	findHolidayRegistrationStatusQuery = `SELECT organization_id, id FROM enrollment.holiday_registrations
		WHERE status_token = ?`
	holidayRegistrationStatusQuery = `SELECT period.name AS period_name, period.registration_closes_on, period.binding, period.allocation_mode,
			registration.submitted_at, registration.status,
			(SELECT min(week.starts_on) FROM enrollment.holiday_period_weeks AS week
				WHERE week.organization_id = registration.organization_id AND week.holiday_period_id = registration.holiday_period_id) AS first_day,
			(SELECT max(week.ends_on) FROM enrollment.holiday_period_weeks AS week
				WHERE week.organization_id = registration.organization_id AND week.holiday_period_id = registration.holiday_period_id) AS last_day
		FROM enrollment.holiday_registrations AS registration
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = registration.organization_id AND period.id = registration.holiday_period_id
		WHERE registration.organization_id = ? AND registration.id = ?`
	// holidayRegistrationStatusWeeksQuery reads every child of the
	// registration with its booked weeks; a child without a booking keeps
	// one row without a week.
	holidayRegistrationStatusWeeksQuery = `SELECT child.position, child.first_name,
			week.starts_on, week.ends_on, week.host_school_id, model.name AS time_model_name,
			to_char(model.starts_at, 'HH24:MI') AS starts_at, to_char(model.ends_at, 'HH24:MI') AS ends_at, booking.placement,
			COALESCE(booking.lunch AND week.lunch_price_cents IS NOT NULL, FALSE) AS lunch
		FROM enrollment.holiday_registration_children AS child
		LEFT JOIN enrollment.holiday_registration_bookings AS booking
			ON booking.organization_id = child.organization_id AND booking.holiday_registration_child_id = child.id
		LEFT JOIN enrollment.holiday_period_weeks AS week
			ON week.organization_id = booking.organization_id AND week.id = booking.holiday_period_week_id
		LEFT JOIN enrollment.holiday_week_time_models AS model
			ON model.organization_id = booking.organization_id AND model.id = booking.holiday_week_time_model_id
		WHERE child.organization_id = ? AND child.holiday_registration_id = ?
		ORDER BY child.position, week.starts_on, week.id`
)

// FindHolidayRegistrationStatus names the Träger and the registration of a
// status token. Only the administrative transaction may read across Träger.
func (s *HolidayRegistrationStore) FindHolidayRegistrationStatus(ctx context.Context, token string) (enrollment.HolidayRegistrationReference, bool, error) {
	if !tenant.IsAdminTx(ctx) {
		return enrollment.HolidayRegistrationReference{}, false, errors.New("enrollment postgres: the status link lookup requires the administrative transaction")
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return enrollment.HolidayRegistrationReference{}, false, err
	}
	var rows []struct {
		OrganizationID int64 `bun:"organization_id"`
		ID             int64 `bun:"id"`
	}
	if err := db.NewRaw(findHolidayRegistrationStatusQuery, token).Scan(ctx, &rows); err != nil {
		return enrollment.HolidayRegistrationReference{}, false, fmt.Errorf("enrollment postgres: read holiday registration status link: %w", err)
	}
	if len(rows) == 0 {
		return enrollment.HolidayRegistrationReference{}, false, nil
	}
	return enrollment.HolidayRegistrationReference{OrganizationID: rows[0].OrganizationID, RegistrationID: rows[0].ID}, true, nil
}

type holidayRegistrationStatusRow struct {
	PeriodName           string         `bun:"period_name"`
	RegistrationClosesOn calendar.Date  `bun:"registration_closes_on"`
	Binding              bool           `bun:"binding"`
	AllocationMode       string         `bun:"allocation_mode"`
	SubmittedAt          time.Time      `bun:"submitted_at"`
	Status               string         `bun:"status"`
	FirstDay             *calendar.Date `bun:"first_day"`
	LastDay              *calendar.Date `bun:"last_day"`
}

type holidayRegistrationStatusWeekRow struct {
	Position      int            `bun:"position"`
	FirstName     string         `bun:"first_name"`
	StartsOn      *calendar.Date `bun:"starts_on"`
	EndsOn        *calendar.Date `bun:"ends_on"`
	HostSchoolID  *int64         `bun:"host_school_id"`
	TimeModelName *string        `bun:"time_model_name"`
	StartsAt      *string        `bun:"starts_at"`
	EndsAt        *string        `bun:"ends_at"`
	Placement     *string        `bun:"placement"`
	Lunch         bool           `bun:"lunch"`
}

// HolidayRegistrationStatus reads the registration of the Träger with its
// period and, per child, the first name and the booked weeks; schools names
// the host schools by ID. It reports whether the registration exists.
func (s *HolidayRegistrationStore) HolidayRegistrationStatus(ctx context.Context, reference enrollment.HolidayRegistrationReference,
	schools map[int64]string) (enrollment.HolidayRegistrationStatus, bool, error) {
	db, err := traegerDB(ctx, reference.OrganizationID)
	if err != nil {
		return enrollment.HolidayRegistrationStatus{}, false, err
	}
	var rows []holidayRegistrationStatusRow
	if err := db.NewRaw(holidayRegistrationStatusQuery, reference.OrganizationID, reference.RegistrationID).Scan(ctx, &rows); err != nil {
		return enrollment.HolidayRegistrationStatus{}, false, fmt.Errorf("enrollment postgres: read holiday registration status: %w", err)
	}
	if len(rows) == 0 {
		return enrollment.HolidayRegistrationStatus{}, false, nil
	}
	row := rows[0]
	status := enrollment.HolidayRegistrationStatus{
		PeriodName: row.PeriodName, RegistrationClosesOn: row.RegistrationClosesOn, Binding: row.Binding, AllocationMode: row.AllocationMode,
		SubmittedAt: row.SubmittedAt, Status: row.Status, Children: []enrollment.HolidayRegistrationStatusChild{},
	}
	if row.FirstDay != nil && row.LastDay != nil {
		status.FirstDay, status.LastDay = *row.FirstDay, *row.LastDay
	}
	var weeks []holidayRegistrationStatusWeekRow
	if err := db.NewRaw(holidayRegistrationStatusWeeksQuery, reference.OrganizationID, reference.RegistrationID).Scan(ctx, &weeks); err != nil {
		return enrollment.HolidayRegistrationStatus{}, false, fmt.Errorf("enrollment postgres: read the weeks of a holiday registration status: %w", err)
	}
	status.Children = holidayRegistrationStatusChildren(weeks, schools)
	return status, true, nil
}

// holidayRegistrationStatusChildren groups the rows, ordered by child
// position, into the children with their weeks.
func holidayRegistrationStatusChildren(rows []holidayRegistrationStatusWeekRow, schools map[int64]string) []enrollment.HolidayRegistrationStatusChild {
	children := []enrollment.HolidayRegistrationStatusChild{}
	lastPosition := 0
	for _, row := range rows {
		if row.Position != lastPosition {
			children = append(children, enrollment.HolidayRegistrationStatusChild{FirstName: row.FirstName, Weeks: []enrollment.HolidayRegistrationStatusWeek{}})
			lastPosition = row.Position
		}
		if row.StartsOn == nil || row.EndsOn == nil {
			continue
		}
		week := enrollment.HolidayRegistrationStatusWeek{
			FirstDay: *row.StartsOn, LastDay: *row.EndsOn, TimeModelName: stringOrEmpty(row.TimeModelName),
			StartsAt: stringOrEmpty(row.StartsAt), EndsAt: stringOrEmpty(row.EndsAt), Placement: stringOrEmpty(row.Placement),
			Lunch: row.Lunch,
		}
		if row.HostSchoolID != nil {
			week.HostSchoolName = schools[*row.HostSchoolID]
		}
		child := &children[len(children)-1]
		child.Weeks = append(child.Weeks, week)
	}
	return children
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
