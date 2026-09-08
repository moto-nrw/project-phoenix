package workforce

import (
	"context"
	"errors"
	"time"
)

// Work session statuses and channels.
const (
	WorkSessionStatusPresent    = "present"
	WorkSessionStatusHomeOffice = "home_office"

	WorkSessionSourceApp     = "app"
	WorkSessionSourceNFC     = "nfc"
	WorkSessionSourceUnknown = "unknown"

	// MaxOpenWorkSessionDuration is the live safety limit for a block that is
	// still open: past it a running block stops counting as work in progress
	// for the balance, the presence map and the running-session lookup.
	MaxOpenWorkSessionDuration = 12 * time.Hour

	// WorkSessionDateColumn is the one date column a retention cleanup may
	// address on work sessions.
	WorkSessionDateColumn = "date"
)

// Stundenkonto correction transaction types (#1420, #2132).
const (
	BalanceAdjustmentTypePayout   = "payout"
	BalanceAdjustmentTypeCompTime = "comp_time"
	BalanceAdjustmentTypeReset    = "reset"
	BalanceAdjustmentTypeOpening  = "opening"
)

var (
	ErrWorkSessionNotFound            = errors.New("work session not found")
	ErrWorkSessionBreakNotFound       = errors.New("work session break not found")
	ErrStaffBalanceAdjustmentNotFound = errors.New("staff balance adjustment not found")
	ErrStaffVacationOpeningNotFound   = errors.New("staff vacation opening not found")
	ErrStaffVacationQuotaNotFound     = errors.New("staff vacation quota not found")
	ErrInvalidWorkSession             = errors.New("invalid work session input")
	// ErrWorkSessionAlreadyOpen reports a second open block for the same
	// staff member and day, rejected by the database's unique index.
	ErrWorkSessionAlreadyOpen = errors.New("work session for the day is already open")
)

// InvalidWorkSessionError carries the caller-facing validation reason; it
// unwraps to ErrInvalidWorkSession so callers classify with errors.Is.
type InvalidWorkSessionError struct{ Reason string }

func (e *InvalidWorkSessionError) Error() string { return e.Reason }
func (e *InvalidWorkSessionError) Unwrap() error { return ErrInvalidWorkSession }

func invalidWorkSession(reason string) error { return &InvalidWorkSessionError{Reason: reason} }

