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
	PreviewStaffOffboarding(context.Context, int64, string) (domain.OffboardingSnapshot, domain.OperationStats, error)
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
	WorkSessionStore
	StaffRecordStore
}

// WorkSessionStore is the persistence port over active.work_sessions,
// active.work_session_breaks, active.staff_balance_adjustments,
// active.staff_vacation_openings and active.staff_vacation_quota. A missing
// row reports found=false; a rejected duplicate reports domain.ConflictError.
type WorkSessionStore interface {
	FindWorkSession(context.Context, int64) (domain.WorkSession, bool, domain.OperationStats, error)
	// LockOpenWorkSession returns and row-locks a still running block.
	LockOpenWorkSession(context.Context, int64) (domain.WorkSession, bool, domain.OperationStats, error)
	// OpenWorkSessionOn returns the running block of a staff member filed on
	// the day, optionally row-locked.
	OpenWorkSessionOn(ctx context.Context, staffID int64, date string, lock bool) (domain.WorkSession, bool, domain.OperationStats, error)
	// LatestOpenWorkSession returns the most recent block that is still
	// running inside the live window as of now, whatever day it was filed on;
	// today is the calendar day of now.
	LatestOpenWorkSession(ctx context.Context, staffID int64, today string, now time.Time) (domain.WorkSession, bool, domain.OperationStats, error)
	ListWorkSessions(context.Context, domain.WorkSessionFilter) ([]domain.WorkSession, domain.OperationStats, error)
	// ListOverlappingWorkSessions returns the blocks of the staff members
	// whose [check-in, check-out) interval intersects [from, to); a nil to is
	// open-ended. Ordered by staff, then check-in.
	ListOverlappingWorkSessions(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) ([]domain.WorkSession, domain.OperationStats, error)
	CountWorkSessions(context.Context, domain.WorkSessionFilter) (int, domain.OperationStats, error)
	OldestWorkSessionDate(ctx context.Context, before string) (string, domain.OperationStats, error)
	DeleteWorkSessionsOlderThan(ctx context.Context, cutoff string) (int64, domain.OperationStats, error)
	// WorkPresenceMap maps staff to their work status as of now: the status of
	// a live open block, otherwise checked_out for a block filed today.
	WorkPresenceMap(ctx context.Context, today string, now time.Time) (map[int64]string, domain.OperationStats, error)
	CreateWorkSession(context.Context, domain.WorkSession) (domain.WorkSession, domain.OperationStats, error)
	UpdateWorkSession(context.Context, domain.WorkSession) (domain.WorkSession, bool, domain.OperationStats, error)
	DeleteWorkSession(context.Context, int64) (domain.OperationStats, error)
	SetWorkSessionBreakMinutes(ctx context.Context, id int64, minutes int) (int64, domain.OperationStats, error)
	// CloseWorkSession stamps the check-out on a still open block; closed
	// reports whether a row was actually closed.
	CloseWorkSession(ctx context.Context, id int64, checkOut time.Time, autoCheckedOut bool) (bool, domain.OperationStats, error)

	FindWorkSessionBreak(context.Context, int64) (domain.WorkSessionBreak, bool, domain.OperationStats, error)
	ListWorkSessionBreaks(context.Context, domain.WorkSessionBreakFilter) ([]domain.WorkSessionBreak, domain.OperationStats, error)
	// ExpiredWorkSessionBreaks returns the running breaks whose planned end
	// has passed.
	ExpiredWorkSessionBreaks(ctx context.Context, before time.Time) ([]domain.WorkSessionBreak, domain.OperationStats, error)
	CreateWorkSessionBreak(context.Context, domain.WorkSessionBreak) (domain.WorkSessionBreak, domain.OperationStats, error)
	UpdateWorkSessionBreak(context.Context, domain.WorkSessionBreak) (domain.WorkSessionBreak, bool, domain.OperationStats, error)
	DeleteWorkSessionBreak(context.Context, int64) (domain.OperationStats, error)
	// EndWorkSessionBreak stamps the end on a still running break only.
	EndWorkSessionBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) (int64, domain.OperationStats, error)
	// SetWorkSessionBreakDuration rewrites the length and end of a break.
	SetWorkSessionBreakDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) (int64, domain.OperationStats, error)

	FindStaffBalanceAdjustment(context.Context, int64) (domain.StaffBalanceAdjustment, bool, domain.OperationStats, error)
	ListStaffBalanceAdjustments(context.Context, domain.StaffBalanceAdjustmentFilter) ([]domain.StaffBalanceAdjustment, domain.OperationStats, error)
	CreateStaffBalanceAdjustment(context.Context, domain.StaffBalanceAdjustment) (domain.StaffBalanceAdjustment, domain.OperationStats, error)
	UpdateStaffBalanceAdjustment(context.Context, domain.StaffBalanceAdjustment) (domain.StaffBalanceAdjustment, bool, domain.OperationStats, error)
	DeleteStaffBalanceAdjustment(context.Context, int64) (domain.OperationStats, error)

	FindStaffVacationOpening(context.Context, int64) (domain.StaffVacationOpening, bool, domain.OperationStats, error)
	ListStaffVacationOpenings(context.Context, domain.StaffVacationFilter) ([]domain.StaffVacationOpening, domain.OperationStats, error)
	CreateStaffVacationOpening(context.Context, domain.StaffVacationOpening) (domain.StaffVacationOpening, domain.OperationStats, error)
	UpdateStaffVacationOpening(context.Context, domain.StaffVacationOpening) (domain.StaffVacationOpening, bool, domain.OperationStats, error)
	DeleteStaffVacationOpening(context.Context, int64) (domain.OperationStats, error)

	FindStaffVacationQuota(context.Context, int64) (domain.StaffVacationQuota, bool, domain.OperationStats, error)
	ListStaffVacationQuotas(context.Context, domain.StaffVacationFilter) ([]domain.StaffVacationQuota, domain.OperationStats, error)
	CreateStaffVacationQuota(context.Context, domain.StaffVacationQuota) (domain.StaffVacationQuota, domain.OperationStats, error)
	UpdateStaffVacationQuota(context.Context, domain.StaffVacationQuota) (domain.StaffVacationQuota, bool, domain.OperationStats, error)
	DeleteStaffVacationQuota(context.Context, int64) (domain.OperationStats, error)
	// UpsertStaffVacationQuota writes the entitlement of one staff member and
	// year, replacing the day counts of an existing row.
	UpsertStaffVacationQuota(context.Context, domain.StaffVacationQuota) (domain.OperationStats, error)
}

