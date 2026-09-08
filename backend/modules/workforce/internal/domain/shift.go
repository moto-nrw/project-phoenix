package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// Recurrence encoding shared with the timetable primitives: every week, or
// only week A / week B of the calendar period's two-week cycle.
const (
	WeekPatternEvery = 0
	WeekPatternA     = 1
	WeekPatternB     = 2

	// MaxStaffShiftBreakMinutes caps the break of one shift at five hours.
	MaxStaffShiftBreakMinutes = 300
	// DefaultShiftTypeColor is the grey a shift type gets without a color.
	DefaultShiftTypeColor = "#6B7280"

	maxShiftTypeNameLength        = 100
	maxShiftTypeDescriptionLength = 500
)

var (
	ErrStaffShiftNotFound  = errors.New("staff shift not found")
	ErrInvalidStaffShift   = errors.New("invalid staff shift input")
	ErrStaffShiftDuplicate = errors.New("staff shift already starts at that time")
	ErrShiftSeriesNotFound = errors.New("staff shift series not found")
	ErrInvalidShiftSeries  = errors.New("invalid staff shift series input")
	ErrShiftTypeNotFound   = errors.New("shift type not found")
	ErrInvalidShiftType    = errors.New("invalid shift type input")
	ErrShiftTypeNameTaken  = errors.New("shift type name is already taken")
)

// InvalidStaffShiftError carries the caller-facing validation reason.
type InvalidStaffShiftError struct{ Reason string }

func (e *InvalidStaffShiftError) Error() string { return e.Reason }
func (e *InvalidStaffShiftError) Unwrap() error { return ErrInvalidStaffShift }

func invalidShift(reason string) error { return &InvalidStaffShiftError{Reason: reason} }

// InvalidShiftSeriesError carries the caller-facing validation reason.
type InvalidShiftSeriesError struct{ Reason string }

func (e *InvalidShiftSeriesError) Error() string { return e.Reason }
func (e *InvalidShiftSeriesError) Unwrap() error { return ErrInvalidShiftSeries }

func invalidSeries(reason string) error { return &InvalidShiftSeriesError{Reason: reason} }

// InvalidShiftTypeError carries the caller-facing validation reason.
type InvalidShiftTypeError struct{ Reason string }

func (e *InvalidShiftTypeError) Error() string { return e.Reason }
func (e *InvalidShiftTypeError) Unwrap() error { return ErrInvalidShiftType }

func invalidShiftType(reason string) error { return &InvalidShiftTypeError{Reason: reason} }

