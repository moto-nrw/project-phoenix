package domain

import (
	"errors"
	"math"
	"slices"
	"time"
)

// Work session statuses and channels. They mirror the public constants; the
// domain keeps its own copy so it never imports the facade.
const (
	WorkSessionStatusPresent    = "present"
	WorkSessionStatusHomeOffice = "home_office"

	WorkSessionSourceApp     = "app"
	WorkSessionSourceNFC     = "nfc"
	WorkSessionSourceUnknown = "unknown"

	// MaxOpenWorkSessionDuration is the live safety limit for a block that is
	// still open: past it a running block stops counting as work in progress.
	MaxOpenWorkSessionDuration = 12 * time.Hour

	// WorkSessionDateColumn is the one date column a retention cleanup may
	// address on work sessions.
	WorkSessionDateColumn = "date"

	// WorkSessionOpenConstraint is the unique index behind "at most one open
	// work session per staff member and day".
	WorkSessionOpenConstraint = "uq_work_sessions_staff_date_open"
)

// Stundenkonto correction transaction types (#1420, #2132).
const (
	BalanceAdjustmentTypePayout   = "payout"
	BalanceAdjustmentTypeCompTime = "comp_time"
	BalanceAdjustmentTypeReset    = "reset"
	BalanceAdjustmentTypeOpening  = "opening"
)

var ValidBalanceAdjustmentTypes = []string{
	BalanceAdjustmentTypePayout, BalanceAdjustmentTypeCompTime,
	BalanceAdjustmentTypeReset, BalanceAdjustmentTypeOpening,
}

// Bounds of the vacation rows.
const (
	minQuotaYear    = 2000
	maxQuotaYear    = 2100
	minQuotaDays    = 0
	maxQuotaDays    = 366
	maxOpeningDays  = 999.0
	openingDayScale = 10
)

var (
	ErrWorkSessionNotFound            = errors.New("work session not found")
	ErrWorkSessionBreakNotFound       = errors.New("work session break not found")
	ErrStaffBalanceAdjustmentNotFound = errors.New("staff balance adjustment not found")
	ErrStaffVacationOpeningNotFound   = errors.New("staff vacation opening not found")
	ErrStaffVacationQuotaNotFound     = errors.New("staff vacation quota not found")
	ErrInvalidWorkSession             = errors.New("invalid work session input")
	// ErrWorkSessionAlreadyOpen reports the unique index rejecting a second
	// open block for the same staff member and day.
	ErrWorkSessionAlreadyOpen = errors.New("work session for the day is already open")
)

// InvalidWorkSessionError carries the caller-facing validation reason of a
// work-session family row. The wording is the established contract the
// legacy models produced.
type InvalidWorkSessionError struct{ Reason string }

func (e *InvalidWorkSessionError) Error() string { return e.Reason }
func (e *InvalidWorkSessionError) Unwrap() error { return ErrInvalidWorkSession }

func invalidWorkSession(reason string) error { return &InvalidWorkSessionError{Reason: reason} }

