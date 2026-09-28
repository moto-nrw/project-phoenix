package planning

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// The Dienstplan rows the planning services work on (#3424). They are
// Workforce's own vocabulary: calendar days are timezone.Date and the shift
// windows are wall clocks anchored at 0001-01-01 UTC, the shape the services
// compare and materialize with. ShiftRows and its siblings map them onto the
// public capability types; the validation below is the row rule set the
// services relied on before the rows moved here, wording included.

// Week pattern values shared with the School Calendar's week-cycle engine
// (schoolcalendar.WeekPatternApplies): every week, week A or week B.
const (
	WeekPatternEvery = domain.WeekPatternEvery
	WeekPatternA     = domain.WeekPatternA
	WeekPatternB     = domain.WeekPatternB
)

// StaffShift is one planned work shift of a staff member on a calendar day.
// Several non-overlapping shifts per staff member and day are allowed.
type StaffShift struct {
	ID        int64         `json:"id"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	TenantID  int64         `json:"tenant_id"`
	StaffID   int64         `json:"staff_id"`
	Date      timezone.Date `json:"date"`
	// StartTime/EndTime carry only a meaningful wall-clock portion.
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	BreakMinutes int       `json:"break_minutes"`
	// ShiftTypeID optionally links the shift to a Schichtart (#1836).
	ShiftTypeID *int64 `json:"shift_type_id,omitempty"`
	Notes       string `json:"notes,omitempty"`
	// SeriesID links a materialized row to its series (#1889); Detached marks
	// a series row edited via "Nur diese Woche", which re-plans keep.
	SeriesID *int64 `json:"series_id,omitempty"`
	Detached bool   `json:"detached"`
	// SeriesOccurrenceDate is the recurrence slot the row materialized for.
	// It does not follow Date when an occurrence is moved, so exceptions,
	// re-plans and splits keep addressing the source slot.
	SeriesOccurrenceDate *timezone.Date `json:"-"`
	// Cancelled marks a shift that does not take place (#1841).
	Cancelled bool `json:"cancelled"`
	// ChangeReason carries the optional "why" of a flexible daily change.
	ChangeReason *string `json:"change_reason,omitempty"`
	// OriginShiftID marks a replacement covering another, cancelled shift.
	OriginShiftID *int64 `json:"origin_shift_id,omitempty"`
	// SickAbsenceID identifies the sick report whose cascade cancelled the
	// shift (#1843).
	SickAbsenceID *int64 `json:"sick_absence_id,omitempty"`
	CreatedBy     int64  `json:"created_by"`
	UpdatedBy     *int64 `json:"updated_by,omitempty"`

	// ShiftType is the resolved Schichtart for ShiftTypeID, filled by the
	// service for readers without access to the shift-type administration
	// (#1844). Nil when the shift has no type or the resolution was skipped.
	ShiftType *ShiftType `json:"shift_type,omitempty"`
}

// Validate ensures shift data is consistent.
func (s *StaffShift) Validate() error {
	if s.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if s.Date.IsZero() {
		return errors.New("date is required")
	}
	if err := validateShiftWindow(s.StartTime, s.EndTime, s.BreakMinutes); err != nil {
		return err
	}
	if s.CreatedBy <= 0 {
		return errors.New("created by is required")
	}
	return nil
}

// Overlaps reports whether the wall-clock windows of two shifts on the same
// date intersect; touching boundaries do not overlap.
func (s *StaffShift) Overlaps(other *StaffShift) bool {
	aStart, aEnd := normalizeWallClock(s.StartTime), normalizeWallClock(s.EndTime)
	bStart, bEnd := normalizeWallClock(other.StartTime), normalizeWallClock(other.EndTime)
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

// Contains reports whether other's wall-clock window lies entirely within
// s's window, shared boundaries included. It keeps a replacement inside the
// gap of the cancelled origin it covers (#1841).
func (s *StaffShift) Contains(other *StaffShift) bool {
	sStart, sEnd := normalizeWallClock(s.StartTime), normalizeWallClock(s.EndTime)
	oStart, oEnd := normalizeWallClock(other.StartTime), normalizeWallClock(other.EndTime)
	return !oStart.Before(sStart) && !oEnd.After(sEnd)
}

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
	ValidFrom    timezone.Date  `json:"valid_from"`
	ValidUntil   *timezone.Date `json:"valid_until,omitempty"`
	SeriesRootID *int64         `json:"series_root_id,omitempty"`
	// RetainedOccurrenceShiftID records the current-day row a same-day
	// permanent edit retained; it is separate from Detached, which one-off
	// deviations set as well.
	RetainedOccurrenceShiftID *int64 `json:"-"`
	CreatedBy                 int64  `json:"created_by"`
	UpdatedBy                 *int64 `json:"updated_by,omitempty"`
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
	Date      timezone.Date `json:"date"`
	CreatedBy int64         `json:"created_by"`
}

// ShiftType is a tenant-defined Schichtart: the kind of duty a planned shift
// stands for, with the color the Dienstplan grid shows it in (#1836).
type ShiftType struct {
	ID          int64     `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	TenantID    int64     `json:"tenant_id"`
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	Description string    `json:"description,omitempty"`
	IsActive    bool      `json:"is_active"`
}

