// Package workforce is the public Workforce capability. It owns
// config.work_time_models, config.work_time_model_entries,
// config.staff_work_schedules, active.staff_absences,
// active.staff_absence_types, active.staff_absence_audit,
// education.group_substitution, schedule.staff_shifts,
// schedule.staff_shift_series, schedule.staff_shift_series_exceptions,
// schedule.shift_types, active.work_sessions, active.work_session_breaks,
// active.staff_balance_adjustments, active.staff_vacation_openings,
// active.staff_vacation_quota, users.staff_master_data,
// users.staff_qualifications, users.staff_financial_data,
// users.staff_documents and users.staff_document_file_cleanup: every read or
// write of those rows by another owner goes through Query or Command instead
// of a foreign SQL join.
//
// The capability stops at the rows themselves. Which staff member is bound to
// a template, and who a staff member or a group is, lives with School
// Membership, People Directory and School Structure; Workforce reaches those
// facts through their owners, never by joining their tables.
package workforce

import (
	"context"
	"errors"
)

const (
	// DateLayout is the calendar-date wire format of every date field.
	DateLayout = "2006-01-02"
	// ClockLayout is the wall-clock wire format of every time-of-day field.
	ClockLayout = "15:04:05"
	// MaxRotationWeeks caps a template at four rotation weeks (A/B/C/D).
	MaxRotationWeeks = 4
	// MaxDailyMinutes caps a single day's target working time at 12 hours.
	MaxDailyMinutes = 720
)

var (
	ErrWorkTimeModelNotFound = errors.New("work time model not found")
	ErrWorkTimeModelAssigned = errors.New("work time model is assigned to staff")
	ErrInvalidWorkTime       = errors.New("invalid work time input")
)

// InvalidWorkTimeError carries the caller-facing validation reason; it unwraps
// to ErrInvalidWorkTime so callers can classify it with errors.Is.
type InvalidWorkTimeError struct{ Reason string }

func (e *InvalidWorkTimeError) Error() string { return e.Reason }
func (e *InvalidWorkTimeError) Unwrap() error { return ErrInvalidWorkTime }

// ErrorCode maps a capability error to the stable operation code used by the
// runtime evidence and the HTTP adapters.
func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrOffboardingConflict):
		return "offboarding_conflict"
	case errors.Is(err, ErrOffboardingInUse):
		return "staff_in_use"
	case err == nil:
		return "none"
	case errors.Is(err, ErrWorkTimeModelNotFound):
		return "not_found"
	case errors.Is(err, ErrWorkTimeModelAssigned):
		return "conflict"
	case errors.Is(err, ErrInvalidWorkTime), errors.Is(err, ErrInvalidStaffAbsence), errors.Is(err, ErrInvalidGroupSubstitution),
		errors.Is(err, ErrAbsenceTypeInvalid), errors.Is(err, ErrAbsenceTypeAllowanceInvalid),
		errors.Is(err, ErrInvalidStaffShift), errors.Is(err, ErrInvalidShiftSeries), errors.Is(err, ErrInvalidShiftType),
		errors.Is(err, ErrInvalidWorkSession), errors.Is(err, ErrInvalidStaffRecord), errors.Is(err, ErrInvalidStaffTargetOverride):
		return "invalid"
	case errors.Is(err, ErrStaffAbsenceNotFound), errors.Is(err, ErrAbsenceTypeNotFound), errors.Is(err, ErrGroupSubstitutionNotFound),
		errors.Is(err, ErrStaffShiftNotFound), errors.Is(err, ErrShiftSeriesNotFound), errors.Is(err, ErrShiftTypeNotFound),
		errors.Is(err, ErrWorkSessionNotFound), errors.Is(err, ErrWorkSessionBreakNotFound), errors.Is(err, ErrStaffBalanceAdjustmentNotFound),
		errors.Is(err, ErrStaffVacationOpeningNotFound), errors.Is(err, ErrStaffVacationQuotaNotFound),
		errors.Is(err, ErrStaffMasterDataNotFound), errors.Is(err, ErrStaffFinancialDataNotFound), errors.Is(err, ErrStaffDocumentNotFound),
		errors.Is(err, ErrStaffEmploymentNotFound), errors.Is(err, ErrStaffTargetOverrideNotFound):
		return "not_found"
	case errors.Is(err, ErrAbsenceTypeNameTaken), errors.Is(err, ErrAbsenceTypeNameReserved), errors.Is(err, ErrAbsenceTypeInUse),
		errors.Is(err, ErrAbsenceTypeInactive), errors.Is(err, ErrAbsenceTypeAllowanceExceeded), errors.Is(err, ErrAllowanceBookingOverlap), errors.Is(err, ErrGroupSubstitutionExists),
		errors.Is(err, ErrStaffShiftDuplicate), errors.Is(err, ErrShiftTypeNameTaken), errors.Is(err, ErrWorkSessionAlreadyOpen),
		errors.Is(err, ErrPersonnelNumberTaken), errors.Is(err, ErrStaffTargetOverrideRejected):
		return "conflict"
	default:
		return "internal_error"
	}
}

// WorkTimeModelEntry is the target working time of one (week index, day of
// week) slot inside a template. StartTime is a wall clock in ClockLayout and
// empty when the slot has no planned start.
type WorkTimeModelEntry struct {
	WeekIndex     int    `json:"week_index"`
	DayOfWeek     int    `json:"day_of_week"`
	TargetMinutes int    `json:"target_minutes"`
	StartTime     string `json:"start_time,omitempty"`
}

