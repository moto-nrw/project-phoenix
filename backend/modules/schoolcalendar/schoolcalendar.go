// Package schoolcalendar is the public School Calendar capability. It owns
// schedule.calendar_periods and schedule.closing_days: every read or write of
// a calendar period or closing day by another owner goes through Query or
// Command instead of a foreign SQL join.
//
// How many planning objects reference a period is a question for the planning
// owners. The capability also owns statutory-holiday calculation and RFC 5545
// calendar rendering because both are school-calendar behavior without storage.
package schoolcalendar

import (
	"context"
	"errors"
	"time"
)

// Period types a calendar period may carry; mirrors the legacy model constants.
const (
	PeriodTypeSchoolYear = "school_year"
	PeriodTypeSemester   = "semester"
	PeriodTypeHoliday    = "holiday"
	PeriodTypeCustom     = "custom"
)

// DateLayout is the calendar-date wire format of every date field.
const DateLayout = "2006-01-02"

const (
	CalendarPeriodNameMaxLength = 255
	ClosingDayReasonMaxLength   = 255
)

var (
	ErrCalendarPeriodNotFound     = errors.New("calendar period not found")
	ErrClosingDayNotFound         = errors.New("closing day not found")
	ErrDateframeNotFound          = errors.New("dateframe not found")
	ErrInvalidCalendarPeriod      = errors.New("invalid calendar period")
	ErrInvalidClosingDay          = errors.New("invalid closing day")
	ErrInvalidDateframe           = errors.New("invalid dateframe")
	ErrInvalidHolidayRange        = errors.New("invalid holiday range")
	ErrCalendarPeriodNameConflict = errors.New("calendar period name already exists")
)

// Holiday is one statutory public holiday on a concrete calendar day.
// Date uses DateLayout and therefore cannot carry a clock or timezone.
type Holiday struct {
	Date string `json:"date"`
	Name string `json:"name"`
}

// CalendarRecurrence describes the RFC 5545 recurrence fields supported by
// moto calendar exports.
type CalendarRecurrence struct {
	Frequency string
	Interval  int
	Weekdays  []string
	MonthDays []int
	Until     string
	Count     *int
}

// CalendarEvent is the owner-neutral input for one RFC 5545 VEVENT.
type CalendarEvent struct {
	UID          string
	Summary      string
	Description  string
	Location     string
	StartDate    string
	EndDate      string
	StartClock   time.Time
	EndClock     time.Time
	AllDay       bool
	Cancelled    bool
	Sequence     int
	Stamp        time.Time
	LastModified time.Time
	Recurrence   *CalendarRecurrence
	ExDates      []string
}

// InvalidDateframeError carries the validation reason; it unwraps to
// ErrInvalidDateframe so callers can classify it with errors.Is.
type InvalidDateframeError struct{ Reason string }

func (e *InvalidDateframeError) Error() string { return e.Reason }
func (e *InvalidDateframeError) Unwrap() error { return ErrInvalidDateframe }

// InvalidCalendarPeriodError carries the validation reason; it unwraps to
// ErrInvalidCalendarPeriod so callers can classify it with errors.Is.
type InvalidCalendarPeriodError struct{ Reason string }

func (e *InvalidCalendarPeriodError) Error() string { return e.Reason }
func (e *InvalidCalendarPeriodError) Unwrap() error { return ErrInvalidCalendarPeriod }

// InvalidClosingDayError carries the validation reason; it unwraps to
// ErrInvalidClosingDay so callers can classify it with errors.Is.
type InvalidClosingDayError struct{ Reason string }

func (e *InvalidClosingDayError) Error() string { return e.Reason }
func (e *InvalidClosingDayError) Unwrap() error { return ErrInvalidClosingDay }

