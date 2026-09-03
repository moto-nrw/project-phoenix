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
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
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

type engine interface {
	FindCalendarPeriod(context.Context, int64) (CalendarPeriod, error)
	ListCalendarPeriods(context.Context, CalendarPeriodFilter) ([]CalendarPeriod, error)
	CreateCalendarPeriod(ctx context.Context, input CreateCalendarPeriod, ifAbsent bool) (CalendarPeriod, bool, error)
	UpdateCalendarPeriod(context.Context, UpdateCalendarPeriod) (CalendarPeriod, error)
	DeleteCalendarPeriod(context.Context, int64) error

	FindClosingDay(context.Context, int64) (ClosingDay, error)
	ListClosingDays(context.Context, ClosingDayFilter) ([]ClosingDay, error)
	CreateClosingDay(context.Context, CreateClosingDay) (ClosingDay, error)
	UpdateClosingDay(context.Context, UpdateClosingDay) (ClosingDay, error)
	DeleteClosingDay(context.Context, int64) error

	FindDateframe(context.Context, int64) (Dateframe, error)
	ListDateframes(context.Context, DateframeFilter) ([]Dateframe, error)
	CreateDateframe(context.Context, CreateDateframe) (Dateframe, error)
	UpdateDateframe(context.Context, UpdateDateframe) (Dateframe, error)
	DeleteDateframe(context.Context, int64) error

	ValidHolidayRegion(string) bool
	ListHolidays(context.Context, string, string, string) ([]Holiday, error)
	HolidayDates(context.Context, string, string, string) (map[string]bool, error)
	RenderCalendar(context.Context, string, []CalendarEvent) (string, error)
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("school calendar: engine is required")
	}
	return &Module{engine: engine}
}

// --- calendar periods ---

func (m *Module) FindCalendarPeriod(ctx context.Context, id int64) (CalendarPeriod, error) {
	if id <= 0 {
		return CalendarPeriod{}, invalidPeriod("calendar period ID is required")
	}
	return m.engine.FindCalendarPeriod(ctx, id)
}

func (m *Module) ListCalendarPeriods(ctx context.Context, filter CalendarPeriodFilter) ([]CalendarPeriod, error) {
	filter.IDs = uniquePositive(filter.IDs)
	if filter.PeriodType != "" && !IsValidPeriodType(filter.PeriodType) {
		return nil, invalidPeriod("invalid period type")
	}
	if err := validateWindow(filter.OverlappingFrom, filter.OverlappingTo, invalidPeriod); err != nil {
		return nil, err
	}
	if filter.ExcludeID < 0 {
		return nil, invalidPeriod("exclude ID must not be negative")
	}
	return m.engine.ListCalendarPeriods(ctx, filter)
}

func (m *Module) CreateCalendarPeriod(ctx context.Context, input CreateCalendarPeriod) (CalendarPeriod, error) {
	if err := validateCalendarPeriodFields(&input.CalendarPeriodFields); err != nil {
		return CalendarPeriod{}, err
	}
	period, _, err := m.engine.CreateCalendarPeriod(ctx, input, false)
	return period, err
}

func (m *Module) CreateCalendarPeriodIfAbsent(ctx context.Context, input CreateCalendarPeriod) (CalendarPeriod, bool, error) {
	if err := validateCalendarPeriodFields(&input.CalendarPeriodFields); err != nil {
		return CalendarPeriod{}, false, err
	}
	return m.engine.CreateCalendarPeriod(ctx, input, true)
}

func (m *Module) UpdateCalendarPeriod(ctx context.Context, input UpdateCalendarPeriod) (CalendarPeriod, error) {
	if input.ID <= 0 {
		return CalendarPeriod{}, invalidPeriod("calendar period ID is required")
	}
	if err := validateCalendarPeriodFields(&input.CalendarPeriodFields); err != nil {
		return CalendarPeriod{}, err
	}
	return m.engine.UpdateCalendarPeriod(ctx, input)
}

func (m *Module) DeleteCalendarPeriod(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidPeriod("calendar period ID is required")
	}
	return m.engine.DeleteCalendarPeriod(ctx, id)
}

// --- closing days ---

func (m *Module) FindClosingDay(ctx context.Context, id int64) (ClosingDay, error) {
	if id <= 0 {
		return ClosingDay{}, invalidClosingDay("closing day ID is required")
	}
	return m.engine.FindClosingDay(ctx, id)
}

func (m *Module) ListClosingDays(ctx context.Context, filter ClosingDayFilter) ([]ClosingDay, error) {
	filter.IDs = uniquePositive(filter.IDs)
	if err := validateWindow(filter.OverlappingFrom, filter.OverlappingTo, invalidClosingDay); err != nil {
		return nil, err
	}
	return m.engine.ListClosingDays(ctx, filter)
}