// StaffShift is one planned presence of a staff member on a calendar day.
// Date and SeriesOccurrenceDate are calendar days in DateLayout; StartTime and
// EndTime are wall clocks in ClockLayout.
type StaffShift struct {
	ID           int64
	TenantID     int64
	StaffID      int64
	Date         string
	StartTime    string
	EndTime      string
	BreakMinutes int
	ShiftTypeID  *int64
	Notes        string
	SeriesID     *int64
	Detached     bool
	// SeriesOccurrenceDate is the recurrence slot a series row materialized
	// for; it stays put when the row is moved. Empty for standalone rows.
	SeriesOccurrenceDate string
	Cancelled            bool
	ChangeReason         *string
	OriginShiftID        *int64
	SickAbsenceID        *int64
	CreatedBy            int64
	UpdatedBy            *int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// StaffShiftOrderField names a column a shift listing may be ordered by.
type StaffShiftOrderField string

const (
	StaffShiftOrderDate      StaffShiftOrderField = "date"
	StaffShiftOrderStaffID   StaffShiftOrderField = "staff_id"
	StaffShiftOrderStartTime StaffShiftOrderField = "start_time"
	StaffShiftOrderID        StaffShiftOrderField = "id"
)

type StaffShiftOrder struct {
	Field      StaffShiftOrderField
	Descending bool
}

// StaffShiftFilter narrows a shift listing; every set field is combined with
// AND. From/To select rows with From <= date <= To. A non-nil StaffIDs,
// Dates or OriginShiftIDs that is empty matches nothing, because a caller
// that resolved an empty set expects an empty answer, not every row.
type StaffShiftFilter struct {
	StaffID        int64
	StaffIDs       []int64
	From           string
	To             string
	Dates          []string
	SeriesID       int64
	OriginShiftID  int64
	OriginShiftIDs []int64
	SickAbsenceID  *int64
	Cancelled      *bool
	Detached       *bool
	Order          []StaffShiftOrder
	Limit          int
	Offset         int
}

// Validate rejects malformed dates in the filter before they reach a query.
func (f StaffShiftFilter) Validate() error {
	if err := validateShiftDate(f.From, "from"); err != nil {
		return err
	}
	if err := validateShiftDate(f.To, "to"); err != nil {
		return err
	}
	for _, date := range f.Dates {
		if date == "" {
			return invalidShift("dates must not contain an empty day")
		}
		if err := validateShiftDate(date, "dates"); err != nil {
			return err
		}
	}
	return nil
}

// StaffShiftSeries is one recurring shift rule bound to a calendar period.
// ValidUntil is exclusive and empty while the series runs to the period end.
type StaffShiftSeries struct {
	ID                        int64
	TenantID                  int64
	StaffID                   int64
	Weekdays                  []int
	StartTime                 string
	EndTime                   string
	BreakMinutes              int
	ShiftTypeID               *int64
	Notes                     string
	CalendarPeriodID          int64
	WeekPattern               int
	ValidFrom                 string
	ValidUntil                string
	SeriesRootID              *int64
	RetainedOccurrenceShiftID *int64
	CreatedBy                 int64
	UpdatedBy                 *int64
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

// StaffShiftSeriesException is one deliberately removed occurrence of a
// series: re-plans and splits never regenerate the date.
type StaffShiftSeriesException struct {
	ID        int64
	TenantID  int64
	SeriesID  int64
	Date      string
	CreatedBy int64
	CreatedAt time.Time
}

// ShiftType is a tenant-defined label and color for planned shifts.
type ShiftType struct {
	ID          int64
	TenantID    int64
	Name        string
	Color       string
	Description string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// StaffShiftUpdatableColumns lists the columns a partial update may stamp
// without rewriting the whole row. Anything else is a whole-row update.
var StaffShiftUpdatableColumns = map[string]bool{
	"sick_absence_id": true,
	"cancelled":       true,
	"change_reason":   true,
	"shift_type_id":   true,
	"notes":           true,
	"updated_by":      true,
	"detached":        true,
	"series_id":       true,
}

// ValidateStaffShiftColumns rejects a partial update that names a column the
// capability does not expose for stamping.
func ValidateStaffShiftColumns(columns []string) error {
	if len(columns) == 0 {
		return invalidShift("at least one column is required")
	}
	for _, column := range columns {
		if !StaffShiftUpdatableColumns[column] {
			return invalidShift("column " + column + " cannot be updated partially")
		}
	}
	return nil
}

// ValidateStaffShift enforces the row rules shared with the legacy model:
// a staff member, a calendar day, a wall-clock window whose break fits, and
// the creating staff member.
func ValidateStaffShift(shift StaffShift) error {
	if shift.StaffID <= 0 {
		return invalidShift("staff ID is required")
	}
	if shift.Date == "" {
		return invalidShift("date is required")
	}
	if err := validateShiftDate(shift.Date, "date"); err != nil {
		return err
	}
	if err := validateShiftDate(shift.SeriesOccurrenceDate, "series_occurrence_date"); err != nil {
		return err
	}
	if err := validateShiftWindow(shift.StartTime, shift.EndTime, shift.BreakMinutes, invalidShift); err != nil {
		return err
	}
	if shift.CreatedBy <= 0 {
		return invalidShift("created by is required")
	}
	return nil
}

// ValidateStaffShiftSeries enforces the series rules that do not need the
// calendar period loaded: weekdays, the window, the pattern encoding and
// the validity bounds.
func ValidateStaffShiftSeries(series StaffShiftSeries) error {
	if series.StaffID <= 0 {
		return invalidSeries("staff ID is required")
	}
	if len(series.Weekdays) == 0 {
		return invalidSeries("at least one weekday is required")
	}
	seen := map[int]bool{}
	for _, weekday := range series.Weekdays {
		if weekday < 1 || weekday > 7 {
			return invalidSeries("weekdays must be ISO weekdays between 1 and 7")
		}
		if seen[weekday] {
			return invalidSeries("weekdays must not repeat")
		}
		seen[weekday] = true
	}
	if err := validateShiftWindow(series.StartTime, series.EndTime, series.BreakMinutes, invalidSeries); err != nil {
		return err
	}
	if series.CalendarPeriodID <= 0 {
		return invalidSeries("calendar period is required")
	}
	if series.WeekPattern < WeekPatternEvery || series.WeekPattern > WeekPatternB {
		return invalidSeries("week pattern must be 0 (every week), 1 (week A), or 2 (week B)")
	}
	if series.ValidFrom == "" {
		return invalidSeries("valid from is required")
	}
	validFrom, err := parseShiftDate(series.ValidFrom, "valid_from", invalidSeries)
	if err != nil {
		return err
	}
	if series.ValidUntil != "" {
		validUntil, err := parseShiftDate(series.ValidUntil, "valid_until", invalidSeries)
		if err != nil {
			return err
		}
		if !validUntil.After(validFrom) {
			return invalidSeries("valid until must be after valid from")
		}
	}
	if series.CreatedBy <= 0 {
		return invalidSeries("created by is required")
	}
	return nil
}

// ValidateStaffShiftSeriesException requires a series and a calendar day.
func ValidateStaffShiftSeriesException(exception StaffShiftSeriesException) error {
	if exception.SeriesID <= 0 {
		return invalidSeries("series ID is required")
	}
	if exception.Date == "" {
		return invalidSeries("date is required")
	}
	_, err := parseShiftDate(exception.Date, "date", invalidSeries)
	return err
}

var shiftTypeHexColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

// NormalizeShiftType trims the name, defaults an empty color to grey,
// prepends a missing "#", rejects anything but a hex color, and expands a
// three-digit shorthand to the six-digit upper-case form the color picker
// round-trips. It mirrors the legacy model's Validate so both paths store
// the same shape.
func NormalizeShiftType(shiftType ShiftType) (ShiftType, error) {
	shiftType.Name = strings.TrimSpace(shiftType.Name)
	if shiftType.Name == "" {
		return shiftType, invalidShiftType("shift type name is required")
	}
	if len([]rune(shiftType.Name)) > maxShiftTypeNameLength {
		return shiftType, invalidShiftType("shift type name must not exceed 100 characters")
	}
	if len([]rune(shiftType.Description)) > maxShiftTypeDescriptionLength {
		return shiftType, invalidShiftType("shift type description must not exceed 500 characters")
	}
	color := strings.TrimSpace(shiftType.Color)
	if color == "" {
		color = DefaultShiftTypeColor
	}
	if !strings.HasPrefix(color, "#") {
		color = "#" + color
	}
	if !shiftTypeHexColorPattern.MatchString(color) {
		return shiftType, invalidShiftType("invalid color format, must be a valid hex color")
	}
	if len(color) == 4 {
		r, g, b := color[1], color[2], color[3]
		color = string([]byte{'#', r, r, g, g, b, b})
	}
	shiftType.Color = strings.ToUpper(color)
	return shiftType, nil
}

// validateShiftWindow holds the wall-clock rules shared by single shifts and
// series: end after start, break within 0..MaxStaffShiftBreakMinutes and not
// longer than the shift itself.
func validateShiftWindow(startTime, endTime string, breakMinutes int, reject func(string) error) error {
	start, err := parseShiftClock(startTime, "start_time", reject)
	if err != nil {
		return err
	}
	end, err := parseShiftClock(endTime, "end_time", reject)
	if err != nil {
		return err
	}
	if !end.After(start) {
		return reject("end time must be after start time")
	}
	if breakMinutes < 0 {
		return reject("break minutes must not be negative")
	}
	if breakMinutes > MaxStaffShiftBreakMinutes {
		return reject("break minutes must not exceed 300")
	}
	if breakMinutes > int(end.Sub(start)/time.Minute) {
		return reject("break minutes must not exceed shift duration")
	}
	return nil
}

func parseShiftClock(value, field string, reject func(string) error) (time.Time, error) {
	if value == "" {
		return time.Time{}, reject(field + " is required")
	}
	parsed, err := time.Parse(ClockLayout, value)
	if err != nil || parsed.Format(ClockLayout) != value {
		return time.Time{}, reject(field + " must be a " + ClockLayout + " wall clock")
	}
	return parsed, nil
}

func parseShiftDate(value, field string, reject func(string) error) (time.Time, error) {
	parsed, err := time.Parse(DateLayout, value)
	if err != nil || parsed.Format(DateLayout) != value {
		return time.Time{}, reject(field + " must be a " + DateLayout + " date")
	}
	return parsed, nil
}

func validateShiftDate(value, field string) error {
	if value == "" {
		return nil
	}
	_, err := parseShiftDate(value, field, invalidShift)
	return err
}
