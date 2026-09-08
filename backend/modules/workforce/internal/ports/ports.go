package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// Store is the persistence port over config.work_time_models,
// config.work_time_model_entries and config.staff_work_schedules. Reads and
// writes honour the tenant in context when one is bound.
type Store interface {
	ListWorkTimeModels(context.Context) ([]domain.WorkTimeModel, domain.OperationStats, error)
	FindWorkTimeModel(context.Context, int64) (domain.WorkTimeModel, bool, domain.OperationStats, error)
	ListWorkTimeModelsByIDs(context.Context, []int64) ([]domain.WorkTimeModel, domain.OperationStats, error)
	CreateWorkTimeModel(context.Context, domain.WorkTimeModelFields) (domain.WorkTimeModel, domain.OperationStats, error)
	// UpdateWorkTimeModel replaces the template metadata and every entry.
	// A missing template reports found=false.
	UpdateWorkTimeModel(context.Context, int64, domain.WorkTimeModelFields) (domain.WorkTimeModel, bool, domain.OperationStats, error)
	DeleteWorkTimeModel(context.Context, int64) (bool, domain.OperationStats, error)

	// CloseStaffSchedules sets an exclusive valid_until on every running
	// version of the given staff members.
	CloseStaffSchedules(ctx context.Context, staffIDs []int64, until string) (domain.OperationStats, error)
	// InsertStaffSchedules writes the given rows as new current versions in
	// one statement.
	InsertStaffSchedules(ctx context.Context, rows []domain.StaffWorkSchedule) (domain.OperationStats, error)

	CurrentStaffSchedule(context.Context, int64) ([]domain.StaffWorkSchedule, domain.OperationStats, error)
	StaffScheduleOn(ctx context.Context, staffID int64, date string) ([]domain.StaffWorkSchedule, domain.OperationStats, error)
	StaffSchedulesInRange(ctx context.Context, staffIDs []int64, from, to string) ([]domain.StaffWorkSchedule, domain.OperationStats, error)
	HasStaffScheduleHistory(context.Context, int64) (bool, domain.OperationStats, error)
	StaffIDsWithScheduleHistory(context.Context, []int64) (map[int64]bool, domain.OperationStats, error)

	AbsenceStore
	SubstitutionStore
	ShiftStore
}

// ShiftStore is the persistence port over schedule.staff_shifts,
// schedule.staff_shift_series, schedule.staff_shift_series_exceptions and
// schedule.shift_types. A missing row reports found=false; a duplicate
// reports domain.ConflictError.
type ShiftStore interface {
	FindStaffShift(context.Context, int64) (domain.StaffShift, bool, domain.OperationStats, error)
	ListStaffShifts(context.Context, domain.StaffShiftFilter) ([]domain.StaffShift, domain.OperationStats, error)
	// UsedStaffShiftWeeks returns the Monday of every ISO week holding at
	// least one non-cancelled shift in the inclusive range.
	UsedStaffShiftWeeks(ctx context.Context, from, to string) ([]string, domain.OperationStats, error)
	CreateStaffShift(context.Context, domain.StaffShift) (domain.StaffShift, domain.OperationStats, error)
	// CreateStaffShifts inserts every row in one statement and returns them
	// with their identities.
	CreateStaffShifts(context.Context, []domain.StaffShift) ([]domain.StaffShift, domain.OperationStats, error)
	UpdateStaffShift(context.Context, domain.StaffShift) (domain.StaffShift, bool, domain.OperationStats, error)
	// UpdateStaffShiftColumns stamps only the named columns of the row.
	UpdateStaffShiftColumns(ctx context.Context, shift domain.StaffShift, columns []string) (int64, domain.OperationStats, error)
	DeleteStaffShift(context.Context, int64) (domain.OperationStats, error)
	DeleteUpcomingStaffShifts(ctx context.Context, staffID int64, from string) (int64, domain.OperationStats, error)
	// DeleteRegenerableSeriesShifts removes a series' non-detached rows on or
	// after from.
	DeleteRegenerableSeriesShifts(ctx context.Context, seriesID int64, from string) (int64, domain.OperationStats, error)
	// RepointDetachedSeriesShifts moves a series' detached rows whose source
	// slot is on or after from to the successor series.
	RepointDetachedSeriesShifts(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, domain.OperationStats, error)

	FindStaffShiftSeries(context.Context, int64) (domain.StaffShiftSeries, bool, domain.OperationStats, error)
	// FindOverlappingSeriesInLineage returns the chronologically first other
	// segment of a split lineage still active on or after from.
	FindOverlappingSeriesInLineage(ctx context.Context, rootID, excludeID int64, from string) (domain.StaffShiftSeries, bool, domain.OperationStats, error)
	CreateStaffShiftSeries(context.Context, domain.StaffShiftSeries) (domain.StaffShiftSeries, domain.OperationStats, error)
	UpdateStaffShiftSeries(context.Context, domain.StaffShiftSeries) (domain.StaffShiftSeries, bool, domain.OperationStats, error)
	DeleteStaffShiftSeries(context.Context, int64) (domain.OperationStats, error)
	// CapStaffShiftSeries bounds one segment at the exclusive date, keeping
	// an already tighter bound and never moving below valid_from.
	CapStaffShiftSeries(ctx context.Context, id int64, until string) (domain.OperationStats, error)
	// CapStaffShiftSeriesForStaff bounds every segment of one staff member.
	CapStaffShiftSeriesForStaff(ctx context.Context, staffID int64, until string) (int64, domain.OperationStats, error)

	// RecordSeriesException stores the removed occurrence; recording the same
	// slot again is a successful no-op.
	RecordSeriesException(context.Context, domain.StaffShiftSeriesException) (domain.OperationStats, error)
	SeriesExceptionDates(ctx context.Context, seriesID int64) ([]string, domain.OperationStats, error)
	RepointSeriesExceptions(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, domain.OperationStats, error)

	ListShiftTypes(context.Context) ([]domain.ShiftType, domain.OperationStats, error)
	FindShiftType(context.Context, int64) (domain.ShiftType, bool, domain.OperationStats, error)
	CreateShiftType(context.Context, domain.ShiftType) (domain.ShiftType, domain.OperationStats, error)
	// CreateShiftTypeIfAbsent inserts unless a type with the same name exists
	// in the tenant; created reports whether a row was written.
	CreateShiftTypeIfAbsent(context.Context, domain.ShiftType) (domain.ShiftType, bool, domain.OperationStats, error)
	UpdateShiftType(context.Context, domain.ShiftType) (domain.ShiftType, bool, domain.OperationStats, error)
	DeleteShiftType(context.Context, int64) (domain.OperationStats, error)
}

