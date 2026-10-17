package postgres

import (
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

type staffShiftRow struct {
	bun.BaseModel        `bun:"table:schedule.staff_shifts,alias:staff_shift"`
	ID                   int64         `bun:"id,pk,autoincrement"`
	TenantID             int64         `bun:"tenant_id,notnull"`
	StaffID              int64         `bun:"staff_id,notnull"`
	Date                 calendarDate  `bun:"date,notnull,type:date"`
	StartTime            time.Time     `bun:"start_time,notnull"`
	EndTime              time.Time     `bun:"end_time,notnull"`
	BreakMinutes         int           `bun:"break_minutes,notnull,default:0"`
	ShiftTypeID          *int64        `bun:"shift_type_id"`
	Notes                string        `bun:"notes"`
	SeriesID             *int64        `bun:"series_id"`
	Detached             bool          `bun:"detached,notnull,default:false"`
	SeriesOccurrenceDate *calendarDate `bun:"series_occurrence_date,type:date"`
	Cancelled            bool          `bun:"cancelled,notnull,default:false"`
	ChangeReason         *string       `bun:"change_reason"`
	OriginShiftID        *int64        `bun:"origin_shift_id"`
	SickAbsenceID        *int64        `bun:"sick_absence_id"`
	CreatedBy            int64         `bun:"created_by,notnull"`
	UpdatedBy            *int64        `bun:"updated_by"`
	CreatedAt            time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt            time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffShiftSeriesRow struct {
	bun.BaseModel             `bun:"table:schedule.staff_shift_series,alias:staff_shift_series"`
	ID                        int64         `bun:"id,pk,autoincrement"`
	TenantID                  int64         `bun:"tenant_id,notnull"`
	StaffID                   int64         `bun:"staff_id,notnull"`
	Weekdays                  []int16       `bun:"weekdays,array,notnull"`
	StartTime                 time.Time     `bun:"start_time,notnull"`
	EndTime                   time.Time     `bun:"end_time,notnull"`
	BreakMinutes              int           `bun:"break_minutes,notnull,default:0"`
	ShiftTypeID               *int64        `bun:"shift_type_id"`
	Notes                     string        `bun:"notes"`
	CalendarPeriodID          int64         `bun:"calendar_period_id,notnull"`
	WeekPattern               int           `bun:"week_pattern,notnull,default:0"`
	ValidFrom                 calendarDate  `bun:"valid_from,notnull,type:date"`
	ValidUntil                *calendarDate `bun:"valid_until,type:date"`
	SeriesRootID              *int64        `bun:"series_root_id"`
	RetainedOccurrenceShiftID *int64        `bun:"retained_occurrence_shift_id"`
	IncludeSchoolBreaks       bool          `bun:"include_school_breaks,notnull,default:false"`
	CreatedBy                 int64         `bun:"created_by,notnull"`
	UpdatedBy                 *int64        `bun:"updated_by"`
	CreatedAt                 time.Time     `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt                 time.Time     `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type staffShiftSeriesExceptionRow struct {
	bun.BaseModel `bun:"table:schedule.staff_shift_series_exceptions,alias:staff_shift_series_exception"`
	ID            int64        `bun:"id,pk,autoincrement"`
	TenantID      int64        `bun:"tenant_id,notnull"`
	SeriesID      int64        `bun:"series_id,notnull"`
	Date          calendarDate `bun:"date,notnull,type:date"`
	CreatedBy     int64        `bun:"created_by,notnull"`
	CreatedAt     time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

type shiftTypeRow struct {
	bun.BaseModel `bun:"table:schedule.shift_types,alias:shift_type"`
	ID            int64  `bun:"id,pk,autoincrement"`
	TenantID      int64  `bun:"tenant_id,notnull"`
	Name          string `bun:"name,notnull"`
	Color         string `bun:"color,notnull"`
	Description   string `bun:"description"`
	// IsActive carries no default tag on purpose: bun would write DEFAULT for
	// a false value and the column default is TRUE.
	IsActive  bool      `bun:"is_active,notnull"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// requiredClock binds a wall clock to a TIME column. The clock is anchored at
// 0001-01-01 UTC: PostgreSQL rejects the year-0 anchor time.Parse produces,
// and the UTC location keeps the driver's conversion from moving the hour.
func requiredClock(value, field string) (time.Time, error) {
	parsed, err := time.Parse(domain.ClockLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("workforce postgres: %s %q is not a %s wall clock: %w", field, value, domain.ClockLayout, err)
	}
	return time.Date(1, time.January, 1, parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.UTC), nil
}

// wallClockString reads the time-of-day components only, discarding the date
// anchor and location the driver attached on scan.
func wallClockString(value time.Time) string {
	return value.Format(domain.ClockLayout)
}

func staffShiftFromDomain(value domain.StaffShift) (*staffShiftRow, error) {
	start, err := requiredClock(value.StartTime, "start time")
	if err != nil {
		return nil, err
	}
	end, err := requiredClock(value.EndTime, "end time")
	if err != nil {
		return nil, err
	}
	return &staffShiftRow{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Date: calendarDate(value.Date),
		StartTime: start, EndTime: end, BreakMinutes: value.BreakMinutes, ShiftTypeID: value.ShiftTypeID,
		Notes: value.Notes, SeriesID: value.SeriesID, Detached: value.Detached,
		SeriesOccurrenceDate: optionalCalendarDate(value.SeriesOccurrenceDate), Cancelled: value.Cancelled,
		ChangeReason: value.ChangeReason, OriginShiftID: value.OriginShiftID, SickAbsenceID: value.SickAbsenceID,
		CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}, nil
}

func staffShiftToDomain(row staffShiftRow) domain.StaffShift {
	return domain.StaffShift{
		ID: row.ID, TenantID: row.TenantID, StaffID: row.StaffID, Date: string(row.Date),
		StartTime: wallClockString(row.StartTime), EndTime: wallClockString(row.EndTime), BreakMinutes: row.BreakMinutes,
		ShiftTypeID: row.ShiftTypeID, Notes: row.Notes, SeriesID: row.SeriesID, Detached: row.Detached,
		SeriesOccurrenceDate: calendarDateString(row.SeriesOccurrenceDate), Cancelled: row.Cancelled,
		ChangeReason: row.ChangeReason, OriginShiftID: row.OriginShiftID, SickAbsenceID: row.SickAbsenceID,
		CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func staffShiftsToDomain(rows []staffShiftRow) []domain.StaffShift {
	result := make([]domain.StaffShift, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffShiftToDomain(row))
	}
	return result
}

func shiftTypeFromDomain(value domain.ShiftType) *shiftTypeRow {
	return &shiftTypeRow{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name, Color: value.Color,
		Description: value.Description, IsActive: value.IsActive, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func shiftTypeToDomain(row shiftTypeRow) domain.ShiftType {
	return domain.ShiftType{
		ID: row.ID, TenantID: row.TenantID, Name: row.Name, Color: row.Color,
		Description: row.Description, IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