// WorkSession is one work block of a staff member. Date is the calendar day
// the block is filed under, in DateLayout; the instants are timestamps.
type WorkSession struct {
	ID             int64      `json:"id"`
	TenantID       int64      `json:"tenant_id"`
	StaffID        int64      `json:"staff_id"`
	Date           string     `json:"date"`
	Status         string     `json:"status"`
	Source         string     `json:"source"`
	CheckInTime    time.Time  `json:"check_in_time"`
	CheckOutTime   *time.Time `json:"check_out_time,omitempty"`
	ReopenedAt     *time.Time `json:"-"`
	BreakMinutes   int        `json:"break_minutes"`
	Notes          string     `json:"notes,omitempty"`
	AutoCheckedOut bool       `json:"auto_checked_out"`
	CreatedBy      int64      `json:"created_by"`
	UpdatedBy      *int64     `json:"updated_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
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

type WorkSessionOrder struct {
	Field      WorkSessionOrderField
	Descending bool
}

// WorkSessionFilter narrows a listing; every set field is combined with AND.
// Open selects running (true) or closed (false) blocks; DateBefore selects
// blocks filed strictly before the day. An explicit empty IDs or StaffIDs set
// matches nobody.
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

// WorkSessionBreak is one break period inside a work block.
type WorkSessionBreak struct {
	ID              int64      `json:"id"`
	TenantID        int64      `json:"tenant_id"`
	SessionID       int64      `json:"session_id"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	DurationMinutes int        `json:"duration_minutes"`
	PlannedEndTime  *time.Time `json:"planned_end_time,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// IsActive reports whether the break has not ended yet.
func (b WorkSessionBreak) IsActive() bool { return b.EndedAt == nil }

// WorkSessionBreakFilter narrows a break listing; Active selects running
// (true) or ended (false) breaks.
type WorkSessionBreakFilter struct {
	SessionID  int64
	SessionIDs []int64
	Active     *bool
	Limit      int
	Offset     int
}

// StaffBalanceAdjustment is one Stundenkonto correction transaction (#1420).
// EffectiveDate is a calendar day in DateLayout; MinutesDelta is signed.
type StaffBalanceAdjustment struct {
	ID            int64     `json:"id"`
	TenantID      int64     `json:"tenant_id"`
	StaffID       int64     `json:"staff_id"`
	Type          string    `json:"type"`
	MinutesDelta  int       `json:"minutes_delta"`
	EffectiveDate string    `json:"effective_date"`
	Note          string    `json:"note"`
	DecidedBy     int64     `json:"decided_by"`
	DecidedAt     time.Time `json:"decided_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// StaffBalanceAdjustmentFilter narrows an adjustment listing; the effective
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

// StaffVacationOpening records the vacation days a staff member had already
// taken before the moto introduction (#2132), per calendar year.
type StaffVacationOpening struct {
	ID                   int64     `json:"id"`
	TenantID             int64     `json:"tenant_id"`
	StaffID              int64     `json:"staff_id"`
	Year                 int       `json:"year"`
	EffectiveDate        string    `json:"effective_date"`
	TakenBeforeDays      float64   `json:"taken_before_days"`
	EnteredRemainingDays float64   `json:"entered_remaining_days"`
	Note                 string    `json:"note"`
	DecidedBy            int64     `json:"decided_by"`
	DecidedAt            time.Time `json:"decided_at"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
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

// StaffVacationOrderField names a column a vacation listing may be ordered by.
type StaffVacationOrderField string

const (
	StaffVacationOrderID           StaffVacationOrderField = "id"
	StaffVacationOrderStaffID      StaffVacationOrderField = "staff_id"
	StaffVacationOrderYear         StaffVacationOrderField = "year"
	StaffVacationOrderEntitledDays StaffVacationOrderField = "entitled_days"
)

type StaffVacationOrder struct {
	Field      StaffVacationOrderField
	Descending bool
}

// StaffVacationFilter narrows an opening or quota listing; a zero Year
// selects every year.
type StaffVacationFilter struct {
	StaffID  int64
	StaffIDs []int64
	Year     int
	Order    []StaffVacationOrder
	Limit    int
	Offset   int
}

// WorkSessionQuery reads the work-session family of rows.
type WorkSessionQuery interface {
	FindWorkSession(context.Context, int64) (WorkSession, error)
	// OpenWorkSessionOn returns the running block of a staff member filed on
	// the calendar day.
	OpenWorkSessionOn(ctx context.Context, staffID int64, date string) (WorkSession, error)
	// TodayOpenWorkSession returns the running block filed on today.
	TodayOpenWorkSession(ctx context.Context, staffID int64) (WorkSession, error)
	// LatestOpenWorkSession returns the most recent block still running
	// inside the live window, whatever day it was filed on.
	LatestOpenWorkSession(ctx context.Context, staffID int64) (WorkSession, error)
	ListWorkSessions(context.Context, WorkSessionFilter) ([]WorkSession, error)
	// ListOverlappingWorkSessions returns the blocks whose [check-in,
	// check-out) interval intersects [from, to); a nil to is open-ended.
	ListOverlappingWorkSessions(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) ([]WorkSession, error)
	CountWorkSessions(context.Context, WorkSessionFilter) (int, error)
	// OldestWorkSessionDate returns the earliest stored day, optionally among
	// blocks filed before the given day; empty when there is none.
	OldestWorkSessionDate(ctx context.Context, column, before string) (string, error)
	// WorkPresenceMap maps staff to their work status as of now: the status
	// of a live open block, otherwise checked_out for a block filed today.
	WorkPresenceMap(context.Context) (map[int64]string, error)

	FindWorkSessionBreak(context.Context, int64) (WorkSessionBreak, error)
	ListWorkSessionBreaks(context.Context, WorkSessionBreakFilter) ([]WorkSessionBreak, error)
	// ExpiredWorkSessionBreaks returns running breaks whose planned end passed.
	ExpiredWorkSessionBreaks(ctx context.Context, before time.Time) ([]WorkSessionBreak, error)

	FindStaffBalanceAdjustment(context.Context, int64) (StaffBalanceAdjustment, error)
	ListStaffBalanceAdjustments(context.Context, StaffBalanceAdjustmentFilter) ([]StaffBalanceAdjustment, error)

	FindStaffVacationOpening(context.Context, int64) (StaffVacationOpening, error)
	ListStaffVacationOpenings(context.Context, StaffVacationFilter) ([]StaffVacationOpening, error)

	FindStaffVacationQuota(context.Context, int64) (StaffVacationQuota, error)
	ListStaffVacationQuotas(context.Context, StaffVacationFilter) ([]StaffVacationQuota, error)
}

// WorkSessionCommand writes the work-session family of rows. Every write runs
// on the caller's ambient tenant transaction or opens one.
type WorkSessionCommand interface {
	// LockStaffBalanceWrites serializes every writer that changes a staff
	// member's Stundenkonto inputs.
	LockStaffBalanceWrites(ctx context.Context, staffID int64) error
	// LockOpenWorkSession returns and row-locks a still running block.
	LockOpenWorkSession(context.Context, int64) (WorkSession, error)
	// LockOpenWorkSessionOn is OpenWorkSessionOn with a row lock.
	LockOpenWorkSessionOn(ctx context.Context, staffID int64, date string) (WorkSession, error)
	CreateWorkSession(context.Context, WorkSession) (WorkSession, error)
	UpdateWorkSession(context.Context, WorkSession) (WorkSession, error)
	DeleteWorkSession(context.Context, int64) error
	// DeleteWorkSessionsOlderThan removes blocks filed strictly before the
	// day and reports how many.
	DeleteWorkSessionsOlderThan(ctx context.Context, column, cutoff string) (int64, error)
	// SetWorkSessionBreakMinutes rewrites the cached break total of a block.
	SetWorkSessionBreakMinutes(ctx context.Context, id int64, minutes int) (int64, error)
	// CloseWorkSession stamps the check-out on a still open block; the
	// boolean reports whether a row was actually closed.
	CloseWorkSession(ctx context.Context, id int64, checkOut time.Time, autoCheckedOut bool) (bool, error)

	CreateWorkSessionBreak(context.Context, WorkSessionBreak) (WorkSessionBreak, error)
	UpdateWorkSessionBreak(context.Context, WorkSessionBreak) (WorkSessionBreak, error)
	DeleteWorkSessionBreak(context.Context, int64) error
	// EndWorkSessionBreak stamps the end on a still running break only and
	// reports how many rows changed.
	EndWorkSessionBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) (int64, error)
	// SetWorkSessionBreakDuration rewrites the length and end of a break.
	SetWorkSessionBreakDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) (int64, error)

	CreateStaffBalanceAdjustment(context.Context, StaffBalanceAdjustment) (StaffBalanceAdjustment, error)
	UpdateStaffBalanceAdjustment(context.Context, StaffBalanceAdjustment) (StaffBalanceAdjustment, error)
	DeleteStaffBalanceAdjustment(context.Context, int64) error

	CreateStaffVacationOpening(context.Context, StaffVacationOpening) (StaffVacationOpening, error)
	UpdateStaffVacationOpening(context.Context, StaffVacationOpening) (StaffVacationOpening, error)
	DeleteStaffVacationOpening(context.Context, int64) error

	CreateStaffVacationQuota(context.Context, StaffVacationQuota) (StaffVacationQuota, error)
	UpdateStaffVacationQuota(context.Context, StaffVacationQuota) (StaffVacationQuota, error)
	DeleteStaffVacationQuota(context.Context, int64) error
	// UpsertStaffVacationQuota writes the entitlement of one staff member and
	// year, replacing the day counts of an existing row.
	UpsertStaffVacationQuota(context.Context, StaffVacationQuota) error
}

type workSessionEngine interface {
	FindWorkSession(context.Context, int64) (WorkSession, error)
	LockOpenWorkSession(context.Context, int64) (WorkSession, error)
	OpenWorkSessionOn(ctx context.Context, staffID int64, date string, lock bool) (WorkSession, error)
	TodayOpenWorkSession(ctx context.Context, staffID int64) (WorkSession, error)
	LatestOpenWorkSession(ctx context.Context, staffID int64) (WorkSession, error)
	ListWorkSessions(context.Context, WorkSessionFilter) ([]WorkSession, error)
	ListOverlappingWorkSessions(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) ([]WorkSession, error)
	CountWorkSessions(context.Context, WorkSessionFilter) (int, error)
	OldestWorkSessionDate(ctx context.Context, column, before string) (string, error)
	DeleteWorkSessionsOlderThan(ctx context.Context, column, cutoff string) (int64, error)
	WorkPresenceMap(context.Context) (map[int64]string, error)
	CreateWorkSession(context.Context, WorkSession) (WorkSession, error)
	UpdateWorkSession(context.Context, WorkSession) (WorkSession, error)
	DeleteWorkSession(context.Context, int64) error
	SetWorkSessionBreakMinutes(ctx context.Context, id int64, minutes int) (int64, error)
	CloseWorkSession(ctx context.Context, id int64, checkOut time.Time, autoCheckedOut bool) (bool, error)
	LockStaffBalanceWrites(ctx context.Context, staffID int64) error

	FindWorkSessionBreak(context.Context, int64) (WorkSessionBreak, error)
	ListWorkSessionBreaks(context.Context, WorkSessionBreakFilter) ([]WorkSessionBreak, error)
	ExpiredWorkSessionBreaks(ctx context.Context, before time.Time) ([]WorkSessionBreak, error)
	CreateWorkSessionBreak(context.Context, WorkSessionBreak) (WorkSessionBreak, error)
	UpdateWorkSessionBreak(context.Context, WorkSessionBreak) (WorkSessionBreak, error)
	DeleteWorkSessionBreak(context.Context, int64) error
	EndWorkSessionBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) (int64, error)
	SetWorkSessionBreakDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) (int64, error)

	FindStaffBalanceAdjustment(context.Context, int64) (StaffBalanceAdjustment, error)
	ListStaffBalanceAdjustments(context.Context, StaffBalanceAdjustmentFilter) ([]StaffBalanceAdjustment, error)
	CreateStaffBalanceAdjustment(context.Context, StaffBalanceAdjustment) (StaffBalanceAdjustment, error)
	UpdateStaffBalanceAdjustment(context.Context, StaffBalanceAdjustment) (StaffBalanceAdjustment, error)
	DeleteStaffBalanceAdjustment(context.Context, int64) error

	FindStaffVacationOpening(context.Context, int64) (StaffVacationOpening, error)
	ListStaffVacationOpenings(context.Context, StaffVacationFilter) ([]StaffVacationOpening, error)
	CreateStaffVacationOpening(context.Context, StaffVacationOpening) (StaffVacationOpening, error)
	UpdateStaffVacationOpening(context.Context, StaffVacationOpening) (StaffVacationOpening, error)
	DeleteStaffVacationOpening(context.Context, int64) error

	FindStaffVacationQuota(context.Context, int64) (StaffVacationQuota, error)
	ListStaffVacationQuotas(context.Context, StaffVacationFilter) ([]StaffVacationQuota, error)
	CreateStaffVacationQuota(context.Context, StaffVacationQuota) (StaffVacationQuota, error)
	UpdateStaffVacationQuota(context.Context, StaffVacationQuota) (StaffVacationQuota, error)
	DeleteStaffVacationQuota(context.Context, int64) error
	UpsertStaffVacationQuota(context.Context, StaffVacationQuota) error
}

// --- work sessions ---

func (m *Module) FindWorkSession(ctx context.Context, id int64) (WorkSession, error) {
	if id <= 0 {
		return WorkSession{}, invalidWorkSession("work session ID is required")
	}
	return m.engine.FindWorkSession(ctx, id)
}

func (m *Module) LockOpenWorkSession(ctx context.Context, id int64) (WorkSession, error) {
	if id <= 0 {
		return WorkSession{}, invalidWorkSession("work session ID is required")
	}
	return m.engine.LockOpenWorkSession(ctx, id)
}

func (m *Module) OpenWorkSessionOn(ctx context.Context, staffID int64, date string) (WorkSession, error) {
	if staffID <= 0 {
		return WorkSession{}, invalidWorkSession("staff ID is required")
	}
	return m.engine.OpenWorkSessionOn(ctx, staffID, date, false)
}

func (m *Module) LockOpenWorkSessionOn(ctx context.Context, staffID int64, date string) (WorkSession, error) {
	if staffID <= 0 {
		return WorkSession{}, invalidWorkSession("staff ID is required")
	}
	return m.engine.OpenWorkSessionOn(ctx, staffID, date, true)
}

func (m *Module) TodayOpenWorkSession(ctx context.Context, staffID int64) (WorkSession, error) {
	if staffID <= 0 {
		return WorkSession{}, invalidWorkSession("staff ID is required")
	}
	return m.engine.TodayOpenWorkSession(ctx, staffID)
}

func (m *Module) LatestOpenWorkSession(ctx context.Context, staffID int64) (WorkSession, error) {
	if staffID <= 0 {
		return WorkSession{}, invalidWorkSession("staff ID is required")
	}
	return m.engine.LatestOpenWorkSession(ctx, staffID)
}

func (m *Module) ListWorkSessions(ctx context.Context, filter WorkSessionFilter) ([]WorkSession, error) {
	return m.engine.ListWorkSessions(ctx, filter)
}

func (m *Module) ListOverlappingWorkSessions(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) ([]WorkSession, error) {
	if len(staffIDs) == 0 {
		return []WorkSession{}, nil
	}
	return m.engine.ListOverlappingWorkSessions(ctx, staffIDs, from, to)
}

func (m *Module) CountWorkSessions(ctx context.Context, filter WorkSessionFilter) (int, error) {
	return m.engine.CountWorkSessions(ctx, filter)
}

func (m *Module) OldestWorkSessionDate(ctx context.Context, column, before string) (string, error) {
	return m.engine.OldestWorkSessionDate(ctx, column, before)
}

func (m *Module) DeleteWorkSessionsOlderThan(ctx context.Context, column, cutoff string) (int64, error) {
	return m.engine.DeleteWorkSessionsOlderThan(ctx, column, cutoff)
}

func (m *Module) WorkPresenceMap(ctx context.Context) (map[int64]string, error) {
	return m.engine.WorkPresenceMap(ctx)
}

func (m *Module) CreateWorkSession(ctx context.Context, value WorkSession) (WorkSession, error) {
	return m.engine.CreateWorkSession(ctx, value)
}

func (m *Module) UpdateWorkSession(ctx context.Context, value WorkSession) (WorkSession, error) {
	if value.ID <= 0 {
		return WorkSession{}, invalidWorkSession("work session ID is required")
	}
	return m.engine.UpdateWorkSession(ctx, value)
}

func (m *Module) DeleteWorkSession(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidWorkSession("work session ID is required")
	}
	return m.engine.DeleteWorkSession(ctx, id)
}

func (m *Module) SetWorkSessionBreakMinutes(ctx context.Context, id int64, minutes int) (int64, error) {
	if id <= 0 {
		return 0, invalidWorkSession("work session ID is required")
	}
	return m.engine.SetWorkSessionBreakMinutes(ctx, id, minutes)
}

func (m *Module) CloseWorkSession(ctx context.Context, id int64, checkOut time.Time, autoCheckedOut bool) (bool, error) {
	if id <= 0 {
		return false, invalidWorkSession("work session ID is required")
	}
	return m.engine.CloseWorkSession(ctx, id, checkOut, autoCheckedOut)
}

func (m *Module) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return invalidWorkSession("staff ID is required")
	}
	return m.engine.LockStaffBalanceWrites(ctx, staffID)
}

// --- breaks ---

func (m *Module) FindWorkSessionBreak(ctx context.Context, id int64) (WorkSessionBreak, error) {
	if id <= 0 {
		return WorkSessionBreak{}, invalidWorkSession("work session break ID is required")
	}
	return m.engine.FindWorkSessionBreak(ctx, id)
}

func (m *Module) ListWorkSessionBreaks(ctx context.Context, filter WorkSessionBreakFilter) ([]WorkSessionBreak, error) {
	return m.engine.ListWorkSessionBreaks(ctx, filter)
}

func (m *Module) ExpiredWorkSessionBreaks(ctx context.Context, before time.Time) ([]WorkSessionBreak, error) {
	return m.engine.ExpiredWorkSessionBreaks(ctx, before)
}

func (m *Module) CreateWorkSessionBreak(ctx context.Context, value WorkSessionBreak) (WorkSessionBreak, error) {
	return m.engine.CreateWorkSessionBreak(ctx, value)
}

func (m *Module) UpdateWorkSessionBreak(ctx context.Context, value WorkSessionBreak) (WorkSessionBreak, error) {
	if value.ID <= 0 {
		return WorkSessionBreak{}, invalidWorkSession("work session break ID is required")
	}
	return m.engine.UpdateWorkSessionBreak(ctx, value)
}

func (m *Module) DeleteWorkSessionBreak(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidWorkSession("work session break ID is required")
	}
	return m.engine.DeleteWorkSessionBreak(ctx, id)
}

func (m *Module) EndWorkSessionBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) (int64, error) {
	if id <= 0 {
		return 0, invalidWorkSession("work session break ID is required")
	}
	return m.engine.EndWorkSessionBreak(ctx, id, endedAt, durationMinutes)
}

func (m *Module) SetWorkSessionBreakDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) (int64, error) {
	if id <= 0 {
		return 0, invalidWorkSession("work session break ID is required")
	}
	return m.engine.SetWorkSessionBreakDuration(ctx, id, durationMinutes, endedAt)
}

// --- staff balance adjustments ---

func (m *Module) FindStaffBalanceAdjustment(ctx context.Context, id int64) (StaffBalanceAdjustment, error) {
	if id <= 0 {
		return StaffBalanceAdjustment{}, invalidWorkSession("staff balance adjustment ID is required")
	}
	return m.engine.FindStaffBalanceAdjustment(ctx, id)
}

func (m *Module) ListStaffBalanceAdjustments(ctx context.Context, filter StaffBalanceAdjustmentFilter) ([]StaffBalanceAdjustment, error) {
	return m.engine.ListStaffBalanceAdjustments(ctx, filter)
}

func (m *Module) CreateStaffBalanceAdjustment(ctx context.Context, value StaffBalanceAdjustment) (StaffBalanceAdjustment, error) {
	return m.engine.CreateStaffBalanceAdjustment(ctx, value)
}

func (m *Module) UpdateStaffBalanceAdjustment(ctx context.Context, value StaffBalanceAdjustment) (StaffBalanceAdjustment, error) {
	if value.ID <= 0 {
		return StaffBalanceAdjustment{}, invalidWorkSession("staff balance adjustment ID is required")
	}
	return m.engine.UpdateStaffBalanceAdjustment(ctx, value)
}

func (m *Module) DeleteStaffBalanceAdjustment(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidWorkSession("staff balance adjustment ID is required")
	}
	return m.engine.DeleteStaffBalanceAdjustment(ctx, id)
}

// --- staff vacation openings ---

func (m *Module) FindStaffVacationOpening(ctx context.Context, id int64) (StaffVacationOpening, error) {
	if id <= 0 {
		return StaffVacationOpening{}, invalidWorkSession("staff vacation opening ID is required")
	}
	return m.engine.FindStaffVacationOpening(ctx, id)
}

func (m *Module) ListStaffVacationOpenings(ctx context.Context, filter StaffVacationFilter) ([]StaffVacationOpening, error) {
	return m.engine.ListStaffVacationOpenings(ctx, filter)
}

func (m *Module) CreateStaffVacationOpening(ctx context.Context, value StaffVacationOpening) (StaffVacationOpening, error) {
	return m.engine.CreateStaffVacationOpening(ctx, value)
}

func (m *Module) UpdateStaffVacationOpening(ctx context.Context, value StaffVacationOpening) (StaffVacationOpening, error) {
	if value.ID <= 0 {
		return StaffVacationOpening{}, invalidWorkSession("staff vacation opening ID is required")
	}
	return m.engine.UpdateStaffVacationOpening(ctx, value)
}

func (m *Module) DeleteStaffVacationOpening(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidWorkSession("staff vacation opening ID is required")
	}
	return m.engine.DeleteStaffVacationOpening(ctx, id)
}

// --- staff vacation quota ---

func (m *Module) FindStaffVacationQuota(ctx context.Context, id int64) (StaffVacationQuota, error) {
	if id <= 0 {
		return StaffVacationQuota{}, invalidWorkSession("staff vacation quota ID is required")
	}
	return m.engine.FindStaffVacationQuota(ctx, id)
}

func (m *Module) ListStaffVacationQuotas(ctx context.Context, filter StaffVacationFilter) ([]StaffVacationQuota, error) {
	return m.engine.ListStaffVacationQuotas(ctx, filter)
}

func (m *Module) CreateStaffVacationQuota(ctx context.Context, value StaffVacationQuota) (StaffVacationQuota, error) {
	return m.engine.CreateStaffVacationQuota(ctx, value)
}

func (m *Module) UpdateStaffVacationQuota(ctx context.Context, value StaffVacationQuota) (StaffVacationQuota, error) {
	if value.ID <= 0 {
		return StaffVacationQuota{}, invalidWorkSession("staff vacation quota ID is required")
	}
	return m.engine.UpdateStaffVacationQuota(ctx, value)
}

func (m *Module) DeleteStaffVacationQuota(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidWorkSession("staff vacation quota ID is required")
	}
	return m.engine.DeleteStaffVacationQuota(ctx, id)
}

func (m *Module) UpsertStaffVacationQuota(ctx context.Context, value StaffVacationQuota) error {
	return m.engine.UpsertStaffVacationQuota(ctx, value)
}