// AbsenceStore is the persistence port over active.staff_absences,
// active.staff_absence_types and active.staff_absence_audit. A missing row
// reports found=false; a duplicate reports domain.ConflictError.
type AbsenceStore interface {
	FindStaffAbsence(context.Context, int64) (domain.StaffAbsence, bool, domain.OperationStats, error)
	ListStaffAbsences(context.Context, domain.StaffAbsenceFilter) ([]domain.StaffAbsence, domain.OperationStats, error)
	CountStaffAbsences(context.Context, domain.StaffAbsenceFilter) (int, domain.OperationStats, error)
	ListStaffAbsenceRequests(context.Context, domain.StaffAbsenceRequestFilter) ([]domain.StaffAbsence, domain.OperationStats, error)
	// EffectiveStaffAbsencesOn returns the effective absences covering the
	// day, ordered by staff, type priority and ID.
	EffectiveStaffAbsencesOn(ctx context.Context, date string) ([]domain.StaffAbsence, domain.OperationStats, error)
	OldestStaffAbsenceDate(ctx context.Context, column, before string) (string, domain.OperationStats, error)
	CreateStaffAbsence(context.Context, domain.StaffAbsence) (domain.StaffAbsence, domain.OperationStats, error)
	UpdateStaffAbsence(context.Context, domain.StaffAbsence) (domain.StaffAbsence, bool, domain.OperationStats, error)
	DeleteStaffAbsence(context.Context, int64) (domain.OperationStats, error)
	DeleteNonHistoricalStaffAbsences(ctx context.Context, staffID int64, from string) (int64, domain.OperationStats, error)
	DeleteStaffAbsencesOlderThan(ctx context.Context, column, cutoff string) (int64, domain.OperationStats, error)

	ListStaffAbsenceTypes(context.Context) ([]domain.StaffAbsenceType, domain.OperationStats, error)
	FindStaffAbsenceType(ctx context.Context, id int64, lock bool) (domain.StaffAbsenceType, bool, domain.OperationStats, error)
	StaffAbsenceTypeInUse(context.Context, int64) (bool, domain.OperationStats, error)
	CreateStaffAbsenceType(context.Context, domain.StaffAbsenceTypeFields) (domain.StaffAbsenceType, domain.OperationStats, error)
	UpdateStaffAbsenceType(context.Context, domain.StaffAbsenceType) (domain.StaffAbsenceType, bool, domain.OperationStats, error)

	RecordStaffAbsenceAudit(context.Context, domain.StaffAbsenceAudit) (domain.StaffAbsenceAudit, domain.OperationStats, error)
}

// SubstitutionStore is the persistence port over education.group_substitution.
type SubstitutionStore interface {
	FindGroupSubstitution(ctx context.Context, id int64, lock bool) (domain.GroupSubstitution, bool, domain.OperationStats, error)
	ListGroupSubstitutions(context.Context, domain.GroupSubstitutionFilter) ([]domain.GroupSubstitution, domain.OperationStats, error)
	CreateGroupSubstitution(context.Context, domain.GroupSubstitution) (domain.GroupSubstitution, domain.OperationStats, error)
	UpdateGroupSubstitution(context.Context, domain.GroupSubstitution) (domain.GroupSubstitution, bool, domain.OperationStats, error)
	DeleteGroupSubstitution(context.Context, int64) (domain.OperationStats, error)
	DeleteGroupSubstitutionsForStaff(ctx context.Context, staffID int64, from string) (int64, domain.OperationStats, error)
}

// StaffAssignments is the consumer-owned port over the School Membership rows
// that bind staff to a template. Workforce never joins users.staff itself.
type StaffAssignments interface {
	// AssignedStaffIDs returns the live staff members assigned to a template.
	AssignedStaffIDs(ctx context.Context, workTimeModelID int64) ([]int64, error)
	// RebaseAnchor stamps the template's rotation anchor onto every live
	// assignee and returns their IDs.
	RebaseAnchor(ctx context.Context, workTimeModelID int64, anchorDate string) ([]int64, error)
}

// Transaction runs a unit of work on the caller's ambient transaction or, when
// there is none, opens one.
type Transaction interface {
	RunWrite(context.Context, func(context.Context) error) error
	// LockStaffBalance serializes writes that change a staff member's Soll.
	LockStaffBalance(ctx context.Context, staffID int64) error
	// LockStaffAbsence serializes overlap-sensitive absence writes of one
	// staff member. Callers take LockStaffBalance first, because an effective
	// absence also changes the Stundenkonto.
	LockStaffAbsence(ctx context.Context, staffID int64) error
}

// Clock supplies the calendar day new schedule versions start on.
type Clock interface {
	Today() string
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
