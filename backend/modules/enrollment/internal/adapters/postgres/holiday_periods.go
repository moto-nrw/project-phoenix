package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Holiday periods of a Träger (#4659). Their tables carry organization_id and
// a Träger policy on app.current_org_id (ADR 0086 section 5, ADR 0105): every
// statement here runs in the Träger transaction of the Träger it names, and
// names it once more in its own filter.

// holidayBookingConstraints are the foreign keys of
// enrollment.holiday_registration_bookings that keep a booked week and a
// booked time model from being removed (#4661).
var holidayBookingConstraints = map[string]bool{
	"fk_holiday_registration_bookings_week":       true,
	"fk_holiday_registration_bookings_time_model": true,
}

// holidayHostSchoolConstraint is the name the host-school trigger of
// enrollment.holiday_period_weeks raises when a host school is not an
// undeleted school of the period's Träger.
const holidayHostSchoolConstraint = "chk_holiday_period_weeks_host_school"

// HolidayPeriodStore persists holiday periods in the Träger transaction.
type HolidayPeriodStore struct{}

// NewHolidayPeriodStore builds the store.
func NewHolidayPeriodStore() *HolidayPeriodStore { return &HolidayPeriodStore{} }

// traegerDB returns the Träger transaction of organizationID. A school,
// administrative or foreign Träger transaction is refused.
func traegerDB(ctx context.Context, organizationID int64) (bun.IDB, error) {
	if organizationID <= 0 || tenant.TraegerTransactionFromContext(ctx) != organizationID {
		return nil, errors.New("enrollment postgres: the Träger transaction of this Träger is required")
	}
	return NewDatabase()(ctx)
}

type holidayPeriodRow struct {
	bun.BaseModel `bun:"table:enrollment.holiday_periods"`

	ID                   int64         `bun:"id"`
	Name                 string        `bun:"name"`
	RegistrationOpensOn  calendar.Date `bun:"registration_opens_on"`
	RegistrationClosesOn calendar.Date `bun:"registration_closes_on"`
	Binding              bool          `bun:"binding"`
	Notes                string        `bun:"notes"`
	Status               string        `bun:"status"`
	PublishedAt          *time.Time    `bun:"published_at"`
	UpdatedAt            time.Time     `bun:"updated_at"`
	PublicToken          string        `bun:"public_token"`
	RegistrationCount    int           `bun:"registration_count"`
}

type holidayWeekRow struct {
	bun.BaseModel `bun:"table:enrollment.holiday_period_weeks"`

	ID              int64         `bun:"id"`
	HolidayPeriodID int64         `bun:"holiday_period_id"`
	StartsOn        calendar.Date `bun:"starts_on"`
	EndsOn          calendar.Date `bun:"ends_on"`
	HostSchoolID    int64         `bun:"host_school_id"`
}

type holidayTimeModelRow struct {
	ID         int64  `bun:"id"`
	WeekID     int64  `bun:"holiday_period_week_id"`
	Name       string `bun:"name"`
	StartsAt   string `bun:"starts_at"`
	EndsAt     string `bun:"ends_at"`
	PriceCents int64  `bun:"price_cents"`
}

const (
	holidayPeriodColumns = `id, name, registration_opens_on, registration_closes_on, binding, notes,
		status, published_at, updated_at, public_token,
		(SELECT count(*) FROM enrollment.holiday_registrations AS registration
			WHERE registration.organization_id = holiday_periods.organization_id
				AND registration.holiday_period_id = holiday_periods.id) AS registration_count`
	listHolidayPeriodsQuery = `SELECT ` + holidayPeriodColumns + ` FROM enrollment.holiday_periods
		WHERE organization_id = ?
		ORDER BY registration_opens_on DESC, id DESC`
	findHolidayPeriodQuery = `SELECT ` + holidayPeriodColumns + ` FROM enrollment.holiday_periods
		WHERE organization_id = ? AND id = ?`
	lockHolidayPeriodQuery = findHolidayPeriodQuery + ` FOR UPDATE`
	listHolidayWeeksQuery  = `SELECT id, holiday_period_id, starts_on, ends_on, host_school_id
		FROM enrollment.holiday_period_weeks
		WHERE organization_id = ? AND holiday_period_id = ANY(?)
		ORDER BY starts_on, id`
	listHolidayTimeModelsQuery = `SELECT model.id, model.holiday_period_week_id, model.name,
			to_char(model.starts_at, 'HH24:MI') AS starts_at, to_char(model.ends_at, 'HH24:MI') AS ends_at,
			model.price_cents
		FROM enrollment.holiday_week_time_models AS model
		JOIN enrollment.holiday_period_weeks AS week ON week.id = model.holiday_period_week_id
		WHERE model.organization_id = ? AND week.holiday_period_id = ANY(?)
		ORDER BY model.starts_at, model.ends_at, model.id`
)

// ListHolidayPeriods reads every period of the Träger in three statements.
func (s *HolidayPeriodStore) ListHolidayPeriods(ctx context.Context, organizationID int64) ([]enrollment.HolidayPeriod, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []holidayPeriodRow
	if err := db.NewRaw(listHolidayPeriodsQuery, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: list holiday periods: %w", err)
	}
	return s.withWeeks(ctx, db, organizationID, rows)
}

// FindHolidayPeriod reads one period; lock holds its row until the end of the
// transaction.
func (s *HolidayPeriodStore) FindHolidayPeriod(ctx context.Context, organizationID, periodID int64, lock bool) (enrollment.HolidayPeriod, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayPeriod{}, false, err
	}
	var rows []holidayPeriodRow
	if lock {
		err = db.NewRaw(lockHolidayPeriodQuery, organizationID, periodID).Scan(ctx, &rows)
	} else {
		err = db.NewRaw(findHolidayPeriodQuery, organizationID, periodID).Scan(ctx, &rows)
	}
	if err != nil {
		return enrollment.HolidayPeriod{}, false, fmt.Errorf("enrollment postgres: read holiday period: %w", err)
	}
	if len(rows) == 0 {
		return enrollment.HolidayPeriod{}, false, nil
	}
	periods, err := s.withWeeks(ctx, db, organizationID, rows)
	if err != nil {
		return enrollment.HolidayPeriod{}, false, err
	}
	return periods[0], true, nil
}

