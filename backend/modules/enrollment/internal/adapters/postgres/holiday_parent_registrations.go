package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The registrations of a parent account (ADR 0216 points 3 and 9). They lie
// at every Träger the account registered with, so each statement runs in the
// administrative transaction and names the account in its filter. The
// statement count does not depend on the rows: one claim, one read of the
// Träger, and for the list one read of the registrations and one of all
// their booked weeks.

const (
	// claimHolidayRegistrationsQuery links the registrations without an
	// account that name the account's address; idx_holiday_registrations_
	// unclaimed_email serves it.
	claimHolidayRegistrationsQuery = `UPDATE enrollment.holiday_registrations
		SET parent_account_id = ?
		WHERE parent_account_id IS NULL AND LOWER(guardian_email) = LOWER(?)`
	holidayParentRegistrationTraegerQuery = `SELECT DISTINCT organization_id FROM enrollment.holiday_registrations
		WHERE parent_account_id = ? ORDER BY organization_id`
	holidayParentRegistrationsQuery = `SELECT registration.id, registration.organization_id, period.name AS period_name,
			registration.submitted_at, registration.status,
			(SELECT min(week.starts_on) FROM enrollment.holiday_period_weeks AS week
				WHERE week.organization_id = registration.organization_id AND week.holiday_period_id = registration.holiday_period_id) AS first_day,
			(SELECT max(week.ends_on) FROM enrollment.holiday_period_weeks AS week
				WHERE week.organization_id = registration.organization_id AND week.holiday_period_id = registration.holiday_period_id) AS last_day
		FROM enrollment.holiday_registrations AS registration
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = registration.organization_id AND period.id = registration.holiday_period_id
		WHERE registration.parent_account_id = ?
		ORDER BY registration.submitted_at DESC, registration.id DESC`
	// holidayParentRegistrationWeeksQuery reads every child of the
	// registrations with its booked weeks; a child without a booking keeps
	// one row without a week.
	holidayParentRegistrationWeeksQuery = `SELECT child.holiday_registration_id AS registration_id, child.position, child.first_name,
			week.starts_on, week.ends_on, model.name AS time_model_name, booking.placement
		FROM enrollment.holiday_registration_children AS child
		JOIN enrollment.holiday_registrations AS registration
			ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
		LEFT JOIN enrollment.holiday_registration_bookings AS booking
			ON booking.organization_id = child.organization_id AND booking.holiday_registration_child_id = child.id
		LEFT JOIN enrollment.holiday_period_weeks AS week
			ON week.organization_id = booking.organization_id AND week.id = booking.holiday_period_week_id
		LEFT JOIN enrollment.holiday_week_time_models AS model
			ON model.organization_id = booking.organization_id AND model.id = booking.holiday_week_time_model_id
		WHERE registration.parent_account_id = ? AND child.holiday_registration_id = ANY(?)
		ORDER BY child.holiday_registration_id, child.position, week.starts_on, week.id`
)

var _ enrollment.HolidayParentRegistrations = (*HolidayRegistrationStore)(nil)

var errHolidayParentRegistrationsAdmin = errors.New("enrollment postgres: the registrations of a parent account require the administrative transaction")

// ClaimHolidayRegistrations links, in one statement, every registration
// without an account whose guardian address equals email to the account.
// No account or no address claims nothing.
func (s *HolidayRegistrationStore) ClaimHolidayRegistrations(ctx context.Context, accountID int64, email string) (int, error) {
	if !tenant.IsAdminTx(ctx) {
		return 0, errHolidayParentRegistrationsAdmin
	}
	email = strings.TrimSpace(email)
	if accountID <= 0 || !strings.Contains(email, "@") {
		return 0, nil
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return 0, err
	}
	claimed, err := affectedRows(db.NewRaw(claimHolidayRegistrationsQuery, accountID, email).Exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("enrollment postgres: claim holiday registrations: %w", err)
	}
	return claimed, nil
}

// HolidayParentRegistrationTraeger reads the Träger at which the account has
// a linked registration.
func (s *HolidayRegistrationStore) HolidayParentRegistrationTraeger(ctx context.Context, accountID int64) ([]int64, error) {
	if !tenant.IsAdminTx(ctx) {
		return nil, errHolidayParentRegistrationsAdmin
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return nil, err
	}
	organizationIDs := []int64{}
	if err := db.NewRaw(holidayParentRegistrationTraegerQuery, accountID).Scan(ctx, &organizationIDs); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read the Träger of holiday parent registrations: %w", err)
	}
	return organizationIDs, nil
}