func (m *Module) CreateClosingDay(ctx context.Context, input CreateClosingDay) (ClosingDay, error) {
	if err := validateClosingDayFields(&input.ClosingDayFields); err != nil {
		return ClosingDay{}, err
	}
	return m.engine.CreateClosingDay(ctx, input)
}

func (m *Module) UpdateClosingDay(ctx context.Context, input UpdateClosingDay) (ClosingDay, error) {
	if input.ID <= 0 {
		return ClosingDay{}, invalidClosingDay("closing day ID is required")
	}
	if err := validateClosingDayFields(&input.ClosingDayFields); err != nil {
		return ClosingDay{}, err
	}
	return m.engine.UpdateClosingDay(ctx, input)
}

func (m *Module) DeleteClosingDay(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidClosingDay("closing day ID is required")
	}
	return m.engine.DeleteClosingDay(ctx, id)
}

// --- dateframes ---

func (m *Module) FindDateframe(ctx context.Context, id int64) (Dateframe, error) {
	if id <= 0 {
		return Dateframe{}, invalidDateframe("dateframe ID is required")
	}
	return m.engine.FindDateframe(ctx, id)
}

func (m *Module) ListDateframes(ctx context.Context, filter DateframeFilter) ([]Dateframe, error) {
	filter.IDs = uniquePositive(filter.IDs)
	if (filter.OverlappingFrom == nil) != (filter.OverlappingTo == nil) {
		return nil, invalidDateframe("overlap window needs both from and to instants")
	}
	if filter.OverlappingFrom != nil && filter.OverlappingTo.Before(*filter.OverlappingFrom) {
		return nil, invalidDateframe("overlap window to must not be before from")
	}
	if filter.Limit < 0 || filter.Offset < 0 {
		return nil, invalidDateframe("limit and offset must not be negative")
	}
	for _, sort := range filter.Sort {
		switch sort.Field {
		case DateframeSortID, DateframeSortName, DateframeSortStartDate, DateframeSortEndDate:
		default:
			return nil, invalidDateframe("unsupported dateframe sort field " + sort.Field)
		}
	}
	return m.engine.ListDateframes(ctx, filter)
}

func (m *Module) CreateDateframe(ctx context.Context, input CreateDateframe) (Dateframe, error) {
	if err := validateDateframeFields(input.DateframeFields); err != nil {
		return Dateframe{}, err
	}
	return m.engine.CreateDateframe(ctx, input)
}

func (m *Module) UpdateDateframe(ctx context.Context, input UpdateDateframe) (Dateframe, error) {
	if input.ID <= 0 {
		return Dateframe{}, invalidDateframe("dateframe ID is required")
	}
	if err := validateDateframeFields(input.DateframeFields); err != nil {
		return Dateframe{}, err
	}
	return m.engine.UpdateDateframe(ctx, input)
}

func (m *Module) DeleteDateframe(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidDateframe("dateframe ID is required")
	}
	return m.engine.DeleteDateframe(ctx, id)
}

// --- holidays and calendar documents ---

func (m *Module) ValidHolidayRegion(region string) bool {
	return m.engine.ValidHolidayRegion(region)
}

func (m *Module) ListHolidays(ctx context.Context, region, from, to string) ([]Holiday, error) {
	if err := validateHolidayWindow(from, to); err != nil {
		return nil, err
	}
	return m.engine.ListHolidays(ctx, region, from, to)
}

func (m *Module) HolidayDates(ctx context.Context, region, from, to string) (map[string]bool, error) {
	if err := validateHolidayWindow(from, to); err != nil {
		return nil, err
	}
	return m.engine.HolidayDates(ctx, region, from, to)
}

func (m *Module) RenderCalendar(ctx context.Context, name string, events []CalendarEvent) (string, error) {
	for _, event := range events {
		if err := validateCalendarEvent(event); err != nil {
			return "", err
		}
	}
	return m.engine.RenderCalendar(ctx, name, events)
}

func validateHolidayWindow(from, to string) error {
	invalid := func(reason string) error { return fmt.Errorf("%w: %s", ErrInvalidHolidayRange, reason) }
	if from == "" || to == "" {
		return invalid("holiday range needs both from and to dates")
	}
	return validateWindow(from, to, invalid)
}

