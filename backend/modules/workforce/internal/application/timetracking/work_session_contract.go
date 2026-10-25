package timetracking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BreakDurationUpdate represents an update to a single break's duration
type BreakDurationUpdate struct {
	ID              int64 `json:"id"`
	DurationMinutes int   `json:"duration_minutes"`
}

// SessionUpdateRequest defines the structure for updating a work session
type SessionUpdateRequest struct {
	Date         *time.Time            `json:"date"`
	CheckInTime  *time.Time            `json:"check_in_time"`
	CheckOutTime *time.Time            `json:"check_out_time"`
	BreakMinutes *int                  `json:"break_minutes"`
	Status       *string               `json:"status"`
	Notes        *string               `json:"notes"`
	Breaks       []BreakDurationUpdate `json:"breaks"`
}

// AdminCreateSessionRequest is the body for the admin nachtragen flow:
// POST /api/staff/{id}/time-tracking/sessions. The admin sets the wall-
// clock times directly, no Anwesenheit, no kiosk involvement. Status and
// Notes are mandatory; everything else is optional with sensible defaults
// (break_minutes defaults to 0).
//
// Date stays a time.Time on the wire (the frontend sends RFC3339 UTC
// midnight); the service converts it to the Berlin calendar day via
// timezone.DateFromTime before it touches the DATE column.
type AdminCreateSessionRequest struct {
	Date         time.Time `json:"date"`
	CheckInTime  time.Time `json:"check_in_time"`
	CheckOutTime time.Time `json:"check_out_time"`
	BreakMinutes int       `json:"break_minutes"`
	Status       string    `json:"status"`
	Notes        string    `json:"notes"`
}

// SessionResponse wraps a work session with calculated fields
type SessionResponse struct {
	*WorkSession
	// BreakMinutes SHADOWS WorkSession.BreakMinutes on the wire — the outer
	// field is at the shallower depth, so encoding/json emits this one for
	// "break_minutes". That is deliberate: the model field caches ENDED breaks
	// only, while NetMinutes below already deducts a RUNNING one, so serializing
	// the raw cache would print "Pause 0:00" next to an Ist that has visibly
	// stopped growing. Both numbers come from the same as-of-now pair
	// (totalBreakMinutes / netMinutesWithBreaks), which keeps
	// gross = net + break true for the reader (#1842).
	BreakMinutes     int                 `json:"break_minutes"`
	NetMinutes       int                 `json:"net_minutes"`
	IsOvertime       bool                `json:"is_overtime"`
	IsBreakCompliant bool                `json:"is_break_compliant"`
	Breaks           []*WorkSessionBreak `json:"breaks"`
	EditCount        int                 `json:"edit_count"`
	AuditCount       int                 `json:"audit_count"`
}

// MarshalJSON emits the session id as a decimal STRING instead of the model's
// int64 number. A day can carry several blocks (#2402) and the frontend picks
// the block to edit by id, so the id has to survive the wire intact:
// JSON.parse() turns a number into a float64-backed JS number and rounds
// anything past 2^53 BEFORE any .toString() in the client can run. Quoting it
// here matches the project-wide int64→string convention (root CLAUDE.md,
// Type Mapping) and keeps the comparison exact for any id PostgreSQL can hand
// out. Marshaling (rather than a field set at each construction site) is what
// makes a future third construction site impossible to get wrong.
func (sr SessionResponse) MarshalJSON() ([]byte, error) {
	// The alias drops the methods, so json does not recurse back in here.
	type alias SessionResponse
	if sr.WorkSession == nil {
		// Nothing to quote — json omits the fields of a nil embedded pointer,
		// which is exactly what an unwrapped response serialized to before.
		return json.Marshal(alias(sr))
	}
	return json.Marshal(struct {
		alias
		// Shallower than the embedded WorkSession.ID, so this wins the "id" tag.
		ID        string  `json:"id"`
		TenantID  string  `json:"tenant_id"`
		StaffID   string  `json:"staff_id"`
		CreatedBy string  `json:"created_by"`
		UpdatedBy *string `json:"updated_by,omitempty"`
	}{
		alias:     alias(sr),
		ID:        strconv.FormatInt(sr.ID, 10),
		TenantID:  strconv.FormatInt(sr.TenantID, 10),
		StaffID:   strconv.FormatInt(sr.StaffID, 10),
		CreatedBy: strconv.FormatInt(sr.CreatedBy, 10),
		UpdatedBy: optionalIDString(sr.UpdatedBy),
	})
}

