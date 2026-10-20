package workforce

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ShiftRecord is one planned work shift of a staff member on a calendar day.
// Several non-overlapping shifts per staff member and day are allowed.
type ShiftRecord struct {
	ID        int64         `json:"id"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	TenantID  int64         `json:"tenant_id"`
	StaffID   int64         `json:"staff_id"`
	Date      calendar.Date `json:"date"`
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
	SeriesOccurrenceDate *calendar.Date `json:"-"`
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
	ShiftType *ShiftTypeRecord `json:"shift_type,omitempty"`
}

// Validate ensures shift data is consistent.
func (s *ShiftRecord) Validate() error {
	if s.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if s.Date.IsZero() {
		return errors.New("date is required")
	}
	if err := ValidateShiftRecordWindow(s.StartTime, s.EndTime, s.BreakMinutes); err != nil {
		return err
	}
	if s.CreatedBy <= 0 {
		return errors.New("created by is required")
	}
	return nil
}

// Overlaps reports whether the wall-clock windows of two shifts on the same
// date intersect; touching boundaries do not overlap.
func (s *ShiftRecord) Overlaps(other *ShiftRecord) bool {
	aStart, aEnd := normalizeWallClock(s.StartTime), normalizeWallClock(s.EndTime)
	bStart, bEnd := normalizeWallClock(other.StartTime), normalizeWallClock(other.EndTime)
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

// Contains reports whether other's wall-clock window lies entirely within
// s's window, shared boundaries included. It keeps a replacement inside the
// gap of the cancelled origin it covers (#1841).
func (s *ShiftRecord) Contains(other *ShiftRecord) bool {
	sStart, sEnd := normalizeWallClock(s.StartTime), normalizeWallClock(s.EndTime)
	oStart, oEnd := normalizeWallClock(other.StartTime), normalizeWallClock(other.EndTime)
	return !oStart.Before(sStart) && !oEnd.After(sEnd)
}

const (
	maxShiftTypeNameLength        = 100
	maxShiftTypeDescriptionLength = 500
	defaultShiftTypeColor         = "#6B7280"
)

var shiftTypeHexColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

// ShiftTypeRecord is a tenant-defined Schichtart: the kind of duty a planned shift
// stands for, with the color the Dienstplan grid shows it in (#1836).
type ShiftTypeRecord struct {
	ID          int64     `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	TenantID    int64     `json:"tenant_id"`
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	Description string    `json:"description,omitempty"`
	IsActive    bool      `json:"is_active"`
}

// Validate normalizes and checks the shift type. It trims the name, defaults
// an empty color to gray, prepends a missing "#", enforces a hex color and
// expands a 3-digit shorthand to the 6-digit form.
func (t *ShiftTypeRecord) Validate() error {
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
		color = defaultShiftTypeColor
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

// ValidateShiftRecordWindow holds the wall-clock rules shared by single shifts and
// series: end after start, break within 0..MaxStaffShiftBreakMinutes and not
// longer than the shift itself.
func ValidateShiftRecordWindow(startTime, endTime time.Time, breakMinutes int) error {
	start := normalizeWallClock(startTime)
	end := normalizeWallClock(endTime)
	if !end.After(start) {
		return errors.New("end time must be after start time")
	}
	if breakMinutes < 0 {
		return errors.New("break minutes must not be negative")
	}
	if breakMinutes > MaxStaffShiftBreakMinutes {
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