// CalendarPeriod is one row of the school calendar: a school year, semester,
// holiday, or custom range. StartDate, EndDate and WeekCycleAnchor are
// calendar dates in DateLayout; the anchor is empty when unset.
type CalendarPeriod struct {
	ID              int64     `json:"id"`
	TenantID        int64     `json:"tenant_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Name            string    `json:"name"`
	PeriodType      string    `json:"period_type"`
	StartDate       string    `json:"start_date"`
	EndDate         string    `json:"end_date"`
	WeekCycleLength int       `json:"week_cycle_length"`
	WeekCycleAnchor string    `json:"week_cycle_anchor,omitempty"`
	IsActive        bool      `json:"is_active"`
}

// CalendarPeriodFields is the writable part of a period shared by create and
// update.
type CalendarPeriodFields struct {
	Name            string
	PeriodType      string
	StartDate       string
	EndDate         string
	WeekCycleLength int
	WeekCycleAnchor string
	IsActive        bool
}

type CreateCalendarPeriod struct{ CalendarPeriodFields }

type UpdateCalendarPeriod struct {
	ID int64
	CalendarPeriodFields
}

// CalendarPeriodFilter narrows a period listing. Every field is optional; an
// empty filter lists every period of the tenant ordered by start date.
// OverlappingFrom/OverlappingTo select periods whose [start_date, end_date]
// range touches the given inclusive window; both must be set together.
type CalendarPeriodFilter struct {
	IDs             []int64
	Name            string
	PeriodType      string
	ActiveOnly      bool
	OverlappingFrom string
	OverlappingTo   string
	// ExcludeID drops one period from the listing, typically the period
	// whose own overlap is being checked.
	ExcludeID int64
}

// HolidaysOverlapping selects the holiday periods in force that touch the
// inclusive window [from, to]; only active periods count.
func HolidaysOverlapping(from, to string) CalendarPeriodFilter {
	return CalendarPeriodFilter{PeriodType: PeriodTypeHoliday, ActiveOnly: true, OverlappingFrom: from, OverlappingTo: to}
}

// ClosingDay is one tenant-defined closure range (pädagogischer Tag,
// Weihnachtswoche). A single closed day has StartDate == EndDate.
type ClosingDay struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	StartDate string    `json:"start_date"`
	EndDate   string    `json:"end_date"`
	Reason    string    `json:"reason"`
}

type ClosingDayFields struct {
	StartDate string
	EndDate   string
	Reason    string
}

type CreateClosingDay struct{ ClosingDayFields }

type UpdateClosingDay struct {
	ID int64
	ClosingDayFields
}

// ClosingDayFilter narrows a closing-day listing. OverlappingFrom and
// OverlappingTo select the ranges touching the inclusive window; both must be
// set together.
type ClosingDayFilter struct {
	IDs             []int64
	OverlappingFrom string
	OverlappingTo   string
}

// Dateframe is a planning date range stored as instants (TIMESTAMPTZ
// columns): the legacy schedules API reads and writes them as timestamps.
type Dateframe struct {
	ID          int64     `json:"id"`
	TenantID    int64     `json:"tenant_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	Name        string    `json:"name,omitempty"`
	Description string    `json:"description,omitempty"`
}

type DateframeFields struct {
	StartDate   time.Time
	EndDate     time.Time
	Name        string
	Description string
}

type CreateDateframe struct{ DateframeFields }

type UpdateDateframe struct {
	ID int64
	DateframeFields
}

// Dateframe sort fields accepted by DateframeFilter.
const (
	DateframeSortID        = "id"
	DateframeSortName      = "name"
	DateframeSortStartDate = "start_date"
	DateframeSortEndDate   = "end_date"
)

type DateframeSort struct {
	Field      string
	Descending bool
}

// DateframeFilter narrows a dateframe listing. Name is a case-insensitive
// exact match, NameFold a case-insensitive exact match, NamePattern a
// case-insensitive SQL LIKE pattern. Contains
// selects the ranges holding the instant; OverlappingFrom/OverlappingTo the
// ranges touching the inclusive window (both must be set together). Limit
// zero means no limit; the listing is ordered by ID when Sort is empty.
type DateframeFilter struct {
	IDs             []int64
	Name            string
	NameFold        string
	NamePattern     string
	Contains        *time.Time
	OverlappingFrom *time.Time
	OverlappingTo   *time.Time
	Sort            []DateframeSort
	Limit           int
	Offset          int
}