// withWeeks attaches the weeks and time models of rows, one statement each.
func (s *HolidayPeriodStore) withWeeks(ctx context.Context, db bun.IDB, organizationID int64, rows []holidayPeriodRow) ([]enrollment.HolidayPeriod, error) {
	periods := make([]enrollment.HolidayPeriod, 0, len(rows))
	if len(rows) == 0 {
		return periods, nil
	}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	var weekRows []holidayWeekRow
	if err := db.NewRaw(listHolidayWeeksQuery, organizationID, pgdialect.Array(ids)).Scan(ctx, &weekRows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: list holiday weeks: %w", err)
	}
	var modelRows []holidayTimeModelRow
	if err := db.NewRaw(listHolidayTimeModelsQuery, organizationID, pgdialect.Array(ids)).Scan(ctx, &modelRows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: list holiday time models: %w", err)
	}
	models := map[int64][]enrollment.HolidayTimeModel{}
	for _, row := range modelRows {
		models[row.WeekID] = append(models[row.WeekID], enrollment.HolidayTimeModel{
			ID: row.ID, Name: row.Name, StartsAt: row.StartsAt, EndsAt: row.EndsAt, PriceCents: row.PriceCents,
		})
	}
	weeks := map[int64][]enrollment.HolidayWeek{}
	for _, row := range weekRows {
		timeModels := models[row.ID]
		if timeModels == nil {
			timeModels = []enrollment.HolidayTimeModel{}
		}
		weeks[row.HolidayPeriodID] = append(weeks[row.HolidayPeriodID], enrollment.HolidayWeek{
			ID: row.ID, StartsOn: row.StartsOn, EndsOn: row.EndsOn, HostSchoolID: row.HostSchoolID, TimeModels: timeModels,
		})
	}
	for _, row := range rows {
		periodWeeks := weeks[row.ID]
		if periodWeeks == nil {
			periodWeeks = []enrollment.HolidayWeek{}
		}
		periods = append(periods, enrollment.HolidayPeriod{
			ID: row.ID, Name: row.Name, RegistrationOpensOn: row.RegistrationOpensOn,
			RegistrationClosesOn: row.RegistrationClosesOn, Binding: row.Binding, Notes: row.Notes,
			Status: row.Status, PublishedAt: row.PublishedAt, UpdatedAt: row.UpdatedAt,
			PublicToken: row.PublicToken, RegistrationCount: row.RegistrationCount, Weeks: periodWeeks,
		})
	}
	return periods, nil
}

