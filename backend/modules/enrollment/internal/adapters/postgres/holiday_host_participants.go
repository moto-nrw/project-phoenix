package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Participant lists of the hosting school (#4665, ADR 0116). Every statement
// runs in the tenant transaction of the hosting school. The host policies
// admit only the school's weeks of published periods until their last day
// and only bookings of confirmed registrations; the children come only
// through enrollment.holiday_host_participants with their minimum fields.
// Each statement names the school, the end of the week by the service's
// calendar day and the confirmed booking once more.

// HolidayHostStore reads the hosted weeks in the hosting school's tenant
// transaction.
type HolidayHostStore struct{}

// NewHolidayHostStore builds the store.
func NewHolidayHostStore() *HolidayHostStore { return &HolidayHostStore{} }

// hostDB returns the tenant transaction of schoolID. An administrative,
// Träger or foreign school transaction is refused.
func hostDB(ctx context.Context, schoolID int64) (bun.IDB, error) {
	if schoolID <= 0 || tenant.IsAdminTx(ctx) || tenant.TraegerTransactionFromContext(ctx) != 0 {
		return nil, errors.New("enrollment postgres: the tenant transaction of the hosting school is required")
	}
	tx, current, err := tenant.CurrentTenantTransaction[bun.Tx](ctx)
	if err != nil {
		return nil, fmt.Errorf("enrollment postgres: the tenant transaction of the hosting school is required: %w", err)
	}
	if current != schoolID {
		return nil, errors.New("enrollment postgres: the tenant transaction belongs to another school")
	}
	return tx, nil
}

type hostedWeekRow struct {
	ID             int64         `bun:"id"`
	OrganizationID int64         `bun:"organization_id"`
	PeriodName     string        `bun:"period_name"`
	StartsOn       calendar.Date `bun:"starts_on"`
	EndsOn         calendar.Date `bun:"ends_on"`
	Participants   int           `bun:"participants"`
	GuestGroupID   *int64        `bun:"guest_group_id"`
	Guests         int           `bun:"guests"`
}

type hostedTimeModelRow struct {
	ID       int64  `bun:"id"`
	Name     string `bun:"name"`
	StartsAt string `bun:"starts_at"`
	EndsAt   string `bun:"ends_at"`
}

type hostedParticipantRow struct {
	ChildID         int64         `bun:"child_id"`
	FirstName       string        `bun:"first_name"`
	LastName        string        `bun:"last_name"`
	BirthDate       calendar.Date `bun:"birth_date"`
	HomeSchoolID    *int64        `bun:"home_school_id"`
	GuestSchoolName string        `bun:"guest_school_name"`
	TimeModelID     int64         `bun:"time_model_id"`
}

const (
	hostedWeekColumns = `SELECT week.id, week.organization_id, period.name AS period_name, week.starts_on, week.ends_on,
			(SELECT COUNT(*) FROM enrollment.holiday_registration_bookings AS booking
				WHERE booking.organization_id = week.organization_id AND booking.holiday_period_week_id = week.id
					AND enrollment.holiday_host_booking(booking.organization_id, booking.holiday_period_week_id,
						booking.holiday_registration_child_id)) AS participants,
			host_group.group_id AS guest_group_id,
			(SELECT COUNT(*) FROM enrollment.holiday_guests AS guest
				WHERE guest.tenant_id = week.host_school_id AND guest.holiday_period_week_id = week.id
					AND guest.ended_on IS NULL) AS guests
		FROM enrollment.holiday_period_weeks AS week
		JOIN enrollment.holiday_periods AS period
			ON period.organization_id = week.organization_id AND period.id = week.holiday_period_id
		LEFT JOIN enrollment.holiday_host_groups AS host_group
			ON host_group.tenant_id = week.host_school_id AND host_group.holiday_period_week_id = week.id
		WHERE week.host_school_id = ? AND week.ends_on >= ? AND period.status = 'published'`
	listHostedWeeksQuery  = hostedWeekColumns + ` ORDER BY week.starts_on, week.id`
	findHostedWeekQuery   = hostedWeekColumns + ` AND week.id = ?`
	hostedTimeModelsQuery = `SELECT id, name, to_char(starts_at, 'HH24:MI') AS starts_at, to_char(ends_at, 'HH24:MI') AS ends_at
		FROM enrollment.holiday_week_time_models
		WHERE organization_id = ? AND holiday_period_week_id = ?
		ORDER BY starts_at, ends_at, id`
	// hostedParticipantsQuery reads the minimum fields of the confirmed
	// children through the database's own narrow function: the school has no
	// row access to the children, so no column beyond these can leave.
	hostedParticipantsQuery = `SELECT child_id, first_name, last_name, birth_date, home_school_id, guest_school_name, time_model_id
		FROM enrollment.holiday_host_participants(?, ?)
		ORDER BY lower(last_name), lower(first_name), child_id`
)

// ListHostedHolidayWeeks lists the weeks of published periods the school
// hosts that end on today or later, with their confirmed children counted.
func (s *HolidayHostStore) ListHostedHolidayWeeks(ctx context.Context, schoolID int64, today calendar.Date) ([]enrollment.HostedHolidayWeek, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var rows []hostedWeekRow
	if err := db.NewRaw(listHostedWeeksQuery, schoolID, today).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: list hosted holiday weeks: %w", err)
	}
	weeks := make([]enrollment.HostedHolidayWeek, len(rows))
	for i, row := range rows {
		weeks[i] = hostedWeek(row)
	}
	return weeks, nil
}

