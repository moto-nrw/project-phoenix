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

func invalid(reason string) error { return &InvalidWorkTimeError{Reason: reason} }

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
		errors.Is(err, ErrInvalidWorkSession), errors.Is(err, ErrInvalidStaffRecord):
		return "invalid"
	case errors.Is(err, ErrStaffAbsenceNotFound), errors.Is(err, ErrAbsenceTypeNotFound), errors.Is(err, ErrGroupSubstitutionNotFound),
		errors.Is(err, ErrStaffShiftNotFound), errors.Is(err, ErrShiftSeriesNotFound), errors.Is(err, ErrShiftTypeNotFound),
		errors.Is(err, ErrWorkSessionNotFound), errors.Is(err, ErrWorkSessionBreakNotFound), errors.Is(err, ErrStaffBalanceAdjustmentNotFound),
		errors.Is(err, ErrStaffVacationOpeningNotFound), errors.Is(err, ErrStaffVacationQuotaNotFound),
		errors.Is(err, ErrStaffMasterDataNotFound), errors.Is(err, ErrStaffFinancialDataNotFound), errors.Is(err, ErrStaffDocumentNotFound):
		return "not_found"
	case errors.Is(err, ErrAbsenceTypeNameTaken), errors.Is(err, ErrAbsenceTypeNameReserved), errors.Is(err, ErrAbsenceTypeInUse),
		errors.Is(err, ErrAbsenceTypeInactive), errors.Is(err, ErrAbsenceTypeAllowanceExceeded), errors.Is(err, ErrGroupSubstitutionExists),
		errors.Is(err, ErrStaffShiftDuplicate), errors.Is(err, ErrShiftTypeNameTaken), errors.Is(err, ErrWorkSessionAlreadyOpen):
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
// leaves the per-version anchor unset.
type ReplaceStaffSchedule struct {
	StaffID            int64
	Entries            []StaffWorkScheduleEntry
	RotationAnchorDate string
}

type Query interface {
	AbsenceQuery
	SubstitutionQuery
	ShiftQuery
	WorkSessionQuery
	StaffRecordQuery

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
	AbsenceCommand
	SubstitutionCommand
	ShiftCommand
	WorkSessionCommand
	StaffRecordCommand

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

type engine interface {
	absenceEngine
	substitutionEngine
	shiftEngine
	workSessionEngine
	staffRecordEngine

	ListWorkTimeModels(context.Context) ([]WorkTimeModel, error)
	FindWorkTimeModel(context.Context, int64) (WorkTimeModel, error)
	ListWorkTimeModelsByIDs(context.Context, []int64) ([]WorkTimeModel, error)
	CreateWorkTimeModel(context.Context, CreateWorkTimeModel) (WorkTimeModel, error)
	UpdateWorkTimeModel(context.Context, UpdateWorkTimeModel) (WorkTimeModel, error)
	DeleteWorkTimeModel(context.Context, int64) error

	ReplaceStaffSchedule(context.Context, ReplaceStaffSchedule) error
	CurrentStaffSchedule(context.Context, int64) ([]StaffWorkSchedule, error)
	StaffScheduleOn(ctx context.Context, staffID int64, date string) ([]StaffWorkSchedule, error)
	StaffSchedulesInRange(ctx context.Context, staffIDs []int64, from, to string) ([]StaffWorkSchedule, error)
	HasStaffScheduleHistory(context.Context, int64) (bool, error)
	StaffIDsWithScheduleHistory(context.Context, []int64) (map[int64]bool, error)
}

// Module is the one Workforce work-time facade. It guards the arguments every
// caller must get right and delegates the rest to the composed engine.
type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("workforce: engine is required")
	}
	return &Module{engine: engine}
}

// --- work-time templates ---

func (m *Module) ListWorkTimeModels(ctx context.Context) ([]WorkTimeModel, error) {
	return m.engine.ListWorkTimeModels(ctx)
}

func (m *Module) FindWorkTimeModel(ctx context.Context, id int64) (WorkTimeModel, error) {
	if id <= 0 {
		return WorkTimeModel{}, invalid("work time model ID is required")
	}
	return m.engine.FindWorkTimeModel(ctx, id)
}

func (m *Module) ListWorkTimeModelsByIDs(ctx context.Context, ids []int64) ([]WorkTimeModel, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return m.engine.ListWorkTimeModelsByIDs(ctx, ids)
}

func (m *Module) CreateWorkTimeModel(ctx context.Context, input CreateWorkTimeModel) (WorkTimeModel, error) {
	return m.engine.CreateWorkTimeModel(ctx, input)
}

func (m *Module) UpdateWorkTimeModel(ctx context.Context, input UpdateWorkTimeModel) (WorkTimeModel, error) {
	if input.ID <= 0 {
		return WorkTimeModel{}, invalid("work time model ID is required")
	}
	return m.engine.UpdateWorkTimeModel(ctx, input)
}

func (m *Module) DeleteWorkTimeModel(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("work time model ID is required")
	}
	return m.engine.DeleteWorkTimeModel(ctx, id)
}

// --- staff schedules ---

func (m *Module) ReplaceStaffSchedule(ctx context.Context, input ReplaceStaffSchedule) error {
	if input.StaffID <= 0 {
		return invalid("staff ID is required")
	}
	return m.engine.ReplaceStaffSchedule(ctx, input)
}

func (m *Module) CurrentStaffSchedule(ctx context.Context, staffID int64) ([]StaffWorkSchedule, error) {
	if staffID <= 0 {
		return nil, invalid("staff ID is required")
	}
	return m.engine.CurrentStaffSchedule(ctx, staffID)
}

func (m *Module) StaffScheduleOn(ctx context.Context, staffID int64, date string) ([]StaffWorkSchedule, error) {
	if staffID <= 0 {
		return nil, invalid("staff ID is required")
	}
	return m.engine.StaffScheduleOn(ctx, staffID, date)
}

func (m *Module) StaffSchedulesInRange(ctx context.Context, staffIDs []int64, from, to string) ([]StaffWorkSchedule, error) {
	if len(staffIDs) == 0 {
		return nil, nil
	}
	return m.engine.StaffSchedulesInRange(ctx, staffIDs, from, to)
}

func (m *Module) HasStaffScheduleHistory(ctx context.Context, staffID int64) (bool, error) {
	if staffID <= 0 {
		return false, invalid("staff ID is required")
	}
	return m.engine.HasStaffScheduleHistory(ctx, staffID)
}

func (m *Module) StaffIDsWithScheduleHistory(ctx context.Context, staffIDs []int64) (map[int64]bool, error) {
	if len(staffIDs) == 0 {
		return map[int64]bool{}, nil
	}
	return m.engine.StaffIDsWithScheduleHistory(ctx, staffIDs)
}