func validateCalendarEvent(event CalendarEvent) error {
	if err := validateDate(event.StartDate, "calendar event start date", invalidPeriod); err != nil {
		return err
	}
	if err := validateDate(event.EndDate, "calendar event end date", invalidPeriod); err != nil {
		return err
	}
	if event.StartDate == "" || event.EndDate == "" {
		return invalidPeriod("calendar event dates are required")
	}
	if event.Recurrence != nil && event.Recurrence.Until != "" {
		if err := validateDate(event.Recurrence.Until, "calendar recurrence until", invalidPeriod); err != nil {
			return err
		}
	}
	for _, date := range event.ExDates {
		if err := validateDate(date, "calendar exception date", invalidPeriod); err != nil {
			return err
		}
	}
	return nil
}

// --- validation ---

// IsValidPeriodType reports whether t is one of the known period types.
func IsValidPeriodType(t string) bool {
	switch t {
	case PeriodTypeSchoolYear, PeriodTypeSemester, PeriodTypeHoliday, PeriodTypeCustom:
		return true
	}
	return false
}

// validateCalendarPeriodFields mirrors the legacy model rules so a caller
// that skipped the model validation gets the same messages.
func validateCalendarPeriodFields(fields *CalendarPeriodFields) error {
	if fields.Name == "" {
		return invalidPeriod("name is required")
	}
	if len(fields.Name) > CalendarPeriodNameMaxLength {
		return invalidPeriod("name cannot exceed 255 characters")
	}
	if !IsValidPeriodType(fields.PeriodType) {
		return invalidPeriod("invalid period type")
	}
	if fields.StartDate == "" {
		return invalidPeriod("start_date is required")
	}
	if fields.EndDate == "" {
		return invalidPeriod("end_date is required")
	}
	if err := validateDate(fields.StartDate, "start_date", invalidPeriod); err != nil {
		return err
	}
	if err := validateDate(fields.EndDate, "end_date", invalidPeriod); err != nil {
		return err
	}
	if fields.EndDate <= fields.StartDate {
		return invalidPeriod("end_date must be after start_date")
	}
	if fields.WeekCycleLength < 1 {
		return invalidPeriod("week_cycle_length must be at least 1")
	}
	if fields.WeekCycleLength > 1 && fields.WeekCycleAnchor == "" {
		return invalidPeriod("week_cycle_anchor is required when week_cycle_length > 1")
	}
	return validateDate(fields.WeekCycleAnchor, "week_cycle_anchor", invalidPeriod)
}

func validateClosingDayFields(fields *ClosingDayFields) error {
	if strings.TrimSpace(fields.Reason) == "" {
		return invalidClosingDay("reason is required")
	}
	if utf8.RuneCountInString(strings.TrimSpace(fields.Reason)) > ClosingDayReasonMaxLength {
		return invalidClosingDay("reason cannot exceed 255 characters")
	}
	if fields.StartDate == "" {
		return invalidClosingDay("start_date is required")
	}
	if fields.EndDate == "" {
		return invalidClosingDay("end_date is required")
	}
	if err := validateDate(fields.StartDate, "start_date", invalidClosingDay); err != nil {
		return err
	}
	if err := validateDate(fields.EndDate, "end_date", invalidClosingDay); err != nil {
		return err
	}
	if fields.EndDate < fields.StartDate {
		return invalidClosingDay("end_date must not be before start_date")
	}
	return nil
}

// validateWindow accepts an unset window or a complete one; from must not
// come after to. DateLayout strings compare lexically as dates.
func validateWindow(from, to string, invalid func(string) error) error {
	if from == "" && to == "" {
		return nil
	}
	if from == "" || to == "" {
		return invalid("overlap window needs both from and to dates")
	}
	if err := validateDate(from, "overlap window from", invalid); err != nil {
		return err
	}
	if err := validateDate(to, "overlap window to", invalid); err != nil {
		return err
	}
	if to < from {
		return invalid("overlap window to must not be before from")
	}
	return nil
}

func validateDate(value, label string, invalid func(string) error) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse(DateLayout, value); err != nil {
		return invalid(label + " must be a calendar date in YYYY-MM-DD format")
	}
	return nil
}

func uniquePositive(ids []int64) []int64 {
	if ids == nil {
		return nil
	}
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

// validateDateframeFields mirrors the legacy model rules: both instants are
// required and the range may not end before it starts.
func validateDateframeFields(fields DateframeFields) error {
	if fields.StartDate.IsZero() {
		return invalidDateframe("start date is required")
	}
	if fields.EndDate.IsZero() {
		return invalidDateframe("end date is required")
	}
	if fields.EndDate.Before(fields.StartDate) {
		return invalidDateframe("end date must be on or after start date")
	}
	return nil
}

func invalidPeriod(reason string) error     { return &InvalidCalendarPeriodError{Reason: reason} }
func invalidClosingDay(reason string) error { return &InvalidClosingDayError{Reason: reason} }
func invalidDateframe(reason string) error  { return &InvalidDateframeError{Reason: reason} }

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
	default:
		return "internal_error"
	}
}