type holidayParentRegistrationRow struct {
	ID             int64          `bun:"id"`
	OrganizationID int64          `bun:"organization_id"`
	PeriodName     string         `bun:"period_name"`
	SubmittedAt    time.Time      `bun:"submitted_at"`
	Status         string         `bun:"status"`
	FirstDay       *calendar.Date `bun:"first_day"`
	LastDay        *calendar.Date `bun:"last_day"`
}

type holidayParentRegistrationWeekRow struct {
	RegistrationID int64          `bun:"registration_id"`
	Position       int            `bun:"position"`
	FirstName      string         `bun:"first_name"`
	StartsOn       *calendar.Date `bun:"starts_on"`
	EndsOn         *calendar.Date `bun:"ends_on"`
	TimeModelName  *string        `bun:"time_model_name"`
	Placement      *string        `bun:"placement"`
}

// HolidayParentRegistrations reads the account's registrations, newest
// first, with the first name and the booked weeks of each child: two
// statements whatever the number of registrations.
func (s *HolidayRegistrationStore) HolidayParentRegistrations(ctx context.Context, accountID int64) ([]enrollment.HolidayParentRegistration, error) {
	if !tenant.IsAdminTx(ctx) {
		return nil, errHolidayParentRegistrationsAdmin
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return nil, err
	}
	var rows []holidayParentRegistrationRow
	if err := db.NewRaw(holidayParentRegistrationsQuery, accountID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday parent registrations: %w", err)
	}
	registrations := make([]enrollment.HolidayParentRegistration, len(rows))
	if len(rows) == 0 {
		return registrations, nil
	}
	ids := make([]int64, len(rows))
	index := make(map[int64]int, len(rows))
	for i, row := range rows {
		ids[i], index[row.ID] = row.ID, i
		registrations[i] = enrollment.HolidayParentRegistration{
			ID: row.ID, OrganizationID: row.OrganizationID, PeriodName: row.PeriodName, SubmittedAt: row.SubmittedAt, Status: row.Status,
			Children: []enrollment.HolidayParentRegistrationChild{},
		}
		if row.FirstDay != nil && row.LastDay != nil {
			registrations[i].FirstDay, registrations[i].LastDay = *row.FirstDay, *row.LastDay
		}
	}
	var weeks []holidayParentRegistrationWeekRow
	if err := db.NewRaw(holidayParentRegistrationWeeksQuery, accountID, pgdialect.Array(ids)).Scan(ctx, &weeks); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read the weeks of holiday parent registrations: %w", err)
	}
	addHolidayParentRegistrationWeeks(registrations, index, weeks)
	return registrations, nil
}

// addHolidayParentRegistrationWeeks groups the rows, ordered by registration
// and child position, into the children of their registration.
func addHolidayParentRegistrationWeeks(registrations []enrollment.HolidayParentRegistration, index map[int64]int, weeks []holidayParentRegistrationWeekRow) {
	lastRegistration, lastPosition := int64(0), 0
	for _, row := range weeks {
		i, ok := index[row.RegistrationID]
		if !ok {
			continue
		}
		registration := &registrations[i]
		if row.RegistrationID != lastRegistration || row.Position != lastPosition {
			registration.Children = append(registration.Children, enrollment.HolidayParentRegistrationChild{
				FirstName: row.FirstName, Weeks: []enrollment.HolidayParentRegistrationWeek{},
			})
			lastRegistration, lastPosition = row.RegistrationID, row.Position
		}
		if row.StartsOn == nil || row.EndsOn == nil {
			continue
		}
		child := &registration.Children[len(registration.Children)-1]
		week := enrollment.HolidayParentRegistrationWeek{FirstDay: *row.StartsOn, LastDay: *row.EndsOn}
		if row.TimeModelName != nil {
			week.TimeModelName = *row.TimeModelName
		}
		if row.Placement != nil {
			week.Placement = *row.Placement
		}
		child.Weeks = append(child.Weeks, week)
	}
}
