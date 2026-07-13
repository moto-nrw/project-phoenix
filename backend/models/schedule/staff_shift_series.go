package schedule

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
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
	base.Model `bun:"schema:schedule,table:staff_shift_series"`
	base.TenantModel
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
	WeekPattern  int            `bun:"week_pattern,notnull,default:0" json:"week_pattern"`
	ValidFrom    timezone.Date  `bun:"valid_from,notnull,type:date" json:"valid_from"`
	ValidUntil   *timezone.Date `bun:"valid_until,type:date" json:"valid_until,omitempty"`
	SeriesRootID *int64         `bun:"series_root_id" json:"series_root_id,omitempty"`
	CreatedBy    int64          `bun:"created_by,notnull" json:"created_by"`
	UpdatedBy    *int64         `bun:"updated_by" json:"updated_by,omitempty"`

	Staff *users.Staff `bun:"rel:belongs-to,join:tenant_id=tenant_id,join:staff_id=id" json:"staff,omitempty"`
}

// Week pattern values shared with the timetable recurrence primitives.
const (
	WeekPatternEvery = 0
	WeekPatternA     = 1
	WeekPatternB     = 2
)

// Validate ensures series data is consistent. Period-dependent rules
// (validity within the period, A/B requires a week cycle) live in the
// service, which has the period loaded.
func (s *StaffShiftSeries) Validate() error {
	if s.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if len(s.Weekdays) == 0 {
		return errors.New("at least one weekday is required")
	}
	seen := map[int16]bool{}
	for _, wd := range s.Weekdays {
		if wd < 1 || wd > 7 {
			return errors.New("weekdays must be ISO weekdays between 1 and 7")
		}
		if seen[wd] {
			return errors.New("weekdays must not repeat")
		}
		seen[wd] = true
	}
	if err := validateShiftWindow(s.StartTime, s.EndTime, s.BreakMinutes); err != nil {
		return err
	}
	if s.CalendarPeriodID <= 0 {
		return errors.New("calendar period is required")
	}
	if s.WeekPattern < WeekPatternEvery || s.WeekPattern > WeekPatternB {
		return errors.New("week pattern must be 0 (every week), 1 (week A), or 2 (week B)")
	}
	if s.ValidFrom.IsZero() {
		return errors.New("valid from is required")
	}
	if s.ValidUntil != nil && !s.ValidUntil.After(s.ValidFrom) {
		return errors.New("valid until must be after valid from")
	}
	if s.CreatedBy <= 0 {
		return errors.New("created by is required")
	}
	return nil
}

// RootID returns the lineage root of this series segment (itself when it was
// never split), mirroring the timetable template convention.
func (s *StaffShiftSeries) RootID() int64 {
	if s.SeriesRootID != nil {
		return *s.SeriesRootID
	}
	return s.ID
}

// ContainsWeekday reports whether the ISO weekday (1=Monday … 7=Sunday) is
// part of the series.
func (s *StaffShiftSeries) ContainsWeekday(weekday int) bool {
	for _, wd := range s.Weekdays {
		if int(wd) == weekday {
			return true
		}
	}
	return false
}

// StaffShiftSeriesException records one deliberately removed occurrence of a
// series: deleting a single materialized series shift removes the concrete
// row and stores its date here so re-plans and splits never regenerate it.
type StaffShiftSeriesException struct {
	base.Model `bun:"schema:schedule,table:staff_shift_series_exceptions"`
	base.TenantModel
	SeriesID  int64         `bun:"series_id,notnull" json:"series_id"`
	Date      timezone.Date `bun:"date,notnull,type:date" json:"date"`
	CreatedBy int64         `bun:"created_by,notnull" json:"created_by"`
}