func optionalIDString(id *int64) *string {
	if id == nil {
		return nil
	}
	value := strconv.FormatInt(*id, 10)
	return &value
}

// WeeklySummary aggregates work session data per ISO week
type WeeklySummary struct {
	WeekNumber      int  `json:"week_number"`
	Year            int  `json:"year"`
	TotalNetMinutes int  `json:"total_net_minutes"`
	TargetMinutes   *int `json:"target_minutes,omitempty"`
	DeltaMinutes    *int `json:"delta_minutes,omitempty"`
	SessionCount    int  `json:"session_count"`
	IsOverWeeklyMax bool `json:"is_over_weekly_max"`
}

type summaryWeekKey struct {
	Year int
	Week int
}

// HistoryResponse wraps session history with weekly aggregation
type HistoryResponse struct {
	Sessions        []*SessionResponse `json:"sessions"`
	WeeklySummaries []WeeklySummary    `json:"weekly_summaries"`
	// LaborDays is the ArbZG view per check-in day inside the range (#4804).
	LaborDays []WorkDayLaborHint `json:"labor_days"`
}

// WorkSessionEditView decorates an audit row with the editor's display name
// and a "selbst geändert" flag. We compute IsSelfEdit on the server because
// audit.work_session_edits.edited_by stores a staff_id, not a role. The
// frontend would otherwise need to fetch the session separately to learn
// whether edited_by == session.staff_id.
type WorkSessionEditView struct {
	*WorkSessionEdit
	EditorName string `json:"editor_name"`
	IsSelfEdit bool   `json:"is_self_edit"`
}

// PlannedStartNotReachedError is returned by CheckIn when the optional
// planned-start enforcement setting is enabled and the day's first planned
// shift (Dienstplan) starts more than the tolerance after the current time.
// PlannedStartTime is the earliest wall-clock time a check-in is accepted,
// i.e. the shift start minus the tolerance — the time the person waits for.
type PlannedStartNotReachedError struct {
	PlannedStartTime string
	CurrentTime      string
}

// PlannedStartNotReached exposes the compared times to consumers that match the
// error by shape instead of importing this package.
func (e *PlannedStartNotReachedError) PlannedStartNotReached() (plannedStartTime, currentTime string) {
	return e.PlannedStartTime, e.CurrentTime
}

func (e *PlannedStartNotReachedError) Error() string {
	return "planned start not reached"
}

// DeviationReasonRequiredError is returned by CheckIn/CheckOut when the
// tenant requires a reason for stamping outside the tolerance window around
// the planned shift window (F9) and the request carried none. The API layer
// renders it as HTTP 409 with the stable code "iot.deviation_reason_required"
// produced by api/time-tracking/errors.go.
type DeviationReasonRequiredError struct {
	Action           string // deviationActionCheckIn or deviationActionCheckOut
	PlannedTime      string // planned reference as Berlin wall clock, "15:04"
	ActualTime       string // actual stamp time as Berlin wall clock, "15:04"
	DeviationMinutes int
}

func (e *DeviationReasonRequiredError) Error() string {
	return "deviation reason required"
}

const (
	deviationActionCheckIn  = "check_in"
	deviationActionCheckOut = "check_out"
)

// settingsResolver answers the tenant settings the time-tracking services
// consult. Each question is named rather than expressed as a registry key, so
// this package does not carry another owner's settings vocabulary.
type settingsResolver interface {
	// EnforcePlannedStart reports whether a check-in before the planned shift
	// start (minus PlannedStartToleranceMinutes) is refused.
	EnforcePlannedStart(ctx context.Context) (bool, error)
	// PlannedStartToleranceMinutes is how many minutes before the planned
	// shift start a check-in opens.
	PlannedStartToleranceMinutes(ctx context.Context) (int, error)
	// RequireDeviationReason reports whether a self-edit that moves recorded
	// times must carry a reason.
	RequireDeviationReason(ctx context.Context) (bool, error)
	// DeviationToleranceMinutes is the grace window before a deviation counts.
	DeviationToleranceMinutes(ctx context.Context) (int, error)
	// TimeTrackingRetentionDays is how long the audit feed keeps its entries.
	TimeTrackingRetentionDays(ctx context.Context) (int, error)
}

