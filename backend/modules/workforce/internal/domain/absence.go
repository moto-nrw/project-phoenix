package domain

import (
	"errors"
	"slices"
	"strings"
	"time"
)

// Canonical staff absence types and statuses. They mirror the public
// constants; the domain keeps its own copy so it never imports the facade.
const (
	AbsenceTypeSick     = "sick"
	AbsenceTypeVacation = "vacation"
	AbsenceTypeTraining = "training"
	AbsenceTypeOther    = "other"
	AbsenceTypeCompTime = "comp_time"

	AbsenceStatusReported  = "reported"
	AbsenceStatusRequested = "requested"
	AbsenceStatusQuestion  = "question"
	AbsenceStatusApproved  = "approved"
	AbsenceStatusDeclined  = "declined"
	AbsenceStatusCanceled  = "canceled"

	AbsenceTypeOverrunWarn  = "warn"
	AbsenceTypeOverrunBlock = "block"

	StaffAbsenceDateStart = "date_start"
	StaffAbsenceDateEnd   = "date_end"

	maxAbsenceTypeNameLength = 100
)

var ValidAbsenceTypes = []string{
	AbsenceTypeSick, AbsenceTypeVacation, AbsenceTypeTraining, AbsenceTypeOther, AbsenceTypeCompTime,
}

var ValidAbsenceStatuses = []string{
	AbsenceStatusReported, AbsenceStatusRequested, AbsenceStatusQuestion,
	AbsenceStatusApproved, AbsenceStatusDeclined, AbsenceStatusCanceled,
}

// EffectiveAbsenceStatuses are the statuses under which an absence counts on
// the day: entered by an admin or approved.
var EffectiveAbsenceStatuses = []string{AbsenceStatusReported, AbsenceStatusApproved}

// PendingAbsenceStatuses are the statuses still awaiting a decision.
var PendingAbsenceStatuses = []string{AbsenceStatusRequested, AbsenceStatusQuestion}

// AbsenceTypePriority ranks overlapping effective absences on one day: a sick
// note beats a training day beats vacation beats comp time beats other.
var AbsenceTypePriority = map[string]int{
	AbsenceTypeSick:     5,
	AbsenceTypeTraining: 4,
	AbsenceTypeVacation: 3,
	AbsenceTypeCompTime: 2,
	AbsenceTypeOther:    1,
}

var (
	ErrStaffAbsenceNotFound      = errors.New("staff absence not found")
	ErrInvalidStaffAbsence       = errors.New("invalid staff absence input")
	ErrAbsenceTypeNotFound       = errors.New("staff absence type not found")
	ErrAbsenceTypeNameTaken      = errors.New("staff absence type name is already taken")
	ErrAbsenceTypeInvalid        = errors.New("invalid staff absence type")
	ErrGroupSubstitutionNotFound = errors.New("group substitution not found")
	ErrGroupSubstitutionExists   = errors.New("group substitution already exists")
	ErrInvalidGroupSubstitution  = errors.New("invalid group substitution input")
)

// InvalidStaffAbsenceError carries the caller-facing validation reason.
type InvalidStaffAbsenceError struct{ Reason string }

func (e *InvalidStaffAbsenceError) Error() string { return e.Reason }
func (e *InvalidStaffAbsenceError) Unwrap() error { return ErrInvalidStaffAbsence }

func invalidAbsence(reason string) error { return &InvalidStaffAbsenceError{Reason: reason} }

// InvalidAbsenceTypeError carries the caller-facing validation reason of an
// absence type; the German wording is the established contract.
type InvalidAbsenceTypeError struct{ Reason string }

func (e *InvalidAbsenceTypeError) Error() string { return e.Reason }
func (e *InvalidAbsenceTypeError) Unwrap() error { return ErrAbsenceTypeInvalid }

func invalidAbsenceType(reason string) error { return &InvalidAbsenceTypeError{Reason: reason} }

// ConflictError marks a duplicate the database rejected while keeping the
// driver error reachable for callers that inspect the violated constraint.
type ConflictError struct {
	Kind  error
	Cause error
}

