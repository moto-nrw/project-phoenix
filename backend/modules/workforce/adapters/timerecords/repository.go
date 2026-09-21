package timerecords

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// WorkSessionRepository defines operations for managing staff work sessions
type WorkSessionRepository interface {
	base.Repository[*WorkSession]

	// LockStaffBalanceWrites serializes all work-session and break mutations
	// with absence and adjustment mutations for the same staff member.
	LockStaffBalanceWrites(ctx context.Context, staffID int64) error
	// GetCurrentByStaffID returns the active (not checked out) session for a staff member
	GetCurrentByStaffID(ctx context.Context, staffID int64) (*WorkSession, error)

	// GetLatestOpenByStaffID returns the most recent not-checked-out session of
	// a staff member across all days. Callers that must not lose sight of a
	// session opened before midnight use this instead of GetCurrentByStaffID.
	GetLatestOpenByStaffID(ctx context.Context, staffID int64) (*WorkSession, error)

	// GetOpenByStaffAndDate returns the not-checked-out session of a staff
	// member on an explicit calendar day, for callers that must not re-derive
	// "today" while the request is running.
	GetOpenByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*WorkSession, error)

	// GetOpenByStaffAndDateForUpdate is GetOpenByStaffAndDate with a row lock.
	GetOpenByStaffAndDateForUpdate(ctx context.Context, staffID int64, date timezone.Date) (*WorkSession, error)

	// LockOpenByIDForUpdate returns and locks an open session row by ID.
	LockOpenByIDForUpdate(ctx context.Context, id int64) (*WorkSession, error)

	// ListOverlappingByStaffID returns the blocks of a staff member whose
	// [check-in, check-out) interval intersects [from, to). A nil "to" means
	// the interval is open-ended. Timestamp-based on purpose: a block can
	// reach past the day it is filed on (#2402).
	ListOverlappingByStaffID(ctx context.Context, staffID int64, from time.Time, to *time.Time) ([]*WorkSession, error)
	// ListOverlappingByStaffIDs is the batched counterpart used by the
	// cross-staff Stundenkonto overview. It keeps interval readers separate
	// from history and export readers, whose contract is the stored date.
	ListOverlappingByStaffIDs(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) (map[int64][]*WorkSession, error)

	// GetHistoryByStaffID returns work sessions for a staff member in a date range
	GetHistoryByStaffID(ctx context.Context, staffID int64, from, to timezone.Date) ([]*WorkSession, error)

	// GetHistoryByStaffIDs is GetHistoryByStaffID batched over many staff
	// members, keyed by staff ID. Its contract follows the stored session date,
	// matching history and export date ranges.
	GetHistoryByStaffIDs(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*WorkSession, error)

	// GetOpenSessions returns all sessions without check-out before a given date
	GetOpenSessions(ctx context.Context, beforeDate timezone.Date) ([]*WorkSession, error)

	// GetTodayPresenceMap returns a map of staff IDs to their work status for today
	GetTodayPresenceMap(ctx context.Context) (map[int64]string, error)

	// CloseSession sets the check-out time and auto_checked_out flag.
	// The boolean reports whether an open row was actually closed.
	CloseSession(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) (bool, error)

	// UpdateBreakMinutes sets the break_minutes cache field on a session
	UpdateBreakMinutes(ctx context.Context, id int64, breakMinutes int) error

	// Generic query helpers promoted from the embedded base repository.
	// Used by the time-tracking retention cleanup.
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)
	OldestBefore(ctx context.Context, dateColumn string, cutoff *timezone.Date) (*timezone.Date, error)
	DeleteOlderThan(ctx context.Context, dateColumn string, cutoff timezone.Date) (int64, error)
}

// StaffAbsenceRepository defines operations for managing staff absences
type StaffAbsenceRepository interface {
	base.Repository[*StaffAbsence]

	// LockStaffAbsenceWrites serializes absence lifecycle writes for one staff
	// member inside the ambient tenant transaction. It also takes the shared
	// balance lock before the absence-specific lock. Callers must acquire it
	// before any overlap read-check-write sequence.
	LockStaffAbsenceWrites(ctx context.Context, staffID int64) error

	// GetByStaffAndDateRange returns absences for a staff member overlapping the given date range
	GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*StaffAbsence, error)

	// GetByStaffIDsAndDateRange is GetByStaffAndDateRange batched over many
	// staff members, keyed by staff ID (same overlap semantics).
	GetByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*StaffAbsence, error)

	// GetByStaffAndDate returns an absence for a staff member on a specific date, or nil
	GetByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*StaffAbsence, error)

	// GetAbsenceMapForDate returns a map of staff IDs to their absence type for the given date.
	// Priority order when multiple absences exist:
	// sick > training > vacation > comp_time > other.
	GetAbsenceMapForDate(ctx context.Context, date timezone.Date) (map[int64]string, error)

	// GetAbsenceTypeIDMapForDate returns staff ID -> school-defined
	// Abwesenheitsart ID for the same winning absence GetAbsenceMapForDate
	// picks (#2403). Only staff whose winner carries one appear.
	GetAbsenceTypeIDMapForDate(ctx context.Context, date timezone.Date) (map[int64]int64, error)

	// ListByStatuses returns all absences whose status is in the given set,
	// ordered by requested_at (used for the /staff inbox: requested + question)
	ListByStatuses(ctx context.Context, statuses []string) ([]*StaffAbsence, error)

	// ListRequests returns absence requests together with the names the
	// Anfragen module shows (#2433): the person the absence belongs to and,
	// once decided, the deciding person.
	ListRequests(ctx context.Context, filter AbsenceRequestFilter) ([]*AbsenceRequestRow, error)

	// Generic query helpers promoted from the embedded base repository.
	// Used by the time-tracking retention cleanup.
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)
	OldestBefore(ctx context.Context, dateColumn string, cutoff *timezone.Date) (*timezone.Date, error)
	DeleteOlderThan(ctx context.Context, dateColumn string, cutoff timezone.Date) (int64, error)
}

