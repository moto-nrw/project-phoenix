package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

type Database func(context.Context) (bun.IDB, int64, error)
type Reader struct{ database Database }

func New(database Database) *Reader { return &Reader{database: database} }

const schoolClosingDaysQuery = `SELECT id, created_at, updated_at, start_date AS start_date, end_date AS end_date, reason, false AS traeger
		FROM schedule.closing_days
		WHERE tenant_id = ? AND start_date <= ? AND end_date >= ?
		UNION ALL
		SELECT id, created_at, updated_at, start_date, end_date, reason, true
		FROM schedule.traeger_closing_days AS traeger_day
		WHERE ? = ANY (school_ids) AND start_date <= ? AND end_date >= ?
			AND EXISTS (SELECT 1 FROM platform.schools AS school
				WHERE school.id = ? AND school.organization_id = traeger_day.organization_id AND school.deleted_at IS NULL)
		ORDER BY start_date, traeger, id`

const careCalendarRangesSQL = `SELECT 'closing' AS kind, closing_day.start_date AS start_date, closing_day.end_date AS end_date
	FROM schedule.closing_days AS closing_day
	WHERE closing_day.tenant_id = ? AND closing_day.start_date <= ? AND closing_day.end_date >= ?
	UNION ALL
	SELECT 'closing', traeger_day.start_date, traeger_day.end_date
	FROM schedule.traeger_closing_days AS traeger_day
	WHERE ? = ANY (traeger_day.school_ids) AND traeger_day.start_date <= ? AND traeger_day.end_date >= ?
		AND EXISTS (SELECT 1 FROM platform.schools AS school
			WHERE school.id = ? AND school.organization_id = traeger_day.organization_id AND school.deleted_at IS NULL)
	UNION ALL
	SELECT 'holiday', calendar_period.start_date, calendar_period.end_date
	FROM schedule.calendar_periods AS calendar_period
	WHERE calendar_period.tenant_id = ? AND calendar_period.period_type = ? AND calendar_period.is_active
		AND calendar_period.start_date <= ? AND calendar_period.end_date >= ?`

type schoolClosingDayRow struct {
	ID        int64         `bun:"id"`
	CreatedAt time.Time     `bun:"created_at"`
	UpdatedAt time.Time     `bun:"updated_at"`
	StartDate calendar.Date `bun:"start_date,type:date"`
	EndDate   calendar.Date `bun:"end_date,type:date"`
	Reason    string        `bun:"reason"`
	Traeger   bool          `bun:"traeger"`
}

func (r *Reader) ListClosingDays(ctx context.Context, filter schoolcalendar.ClosingDayFilter) ([]schoolcalendar.ClosingDay, error) {
	db, schoolID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	from, to := filter.OverlappingFrom, filter.OverlappingTo
	if from == "" {
		from, to = "0001-01-01", "9999-12-31"
	}
	var rows []schoolClosingDayRow
	err = db.NewRaw(schoolClosingDaysQuery, schoolID, calendar.Date(to), calendar.Date(from), schoolID, calendar.Date(to), calendar.Date(from), schoolID).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("school closing days view: list closing days: %w", err)
	}
	result := make([]schoolcalendar.ClosingDay, 0, len(rows))
	for _, row := range rows {
		result = append(result, schoolcalendar.ClosingDay{ID: row.ID, TenantID: schoolID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StartDate: row.StartDate.String(), EndDate: row.EndDate.String(), Reason: row.Reason, Traeger: row.Traeger})
	}
	return result, nil
}

type careCalendarRangeRow struct {
	Kind      string        `bun:"kind"`
	StartDate calendar.Date `bun:"start_date,type:date"`
	EndDate   calendar.Date `bun:"end_date,type:date"`
}

func (r *Reader) ListCareCalendarRanges(ctx context.Context, from, to string) ([]schoolcalendar.ClosingDay, []schoolcalendar.ClosingDay, error) {
	db, schoolID, err := r.database(ctx)
	if err != nil {
		return nil, nil, err
	}
	var rows []careCalendarRangeRow
	err = db.NewRaw(careCalendarRangesSQL, schoolID, calendar.Date(to), calendar.Date(from), schoolID, calendar.Date(to), calendar.Date(from), schoolID,
		schoolID, schoolcalendar.PeriodTypeHoliday, calendar.Date(to), calendar.Date(from)).Scan(ctx, &rows)
	if err != nil {
		return nil, nil, fmt.Errorf("school closing days view: care calendar: %w", err)
	}
	var closing, holidays []schoolcalendar.ClosingDay
	for _, row := range rows {
		value := schoolcalendar.ClosingDay{StartDate: row.StartDate.String(), EndDate: row.EndDate.String()}
		if row.Kind == "closing" {
			closing = append(closing, value)
		} else {
			holidays = append(holidays, value)
		}
	}
	return closing, holidays, nil
}