const insertHolidayPeriodQuery = `INSERT INTO enrollment.holiday_periods
		(organization_id, name, registration_opens_on, registration_closes_on, binding, notes)
	VALUES (?, ?, ?, ?, ?, ?)
	RETURNING id`

// InsertHolidayPeriod adds a draft period with its weeks and time models.
func (s *HolidayPeriodStore) InsertHolidayPeriod(ctx context.Context, organizationID int64, input enrollment.HolidayPeriodInput) (int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	var periodID int64
	if err := db.NewRaw(insertHolidayPeriodQuery, organizationID, input.Name, input.RegistrationOpensOn,
		input.RegistrationClosesOn, input.Binding, input.Notes).Scan(ctx, &periodID); err != nil {
		return 0, fmt.Errorf("enrollment postgres: insert holiday period: %w", err)
	}
	if err := writeHolidayWeeks(ctx, db, organizationID, periodID, input.Weeks); err != nil {
		return 0, err
	}
	return periodID, nil
}

// checkHolidayBookingsNowQuery makes the deferred keys of the bookings check
// each removal of a week or time model at once, so the change names the
// refusal instead of failing at commit (#4661).
const checkHolidayBookingsNowQuery = `SET CONSTRAINTS enrollment.fk_holiday_registration_bookings_week,
	enrollment.fk_holiday_registration_bookings_time_model IMMEDIATE`

const updateHolidayPeriodQuery = `UPDATE enrollment.holiday_periods
	SET name = ?, registration_opens_on = ?, registration_closes_on = ?, binding = ?, notes = ?
	WHERE organization_id = ? AND id = ?`

// UpdateHolidayPeriod writes the period and its weeks and time models.
func (s *HolidayPeriodStore) UpdateHolidayPeriod(ctx context.Context, organizationID, periodID int64, input enrollment.HolidayPeriodInput) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(updateHolidayPeriodQuery, input.Name, input.RegistrationOpensOn, input.RegistrationClosesOn,
		input.Binding, input.Notes, organizationID, periodID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: update holiday period: %w", err)
	}
	if _, err := db.NewRaw(checkHolidayBookingsNowQuery).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: check holiday bookings: %w", err)
	}
	return writeHolidayWeeks(ctx, db, organizationID, periodID, input.Weeks)
}

const publishHolidayPeriodQuery = `UPDATE enrollment.holiday_periods
	SET status = 'published', published_at = statement_timestamp()
	WHERE organization_id = ? AND id = ? AND status = 'draft'`

// PublishHolidayPeriod marks a draft period published.
func (s *HolidayPeriodStore) PublishHolidayPeriod(ctx context.Context, organizationID, periodID int64) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(publishHolidayPeriodQuery, organizationID, periodID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: publish holiday period: %w", err)
	}
	return nil
}

const (
	deleteHolidayWeeksQuery = `DELETE FROM enrollment.holiday_period_weeks
		WHERE organization_id = ? AND holiday_period_id = ? AND NOT (id = ANY(?))`
	insertHolidayWeekQuery = `INSERT INTO enrollment.holiday_period_weeks
			(organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id`
	updateHolidayWeekQuery = `UPDATE enrollment.holiday_period_weeks
		SET starts_on = ?, ends_on = ?, host_school_id = ?
		WHERE organization_id = ? AND holiday_period_id = ? AND id = ?`
	deleteHolidayTimeModelsQuery = `DELETE FROM enrollment.holiday_week_time_models
		WHERE organization_id = ? AND holiday_period_week_id = ? AND NOT (id = ANY(?))`
	insertHolidayTimeModelQuery = `INSERT INTO enrollment.holiday_week_time_models
			(organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, ?, ?::time, ?::time, ?)`
	updateHolidayTimeModelQuery = `UPDATE enrollment.holiday_week_time_models
		SET name = ?, starts_at = ?::time, ends_at = ?::time, price_cents = ?
		WHERE organization_id = ? AND holiday_period_week_id = ? AND id = ?`
)