type StaffAbsenceAuditRepository interface {
	Create(ctx context.Context, audit *StaffAbsenceAudit) error
}

type StaffAbsenceTypeAllowanceRepository interface {
	base.Repository[*StaffAbsenceTypeAllowance]
}

type StaffAbsenceTypeAllowanceChangeRepository interface {
	base.Repository[*StaffAbsenceTypeAllowanceChange]
}

// StaffBalanceAdjustmentRepository defines operations for Stundenkonto
// correction transactions (#1420).
type StaffBalanceAdjustmentRepository interface {
	base.Repository[*StaffBalanceAdjustment]

	// LockStaffBalanceWrites serializes balance-adjustment writes for one
	// staff member inside the ambient tenant transaction. Every adjustment
	// mutation acquires it before its first read or write.
	LockStaffBalanceWrites(ctx context.Context, staffID int64) error

	// GetByStaffAndDateRange returns adjustments whose effective_date lies in
	// [from, to], ordered by effective_date.
	GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*StaffBalanceAdjustment, error)

	// GetByStaffIDsAndDateRange is GetByStaffAndDateRange batched over many
	// staff members, keyed by staff ID.
	GetByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*StaffBalanceAdjustment, error)
}

// StaffVacationQuotaRepository defines operations for managing per-staff yearly entitlement
type StaffVacationQuotaRepository interface {
	base.Repository[*StaffVacationQuota]

	// GetByStaffAndYear returns the quota row for a specific staff/year, or nil
	GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*StaffVacationQuota, error)

	// GetByStaffIDsAndYear is GetByStaffAndYear batched over many staff
	// members, keyed by staff ID. Missing rows are absent from the map.
	GetByStaffIDsAndYear(ctx context.Context, staffIDs []int64, year int) (map[int64]*StaffVacationQuota, error)

	// Upsert creates or updates the quota for a staff/year combination
	Upsert(ctx context.Context, quota *StaffVacationQuota) error
}

// StaffVacationOpeningRepository manages per-staff vacation takeover rows
// (#2132). One row per staff and year, append-only at the service level —
// corrections are delete + re-create with a deletion tombstone.
type StaffVacationOpeningRepository interface {
	base.Repository[*StaffVacationOpening]

	// GetByStaffAndYear returns the opening row for a staff/year, or nil.
	GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*StaffVacationOpening, error)

	// GetByStaffIDsAndYear is GetByStaffAndYear batched over many staff
	// members, keyed by staff ID. Missing rows are absent from the map.
	GetByStaffIDsAndYear(ctx context.Context, staffIDs []int64, year int) (map[int64]*StaffVacationOpening, error)
}

// WorkSessionBreakRepository defines operations for managing work session breaks
type WorkSessionBreakRepository interface {
	base.Repository[*WorkSessionBreak]

	// GetBySessionID returns all breaks for a given session ordered by started_at
	GetBySessionID(ctx context.Context, sessionID int64) ([]*WorkSessionBreak, error)
	// GetBySessionIDs returns all breaks for multiple sessions, keyed by session
	// ID. It is used by the time-tracking overview to preserve the same
	// interval arithmetic as the single-staff Monatskarte without N+1 queries.
	GetBySessionIDs(ctx context.Context, sessionIDs []int64) (map[int64][]*WorkSessionBreak, error)

	// GetActiveBySessionID returns the currently active (no ended_at) break for a session, or nil
	GetActiveBySessionID(ctx context.Context, sessionID int64) (*WorkSessionBreak, error)

	// EndBreak sets ended_at and duration_minutes on a break
	EndBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) error

	// UpdateDuration updates the duration and ended_at of a completed break
	UpdateDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) error

	// GetExpiredBreaks returns all active breaks with planned_end_time <= before
	// Used by the scheduler to auto-end breaks
	GetExpiredBreaks(ctx context.Context, before time.Time) ([]*WorkSessionBreak, error)
}