func (e *ConflictError) Error() string {
	if e.Cause != nil {
		return e.Kind.Error() + ": " + e.Cause.Error()
	}
	return e.Kind.Error()
}

func (e *ConflictError) Is(target error) bool { return target == e.Kind }
func (e *ConflictError) Unwrap() error        { return e.Cause }

// StaffAbsence is one absence record. Dates are calendar days in DateLayout.
type StaffAbsence struct {
	ID                int64
	TenantID          int64
	StaffID           int64
	AbsenceType       string
	AbsenceTypeID     *int64
	DateStart         string
	DateEnd           string
	HalfDay           bool
	StartHalfDay      bool
	EndHalfDay        bool
	Note              string
	Status            string
	ApprovedBy        *int64
	ApprovedAt        *time.Time
	CreatedBy         int64
	WorkingDays       *float64
	DecisionNote      string
	RequestedAt       time.Time
	SubstituteStaffID *int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Validate enforces the invariants every stored absence carries.
func (a StaffAbsence) Validate() error {
	if a.StaffID <= 0 {
		return invalidAbsence("staff ID is required")
	}
	if !slices.Contains(ValidAbsenceTypes, a.AbsenceType) {
		return invalidAbsence("invalid absence type")
	}
	if !slices.Contains(ValidAbsenceStatuses, a.Status) {
		return invalidAbsence("invalid absence status")
	}
	if a.DateStart == "" {
		return invalidAbsence("date_start is required")
	}
	if a.DateEnd == "" {
		return invalidAbsence("date_end is required")
	}
	if err := validateAbsenceDate(a.DateStart, "date_start"); err != nil {
		return err
	}
	if err := validateAbsenceDate(a.DateEnd, "date_end"); err != nil {
		return err
	}
	if a.DateStart > a.DateEnd {
		return invalidAbsence("date_start must be before or equal to date_end")
	}
	if a.CreatedBy <= 0 {
		return invalidAbsence("created_by is required")
	}
	return nil
}

func validateAbsenceDate(value, field string) error {
	parsed, err := time.Parse(DateLayout, value)
	if err != nil || parsed.Format(DateLayout) != value {
		return invalidAbsence(field + " must be a " + DateLayout + " date")
	}
	return nil
}

type StaffAbsenceOrderField string

const (
	StaffAbsenceOrderID          StaffAbsenceOrderField = "id"
	StaffAbsenceOrderStaffID     StaffAbsenceOrderField = "staff_id"
	StaffAbsenceOrderDateStart   StaffAbsenceOrderField = "date_start"
	StaffAbsenceOrderDateEnd     StaffAbsenceOrderField = "date_end"
	StaffAbsenceOrderRequestedAt StaffAbsenceOrderField = "requested_at"
)

var validStaffAbsenceOrderFields = []StaffAbsenceOrderField{
	StaffAbsenceOrderID, StaffAbsenceOrderStaffID, StaffAbsenceOrderDateStart, StaffAbsenceOrderDateEnd, StaffAbsenceOrderRequestedAt,
}

type StaffAbsenceOrder struct {
	Field      StaffAbsenceOrderField
	Descending bool
}

// StaffAbsenceFilter narrows a listing; see the public facade for semantics.
type StaffAbsenceFilter struct {
	StaffID           int64
	StaffIDs          []int64
	Statuses          []string
	Types             []string
	OverlapFrom       string
	OverlapTo         string
	DateEndBefore     string
	DateStartBefore   string
	NonHistoricalFrom string
	Order             []StaffAbsenceOrder
	Limit             int
	Offset            int
}

// Validate rejects malformed dates and unknown ordering columns before the
// filter reaches SQL.
func (f StaffAbsenceFilter) Validate() error {
	for _, pair := range []struct{ value, field string }{
		{f.OverlapFrom, "overlap_from"}, {f.OverlapTo, "overlap_to"},
		{f.DateEndBefore, "date_end_before"}, {f.DateStartBefore, "date_start_before"},
		{f.NonHistoricalFrom, "non_historical_from"},
	} {
		if pair.value == "" {
			continue
		}
		if err := validateAbsenceDate(pair.value, pair.field); err != nil {
			return err
		}
	}
	for _, order := range f.Order {
		if !slices.Contains(validStaffAbsenceOrderFields, order.Field) {
			return invalidAbsence("unsupported order field " + string(order.Field))
		}
	}
	if f.Limit < 0 || f.Offset < 0 {
		return invalidAbsence("limit and offset must not be negative")
	}
	return nil
}

// ValidateDateColumn accepts only the two calendar columns a retention
// cleanup may address.
func ValidateDateColumn(column string) error {
	if column != StaffAbsenceDateStart && column != StaffAbsenceDateEnd {
		return invalidAbsence("unsupported date column " + column)
	}
	return nil
}

type StaffAbsenceRequestFilter struct {
	Statuses        []string
	Types           []string
	FilterSubjects  bool
	SubjectStaffIDs []int64
	Limit           int
	Decided         bool
}

// WinningAbsences picks, per staff member, the effective absence with the
// highest type priority. Candidates must be ordered by staff, priority and
// ID, so equal priorities resolve to the lowest ID deterministically.
func WinningAbsences(candidates []StaffAbsence) map[int64]StaffAbsence {
	winner := make(map[int64]StaffAbsence, len(candidates))
	for _, candidate := range candidates {
		existing, exists := winner[candidate.StaffID]
		if !exists || AbsenceTypePriority[candidate.AbsenceType] > AbsenceTypePriority[existing.AbsenceType] {
			winner[candidate.StaffID] = candidate
		}
	}
	return winner
}

// StaffAbsenceType is a tenant-defined display name for an absence.
type StaffAbsenceType struct {
	ID               int64
	TenantID         int64
	Name             string
	BaseType         string
	IsActive         bool
	AllowanceEnabled bool
	OverrunPolicy    string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// StaffAbsenceTypeFields is the writable part of an absence type.
type StaffAbsenceTypeFields struct {
	Name             string
	BaseType         string
	IsActive         bool
	AllowanceEnabled bool
	OverrunPolicy    string
}

// Normalize trims the name and defaults the base type and overrun policy,
// then validates. The German reasons are the established contract of the
// absence-type route.
func (f *StaffAbsenceTypeFields) Normalize() error {
	f.Name = strings.TrimSpace(f.Name)
	if f.Name == "" {
		return invalidAbsenceType("fehlender Name der Abwesenheitsart")
	}
	if len([]rune(f.Name)) > maxAbsenceTypeNameLength {
		return invalidAbsenceType("zu langer Name der Abwesenheitsart (höchstens 100 Zeichen)")
	}
	if f.BaseType == "" {
		f.BaseType = AbsenceTypeOther
	}
	if !slices.Contains(ValidAbsenceTypes, f.BaseType) {
		return invalidAbsenceType("ungültiger Grundtyp der Abwesenheit")
	}
	if f.OverrunPolicy == "" {
		f.OverrunPolicy = AbsenceTypeOverrunWarn
	}
	if f.OverrunPolicy != AbsenceTypeOverrunWarn && f.OverrunPolicy != AbsenceTypeOverrunBlock {
		return invalidAbsenceType("ungültige Regel bei Überschreitung")
	}
	return nil
}

// StaffAbsenceAudit is one status transition of an absence.
type StaffAbsenceAudit struct {
	ID         int64
	TenantID   int64
	AbsenceID  int64
	FromStatus *string
	ToStatus   string
	ActorID    int64
	Note       string
	ChangedAt  time.Time
}

func (a StaffAbsenceAudit) Validate() error {
	if a.AbsenceID <= 0 {
		return invalidAbsence("absence_id is required")
	}
	if a.ToStatus == "" {
		return invalidAbsence("to_status is required")
	}
	if a.ActorID <= 0 {
		return invalidAbsence("actor_id is required")
	}
	return nil
}