type Query interface {
	// FindCalendarPeriod returns one period visible in the caller's
	// transaction.
	FindCalendarPeriod(context.Context, int64) (CalendarPeriod, error)
	// ListCalendarPeriods returns the periods matching the filter ordered by
	// start date, then ID.
	ListCalendarPeriods(context.Context, CalendarPeriodFilter) ([]CalendarPeriod, error)

	FindClosingDay(context.Context, int64) (ClosingDay, error)
	// ListClosingDays returns the closing days matching the filter ordered by
	// start date, then ID.
	ListClosingDays(context.Context, ClosingDayFilter) ([]ClosingDay, error)

	FindDateframe(context.Context, int64) (Dateframe, error)
	ListDateframes(context.Context, DateframeFilter) ([]Dateframe, error)

	HolidayQuery
	CalendarRenderer
}

// HolidayQuery exposes the locally computed statutory holidays for a German
// federal-state ISO code. It performs no network or database access.
type HolidayQuery interface {
	ValidHolidayRegion(string) bool
	ListHolidays(context.Context, string, string, string) ([]Holiday, error)
	HolidayDates(context.Context, string, string, string) (map[string]bool, error)
}

// CalendarRenderer renders complete RFC 5545 calendar documents.
type CalendarRenderer interface {
	RenderCalendar(context.Context, string, []CalendarEvent) (string, error)
	RenderCalendarObject(context.Context, CalendarEvent) (string, error)
}

type Command interface {
	CreateCalendarPeriod(context.Context, CreateCalendarPeriod) (CalendarPeriod, error)
	// CreateCalendarPeriodIfAbsent inserts the period unless one with the
	// same name already exists for the tenant. It reports whether the insert
	// happened; when it did not, the returned period is empty. The insert is
	// race-free (ON CONFLICT DO NOTHING) so concurrent bootstraps yield
	// exactly one row and no error.
	CreateCalendarPeriodIfAbsent(context.Context, CreateCalendarPeriod) (CalendarPeriod, bool, error)
	UpdateCalendarPeriod(context.Context, UpdateCalendarPeriod) (CalendarPeriod, error)
	// DeleteCalendarPeriod removes the period; a period still referenced by a
	// non-nullable foreign key keeps the driver error in the chain.
	DeleteCalendarPeriod(context.Context, int64) error

	CreateClosingDay(context.Context, CreateClosingDay) (ClosingDay, error)
	UpdateClosingDay(context.Context, UpdateClosingDay) (ClosingDay, error)
	DeleteClosingDay(context.Context, int64) error

	CreateDateframe(context.Context, CreateDateframe) (Dateframe, error)
	UpdateDateframe(context.Context, UpdateDateframe) (Dateframe, error)
	DeleteDateframe(context.Context, int64) error
}

type Capability interface {
	Query
	Command
}

// Days is the School Calendar without the period administration: the
// Capability and the tenant non-working-day reads. *Records serves it before
// the owners the period administration asks exist; *Module serves it too.
type Days interface {
	Capability
	NonWorkingDayQuery
}

func IsValidPeriodType(t string) bool {
	switch t {
	case PeriodTypeSchoolYear, PeriodTypeSemester, PeriodTypeHoliday, PeriodTypeCustom:
		return true
	}
	return false
}

// ErrorCode is the stable label recorded per operation outcome.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrCalendarPeriodNotFound), errors.Is(err, ErrClosingDayNotFound), errors.Is(err, ErrDateframeNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidCalendarPeriod), errors.Is(err, ErrInvalidClosingDay), errors.Is(err, ErrInvalidDateframe):
		return "invalid"
	case errors.Is(err, ErrInvalidHolidayRange):
		return "invalid"
	case errors.Is(err, ErrCalendarPeriodNameConflict):
		return "calendar_period_name_conflict"
	case errors.Is(err, ErrCalendarPeriodOverlapConflict):
		return "calendar_period_overlap_conflict"
	case errors.Is(err, ErrCalendarPeriodRequiredByCareOffering):
		return "calendar_period_care_offering_conflict"
	case errors.Is(err, ErrFederalStateUnavailable):
		return "federal_state_unavailable"
	default:
		return "internal_error"
	}
}