// WorkTimeModel is a tenant-scoped, named working-time template that admins
// assign to staff. RotationAnchorDate is a calendar date in DateLayout.
type WorkTimeModel struct {
	ID                 int64                `json:"id"`
	TenantID           int64                `json:"tenant_id"`
	Name               string               `json:"name"`
	RotationLength     int                  `json:"rotation_length"`
	RotationAnchorDate string               `json:"rotation_anchor_date"`
	Entries            []WorkTimeModelEntry `json:"entries,omitempty"`
}

// WorkTimeModelFields is the writable part of a template. The entries replace
// the stored ones wholesale; a template carries at most one entry per slot.
type WorkTimeModelFields struct {
	Name               string
	RotationLength     int
	RotationAnchorDate string
	Entries            []WorkTimeModelEntry
}

type CreateWorkTimeModel struct {
	WorkTimeModelFields
}

type UpdateWorkTimeModel struct {
	ID int64
	WorkTimeModelFields
}

// StaffWorkSchedule is one version of a staff member's target working time for
// a single weekday. ValidUntil is exclusive and empty while the version is the
// current one. RotationAnchorDate is the anchor the version was written with;
// it is empty for a single-week schedule, which has no parity.
type StaffWorkSchedule struct {
	ID                 int64  `json:"id"`
	TenantID           int64  `json:"tenant_id"`
	StaffID            int64  `json:"staff_id"`
	WeekIndex          int    `json:"week_index"`
	RotationLength     int    `json:"rotation_length"`
	DayOfWeek          int    `json:"day_of_week"`
	TargetMinutes      int    `json:"target_minutes"`
	StartTime          string `json:"start_time,omitempty"`
	RotationAnchorDate string `json:"rotation_anchor_date,omitempty"`
	ValidFrom          string `json:"valid_from"`
	ValidUntil         string `json:"valid_until,omitempty"`
}

// StaffWorkScheduleEntry is one weekday of a schedule version being written.
type StaffWorkScheduleEntry struct {
	WeekIndex      int
	RotationLength int
	DayOfWeek      int
	TargetMinutes  int
	StartTime      string
}

// ReplaceStaffSchedule closes the staff member's running schedule versions and
// writes Entries as the new current version. An empty RotationAnchorDate
// leaves the per-version anchor unset. An empty ValidFrom starts the version
// today; an earlier day is accepted for the first schedule of a staff member
// only, and never inside a closed month.
type ReplaceStaffSchedule struct {
	StaffID            int64
	Entries            []StaffWorkScheduleEntry
	RotationAnchorDate string
	ValidFrom          string
}

type Query interface {
	MonthSnapshotQuery
	AbsenceQuery
	SubstitutionQuery
	ShiftQuery
	WorkSessionQuery
	StaffRecordQuery
	StaffTargetOverrideQuery

	// ListWorkTimeModels returns every template of the caller's tenant with
	// its entries, ordered by name.
	ListWorkTimeModels(context.Context) ([]WorkTimeModel, error)
	// FindWorkTimeModel resolves one template with its entries.
	FindWorkTimeModel(context.Context, int64) (WorkTimeModel, error)
	// ListWorkTimeModelsByIDs resolves the given templates in one batched
	// read; IDs that do not exist are absent from the result.
	ListWorkTimeModelsByIDs(context.Context, []int64) ([]WorkTimeModel, error)

	// CurrentStaffSchedule returns the running schedule version of a staff
	// member, ordered by day of week.
	CurrentStaffSchedule(context.Context, int64) ([]StaffWorkSchedule, error)
	// StaffScheduleOn returns the schedule version valid on a calendar day.
	StaffScheduleOn(ctx context.Context, staffID int64, date string) ([]StaffWorkSchedule, error)
	// StaffSchedulesInRange returns every version of the given staff members
	// whose validity window intersects [from, to]; valid_until is exclusive.
	StaffSchedulesInRange(ctx context.Context, staffIDs []int64, from, to string) ([]StaffWorkSchedule, error)
	StaffPlanningProfiles(ctx context.Context, staffIDs []int64, from, to string) ([]StaffPlanningProfile, error)
	// HasStaffScheduleHistory reports whether a staff member ever had a
	// schedule version, closed-out ones included. It separates "no schedule
	// was ever written", where the assigned template may stand in, from "no
	// version is valid here", where the target time is genuinely zero.
	HasStaffScheduleHistory(context.Context, int64) (bool, error)
	// StaffIDsWithScheduleHistory is HasStaffScheduleHistory batched; only
	// staff with at least one version appear.
	StaffIDsWithScheduleHistory(context.Context, []int64) (map[int64]bool, error)
}

type Command interface {
	MonthSnapshotCommand
	AbsenceCommand
	SubstitutionCommand
	ShiftCommand
	WorkSessionCommand
	StaffRecordCommand
	StaffTargetOverrideCommand

	CreateWorkTimeModel(context.Context, CreateWorkTimeModel) (WorkTimeModel, error)
	// UpdateWorkTimeModel replaces the template metadata and every entry, and
	// in the same unit of work rewrites the schedule versions of every staff
	// member bound to it: their running versions are closed at today so a
	// template edit cannot re-parity a week that is already accounted for.
	// Both writes commit or roll back together.
	UpdateWorkTimeModel(context.Context, UpdateWorkTimeModel) (WorkTimeModel, error)
	// DeleteWorkTimeModel removes a template no staff member is bound to;
	// an assigned template yields ErrWorkTimeModelAssigned.
	DeleteWorkTimeModel(context.Context, int64) error
	ReplaceStaffSchedule(context.Context, ReplaceStaffSchedule) error
}

type Capability interface {
	Query
	Command
}

// --- work-time templates ---

// --- staff schedules ---