// ReadHostedHolidayWeek reads one hosted week, its time models and its
// confirmed children: three statements for any number of children.
func (s *HolidayHostStore) ReadHostedHolidayWeek(ctx context.Context, schoolID, weekID int64, today calendar.Date) (enrollment.HostedHolidayWeek, []enrollment.HolidayTimeModelCount, []enrollment.HolidayParticipant, bool, error) {
	db, err := hostDB(ctx, schoolID)
	if err != nil {
		return enrollment.HostedHolidayWeek{}, nil, nil, false, err
	}
	var weeks []hostedWeekRow
	if err := db.NewRaw(findHostedWeekQuery, schoolID, today, weekID).Scan(ctx, &weeks); err != nil {
		return enrollment.HostedHolidayWeek{}, nil, nil, false, fmt.Errorf("enrollment postgres: read hosted holiday week: %w", err)
	}
	if len(weeks) == 0 {
		return enrollment.HostedHolidayWeek{}, nil, nil, false, nil
	}
	week := weeks[0]
	var models []hostedTimeModelRow
	if err := db.NewRaw(hostedTimeModelsQuery, week.OrganizationID, week.ID).Scan(ctx, &models); err != nil {
		return enrollment.HostedHolidayWeek{}, nil, nil, false, fmt.Errorf("enrollment postgres: read hosted holiday time models: %w", err)
	}
	var children []hostedParticipantRow
	if err := db.NewRaw(hostedParticipantsQuery, week.OrganizationID, week.ID).Scan(ctx, &children); err != nil {
		return enrollment.HostedHolidayWeek{}, nil, nil, false, fmt.Errorf("enrollment postgres: read holiday participants: %w", err)
	}
	counted := make([]enrollment.HolidayTimeModelCount, len(models))
	for i, model := range models {
		counted[i] = enrollment.HolidayTimeModelCount{ID: model.ID, Name: model.Name, StartsAt: model.StartsAt, EndsAt: model.EndsAt}
	}
	participants := make([]enrollment.HolidayParticipant, len(children))
	for i, child := range children {
		participants[i] = enrollment.HolidayParticipant(child)
	}
	return hostedWeek(week), counted, participants, true, nil
}

func hostedWeek(row hostedWeekRow) enrollment.HostedHolidayWeek {
	return enrollment.HostedHolidayWeek{
		ID: row.ID, PeriodName: row.PeriodName, StartsOn: row.StartsOn, EndsOn: row.EndsOn, Participants: row.Participants,
		GuestGroupID: row.GuestGroupID, Guests: row.Guests,
	}
}
