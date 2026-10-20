package planning

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Dienstplan rows the planning services work on (#3424). They are
// Workforce's own vocabulary: calendar days are timezone.Date and the shift
// windows are wall clocks anchored at 0001-01-01 UTC, the shape the services
// compare and materialize with. Business operations map them onto the
// native capability values; the validation below is the rule set the
// services relied on before the rows moved here, wording included.

// Week pattern values shared with the School Calendar's week-cycle engine
// (schoolcalendar.WeekPatternApplies): every week, week A or week B.
const (
	WeekPatternEvery = domain.WeekPatternEvery
	WeekPatternA     = domain.WeekPatternA
	WeekPatternB     = domain.WeekPatternB
)

// StaffShiftSeries is one recurring shift rule (#1889): the same wall-clock
// window on a set of ISO weekdays, bound to a calendar period. ValidUntil is
// exclusive (nil runs to the period end); SeriesRootID links the segments of
// "Ab jetzt dauerhaft" splits.
type StaffShiftSeries struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TenantID  int64     `json:"tenant_id"`
	StaffID   int64     `json:"staff_id"`
	// Weekdays are ISO weekdays (1=Monday … 7=Sunday), non-empty.
	Weekdays     []int16   `json:"weekdays"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	BreakMinutes int       `json:"break_minutes"`
	ShiftTypeID  *int64    `json:"shift_type_id,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	// CalendarPeriodID bounds the series; the period carries the week A/B
	// cycle the School Calendar's week-pattern engine evaluates.
	CalendarPeriodID int64 `json:"calendar_period_id"`
	// WeekPattern: 0 = every week, 1 = week A, 2 = week B.
	WeekPattern  int            `json:"week_pattern"`
	ValidFrom    calendar.Date  `json:"valid_from"`
	ValidUntil   *calendar.Date `json:"valid_until,omitempty"`
	SeriesRootID *int64         `json:"series_root_id,omitempty"`
	// RetainedOccurrenceShiftID records the current-day row a same-day
	// permanent edit retained; it is separate from Detached, which one-off
	// deviations set as well.
	RetainedOccurrenceShiftID *int64 `json:"-"`
	// IncludeSchoolBreaks also plans the series in the Ferien and on closing
	// days; statutory holidays stay skipped either way (#3820).
	IncludeSchoolBreaks bool   `json:"include_school_breaks"`
	CreatedBy           int64  `json:"created_by"`
	UpdatedBy           *int64 `json:"updated_by,omitempty"`
}

// Validate ensures series data is consistent. Period-dependent rules
// (validity within the period, A/B requires a week cycle) live in the
// service, which has the period loaded.
func (s *StaffShiftSeries) Validate() error {
	if s.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if err := validateSeriesWeekdays(s.Weekdays); err != nil {
		return err
	}
	if err := workforce.ValidateShiftRecordWindow(s.StartTime, s.EndTime, s.BreakMinutes); err != nil {
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

// validateSeriesWeekdays requires a non-empty set of distinct ISO weekdays.
func validateSeriesWeekdays(weekdays []int16) error {
	if len(weekdays) == 0 {
		return errors.New("at least one weekday is required")
	}
	seen := map[int16]bool{}
	for _, wd := range weekdays {
		if wd < 1 || wd > 7 {
			return errors.New("weekdays must be ISO weekdays between 1 and 7")
		}
		if seen[wd] {
			return errors.New("weekdays must not repeat")
		}
		seen[wd] = true
	}
	return nil
}

// RootID returns the lineage root of this series segment (itself when it was
// never split).
func (s *StaffShiftSeries) RootID() int64 {
	if s.SeriesRootID != nil {
		return *s.SeriesRootID
	}
	return s.ID
}

// ContainsWeekday reports whether the ISO weekday is part of the series.
func (s *StaffShiftSeries) ContainsWeekday(weekday int) bool {
	for _, wd := range s.Weekdays {
		if int(wd) == weekday {
			return true
		}
	}
	return false
}

// StaffShiftSeriesException records one deliberately removed occurrence of a
// series, so re-plans and splits never regenerate it.
type StaffShiftSeriesException struct {
	ID        int64         `json:"id"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	TenantID  int64         `json:"tenant_id"`
	SeriesID  int64         `json:"series_id"`
	Date      calendar.Date `json:"date"`
	CreatedBy int64         `json:"created_by"`
}