// StaffRecordStore is the persistence port over users.staff_master_data,
// users.staff_qualifications, users.staff_financial_data,
// users.staff_documents and users.staff_document_file_cleanup.
type StaffRecordStore interface {
	FindStaffMasterData(ctx context.Context, staffID int64) (domain.StaffMasterData, bool, domain.OperationStats, error)
	CreateStaffMasterData(context.Context, domain.StaffMasterData) (domain.StaffMasterData, domain.OperationStats, error)
	UpdateStaffMasterData(context.Context, domain.StaffMasterData) (domain.StaffMasterData, bool, domain.OperationStats, error)

	ListStaffQualifications(ctx context.Context, staffID int64) ([]domain.StaffQualification, domain.OperationStats, error)
	DeleteStaffQualifications(ctx context.Context, staffID int64) (domain.OperationStats, error)
	InsertStaffQualifications(context.Context, []domain.StaffQualification) ([]domain.StaffQualification, domain.OperationStats, error)

	FindStaffFinancialData(ctx context.Context, staffID int64) (domain.StaffFinancialData, bool, domain.OperationStats, error)
	CreateStaffFinancialData(context.Context, domain.StaffFinancialData) (domain.StaffFinancialData, domain.OperationStats, error)
	UpdateStaffFinancialData(context.Context, domain.StaffFinancialData) (domain.StaffFinancialData, bool, domain.OperationStats, error)

	CreateStaffDocument(context.Context, domain.StaffDocument) (domain.StaffDocument, domain.OperationStats, error)
	// FindStaffDocument loads one document by the staff/document pair.
	FindStaffDocument(ctx context.Context, staffID, documentID int64, includeDeleted bool) (domain.StaffDocument, bool, domain.OperationStats, error)
	ListStaffDocuments(context.Context, domain.StaffDocumentFilter) ([]domain.StaffDocument, domain.OperationStats, error)
	// SoftDeleteStaffDocument stamps deleted_at and deleted_by on a live row.
	SoftDeleteStaffDocument(ctx context.Context, id, deletedBy int64, at time.Time) (int64, domain.OperationStats, error)
	MarkStaffDocumentFileDeleted(ctx context.Context, id int64, at time.Time) (domain.OperationStats, error)

	// QueueStaffDocumentFileCleanup records the intent once per stored name.
	QueueStaffDocumentFileCleanup(context.Context, domain.StaffDocumentFileCleanup) (domain.OperationStats, error)
	// ListQueuedStaffDocumentFileCleanups returns and row-locks the eligible
	// intents; a positive staffID narrows them to one staff member.
	ListQueuedStaffDocumentFileCleanups(ctx context.Context, staffID int64, now time.Time) ([]domain.StaffDocumentFileCleanup, domain.OperationStats, error)
	CompleteStaffDocumentFileCleanup(ctx context.Context, id int64, at time.Time) (domain.OperationStats, error)
	CompleteStaffDocumentFileCleanupByFilename(ctx context.Context, filename string, at time.Time) (domain.OperationStats, error)
	ActivateStaffDocumentFileCleanup(ctx context.Context, filename string, at time.Time) (domain.OperationStats, error)
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
	// SetStaffShiftSickAbsence writes only the sick-absence association.
	SetStaffShiftSickAbsence(ctx context.Context, shiftID int64, absenceID *int64) (int64, domain.OperationStats, error)
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
	LockStaffShifts(ctx context.Context, staffID int64) error
}

// Clock supplies the calendar day new schedule versions start on and the
// instant live work-session windows are measured against.
type Clock interface {
	Today() string
	Now() time.Time
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
