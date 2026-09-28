package schedule

import (
	"time"
)

// StaffShiftSeries is one recurring shift rule (#1889): the same wall-clock
// window on a set of weekdays, bound to a calendar period. It materializes
// concrete StaffShift rows upfront — every reader (time tracking,
// auto-checkout, coverage, weekly summaries) keeps seeing only concrete rows.
// ValidFrom/ValidUntil are split segment bounds within the period
// (ValidUntil exclusive, nil = until period end); SeriesRootID links all
// segments produced by "Ab jetzt dauerhaft" splits, mirroring the timetable
// template lineage.
type StaffShiftSeries struct {
	Model `bun:"schema:schedule,table:staff_shift_series"`
	TenantModel
	StaffID int64 `bun:"staff_id,notnull" json:"staff_id"`
	// Weekdays are ISO weekdays (1=Monday … 7=Sunday), non-empty.
	Weekdays []int16 `bun:"weekdays,array,notnull" json:"weekdays"`
	// StartTime/EndTime map TIME WITHOUT TIME ZONE columns; only the
	// wall-clock portion is meaningful (same semantics as StaffShift).
	StartTime    time.Time `bun:"start_time,notnull" json:"start_time"`
	EndTime      time.Time `bun:"end_time,notnull" json:"end_time"`
	BreakMinutes int       `bun:"break_minutes,notnull,default:0" json:"break_minutes"`
	ShiftTypeID  *int64    `bun:"shift_type_id" json:"shift_type_id,omitempty"`
	Notes        string    `bun:"notes" json:"notes,omitempty"`
	// CalendarPeriodID is required: the period bounds the series and carries
	// the week A/B cycle anchor that ShouldMaterializeWeekPattern evaluates.
	CalendarPeriodID int64 `bun:"calendar_period_id,notnull" json:"calendar_period_id"`
	// WeekPattern: 0 = every week, 1 = week A, 2 = week B (the encoding
	// ShouldMaterializeWeekPattern expects).
	WeekPattern  int    `bun:"week_pattern,notnull,default:0" json:"week_pattern"`
	ValidFrom    Date   `bun:"valid_from,notnull,type:date" json:"valid_from"`
	ValidUntil   *Date  `bun:"valid_until,type:date" json:"valid_until,omitempty"`
	SeriesRootID *int64 `bun:"series_root_id" json:"series_root_id,omitempty"`
	// RetainedOccurrenceShiftID records the concrete current-day row that a
	// same-day permanent edit retained. It is deliberately separate from
	// Detached: detached rows can also be ordinary one-off deviations, which
	// subsequent permanent edits must not overwrite.
	RetainedOccurrenceShiftID *int64 `bun:"retained_occurrence_shift_id" json:"-"`
	CreatedBy                 int64  `bun:"created_by,notnull" json:"created_by"`
	UpdatedBy                 *int64 `bun:"updated_by" json:"updated_by,omitempty"`
}

// Week pattern values shared with the timetable recurrence primitives.
const (
	WeekPatternEvery = 0
	WeekPatternA     = 1
	WeekPatternB     = 2
)

// StaffShiftSeriesException records one deliberately removed occurrence of a
// series: deleting a single materialized series shift removes the concrete
// row and stores its date here so re-plans and splits never regenerate it.
type StaffShiftSeriesException struct {
	Model `bun:"schema:schedule,table:staff_shift_series_exceptions"`
	TenantModel
	SeriesID  int64 `bun:"series_id,notnull" json:"series_id"`
	Date      Date  `bun:"date,notnull,type:date" json:"date"`
	CreatedBy int64 `bun:"created_by,notnull" json:"created_by"`
}