// writeHolidayWeeks makes the period's weeks those of weeks: the weeks left
// out go with their time models, the named ones change, the new ones arrive.
func writeHolidayWeeks(ctx context.Context, db bun.IDB, organizationID, periodID int64, weeks []enrollment.HolidayWeekInput) error {
	keep := make([]int64, 0, len(weeks))
	for _, week := range weeks {
		if week.ID != 0 {
			keep = append(keep, week.ID)
		}
	}
	if _, err := db.NewRaw(deleteHolidayWeeksQuery, organizationID, periodID, pgdialect.Array(keep)).Exec(ctx); err != nil {
		return bookedHolidayWeekError(fmt.Errorf("enrollment postgres: remove holiday weeks: %w", err))
	}
	for index, week := range weeks {
		weekID, err := writeHolidayWeek(ctx, db, organizationID, periodID, week)
		if err != nil {
			return markHolidayWeekError(index, err)
		}
		if err := writeHolidayTimeModels(ctx, db, organizationID, weekID, week.TimeModels); err != nil {
			return err
		}
	}
	return nil
}

func writeHolidayWeek(ctx context.Context, db bun.IDB, organizationID, periodID int64, week enrollment.HolidayWeekInput) (int64, error) {
	if week.ID == 0 {
		var weekID int64
		err := db.NewRaw(insertHolidayWeekQuery, organizationID, periodID, week.StartsOn, week.EndsOn, week.HostSchoolID).Scan(ctx, &weekID)
		if err != nil {
			return 0, fmt.Errorf("enrollment postgres: insert holiday week: %w", err)
		}
		return weekID, nil
	}
	if _, err := db.NewRaw(updateHolidayWeekQuery, week.StartsOn, week.EndsOn, week.HostSchoolID,
		organizationID, periodID, week.ID).Exec(ctx); err != nil {
		return 0, fmt.Errorf("enrollment postgres: update holiday week: %w", err)
	}
	return week.ID, nil
}

func writeHolidayTimeModels(ctx context.Context, db bun.IDB, organizationID, weekID int64, models []enrollment.HolidayTimeModelInput) error {
	keep := make([]int64, 0, len(models))
	for _, model := range models {
		if model.ID != 0 {
			keep = append(keep, model.ID)
		}
	}
	if _, err := db.NewRaw(deleteHolidayTimeModelsQuery, organizationID, weekID, pgdialect.Array(keep)).Exec(ctx); err != nil {
		return bookedHolidayWeekError(fmt.Errorf("enrollment postgres: remove holiday time models: %w", err))
	}
	for _, model := range models {
		var err error
		if model.ID == 0 {
			_, err = db.NewRaw(insertHolidayTimeModelQuery, organizationID, weekID, model.Name, model.StartsAt,
				model.EndsAt, model.PriceCents).Exec(ctx)
		} else {
			_, err = db.NewRaw(updateHolidayTimeModelQuery, model.Name, model.StartsAt, model.EndsAt, model.PriceCents,
				organizationID, weekID, model.ID).Exec(ctx)
		}
		if err != nil {
			return fmt.Errorf("enrollment postgres: write holiday time model: %w", err)
		}
	}
	return nil
}

// markHolidayWeekError names the refused week when its host school is
// outside the Träger; the database decides it, nothing is pre-checked.
func markHolidayWeekError(index int, err error) error {
	var postgresError pgdriver.Error
	if errors.As(err, &postgresError) && postgresError.IntegrityViolation() && postgresError.Field('n') == holidayHostSchoolConstraint {
		return &enrollment.HolidayHostSchoolRefusal{Week: index, Err: err}
	}
	return err
}

// bookedHolidayWeekError names the removal of a booked week or time model;
// the foreign keys of the bookings decide it, nothing is pre-checked.
func bookedHolidayWeekError(err error) error {
	var postgresError pgdriver.Error
	if errors.As(err, &postgresError) && postgresError.IntegrityViolation() && holidayBookingConstraints[postgresError.Field('n')] {
		return fmt.Errorf("%w: %w", enrollment.ErrHolidayWeekBooked, err)
	}
	return err
}