// WorkSession is one work block of a staff member. Date is the calendar day
// the block is filed under in DateLayout; the instants are timestamps.
type WorkSession struct {
	ID             int64
	TenantID       int64
	StaffID        int64
	Date           string
	Status         string
	Source         string
	CheckInTime    time.Time
	CheckOutTime   *time.Time
	ReopenedAt     *time.Time
	BreakMinutes   int
	Notes          string
	AutoCheckedOut bool
	CreatedBy      int64
	UpdatedBy      *int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Validate enforces the invariants every stored work session carries.
func (ws WorkSession) Validate() error {
	if ws.StaffID <= 0 {
		return invalidWorkSession("staff ID is required")
	}
	if ws.CheckInTime.IsZero() {
		return invalidWorkSession("check-in time is required")
	}
	if ws.Status != WorkSessionStatusPresent && ws.Status != WorkSessionStatusHomeOffice {
		return invalidWorkSession("status must be 'present' or 'home_office'")
	}
	if ws.CheckOutTime != nil && ws.CheckInTime.After(*ws.CheckOutTime) {
		return invalidWorkSession("check-in time must be before check-out time")
	}
	if ws.BreakMinutes < 0 {
		return invalidWorkSession("break minutes cannot be negative")
	}
	if ws.CreatedBy <= 0 {
		return invalidWorkSession("created_by is required")
	}
	if err := ValidateDate(ws.Date, "date"); err != nil {
		return invalidWorkSession(err.Error())
	}
	return nil
}

// IsOpen reports whether the block is still running.
func (ws WorkSession) IsOpen() bool { return ws.CheckOutTime == nil }

// WorkSessionOrderField names a column a work-session listing may be ordered by.
type WorkSessionOrderField string

const (
	WorkSessionOrderID          WorkSessionOrderField = "id"
	WorkSessionOrderStaffID     WorkSessionOrderField = "staff_id"
	WorkSessionOrderDate        WorkSessionOrderField = "date"
	WorkSessionOrderCheckInTime WorkSessionOrderField = "check_in_time"
)

var validWorkSessionOrderFields = []WorkSessionOrderField{
	WorkSessionOrderID, WorkSessionOrderStaffID, WorkSessionOrderDate, WorkSessionOrderCheckInTime,
}

type WorkSessionOrder struct {
	Field      WorkSessionOrderField
	Descending bool
}

// WorkSessionFilter narrows a listing; every set field is combined with AND.
// Open selects running (true) or closed (false) blocks. DateBefore selects
// blocks filed strictly before the day.
type WorkSessionFilter struct {
	IDs        []int64
	StaffID    int64
	StaffIDs   []int64
	Date       string
	DateFrom   string
	DateTo     string
	DateBefore string
	Open       *bool
	Order      []WorkSessionOrder
	Limit      int
	Offset     int
}

func (f WorkSessionFilter) Validate() error {
	for _, pair := range []struct{ value, field string }{
		{f.Date, "date"}, {f.DateFrom, "date_from"}, {f.DateTo, "date_to"}, {f.DateBefore, "date_before"},
	} {
		if pair.value == "" {
			continue
		}
		if err := ValidateDate(pair.value, pair.field); err != nil {
			return invalidWorkSession(err.Error())
		}
	}
	for _, order := range f.Order {
		if !slices.Contains(validWorkSessionOrderFields, order.Field) {
			return invalidWorkSession("unsupported order field " + string(order.Field))
		}
	}
	if f.Limit < 0 || f.Offset < 0 {
		return invalidWorkSession("limit and offset must not be negative")
	}
	return nil
}

// WorkSessionBreak is one break period inside a work block.
type WorkSessionBreak struct {
	ID              int64
	TenantID        int64
	SessionID       int64
	StartedAt       time.Time
	EndedAt         *time.Time
	DurationMinutes int
	PlannedEndTime  *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (b WorkSessionBreak) Validate() error {
	if b.SessionID <= 0 {
		return invalidWorkSession("session ID is required")
	}
	if b.StartedAt.IsZero() {
		return invalidWorkSession("started_at is required")
	}
	if b.EndedAt != nil && b.StartedAt.After(*b.EndedAt) {
		return invalidWorkSession("started_at must be before ended_at")
	}
	if b.DurationMinutes < 0 {
		return invalidWorkSession("duration_minutes cannot be negative")
	}
	return nil
}

// IsActive reports whether the break has not ended yet.
func (b WorkSessionBreak) IsActive() bool { return b.EndedAt == nil }

// WorkSessionBreakFilter narrows a break listing. Active selects running
// (true) or ended (false) breaks.
type WorkSessionBreakFilter struct {
	SessionID  int64
	SessionIDs []int64
	Active     *bool
	Limit      int
	Offset     int
}

func (f WorkSessionBreakFilter) Validate() error {
	if f.Limit < 0 || f.Offset < 0 {
		return invalidWorkSession("limit and offset must not be negative")
	}
	return nil
}

// StaffBalanceAdjustment is one Stundenkonto correction transaction. The row
// is its own audit record; EffectiveDate is a calendar day in DateLayout.
type StaffBalanceAdjustment struct {
	ID            int64
	TenantID      int64
	StaffID       int64
	Type          string
	MinutesDelta  int
	EffectiveDate string
	Note          string
	DecidedBy     int64
	DecidedAt     time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (a StaffBalanceAdjustment) Validate() error {
	if a.StaffID <= 0 {
		return invalidWorkSession("staff ID is required")
	}
	if !slices.Contains(ValidBalanceAdjustmentTypes, a.Type) {
		return invalidWorkSession("invalid balance adjustment type")
	}
	if a.EffectiveDate == "" {
		return invalidWorkSession("effective_date is required")
	}
	if err := ValidateDate(a.EffectiveDate, "effective_date"); err != nil {
		return invalidWorkSession(err.Error())
	}
	if a.DecidedBy <= 0 {
		return invalidWorkSession("decided_by is required")
	}
	return nil
}

// StaffBalanceAdjustmentFilter narrows an adjustment listing. Effective
// bounds are inclusive calendar days.
type StaffBalanceAdjustmentFilter struct {
	StaffID       int64
	StaffIDs      []int64
	Types         []string
	EffectiveFrom string
	EffectiveTo   string
	Limit         int
	Offset        int
}

func (f StaffBalanceAdjustmentFilter) Validate() error {
	for _, pair := range []struct{ value, field string }{
		{f.EffectiveFrom, "effective_from"}, {f.EffectiveTo, "effective_to"},
	} {
		if pair.value == "" {
			continue
		}
		if err := ValidateDate(pair.value, pair.field); err != nil {
			return invalidWorkSession(err.Error())
		}
	}
	if f.Limit < 0 || f.Offset < 0 {
		return invalidWorkSession("limit and offset must not be negative")
	}
	return nil
}

// StaffVacationOpening records vacation days a staff member had already taken
// before the moto introduction (#2132), per calendar year. TakenBeforeDays is
// signed: an entered Resturlaub above entitled plus carryover yields a
// negative value.
type StaffVacationOpening struct {
	ID                   int64
	TenantID             int64
	StaffID              int64
	Year                 int
	EffectiveDate        string
	TakenBeforeDays      float64
	EnteredRemainingDays float64
	Note                 string
	DecidedBy            int64
	DecidedAt            time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func hasOneDecimalPlace(value float64) bool {
	return math.Abs(value*openingDayScale-math.Round(value*openingDayScale)) < 1e-9
}

func (o StaffVacationOpening) Validate() error {
	if o.StaffID <= 0 {
		return invalidWorkSession("staff_id is required")
	}
	if o.Year < minQuotaYear || o.Year > maxQuotaYear {
		return invalidWorkSession("year out of range")
	}
	if o.EffectiveDate == "" {
		return invalidWorkSession("effective_date is required")
	}
	effective, err := time.Parse(DateLayout, o.EffectiveDate)
	if err != nil || effective.Format(DateLayout) != o.EffectiveDate {
		return invalidWorkSession("effective_date must be a " + DateLayout + " date")
	}
	if effective.Year() != o.Year {
		return invalidWorkSession("effective_date must lie in the opening year")
	}
	if o.TakenBeforeDays < -maxOpeningDays || o.TakenBeforeDays > maxOpeningDays {
		return invalidWorkSession("taken_before_days out of range")
	}
	if o.EnteredRemainingDays < -maxOpeningDays || o.EnteredRemainingDays > maxOpeningDays {
		return invalidWorkSession("entered_remaining_days out of range")
	}
	if !hasOneDecimalPlace(o.TakenBeforeDays) || !hasOneDecimalPlace(o.EnteredRemainingDays) {
		return invalidWorkSession("vacation opening days must have at most one decimal place")
	}
	if o.DecidedBy <= 0 {
		return invalidWorkSession("decided_by is required")
	}
	return nil
}

// StaffVacationQuota is the yearly vacation entitlement of a staff member.
type StaffVacationQuota struct {
	ID            int64
	TenantID      int64
	StaffID       int64
	Year          int
	EntitledDays  float64
	CarryoverDays float64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (q StaffVacationQuota) Validate() error {
	if q.StaffID <= 0 {
		return invalidWorkSession("staff_id is required")
	}
	if q.Year < minQuotaYear || q.Year > maxQuotaYear {
		return invalidWorkSession("year out of range")
	}
	if q.EntitledDays < minQuotaDays || q.EntitledDays > maxQuotaDays {
		return invalidWorkSession("entitled_days out of range")
	}
	if q.CarryoverDays < minQuotaDays || q.CarryoverDays > maxQuotaDays {
		return invalidWorkSession("carryover_days out of range")
	}
	if !hasOneDecimalPlace(q.EntitledDays) || !hasOneDecimalPlace(q.CarryoverDays) {
		return invalidWorkSession("vacation quota days must have at most one decimal place")
	}
	return nil
}

// StaffVacationOrderField names a column a vacation listing may be ordered by.
type StaffVacationOrderField string

const (
	StaffVacationOrderID           StaffVacationOrderField = "id"
	StaffVacationOrderStaffID      StaffVacationOrderField = "staff_id"
	StaffVacationOrderYear         StaffVacationOrderField = "year"
	StaffVacationOrderEntitledDays StaffVacationOrderField = "entitled_days"
)

var validStaffVacationOrderFields = []StaffVacationOrderField{
	StaffVacationOrderID, StaffVacationOrderStaffID, StaffVacationOrderYear, StaffVacationOrderEntitledDays,
}

type StaffVacationOrder struct {
	Field      StaffVacationOrderField
	Descending bool
}

// StaffVacationFilter narrows a vacation opening or quota listing. A zero
// Year selects every year.
type StaffVacationFilter struct {
	StaffID  int64
	StaffIDs []int64
	Year     int
	Order    []StaffVacationOrder
	Limit    int
	Offset   int
}

func (f StaffVacationFilter) Validate() error {
	for _, order := range f.Order {
		if !slices.Contains(validStaffVacationOrderFields, order.Field) {
			return invalidWorkSession("unsupported order field " + string(order.Field))
		}
	}
	if f.Limit < 0 || f.Offset < 0 {
		return invalidWorkSession("limit and offset must not be negative")
	}
	return nil
}

// ValidateWorkSessionDateColumn accepts the one date column retention cleanup
// may address on work sessions.
func ValidateWorkSessionDateColumn(column string) error {
	if column != WorkSessionDateColumn {
		return invalidWorkSession("unsupported date column " + column)
	}
	return nil
}