const (
	maxShiftTypeNameLength        = 100
	maxShiftTypeDescriptionLength = 500
)

// shiftTypeHexColorPattern matches #RRGGBB or #RGB hex colors.
var shiftTypeHexColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

// Validate normalizes and checks the shift type. It trims the name, defaults
// an empty color to gray, prepends a missing "#", enforces a hex color and
// expands a 3-digit shorthand to the 6-digit form.
func (t *ShiftType) Validate() error {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		return errors.New("shift type name is required")
	}
	if len([]rune(t.Name)) > maxShiftTypeNameLength {
		return errors.New("shift type name must not exceed 100 characters")
	}
	if len([]rune(t.Description)) > maxShiftTypeDescriptionLength {
		return errors.New("shift type description must not exceed 500 characters")
	}

	color := strings.TrimSpace(t.Color)
	if color == "" {
		color = domain.DefaultShiftTypeColor
	}
	if !strings.HasPrefix(color, "#") {
		color = "#" + color
	}
	if !shiftTypeHexColorPattern.MatchString(color) {
		return errors.New("invalid color format, must be a valid hex color")
	}
	t.Color = strings.ToUpper(expandShorthandHex(color))
	return nil
}

// expandShorthandHex turns a validated "#RGB" color into "#RRGGBB", the form
// the frontend's native color input round-trips.
func expandShorthandHex(color string) string {
	if len(color) != 4 { // "#" + 3 hex digits
		return color
	}
	r, g, b := color[1], color[2], color[3]
	return string([]byte{'#', r, r, g, g, b, b})
}

// validateShiftWindow holds the wall-clock rules shared by single shifts and
// series: end after start, break within 0..MaxStaffShiftBreakMinutes and not
// longer than the shift itself.
func validateShiftWindow(startTime, endTime time.Time, breakMinutes int) error {
	start := normalizeWallClock(startTime)
	end := normalizeWallClock(endTime)
	if !end.After(start) {
		return errors.New("end time must be after start time")
	}
	if breakMinutes < 0 {
		return errors.New("break minutes must not be negative")
	}
	if breakMinutes > domain.MaxStaffShiftBreakMinutes {
		return errors.New("break minutes must not exceed 300")
	}
	durationMinutes := int(end.Sub(start) / time.Minute)
	if breakMinutes > durationMinutes {
		return errors.New("break minutes must not exceed shift duration")
	}
	return nil
}

// normalizeWallClock keeps only the time of day, nanoseconds included, at
// the 0001-01-01 UTC anchor the row comparisons use.
func normalizeWallClock(value time.Time) time.Time {
	return time.Date(1, time.January, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}