// WorkSessionService defines operations for staff time tracking
type WorkSessionService interface {
	// CheckIn opens a new work block for staffID. `source` records the
	// channel that triggered the check-in (app/nfc) so the export can label
	// "Vor Ort (App)" vs "Vor Ort (NFC)" without inferring it from status
	// alone (Issue #1368).
	//
	// Since #2402 a day can carry several blocks: checking in again after a
	// checkout creates a NEW session with its own check-in, checkout and
	// status instead of reopening the closed one. At most one block may be
	// open at a time ("already checked in" otherwise) — the guard spans every
	// day, so a block that crossed Berlin midnight blocks a new one until it
	// is closed. The database constraint also rejects concurrent inserts that
	// bypass this service-level guard.
	//
	// `reason` is the F9 deviation reason. It is only consulted when the
	// tenant setting operations.time_tracking_require_deviation_reason is
	// active AND the stamp falls outside the tolerance window around the
	// planned shift window of the day; an empty reason then yields
	// *DeviationReasonRequiredError, a non-empty one is written to
	// audit.work_session_edits. Same contract on CheckOut.
	CheckIn(ctx context.Context, staffID int64, status, source, reason string) (*WorkSession, error)
	CheckOut(ctx context.Context, staffID int64, reason string) (*WorkSession, error)
	StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*WorkSessionBreak, error)
	EndBreak(ctx context.Context, staffID int64) (*WorkSession, error)

	// The *On variants act on the open session of an explicit calendar day
	// instead of re-deriving "today" from the server clock. A caller whose
	// request straddles Berlin midnight (a kiosk stamp at 23:59:59) would
	// otherwise look up a day the session was never written on and fail with
	// "no active session found". The break variants above delegate here with
	// the current day and keep their behaviour; CheckIn and CheckOut resolve
	// the running block across days and ignore the pin.
	CheckInOn(ctx context.Context, staffID int64, day calendar.Date, status, source, reason string) (*WorkSession, error)
	CheckOutOn(ctx context.Context, staffID int64, day calendar.Date, reason string) (*WorkSession, error)
	StartBreakOn(ctx context.Context, staffID int64, day calendar.Date, plannedDurationMinutes *int) (*WorkSessionBreak, error)
	EndBreakOn(ctx context.Context, staffID int64, day calendar.Date) (*WorkSession, error)
	GetSessionBreaks(ctx context.Context, staffID, sessionID int64) ([]*WorkSessionBreak, error)
	UpdateSession(ctx context.Context, staffID int64, sessionID int64, updates SessionUpdateRequest) (*WorkSession, error)
	// UpdateSessionAsAdmin is the admin-facing counterpart. editorStaffID is
	// the staff record of the admin performing the edit (lands in
	// audit.work_session_edits.edited_by). targetStaffID is the owner of the
	// session. We verify session.StaffID == targetStaffID so the route can't
	// be used to leak edits across staff in the same tenant. Notes are always
	// required (BAG "Verlässlichkeit" for foreign edits).
	UpdateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID, sessionID int64, updates SessionUpdateRequest) (*WorkSession, error)
	// CreateSessionAsAdmin records a session an admin nachträgt for another
	// staff member, typically because that staff member forgot to stamp.
	// Notes are required to preserve the audit trail's "Verlässlichkeit".
	CreateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID int64, req AdminCreateSessionRequest) (*WorkSession, error)
	// GetCurrentSession only sees a session that is open AND dated today. Use
	// it only where "today" is the actual question — asking it whether the
	// person is clocked in gives the wrong answer for a block that crossed
	// Berlin midnight; GetLatestOpenSession answers that one.
	GetCurrentSession(ctx context.Context, staffID int64) (*WorkSession, error)
	// GetLatestOpenSession is GetCurrentSession without the "today" filter: it
	// finds a session that is still running even when it was opened on an
	// earlier calendar day.
	GetLatestOpenSession(ctx context.Context, staffID int64) (*WorkSession, error)
	// GetHistory reads by the stored session date: one block is one row, filed
	// under the day it started. That is the export contract, where a night
	// block must appear in exactly one period, never in two.
	GetHistory(ctx context.Context, staffID int64, from, to calendar.Date) (*HistoryResponse, error)
	// GetHistoryIntersecting reads by interval and is what every reader needs
	// that splits minutes onto Berlin calendar days: the history tables, the
	// kiosk day state and the balances all have to see the block that started
	// the evening before `from` and reaches into the range (#2402).
	GetHistoryIntersecting(ctx context.Context, staffID int64, from, to calendar.Date) (*HistoryResponse, error)
	GetSessionEdits(ctx context.Context, staffID, sessionID int64) ([]*WorkSessionEditView, error)
	// GetSessionEditsForStaff returns the audit trail of a session for a
	// specific staff id. The caller is expected to have an admin-level
	// permission (users:read or time_tracking:manage), no ownership check
	// against the JWT subject. We still verify that the session actually
	// belongs to the target staff so the URL can't be used to leak edits
	// across staff members in the same tenant.
	GetSessionEditsForStaff(ctx context.Context, staffID, sessionID int64) ([]*WorkSessionEditView, error)
	GetTodayPresenceMap(ctx context.Context) (map[int64]string, error)
	CleanupOpenSessions(ctx context.Context) (int, error)
	// AutoCheckoutDueSessions closes open sessions whose staff member has a
	// planned shift (schedule.staff_shifts) that ended more than `grace` ago,
	// stamping the checkout at the planned shift end (#1798). Sessions
	// without a shift on their date are untouched (the nightly
	// CleanupOpenSessions still catches them); sessions checked in after the
	// shift end are skipped. Each close is marked AutoCheckedOut and audited
	// with the SystemEditorID actor. No-op when no shift repo is injected.
	AutoCheckoutDueSessions(ctx context.Context, grace time.Duration) (int, error)
	// EnsureCheckedIn opens today's session if the staff member has no active
	// row. The caller passes `source` (`app`/`nfc`) to record which channel
	// triggered the auto-stamp; this avoids hard-coding the channel inside
	// the service so future callers (web action triggers, schedulers) cannot
	// be silently mislabelled as NFC. Returns nil if the staff member is
	// already checked out today (no re-open).
	EnsureCheckedIn(ctx context.Context, staffID int64, source string) (*WorkSession, error)
	// ExportSessions renders the single-staff export. CSV serializes locally
	// (data format); XLSX uses the workbook writer, and PDF delegates to the
	// injected renderer for the shared document design (#1568).
	ExportSessions(ctx context.Context, staffID int64, from, to calendar.Date, format string) (*ExportFile, error)
	// DayExportRows exposes the export's merged session/absence day rows for
	// the cross-staff export (#1417 2b) — same loading, same cell rendering.
	DayExportRows(ctx context.Context, staffID int64, from, to calendar.Date) ([]DayExportRow, error)
	// DayExportRowsByStaffIDs loads the same rows for many staff members with
	// batched repository calls, keyed by staff ID.
	DayExportRowsByStaffIDs(ctx context.Context, staffIDs []int64, from, to calendar.Date) (map[int64][]DayExportRow, error)
	AutoEndExpiredBreaks(ctx context.Context) (int, error)

	// Staff work-schedule operations (issue #584: moved out of api/staff).
	GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error)
	// UpdateSchedule is the whole schedule-write surface this interface
	// exposes. The template, custom-rows and save-as-template steps it
	// dispatches to are implementation detail: nothing outside this package
	// called them, and each carried the legacy work-schedule row types through
	// the contract (#2737).
	//
	// UpdateSchedule resolves the requested mode (template vs custom, with the
	// legacy empty-mode fallback), validates and applies the change. Validation
	// failures wrap ErrScheduleValidation so callers can map them to 400.
	UpdateSchedule(ctx context.Context, staff *StaffScheduleBinding, in ScheduleUpdateInput) error
}

// scheduleEntryMaxTargetMinutes caps a single schedule day at 12 hours.
const scheduleEntryMaxTargetMinutes = 720

// ErrScheduleValidation marks a schedule-update failure caused by invalid
// request input (bad mode, out-of-range values, duplicate slots) so callers
// render a 400 instead of a 500.
var ErrScheduleValidation = errors.New("schedule validation")

type scheduleValidationError struct {
	message string
}

func (e scheduleValidationError) Error() string {
	return e.message
}

func (e scheduleValidationError) Is(target error) bool {
	return target == ErrScheduleValidation
}

func scheduleValidationErrorf(format string, args ...any) error {
	return scheduleValidationError{message: fmt.Sprintf(format, args...)}
}
