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

// OnDutyStaffIDs returns the staff members of a presence map (staff ID →
// status, as GetTodayPresenceMap answers) who are working right now. It
// matches the two real statuses positively: the map also carries a synthetic
// "checked out" value, and an unknown future status must not silently count
// as working.
func OnDutyStaffIDs(presence map[int64]string) map[int64]struct{} {
	onDuty := make(map[int64]struct{}, len(presence))
	for staffID, status := range presence {
		if status == WorkSessionStatusPresent || status == WorkSessionStatusHomeOffice {
			onDuty[staffID] = struct{}{}
		}
	}
	return onDuty
}

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
	// ChangeReason and ChangedBy are write-only. With ChangedBy set, the
	// upsert records the change with its reason in the same transaction.
	ChangeReason string
	ChangedBy    int64
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

// WorkSessionQuery reads the work-session family of rows: the blocks
// themselves here, their breaks, the Stundenkonto corrections and the
// vacation rows through the embedded readers.
type WorkSessionQuery interface {
	WorkSessionBreakQuery
	StaffBalanceAdjustmentQuery
	StaffVacationQuery

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
}

// WorkSessionBreakQuery reads the break periods inside work blocks.
type WorkSessionBreakQuery interface {
	FindWorkSessionBreak(context.Context, int64) (WorkSessionBreak, error)
	ListWorkSessionBreaks(context.Context, WorkSessionBreakFilter) ([]WorkSessionBreak, error)
	// ExpiredWorkSessionBreaks returns running breaks whose planned end passed.
	ExpiredWorkSessionBreaks(ctx context.Context, before time.Time) ([]WorkSessionBreak, error)
}

// StaffBalanceAdjustmentQuery reads the Stundenkonto correction transactions.
type StaffBalanceAdjustmentQuery interface {
	FindStaffBalanceAdjustment(context.Context, int64) (StaffBalanceAdjustment, error)
	ListStaffBalanceAdjustments(context.Context, StaffBalanceAdjustmentFilter) ([]StaffBalanceAdjustment, error)
}

// StaffVacationQuery reads the vacation takeovers and the yearly
// entitlements.
type StaffVacationQuery interface {
	FindStaffVacationOpening(context.Context, int64) (StaffVacationOpening, error)
	ListStaffVacationOpenings(context.Context, StaffVacationFilter) ([]StaffVacationOpening, error)

	FindStaffVacationQuota(context.Context, int64) (StaffVacationQuota, error)
	ListStaffVacationQuotas(context.Context, StaffVacationFilter) ([]StaffVacationQuota, error)
}

// WorkSessionCommand writes the work-session family of rows: the blocks
// themselves here, their breaks, the Stundenkonto corrections and the
// vacation rows through the embedded writers. Every write runs on the
// caller's ambient tenant transaction or opens one.
type WorkSessionCommand interface {
	WorkSessionBreakCommand
	StaffBalanceAdjustmentCommand
	StaffVacationCommand

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
}

// WorkSessionBreakCommand writes the break periods inside work blocks.
type WorkSessionBreakCommand interface {
	CreateWorkSessionBreak(context.Context, WorkSessionBreak) (WorkSessionBreak, error)
	UpdateWorkSessionBreak(context.Context, WorkSessionBreak) (WorkSessionBreak, error)
	DeleteWorkSessionBreak(context.Context, int64) error
	// EndWorkSessionBreak stamps the end on a still running break only and
	// reports how many rows changed.
	EndWorkSessionBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) (int64, error)
	// SetWorkSessionBreakDuration rewrites the length and end of a break.
	SetWorkSessionBreakDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) (int64, error)
}

// StaffBalanceAdjustmentCommand writes the Stundenkonto correction
// transactions.
type StaffBalanceAdjustmentCommand interface {
	CreateStaffBalanceAdjustment(context.Context, StaffBalanceAdjustment) (StaffBalanceAdjustment, error)
	UpdateStaffBalanceAdjustment(context.Context, StaffBalanceAdjustment) (StaffBalanceAdjustment, error)
	DeleteStaffBalanceAdjustment(context.Context, int64) error
}

// StaffVacationCommand writes the vacation takeovers and the yearly
// entitlements.
type StaffVacationCommand interface {
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

// --- work sessions ---

// --- breaks ---

// --- staff balance adjustments ---

// --- staff vacation openings ---

// --- staff vacation quota ---
