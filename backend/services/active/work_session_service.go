package active

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Error message constants to avoid duplication
const (
	errNoActiveSession             = "no active session found"
	errGetCurrentSession           = "failed to get current session: %w"
	errInvalidSessionData          = "invalid session data: %w"
	maxPlannedBreakDurationMinutes = 240
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
	*activeModels.WorkSession
	// BreakMinutes SHADOWS WorkSession.BreakMinutes on the wire — the outer
	// field is at the shallower depth, so encoding/json emits this one for
	// "break_minutes". That is deliberate: the model field caches ENDED breaks
	// only, while NetMinutes below already deducts a RUNNING one, so serializing
	// the raw cache would print "Pause 0:00" next to an Ist that has visibly
	// stopped growing. Both numbers come from the same as-of-now pair
	// (totalBreakMinutes / netMinutesWithBreaks), which keeps
	// gross = net + break true for the reader (#1842).
	BreakMinutes     int                              `json:"break_minutes"`
	NetMinutes       int                              `json:"net_minutes"`
	IsOvertime       bool                             `json:"is_overtime"`
	IsBreakCompliant bool                             `json:"is_break_compliant"`
	Breaks           []*activeModels.WorkSessionBreak `json:"breaks"`
	EditCount        int                              `json:"edit_count"`
	AuditCount       int                              `json:"audit_count"`
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
}

// WorkSessionEditView decorates an audit row with the editor's display name
// and a "selbst geändert" flag. We compute IsSelfEdit on the server because
// audit.work_session_edits.edited_by stores a staff_id, not a role. The
// frontend would otherwise need to fetch the session separately to learn
// whether edited_by == session.staff_id.
type WorkSessionEditView struct {
	*auditModels.WorkSessionEdit
	EditorName string `json:"editor_name"`
	IsSelfEdit bool   `json:"is_self_edit"`
}

// PlannedStartNotReachedError is returned by CheckIn when the optional
// planned-start enforcement setting is enabled and today's work schedule has a
// start time later than the current wall clock.
type PlannedStartNotReachedError struct {
	PlannedStartTime string
	CurrentTime      string
}

func (e *PlannedStartNotReachedError) Error() string {
	return "planned start not reached"
}

// DeviationReasonRequiredError is returned by CheckIn/CheckOut when the
// tenant requires a reason for stamping outside the tolerance window around
// the planned shift window (F9) and the request carried none. The API layer
// renders it as HTTP 409 with the stable code "deviation_reason_required"
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

type settingsResolver interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
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
	CheckIn(ctx context.Context, staffID int64, status, source, reason string) (*activeModels.WorkSession, error)
	CheckOut(ctx context.Context, staffID int64, reason string) (*activeModels.WorkSession, error)
	StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*activeModels.WorkSessionBreak, error)
	EndBreak(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)

	// The *On variants act on the open session of an explicit calendar day
	// instead of re-deriving "today" from the server clock. A caller whose
	// request straddles Berlin midnight (a kiosk stamp at 23:59:59) would
	// otherwise look up a day the session was never written on and fail with
	// "no active session found". The break variants above delegate here with
	// the current day and keep their behaviour; CheckIn and CheckOut resolve
	// the running block across days and ignore the pin.
	CheckInOn(ctx context.Context, staffID int64, day timezone.Date, status, source, reason string) (*activeModels.WorkSession, error)
	CheckOutOn(ctx context.Context, staffID int64, day timezone.Date, reason string) (*activeModels.WorkSession, error)
	StartBreakOn(ctx context.Context, staffID int64, day timezone.Date, plannedDurationMinutes *int) (*activeModels.WorkSessionBreak, error)
	EndBreakOn(ctx context.Context, staffID int64, day timezone.Date) (*activeModels.WorkSession, error)
	GetSessionBreaks(ctx context.Context, staffID, sessionID int64) ([]*activeModels.WorkSessionBreak, error)
	UpdateSession(ctx context.Context, staffID int64, sessionID int64, updates SessionUpdateRequest) (*activeModels.WorkSession, error)
	// UpdateSessionAsAdmin is the admin-facing counterpart. editorStaffID is
	// the staff record of the admin performing the edit (lands in
	// audit.work_session_edits.edited_by). targetStaffID is the owner of the
	// session. We verify session.StaffID == targetStaffID so the route can't
	// be used to leak edits across staff in the same tenant. Notes are always
	// required (BAG "Verlässlichkeit" for foreign edits).
	UpdateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID, sessionID int64, updates SessionUpdateRequest) (*activeModels.WorkSession, error)
	// CreateSessionAsAdmin records a session an admin nachträgt for another
	// staff member, typically because that staff member forgot to stamp.
	// Notes are required to preserve the audit trail's "Verlässlichkeit".
	CreateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID int64, req AdminCreateSessionRequest) (*activeModels.WorkSession, error)
	// GetCurrentSession only sees a session that is open AND dated today. Use
	// it only where "today" is the actual question — asking it whether the
	// person is clocked in gives the wrong answer for a block that crossed
	// Berlin midnight; GetLatestOpenSession answers that one.
	GetCurrentSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	// GetLatestOpenSession is GetCurrentSession without the "today" filter: it
	// finds a session that is still running even when it was opened on an
	// earlier calendar day.
	GetLatestOpenSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	// GetHistory reads by the stored session date: one block is one row, filed
	// under the day it started. That is the export contract, where a night
	// block must appear in exactly one period, never in two.
	GetHistory(ctx context.Context, staffID int64, from, to timezone.Date) (*HistoryResponse, error)
	// GetHistoryIntersecting reads by interval and is what every reader needs
	// that splits minutes onto Berlin calendar days: the history tables, the
	// kiosk day state and the balances all have to see the block that started
	// the evening before `from` and reaches into the range (#2402).
	GetHistoryIntersecting(ctx context.Context, staffID int64, from, to timezone.Date) (*HistoryResponse, error)
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
	// SetStaffShiftRepo injects the planned-shift repository consumed by
	// AutoCheckoutDueSessions (wired by the factory after construction).
	SetStaffShiftRepo(repo scheduleModels.StaffShiftRepository)
	// EnsureCheckedIn opens today's session if the staff member has no active
	// row. The caller passes `source` (`app`/`nfc`) to record which channel
	// triggered the auto-stamp; this avoids hard-coding the channel inside
	// the service so future callers (web action triggers, schedulers) cannot
	// be silently mislabelled as NFC. Returns nil if the staff member is
	// already checked out today (no re-open).
	EnsureCheckedIn(ctx context.Context, staffID int64, source string) (*activeModels.WorkSession, error)
	// ExportSessions renders the single-staff export. CSV serializes locally
	// (data format); xlsx/pdf render through services/listexport so all
	// downloadable files share one design (#1568).
	ExportSessions(ctx context.Context, staffID int64, from, to timezone.Date, format string) (*ExportFile, error)
	// DayExportRows exposes the export's merged session/absence day rows for
	// the cross-staff export (#1417 2b) — same loading, same cell rendering.
	DayExportRows(ctx context.Context, staffID int64, from, to timezone.Date) ([]DayExportRow, error)
	// DayExportRowsByStaffIDs loads the same rows for many staff members with
	// batched repository calls, keyed by staff ID.
	DayExportRowsByStaffIDs(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]DayExportRow, error)
	AutoEndExpiredBreaks(ctx context.Context) (int, error)

	// Staff work-schedule operations (issue #584: moved out of api/staff).
	// The three Get* lookups return repository results verbatim.
	GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error)
	GetWorkTimeModelByID(ctx context.Context, id int64) (*configModels.WorkTimeModel, error)
	GetCurrentScheduleRows(ctx context.Context, staffID int64) ([]*configModels.StaffWorkSchedule, error)
	// AssignScheduleTemplate snapshots the template's entries as the staff
	// member's schedule and binds the template (model id + rotation anchor).
	AssignScheduleTemplate(ctx context.Context, staff *userModels.Staff, modelID int64) error
	// ApplyCustomScheduleRows replaces the schedule with custom rows and
	// unbinds any assigned template.
	ApplyCustomScheduleRows(ctx context.Context, staff *userModels.Staff, entries []*configModels.StaffWorkSchedule, anchor timezone.Date) error
	// SaveCustomScheduleAsTemplate persists the rows as a new reusable work
	// time model and binds it to the staff member.
	SaveCustomScheduleAsTemplate(ctx context.Context, staff *userModels.Staff, name string, rotation int, anchor timezone.Date, entries []*configModels.WorkTimeModelEntry) error
	// UpdateSchedule resolves the requested mode (template vs custom, with the
	// legacy empty-mode fallback), validates and applies the change. Validation
	// failures wrap ErrScheduleValidation so callers can map them to 400.
	UpdateSchedule(ctx context.Context, staff *userModels.Staff, in ScheduleUpdateInput) error
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

// ScheduleEntry mirrors a single day of the schedule-update request at the
// service boundary (no api DTO types).
type ScheduleEntry struct {
	WeekIndex     int
	DayOfWeek     int
	TargetMinutes int
	StartTime     *string
}

// ScheduleUpdateInput mirrors the PUT /staff/{id}/schedule request body at the
// service boundary.
type ScheduleUpdateInput struct {
	Mode               string
	ModelID            *int64
	RotationLength     int
	RotationAnchorDate string
	Entries            []ScheduleEntry
	SaveAsTemplateName string
}

// workSessionService implements WorkSessionService
type workSessionService struct {
	repo        activeModels.WorkSessionRepository
	breakRepo   activeModels.WorkSessionBreakRepository
	auditRepo   auditModels.WorkSessionEditRepository
	absenceRepo activeModels.StaffAbsenceRepository
	// absenceTypes resolves school-defined Abwesenheitsarten for exports
	// (#2403). Setter injection; nil in bare-constructed unit fixtures.
	absenceTypes   StaffAbsenceTypeService
	supervisorRepo activeModels.GroupSupervisorRepository
	groupRepo      activeModels.GroupRepository
	db             *bun.DB
	staffRepo      userModels.StaffRepository
	scheduleRepo   configModels.StaffWorkScheduleRepository
	workModelRepo  configModels.WorkTimeModelRepository
	settings       settingsResolver
	staffShiftRepo scheduleModels.StaffShiftRepository
	holidayReader  HolidayDatesReader
	broadcaster    realtime.Broadcaster
	logger         *slog.Logger
	nowFunc        func() time.Time
}

// SetStaffShiftRepo injects the planned-shift repository used by
// AutoCheckoutDueSessions (#1798). Setter instead of a constructor param to
// keep the already long NewWorkSessionService signature stable; the factory
// calls it right after construction.
func (s *workSessionService) SetStaffShiftRepo(repo scheduleModels.StaffShiftRepository) {
	s.staffShiftRepo = repo
}

// SetHolidayReader injects the public-holiday resolver (#1418 3a): holidays
// reduce the weekly Soll shown in the history summaries. Deliberately NOT
// part of the WorkSessionService interface — the factory injects it via a
// type assertion, so external mocks of the interface stay untouched.
func (s *workSessionService) SetHolidayReader(reader HolidayDatesReader) {
	s.holidayReader = reader
}

// SetBroadcaster injects the tenant-wide SSE broadcaster. It stays outside
// WorkSessionService so existing API-layer mocks do not need a no-op setter.
func (s *workSessionService) SetBroadcaster(broadcaster realtime.Broadcaster) {
	s.broadcaster = broadcaster
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (s *workSessionService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

func (s *workSessionService) broadcastTimeTrackingChanged(ctx context.Context) {
	queueStaffTimeTrackingChanged(ctx, s.broadcaster, s.getLogger())
}

func (s *workSessionService) lockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	if err := s.repo.LockStaffBalanceWrites(ctx, staffID); err != nil {
		return fmt.Errorf("failed to lock staff balance writes: %w", err)
	}
	return nil
}

func (s *workSessionService) lockStaffBalanceWritesOrdered(ctx context.Context, staffIDs []int64) error {
	unique := make(map[int64]struct{}, len(staffIDs))
	for _, staffID := range staffIDs {
		unique[staffID] = struct{}{}
	}
	ordered := slices.Collect(maps.Keys(unique))
	slices.Sort(ordered)
	for _, staffID := range ordered {
		if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
			return err
		}
	}
	return nil
}

// NewWorkSessionService creates a new work session service
// SetAbsenceTypeService wires the school-defined absence names (#2403).
func (s *workSessionService) SetAbsenceTypeService(svc StaffAbsenceTypeService) {
	s.absenceTypes = svc
}

func NewWorkSessionService(repo activeModels.WorkSessionRepository, breakRepo activeModels.WorkSessionBreakRepository, auditRepo auditModels.WorkSessionEditRepository, absenceRepo activeModels.StaffAbsenceRepository, supervisorRepo activeModels.GroupSupervisorRepository, groupRepo activeModels.GroupRepository, staffRepo userModels.StaffRepository, scheduleRepo configModels.StaffWorkScheduleRepository, workModelRepo configModels.WorkTimeModelRepository, settings settingsResolver, logger *slog.Logger, db *bun.DB) WorkSessionService {
	return &workSessionService{repo: repo, breakRepo: breakRepo, auditRepo: auditRepo, absenceRepo: absenceRepo, supervisorRepo: supervisorRepo, groupRepo: groupRepo, staffRepo: staffRepo, scheduleRepo: scheduleRepo, workModelRepo: workModelRepo, settings: settings, logger: logger, db: db}
}

func (s *workSessionService) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
}

// listSessionsByStaffAndDate applies the ordinary repository filter path for
// the two equality conditions used by check-in and auto-check-in. Keeping the
// lookup here avoids a per-field repository method while retaining the
// chronological block order required by the caller.
func (s *workSessionService) listSessionsByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) ([]*activeModels.WorkSession, error) {
	options := modelBase.NewQueryOptions()
	options.Filter.Equal("staff_id", staffID).Equal("date", date)
	options.Sorting = (&modelBase.Sorting{}).AddField("check_in_time", modelBase.SortAsc)

	sessions, err := s.repo.List(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to list work sessions by staff and date: %w", err)
	}
	return sessions, nil
}

// CheckIn creates a new work session for the staff member.
// Status must be explicitly chosen. Empty values are rejected so the caller
// (HTTP handler or internal worker) cannot accidentally fall back to "present".
func (s *workSessionService) CheckIn(ctx context.Context, staffID int64, status, source, reason string) (*activeModels.WorkSession, error) {
	return s.checkIn(ctx, staffID, status, source, reason, true)
}

// CheckInOn is the day-pinned entry point of the kiosk, kept symmetric with
// CheckOutOn. The pinned day no longer selects anything: the open-block guard
// in checkIn scans every day (a block opened before Berlin midnight is still
// open after it), and a session that is created fresh is filed on the day its
// own stamp falls on — see checkIn.
func (s *workSessionService) CheckInOn(ctx context.Context, staffID int64, _ timezone.Date, status, source, reason string) (*activeModels.WorkSession, error) {
	return s.checkIn(ctx, staffID, status, source, reason, true)
}

// checkIn is the shared body behind CheckIn and EnsureCheckedIn.
// enforceDeviationGate switches the F9 reason requirement: deliberate
// stamping (the time-tracking page) enforces it, auto-stamps triggered by
// starting a supervision (EnsureCheckedIn) bypass it — those flows have no
// way to collect a reason, and refusing the stamp would leave a supervisor
// row without a matching work session.
//
// A session is always filed on the day of its own check_in_time: storing a
// pre-midnight day next to a post-midnight stamp would misfile the session in
// the daily history, in shift and deviation lookups, and in every total built
// from the date column.
func (s *workSessionService) checkIn(ctx context.Context, staffID int64, status, source, reason string, enforceDeviationGate bool) (*activeModels.WorkSession, error) {
	if status != activeModels.WorkSessionStatusPresent && status != activeModels.WorkSessionStatusHomeOffice {
		return nil, fmt.Errorf("status must be 'present' or 'home_office'")
	}
	if source != activeModels.WorkSessionSourceApp && source != activeModels.WorkSessionSourceNFC {
		return nil, fmt.Errorf("source must be 'app' or 'nfc'")
	}
	if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, err
	}

	now := s.now()
	stampDay := timezone.DateFromTime(now)

	// Every block that reaches into the present, in one read: the open ones
	// (they run to infinity, so the query returns them whatever day they were
	// filed on) and the closed ones that still end after "now". The same rows
	// answer both questions below — whether somebody is clocked in, and
	// whether the new block would overlap an existing one.
	siblings, err := s.repo.ListOverlappingByStaffID(ctx, staffID, now, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing session: %w", err)
	}

	// An open block anywhere means the person is clocked in — the check is
	// deliberately NOT limited to the stamp's day. A block opened before
	// Berlin midnight is still running after it; starting a second one there
	// would leave two open blocks, and the checkout (which closes exactly
	// one) would then report a different day's work than the totals do.
	// The still-open block has to be closed first.
	//
	// A block whose live limit has passed is the exception, and it has to be
	// CLOSED here, not merely skipped: the database permits a single open
	// block per staff member (uq_work_sessions_staff_date_open), so a
	// forgotten checkout that is only ignored lets this guard pass and then
	// fails the INSERT — locking the person out of the clock entirely, which
	// is exactly what #2402 set out to fix. It is closed at the instant it
	// stopped counting as work everywhere else (BalanceSessionEnd), so no
	// total, balance or history row changes value; the row merely stops
	// hanging open and disappears from the kiosk's running block.
	for _, sibling := range siblings {
		if sibling.CheckOutTime != nil {
			continue
		}
		expired, stale := ExpireStaleOpenBlock(sibling, now)
		if !stale {
			return nil, fmt.Errorf("already checked in")
		}
		if err := s.closeStaleOpenBlock(ctx, sibling, *expired.CheckOutTime); err != nil {
			return nil, err
		}
	}

	// Since #2402 a day carries a LIST of blocks. Closed blocks never block a
	// new check-in — checking in again after a checkout starts a new block
	// with its own status.
	existingBlocks, err := s.listSessionsByStaffAndDate(ctx, staffID, stampDay)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing sessions: %w", err)
	}

	// A closed block can still reach past "now" (an admin Nachtrag for the
	// afternoon, an edited checkout in the future, a night block from
	// yesterday that ran into this morning). A new block starting inside that
	// interval would double-count the overlap in every sum built from the
	// day's rows, so it is rejected like any other overlap. The blocks just
	// closed above end before "now" and no longer reach it.
	if err := assertNoBlockOverlapIn(expireStaleOpenBlocks(siblings, now), 0, now, nil); err != nil {
		return nil, err
	}

	// The session is created on the day of its own stamp: the schedule and
	// deviation checks below have to be read on that same day, or a stamp
	// taken just after midnight is measured against the previous day's shift.
	if err := s.ensurePlannedStartReached(ctx, staffID, stampDay, now); err != nil {
		return nil, err
	}

	// F9: checking in more than the tolerance before the planned shift start
	// needs a reason ("früher kommen"). Only the day's FIRST block is gated —
	// a later block resumes an already-started work day, exactly like the
	// pre-#2402 reopen path, which was exempt for the same reason.
	var deviation *plannedDeviation
	if enforceDeviationGate && len(existingBlocks) == 0 {
		var err error
		deviation, err = s.detectPlannedDeviation(ctx, staffID, stampDay, now, deviationActionCheckIn)
		if err != nil {
			return nil, err
		}
		if deviation != nil && strings.TrimSpace(reason) == "" {
			return nil, deviation.requiredError()
		}
	}

	// Create new session
	session := &activeModels.WorkSession{
		StaffID:      staffID,
		Date:         stampDay,
		Status:       status,
		Source:       source,
		CheckInTime:  now,
		CheckOutTime: nil,
		BreakMinutes: 0,
		CreatedBy:    staffID,
	}

	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf(errInvalidSessionData, err)
	}

	session.SetTenantID(tenant.FromContext(ctx))
	if err := s.repo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create work session: %w", err)
	}

	if deviation != nil {
		if err := s.recordDeviationReason(ctx, session, deviation, reason, now); err != nil {
			return nil, err
		}
	}

	s.broadcastTimeTrackingChanged(ctx)
	return session, nil
}

func (s *workSessionService) ensurePlannedStartReached(ctx context.Context, staffID int64, today timezone.Date, now time.Time) error {
	if s.settings == nil {
		return nil
	}
	enabled, err := s.settings.ResolveBool(ctx, configModels.KeyTimeTrackingEnforcePlannedStart)
	if err != nil {
		return fmt.Errorf("failed to resolve planned-start setting: %w", err)
	}
	if !enabled {
		return nil
	}
	if s.scheduleRepo == nil {
		return fmt.Errorf("staff work schedule repository not configured")
	}

	entries, err := s.scheduleRepo.GetByStaffIDAndDate(ctx, staffID, workforceDate(today))
	if err != nil {
		return fmt.Errorf("failed to load planned start schedule: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}

	staff := s.resolveStaffForTargets(ctx, staffID)
	anchor := configModels.ResolveScheduleAnchor(workforceDatePointer(staffAnchorOf(staff)), entries)
	if anchor.IsZero() {
		return nil
	}
	rotationWeek := configModels.ResolveWeekIndex(configModels.ScheduleRotationLength(entries), configModels.MondayOf(anchor), configModels.MondayOf(workforceDate(today)))
	dayIndex := configModels.ISODayIndex(workforceDate(today))
	for _, entry := range entries {
		if entry.WeekIndex != rotationWeek || entry.DayOfWeek != dayIndex || entry.StartTime == nil {
			continue
		}
		plannedStart := timezone.NormalizeWallClock(*entry.StartTime)
		currentClock := timezone.NormalizeWallClock(now.In(timezone.Berlin))
		if currentClock.Before(plannedStart) {
			return &PlannedStartNotReachedError{
				PlannedStartTime: plannedStart.Format("15:04"),
				CurrentTime:      currentClock.Format("15:04"),
			}
		}
		return nil
	}
	return nil
}

// CheckOut ends the running work session of the staff member, whichever day it
// was opened on. Resolving the day from the clock instead would strand a block
// that crossed Berlin midnight: check-in refuses while it is open, so a
// today-only checkout would leave no way to close it.
func (s *workSessionService) CheckOut(ctx context.Context, staffID int64, reason string) (*activeModels.WorkSession, error) {
	day, err := s.openBlockDay(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return s.CheckOutOn(ctx, staffID, day, reason)
}

// openBlockDay resolves the calendar day the currently running block is filed
// on, for the web endpoints that stamp without carrying a day themselves. It
// is the service-side twin of the kiosk's clockDay: the running block's own
// day, or today when nobody is clocked in.
//
// Asking the clock unconditionally would strand every block that crossed
// Berlin midnight: it stays open and is still dated yesterday, so a today-only
// lookup reports "no active session found" for somebody who is demonstrably
// clocked in — leaving no way to check out or end the running break at all.
//
// Finding nothing open is not decided here: the day-scoped lookup each caller
// performs next says "no active session found" on its own, and keeping that
// one authority avoids two lookups disagreeing about whether a stamp exists.
func (s *workSessionService) openBlockDay(ctx context.Context, staffID int64) (timezone.Date, error) {
	open, err := s.repo.GetLatestOpenByStaffID(ctx, staffID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return timezone.Date(""), fmt.Errorf(errGetCurrentSession, err)
	}
	if open != nil {
		return open.Date, nil
	}
	return timezone.DateFromTime(s.now()), nil
}

// CheckOutOn ends the open session of an explicit calendar day.
func (s *workSessionService) CheckOutOn(ctx context.Context, staffID int64, day timezone.Date, reason string) (*activeModels.WorkSession, error) {
	if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	// Get the open session of the requested day
	session, err := s.repo.GetOpenByStaffAndDate(ctx, staffID, day)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(errNoActiveSession)
		}
		return nil, fmt.Errorf(errGetCurrentSession, err)
	}

	if session == nil {
		return nil, errors.New(errNoActiveSession)
	}

	now := s.now()

	// F9: checking out more than the tolerance after the planned shift end
	// needs a reason ("später gehen"). Referenced against the session's own
	// date so a forgotten session from yesterday is measured against
	// yesterday's plan.
	deviation, err := s.detectPlannedDeviation(ctx, staffID, session.Date, now, deviationActionCheckOut)
	if err != nil {
		return nil, err
	}
	if deviation != nil && strings.TrimSpace(reason) == "" {
		return nil, deviation.requiredError()
	}

	// End any active break before checkout
	if err := s.endActiveBreakIfExists(ctx, session.ID, now); err != nil {
		return nil, err
	}

	// Close the session using repository method
	closed, err := s.repo.CloseSession(ctx, session.ID, now, false)
	if err != nil {
		return nil, fmt.Errorf("failed to close session: %w", err)
	}
	if !closed {
		return nil, errors.New(errNoActiveSession)
	}

	if deviation != nil {
		if err := s.recordDeviationReason(ctx, session, deviation, reason, now); err != nil {
			return nil, err
		}
	}

	// End all active supervisions for this staff member (fire-and-forget)
	s.endActiveSupervisionsOnCheckout(ctx, staffID)

	// Re-fetch the updated session
	updatedSession, err := s.repo.FindByID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated session: %w", err)
	}

	s.broadcastTimeTrackingChanged(ctx)
	return updatedSession, nil
}

// plannedDeviation describes a stamp that falls outside the tolerance window
// around the planned shift window of the day (F9).
type plannedDeviation struct {
	action  string
	planned time.Time
	actual  time.Time
	minutes int
}

func (d *plannedDeviation) requiredError() *DeviationReasonRequiredError {
	return &DeviationReasonRequiredError{
		Action:           d.action,
		PlannedTime:      d.planned.In(timezone.Berlin).Format("15:04"),
		ActualTime:       d.actual.In(timezone.Berlin).Format("15:04"),
		DeviationMinutes: d.minutes,
	}
}

// detectPlannedDeviation implements the F9 rule: with the tenant setting
// active, a check-in earlier than the earliest planned shift start minus the
// tolerance, or a check-out later than the latest planned shift end plus the
// tolerance, is a deviation that needs a reason. The plan source is
// schedule.staff_shifts (real start AND end times); days without a shift
// never deviate ("kein Plan, keine Abweichung"). Late arrivals and early
// leaves are deliberately not gated — F9 targets unnoticed extra hours
// ("früher kommen oder später gehen"), and missing time is already visible
// in the saldo.
func (s *workSessionService) detectPlannedDeviation(ctx context.Context, staffID int64, day timezone.Date, now time.Time, action string) (*plannedDeviation, error) {
	if s.settings == nil || s.staffShiftRepo == nil {
		return nil, nil
	}
	enabled, err := s.settings.ResolveBool(ctx, configModels.KeyTimeTrackingRequireDeviationReason)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve deviation-reason setting: %w", err)
	}
	if !enabled {
		return nil, nil
	}

	allShifts, err := s.staffShiftRepo.FindByStaffIDsAndDate(ctx, []int64{staffID}, day)
	if err != nil {
		return nil, fmt.Errorf("failed to load planned shifts: %w", err)
	}
	// A cancelled shift does not take place (#1841), so it must not widen the
	// planned window: with an active 08:00–12:00 shift and a cancelled
	// 12:00–16:00 shift, a 15:00 checkout IS a deviation.
	shifts := make([]*scheduleModels.StaffShift, 0, len(allShifts))
	for _, shift := range allShifts {
		if !shift.Cancelled {
			shifts = append(shifts, shift)
		}
	}
	if len(shifts) == 0 {
		return nil, nil
	}

	toleranceMinutes, err := s.settings.ResolveInt(ctx, configModels.KeyTimeTrackingDeviationToleranceMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve deviation-tolerance setting: %w", err)
	}

	var planned time.Time
	var delta time.Duration
	switch action {
	case deviationActionCheckIn:
		planned = shifts[0].StartInstant()
		for _, shift := range shifts[1:] {
			if start := shift.StartInstant(); start.Before(planned) {
				planned = start
			}
		}
		delta = planned.Sub(now) // positive when arriving early
	case deviationActionCheckOut:
		planned = shifts[0].EndInstant()
		for _, shift := range shifts[1:] {
			if end := shift.EndInstant(); end.After(planned) {
				planned = end
			}
		}
		delta = now.Sub(planned) // positive when leaving late
	default:
		return nil, fmt.Errorf("unknown deviation action %q", action)
	}

	if delta <= time.Duration(toleranceMinutes)*time.Minute {
		return nil, nil
	}

	return &plannedDeviation{
		action:  action,
		planned: planned,
		actual:  now,
		minutes: int(math.Round(delta.Minutes())),
	}, nil
}

// recordDeviationReason persists the F9 reason as an audit edit on the
// session: old value = planned wall clock, new value = actual wall clock,
// notes = the reason. The stamp and its audit row share the handler's tenant
// transaction, so a failed audit write rolls the stamp back.
func (s *workSessionService) recordDeviationReason(ctx context.Context, session *activeModels.WorkSession, dev *plannedDeviation, reason string, now time.Time) error {
	planned := dev.planned.In(timezone.Berlin).Format("15:04")
	actual := dev.actual.In(timezone.Berlin).Format("15:04")
	trimmed := strings.TrimSpace(reason)
	edit := &auditModels.WorkSessionEdit{
		SessionID: session.ID,
		StaffID:   session.StaffID,
		EditedBy:  session.StaffID,
		FieldName: auditModels.FieldDeviationReason,
		OldValue:  &planned,
		NewValue:  &actual,
		Notes:     &trimmed,
		CreatedAt: now,
	}
	edit.SetTenantID(tenant.FromContext(ctx))
	if err := s.auditRepo.CreateBatch(ctx, []*auditModels.WorkSessionEdit{edit}); err != nil {
		return fmt.Errorf("failed to record deviation reason: %w", err)
	}
	return nil
}

// endActiveBreakIfExists ends any active break for a session at endAt and
// recalculates break minutes. endAt must not be after the session's checkout
// time, or break rows end up ending after check_out_time.
func (s *workSessionService) endActiveBreakIfExists(ctx context.Context, sessionID int64, endAt time.Time) error {
	activeBreak, err := s.breakRepo.GetActiveBySessionID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to check active break: %w", err)
	}
	if activeBreak == nil {
		return nil
	}

	// A break started after endAt (e.g. during the auto-checkout grace
	// window) is capped to zero length instead of a negative duration.
	if endAt.Before(activeBreak.StartedAt) {
		endAt = activeBreak.StartedAt
	}
	duration := int(math.Round(endAt.Sub(activeBreak.StartedAt).Minutes()))
	if err := s.breakRepo.EndBreak(ctx, activeBreak.ID, endAt, duration); err != nil {
		return fmt.Errorf("failed to end active break: %w", err)
	}

	if err := s.recalcBreakMinutes(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to update break minutes: %w", err)
	}
	return nil
}

// endActiveSupervisionsOnCheckout ends all active supervisions for a staff member (fire-and-forget).
func (s *workSessionService) endActiveSupervisionsOnCheckout(ctx context.Context, staffID int64) {
	if s.supervisorRepo == nil {
		return
	}
	ended, err := s.endActiveSupervisionsWithGroupLocks(ctx, staffID)
	if err != nil {
		s.getLogger().WarnContext(ctx, "failed to end active supervisions on checkout",
			slog.Int64("staff_id", staffID),
			slog.String("error", err.Error()))
		return
	}
	if ended > 0 {
		s.getLogger().InfoContext(ctx, "ended active supervisions on checkout",
			slog.Int("ended_count", ended),
			slog.Int64("staff_id", staffID))
	}
}

func (s *workSessionService) endActiveSupervisionsWithGroupLocks(ctx context.Context, staffID int64) (int, error) {
	if s.groupRepo == nil {
		return s.supervisorRepo.EndAllActiveByStaffID(ctx, staffID)
	}
	ended := 0
	err := s.runInWorkSessionTx(ctx, func(txCtx context.Context) error {
		supervisions, err := s.supervisorRepo.FindActiveByStaffID(txCtx, staffID)
		if err != nil {
			return err
		}
		groupIDs := make([]int64, 0, len(supervisions))
		for _, supervision := range supervisions {
			groupIDs = append(groupIDs, supervision.GroupID)
		}
		if err := s.lockWorkSessionGroupRows(txCtx, groupIDs); err != nil {
			return err
		}
		ended, err = s.supervisorRepo.EndAllActiveByStaffID(txCtx, staffID)
		return err
	})
	return ended, err
}

func (s *workSessionService) lockWorkSessionGroupRows(ctx context.Context, groupIDs []int64) error {
	unique := make(map[int64]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		unique[id] = struct{}{}
	}
	ordered := slices.Collect(maps.Keys(unique))
	slices.Sort(ordered)
	for _, id := range ordered {
		if _, err := s.groupRepo.FindByIDForUpdate(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *workSessionService) runInWorkSessionTx(ctx context.Context, fn func(context.Context) error) error {
	if s.db == nil {
		return fn(ctx)
	}
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}

// StartBreak starts a new break on the running block, whichever day it was
// opened on (see openBlockDay).
// If plannedDurationMinutes is provided (1-240), sets planned_end_time for auto-end
func (s *workSessionService) StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*activeModels.WorkSessionBreak, error) {
	day, err := s.openBlockDay(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return s.StartBreakOn(ctx, staffID, day, plannedDurationMinutes)
}

// StartBreakOn starts a break on the open session of an explicit calendar day.
func (s *workSessionService) StartBreakOn(ctx context.Context, staffID int64, day timezone.Date, plannedDurationMinutes *int) (*activeModels.WorkSessionBreak, error) {
	if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	// Get the open session of the requested day
	session, err := s.repo.GetOpenByStaffAndDateForUpdate(ctx, staffID, day)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(errNoActiveSession)
		}
		return nil, fmt.Errorf(errGetCurrentSession, err)
	}
	if session == nil {
		return nil, errors.New(errNoActiveSession)
	}

	// Check no active break exists
	activeBreak, err := s.breakRepo.GetActiveBySessionID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check active break: %w", err)
	}
	if activeBreak != nil {
		return nil, errors.New("break already active")
	}

	// Validate plannedDurationMinutes if provided
	if plannedDurationMinutes != nil {
		if *plannedDurationMinutes < 1 || *plannedDurationMinutes > maxPlannedBreakDurationMinutes {
			return nil, fmt.Errorf("planned_duration_minutes must be between 1 and %d", maxPlannedBreakDurationMinutes)
		}
	}

	// Create a new break
	now := s.now()
	brk := &activeModels.WorkSessionBreak{
		SessionID: session.ID,
		StartedAt: now,
	}

	// Set planned_end_time if duration is specified
	if plannedDurationMinutes != nil {
		plannedEnd := now.Add(time.Duration(*plannedDurationMinutes) * time.Minute)
		brk.PlannedEndTime = &plannedEnd
	}

	brk.CreatedAt = now
	brk.UpdatedAt = now

	brk.SetTenantID(tenant.FromContext(ctx))
	if err := s.breakRepo.Create(ctx, brk); err != nil {
		return nil, fmt.Errorf("failed to create break: %w", err)
	}

	s.broadcastTimeTrackingChanged(ctx)
	return brk, nil
}

// EndBreak ends the active break on the running block, whichever day it was
// opened on (see openBlockDay).
func (s *workSessionService) EndBreak(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	day, err := s.openBlockDay(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return s.EndBreakOn(ctx, staffID, day)
}

// EndBreakOn ends the active break on the open session of an explicit calendar day.
func (s *workSessionService) EndBreakOn(ctx context.Context, staffID int64, day timezone.Date) (*activeModels.WorkSession, error) {
	if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	// Get the open session of the requested day
	session, err := s.repo.GetOpenByStaffAndDate(ctx, staffID, day)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(errNoActiveSession)
		}
		return nil, fmt.Errorf(errGetCurrentSession, err)
	}
	if session == nil {
		return nil, errors.New(errNoActiveSession)
	}

	// Find active break
	activeBreak, err := s.breakRepo.GetActiveBySessionID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active break: %w", err)
	}
	if activeBreak == nil {
		return nil, fmt.Errorf("no active break found")
	}

	// End the break
	now := time.Now()
	duration := int(math.Round(now.Sub(activeBreak.StartedAt).Minutes()))
	if err := s.breakRepo.EndBreak(ctx, activeBreak.ID, now, duration); err != nil {
		return nil, fmt.Errorf("failed to end break: %w", err)
	}

	// Recalculate break_minutes cache on session
	if err := s.recalcBreakMinutes(ctx, session.ID); err != nil {
		return nil, fmt.Errorf("failed to update break minutes: %w", err)
	}

	// Re-fetch updated session
	updatedSession, err := s.repo.FindByID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated session: %w", err)
	}

	s.broadcastTimeTrackingChanged(ctx)
	return updatedSession, nil
}

// GetSessionBreaks returns all breaks for a given session
func (s *workSessionService) GetSessionBreaks(ctx context.Context, staffID, sessionID int64) ([]*activeModels.WorkSessionBreak, error) {
	// Verify ownership: session must belong to requesting staff
	session, err := s.repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, s.handleSessionNotFoundError(err)
	}
	if session.StaffID != staffID {
		return nil, fmt.Errorf("session does not belong to requesting staff")
	}

	breaks, err := s.breakRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session breaks: %w", err)
	}
	return breaks, nil
}

// recalcBreakMinutes sums the ENDED break durations for a session and updates
// the cache. A still-running break contributes nothing: its elapsed time is
// live data, and readers that need it add it themselves against their own
// clock (netMinutes + runningBreakMinutes in the month service, the live
// session card). Folding the running duration in here would double-count it —
// once from the cache, once from the reader — for every recalc that happens
// while a break is open (editing an ended break on a session whose second
// break is still running), and would leave a stale snapshot in the cache the
// moment the recalc returned.
func (s *workSessionService) recalcBreakMinutes(ctx context.Context, sessionID int64) error {
	breaks, err := s.breakRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get breaks for recalc: %w", err)
	}

	totalMinutes := 0
	for _, brk := range breaks {
		if brk.EndedAt != nil {
			totalMinutes += brk.DurationMinutes
		}
	}

	return s.repo.UpdateBreakMinutes(ctx, sessionID, totalMinutes)
}

// sessionUpdateContext holds state during session update to avoid passing many parameters.
type sessionUpdateContext struct {
	session    *activeModels.WorkSession
	sessionID  int64
	staffID    int64
	now        time.Time
	auditEdits []*auditModels.WorkSessionEdit
	notes      *string
}

func (uc *sessionUpdateContext) addAuditEdit(field string, oldVal, newVal *string) {
	uc.auditEdits = append(uc.auditEdits, &auditModels.WorkSessionEdit{
		SessionID: uc.sessionID,
		StaffID:   uc.session.StaffID,
		EditedBy:  uc.staffID,
		FieldName: field,
		OldValue:  oldVal,
		NewValue:  newVal,
		Notes:     uc.notes,
		CreatedAt: uc.now,
	})
}

// UpdateSession updates a work session with the provided fields and creates
// audit entries. Self-edit path: the requesting staff must own the session.
// Notes are only required when changing status (Vor Ort ↔ Homeoffice).
func (s *workSessionService) UpdateSession(ctx context.Context, staffID int64, sessionID int64, updates SessionUpdateRequest) (*activeModels.WorkSession, error) {
	if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	session, err := s.repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, s.handleSessionNotFoundError(err)
	}

	if session.StaffID != staffID {
		return nil, fmt.Errorf("can only update own sessions")
	}

	// A status change (Vor Ort ↔ Homeoffice) carries audit weight and must be
	// justified. Otherwise the audit trail loses meaning. Requires the caller
	// to send the reason in `notes` on the same request.
	if updates.Status != nil && *updates.Status != session.Status {
		if updates.Notes == nil || strings.TrimSpace(*updates.Notes) == "" {
			return nil, fmt.Errorf("notes required when changing status")
		}
	}

	// F8: with the deviation-reason setting active, self-edits of the
	// recorded times (check-in, check-out, break minutes — including
	// per-break duration edits, which are break-minute changes) need notes
	// too, not only status changes. For the scalar fields the change
	// detection mirrors applyTimeFieldUpdates/applyBreakUpdates, so an
	// unchanged resend never trips the gate; a non-empty per-break list
	// always counts as an edit intent (comparing it against stored rows
	// would need an extra query before the ownership checks ran).
	if s.selfEditChangesRecordedTimes(session, updates) {
		if updates.Notes == nil || strings.TrimSpace(*updates.Notes) == "" {
			required, err := s.deviationReasonRequired(ctx)
			if err != nil {
				return nil, err
			}
			if required {
				return nil, fmt.Errorf("notes required when changing recorded times")
			}
		}
	}

	return s.applySessionUpdate(ctx, staffID, session, updates)
}

// deviationReasonRequired resolves the F8/F9 tenant setting; nil settings
// (tests, minimal wiring) mean the gate is off.
func (s *workSessionService) deviationReasonRequired(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	enabled, err := s.settings.ResolveBool(ctx, configModels.KeyTimeTrackingRequireDeviationReason)
	if err != nil {
		return false, fmt.Errorf("failed to resolve deviation-reason setting: %w", err)
	}
	return enabled, nil
}

// selfEditChangesRecordedTimes reports whether the update would change
// check_in_time, check_out_time, or break minutes on the session.
func (s *workSessionService) selfEditChangesRecordedTimes(session *activeModels.WorkSession, updates SessionUpdateRequest) bool {
	if updates.CheckInTime != nil && !session.CheckInTime.Equal(*updates.CheckInTime) {
		return true
	}
	if updates.CheckOutTime != nil && (session.CheckOutTime == nil || !session.CheckOutTime.Equal(*updates.CheckOutTime)) {
		return true
	}
	if updates.BreakMinutes != nil && *updates.BreakMinutes != session.BreakMinutes {
		return true
	}
	return len(updates.Breaks) > 0
}

// UpdateSessionAsAdmin applies an admin correction. editorStaffID is the
// admin doing the edit (goes into edited_by). targetStaffID is the owner of
// the session we expect to mutate, verified against session.StaffID so the
// route can't be used to leak edits across staff in the same tenant.
//
// Notes are unconditionally required: BAG demands "Verlässlichkeit" of the
// audit trail, and any foreign edit needs a reason. We're stricter than
// self-edit on purpose.
func (s *workSessionService) UpdateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID, sessionID int64, updates SessionUpdateRequest) (*activeModels.WorkSession, error) {
	if err := s.lockStaffBalanceWrites(ctx, targetStaffID); err != nil {
		return nil, err
	}
	session, err := s.repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, s.handleSessionNotFoundError(err)
	}

	if session.StaffID != targetStaffID {
		return nil, fmt.Errorf("session does not belong to staff %d", targetStaffID)
	}

	if updates.Notes == nil || strings.TrimSpace(*updates.Notes) == "" {
		return nil, fmt.Errorf("notes required for admin edits")
	}

	return s.applySessionUpdate(ctx, editorStaffID, session, updates)
}

// applySessionUpdate is the shared body used by both self-edit and admin
// edit. editorStaffID lands in audit.work_session_edits.edited_by so the
// MA-side can distinguish "ich selbst" from "Florian (Admin)" without an
// extra column.
func (s *workSessionService) applySessionUpdate(ctx context.Context, editorStaffID int64, session *activeModels.WorkSession, updates SessionUpdateRequest) (*activeModels.WorkSession, error) {
	uc := &sessionUpdateContext{
		session:   session,
		sessionID: session.ID,
		staffID:   editorStaffID,
		now:       time.Now(),
		notes:     updates.Notes,
	}

	s.applyTimeFieldUpdates(uc, updates)
	s.applySimpleFieldUpdates(uc, updates)

	session.UpdatedBy = &editorStaffID
	session.UpdatedAt = uc.now

	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf(errInvalidSessionData, err)
	}

	// Edited times must not slide into a sibling block.
	if err := s.assertNoBlockOverlap(ctx, session.StaffID, session.ID, session.CheckInTime, session.CheckOutTime); err != nil {
		return nil, err
	}

	// Individual break edits write their rows immediately. Do that only after
	// validation and the sibling-block check, so callers without an enclosing
	// transaction cannot retain a partial break update after a rejected edit.
	if err := s.applyBreakUpdates(ctx, uc, updates); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	if len(uc.auditEdits) > 0 {
		for _, edit := range uc.auditEdits {
			edit.SetTenantID(tenant.FromContext(ctx))
		}
		if err := s.auditRepo.CreateBatch(ctx, uc.auditEdits); err != nil {
			return nil, fmt.Errorf("failed to create audit entries: %w", err)
		}
	}

	s.broadcastTimeTrackingChanged(ctx)
	return session, nil
}

// CreateSessionAsAdmin records a "nachgetragene" session for another staff
// member. Each field that ends up on the new row is also captured as an
// audit edit (old_value = NULL, new_value = field) so the MA-side audit log
// shows the create as a series of explicit "field war leer → field ist jetzt
// X" entries. Notes are stored both on the session and on every audit row
// (consistent with edit semantics).
func (s *workSessionService) CreateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID int64, req AdminCreateSessionRequest) (*activeModels.WorkSession, error) {
	if targetStaffID <= 0 {
		return nil, fmt.Errorf("target staff id is required")
	}
	if strings.TrimSpace(req.Notes) == "" {
		return nil, fmt.Errorf("notes required for admin-created sessions")
	}
	if req.CheckInTime.IsZero() || req.CheckOutTime.IsZero() {
		return nil, fmt.Errorf("check-in and check-out are required")
	}
	if !req.CheckOutTime.After(req.CheckInTime) {
		return nil, fmt.Errorf("check_out_time must be after check_in_time")
	}
	if req.Status == "" {
		req.Status = activeModels.WorkSessionStatusPresent
	}
	if req.BreakMinutes < 0 {
		return nil, fmt.Errorf("break_minutes must not be negative")
	}
	if err := s.lockStaffBalanceWrites(ctx, targetStaffID); err != nil {
		return nil, err
	}

	checkOut := req.CheckOutTime
	date := timezone.DateFromTime(req.Date)
	// A day can carry several blocks since #2402, so the Nachtrag no longer
	// collides with an existing session per se — but it must not overlap one.
	if err := s.assertNoBlockOverlap(ctx, targetStaffID, 0, req.CheckInTime, &checkOut); err != nil {
		return nil, err
	}
	session := &activeModels.WorkSession{
		StaffID:      targetStaffID,
		Date:         date,
		Status:       req.Status,
		Source:       activeModels.WorkSessionSourceApp,
		CheckInTime:  req.CheckInTime,
		CheckOutTime: &checkOut,
		BreakMinutes: req.BreakMinutes,
		Notes:        req.Notes,
		CreatedBy:    editorStaffID,
		UpdatedBy:    &editorStaffID,
	}
	session.SetTenantID(tenant.FromContext(ctx))

	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf(errInvalidSessionData, err)
	}

	if err := s.repo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Audit trail: one entry per field so the EditHistoryAccordion can render
	// the create as a normal change list with empty "Vorher" values.
	now := time.Now()
	notesPtr := req.Notes
	strPtr := func(s string) *string { return &s }
	tenantID := tenant.FromContext(ctx)
	baseEdit := func(field string, newVal string) *auditModels.WorkSessionEdit {
		e := &auditModels.WorkSessionEdit{
			SessionID: session.ID,
			StaffID:   targetStaffID,
			EditedBy:  editorStaffID,
			FieldName: field,
			OldValue:  nil,
			NewValue:  strPtr(newVal),
			Notes:     &notesPtr,
			CreatedAt: now,
		}
		e.SetTenantID(tenantID)
		return e
	}
	edits := []*auditModels.WorkSessionEdit{
		baseEdit(auditModels.FieldDate, date.String()),
		baseEdit(auditModels.FieldCheckInTime, req.CheckInTime.Format(time.RFC3339)),
		baseEdit(auditModels.FieldCheckOutTime, req.CheckOutTime.Format(time.RFC3339)),
		baseEdit(auditModels.FieldBreakMinutes, fmt.Sprintf("%d", req.BreakMinutes)),
		baseEdit(auditModels.FieldStatus, req.Status),
	}
	if err := s.auditRepo.CreateBatch(ctx, edits); err != nil {
		return nil, fmt.Errorf("failed to create audit entries: %w", err)
	}

	s.broadcastTimeTrackingChanged(ctx)
	return session, nil
}

// assertNoBlockOverlap rejects a block whose [check-in, check-out) interval
// overlaps another block of the same staff member (#2402). Overlapping blocks
// would double-count work time in every sum built from the day's rows.
// Touching boundaries (one block ends exactly when the next starts) are
// allowed; an open sibling block occupies [check-in, ∞) — including one whose
// live limit has passed. Only live stamping softens that, see
// assertNoRunningBlockOverlap.
//
// The siblings are selected by their timestamps, not by the date column: a
// block is filed on the day of its check-in but may reach into later days (a
// night block, an auto-checkout that never ran, an admin Nachtrag reaching
// into tomorrow). Any date window — even a generous one — would miss a block
// that started days earlier and is still running, so the query asks the
// database for exactly the intersecting rows instead.
func (s *workSessionService) assertNoBlockOverlap(ctx context.Context, staffID int64, excludeID int64, checkIn time.Time, checkOut *time.Time) error {
	siblings, err := s.repo.ListOverlappingByStaffID(ctx, staffID, checkIn, checkOut)
	if err != nil {
		return fmt.Errorf("failed to load sessions for overlap check: %w", err)
	}
	return assertNoBlockOverlapIn(siblings, excludeID, checkIn, checkOut)
}

// closeStaleOpenBlock ends a forgotten checkout at the instant it stopped
// counting as work (BalanceSessionEnd) and marks it as auto-checked-out, the
// same repair the nightly cleanup performs (CleanupOpenSessions). Live
// stamping does it inline because the open-block guard in the database is
// stricter than the arithmetic: the balance simply stops crediting a stale
// block, but uq_work_sessions_staff_date_open keeps rejecting a second open
// row until this one is closed.
//
// The asymmetry against the administrative writes is deliberate. A stamping
// staff member has no way to repair the row and would be locked out of the
// clock entirely (#2402); an admin IS the person who repairs it, so there a
// Nachtrag keeps being refused while an open block is unresolved.
func (s *workSessionService) closeStaleOpenBlock(ctx context.Context, session *activeModels.WorkSession, at time.Time) error {
	if err := s.endActiveBreakIfExists(ctx, session.ID, at); err != nil {
		// Mirrors CleanupOpenSessions: a break that cannot be ended must not
		// keep the block open — that would leave the staff member unable to
		// stamp, which is the very lockout this repair exists to prevent.
		s.getLogger().WarnContext(ctx, "failed to end active break of a stale work session",
			slog.Int64("session_id", session.ID),
			slog.String("error", err.Error()))
	}
	if _, err := s.repo.CloseSession(ctx, session.ID, at, true); err != nil {
		return fmt.Errorf("failed to close stale work session %d: %w", session.ID, err)
	}
	s.getLogger().InfoContext(ctx, "closed stale open work session before new check-in",
		slog.Int64("session_id", session.ID),
		slog.Time("closed_at", at))
	return nil
}

// ExpireStaleOpenBlock decides whether an open block is still running. A
// forgotten checkout stops counting as work at its live limit
// (BalanceSessionEnd): the balance no longer credits it, the presence map no
// longer reports its owner as present, and the open-session lookup no longer
// returns it. It therefore reports `true` together with a COPY closed at that
// limit, so every reader that has to answer "is this person clocked in right
// now" — the check-in guard, the overlap arithmetic, the kiosk state — cuts at
// the same instant instead of each inventing its own. A block that is closed,
// or open and still inside its window, is returned unchanged with `false`.
//
// The original row is never modified; closing it for real is the check-in's
// job (closeStaleOpenBlock) or the nightly auto-checkout's.
func ExpireStaleOpenBlock(session *activeModels.WorkSession, now time.Time) (*activeModels.WorkSession, bool) {
	if session == nil || session.CheckOutTime != nil {
		return session, false
	}
	end := BalanceSessionEnd(session, now)
	if !end.Before(now) {
		return session, false // still running
	}
	clamped := *session
	clamped.CheckOutTime = &end
	return &clamped, true
}

// expireStaleOpenBlocks applies ExpireStaleOpenBlock to a whole list, leaving
// closed and still-running blocks untouched.
func expireStaleOpenBlocks(siblings []*activeModels.WorkSession, now time.Time) []*activeModels.WorkSession {
	expired := make([]*activeModels.WorkSession, len(siblings))
	for i, sibling := range siblings {
		expired[i], _ = ExpireStaleOpenBlock(sibling, now)
	}
	return expired
}

// assertNoBlockOverlapIn is the list-based body of assertNoBlockOverlap, kept
// separate so the interval arithmetic can be exercised without a database.
func assertNoBlockOverlapIn(siblings []*activeModels.WorkSession, excludeID int64, checkIn time.Time, checkOut *time.Time) error {
	for _, sibling := range siblings {
		if sibling.ID == excludeID {
			continue
		}
		if checkOut != nil && !checkOut.After(sibling.CheckInTime) {
			continue // new block ends before (or exactly when) the sibling starts
		}
		if sibling.CheckOutTime != nil && !sibling.CheckOutTime.After(checkIn) {
			continue // sibling ends before (or exactly when) the new block starts
		}
		return fmt.Errorf("work session overlaps an existing block (%s–%s)",
			sibling.CheckInTime.In(timezone.Berlin).Format("15:04"),
			formatBlockEnd(sibling.CheckOutTime))
	}
	return nil
}

func formatBlockEnd(checkOut *time.Time) string {
	if checkOut == nil {
		return "offen"
	}
	return checkOut.In(timezone.Berlin).Format("15:04")
}

func (s *workSessionService) handleSessionNotFoundError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("session not found")
	}
	return fmt.Errorf("failed to get session: %w", err)
}

func (s *workSessionService) applyTimeFieldUpdates(uc *sessionUpdateContext, updates SessionUpdateRequest) {
	strPtr := func(str string) *string { return &str }

	if updates.Date != nil {
		// The wire carries an RFC3339 instant; only its Berlin calendar day
		// matters for the DATE column.
		newDate := timezone.DateFromTime(*updates.Date)
		if uc.session.Date != newDate {
			uc.addAuditEdit(auditModels.FieldDate, strPtr(uc.session.Date.String()), strPtr(newDate.String()))
			uc.session.Date = newDate
		}
	}

	if updates.CheckInTime != nil {
		// Compare actual time points, not string representations (timezone-safe)
		if !uc.session.CheckInTime.Equal(*updates.CheckInTime) {
			oldVal := uc.session.CheckInTime.Format(time.RFC3339)
			newVal := updates.CheckInTime.Format(time.RFC3339)
			uc.addAuditEdit(auditModels.FieldCheckInTime, strPtr(oldVal), strPtr(newVal))
		}
		uc.session.CheckInTime = *updates.CheckInTime
	}

	if updates.CheckOutTime != nil {
		// Compare actual time points, not string representations (timezone-safe)
		oldTime := uc.session.CheckOutTime
		changed := oldTime == nil || !oldTime.Equal(*updates.CheckOutTime)
		if changed {
			var oldVal string
			if oldTime != nil {
				oldVal = oldTime.Format(time.RFC3339)
			}
			newVal := updates.CheckOutTime.Format(time.RFC3339)
			uc.addAuditEdit(auditModels.FieldCheckOutTime, strPtr(oldVal), strPtr(newVal))
		}
		uc.session.CheckOutTime = updates.CheckOutTime
	}
}

func (s *workSessionService) applyBreakUpdates(ctx context.Context, uc *sessionUpdateContext, updates SessionUpdateRequest) error {
	strPtr := func(str string) *string { return &str }

	if len(updates.Breaks) > 0 {
		return s.processIndividualBreakUpdates(ctx, uc, updates.Breaks, strPtr)
	}

	if updates.BreakMinutes != nil {
		oldVal := strconv.Itoa(uc.session.BreakMinutes)
		newVal := strconv.Itoa(*updates.BreakMinutes)
		if oldVal != newVal {
			uc.addAuditEdit(auditModels.FieldBreakMinutes, strPtr(oldVal), strPtr(newVal))
		}
		uc.session.BreakMinutes = *updates.BreakMinutes
	}
	return nil
}

func (s *workSessionService) processIndividualBreakUpdates(ctx context.Context, uc *sessionUpdateContext, breaks []BreakDurationUpdate, strPtr func(string) *string) error {
	sessionBreaks, err := s.breakRepo.GetBySessionID(ctx, uc.sessionID)
	if err != nil {
		return fmt.Errorf("failed to load session breaks: %w", err)
	}

	breakMap := make(map[int64]*activeModels.WorkSessionBreak, len(sessionBreaks))
	for _, b := range sessionBreaks {
		breakMap[b.ID] = b
	}

	for _, bu := range breaks {
		if err := s.updateSingleBreak(ctx, uc, breakMap, bu, strPtr); err != nil {
			return err
		}
	}

	if err := s.recalcBreakMinutes(ctx, uc.sessionID); err != nil {
		return fmt.Errorf("failed to recalculate break minutes: %w", err)
	}

	// Keep the in-memory session consistent without reloading it. Reloading
	// would discard time and status edits already validated above.
	uc.session.BreakMinutes = 0
	for _, brk := range sessionBreaks {
		if brk.EndedAt != nil {
			uc.session.BreakMinutes += brk.DurationMinutes
		}
	}
	return nil
}

func (s *workSessionService) updateSingleBreak(ctx context.Context, uc *sessionUpdateContext, breakMap map[int64]*activeModels.WorkSessionBreak, bu BreakDurationUpdate, strPtr func(string) *string) error {
	brk, ok := breakMap[bu.ID]
	if !ok {
		return fmt.Errorf("break %d does not belong to this session", bu.ID)
	}
	if brk.EndedAt == nil {
		return fmt.Errorf("cannot edit duration of an active break")
	}

	if bu.DurationMinutes < 0 {
		return fmt.Errorf("break duration cannot be negative")
	}

	if brk.DurationMinutes == bu.DurationMinutes {
		return nil
	}

	oldVal := strconv.Itoa(brk.DurationMinutes)
	newEndedAt := brk.StartedAt.Add(time.Duration(bu.DurationMinutes) * time.Minute)
	if err := s.breakRepo.UpdateDuration(ctx, bu.ID, bu.DurationMinutes, newEndedAt); err != nil {
		return fmt.Errorf("failed to update break %d: %w", bu.ID, err)
	}
	brk.DurationMinutes = bu.DurationMinutes
	brk.EndedAt = &newEndedAt

	newVal := strconv.Itoa(bu.DurationMinutes)
	uc.addAuditEdit(auditModels.FieldBreakDuration, strPtr(oldVal), strPtr(newVal))
	return nil
}

func (s *workSessionService) applySimpleFieldUpdates(uc *sessionUpdateContext, updates SessionUpdateRequest) {
	strPtr := func(str string) *string { return &str }

	if updates.Status != nil && uc.session.Status != *updates.Status {
		uc.addAuditEdit(auditModels.FieldStatus, strPtr(uc.session.Status), updates.Status)
		uc.session.Status = *updates.Status
	}

	if updates.Notes != nil && uc.session.Notes != *updates.Notes {
		uc.addAuditEdit(auditModels.FieldNotes, strPtr(uc.session.Notes), updates.Notes)
		uc.session.Notes = *updates.Notes
	}
}

// GetCurrentSession returns the current active session for a staff member
func (s *workSessionService) GetCurrentSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	session, err := s.repo.GetCurrentByStaffID(ctx, staffID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf(errGetCurrentSession, err)
	}

	return session, nil
}

// GetLatestOpenSession returns the most recent still-running session of a staff
// member across all days, or nil when none is open. It answers "is this person
// clocked in right now" without assuming the session was opened today — a night
// stamp survives the Berlin midnight rollover.
func (s *workSessionService) GetLatestOpenSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	session, err := s.repo.GetLatestOpenByStaffID(ctx, staffID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest open session: %w", err)
	}

	return session, nil
}

// GetHistory returns work sessions for a staff member in a date range with weekly aggregation
func (s *workSessionService) GetHistory(ctx context.Context, staffID int64, from, to timezone.Date) (*HistoryResponse, error) {
	sessions, err := s.repo.GetHistoryByStaffID(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get session history: %w", err)
	}
	return s.historyResponse(ctx, staffID, sessions, from, to)
}

// GetHistoryIntersecting returns blocks that overlap a calendar range. Every
// consumer that lays minutes out on calendar days uses it: the history tables,
// the kiosk day state, the balances. GetHistory keeps the stored-date contract
// for the export, where a block is a single row under its start day.
func (s *workSessionService) GetHistoryIntersecting(ctx context.Context, staffID int64, from, to timezone.Date) (*HistoryResponse, error) {
	end := to.AddDays(1).BerlinMidnight()
	sessions, err := s.repo.ListOverlappingByStaffID(ctx, staffID, from.BerlinMidnight(), &end)
	if err != nil {
		return nil, fmt.Errorf("failed to get intersecting session history: %w", err)
	}
	return s.historyResponse(ctx, staffID, sessions, from, to)
}

// historyResponse builds the wire response for the blocks of [from, to]. The
// range is carried through because the weekly summaries are aggregated per
// calendar day: a block that reaches beyond the requested range (a night block
// at either border) must not contribute the minutes it spends outside it.
func (s *workSessionService) historyResponse(ctx context.Context, staffID int64, sessions []*activeModels.WorkSession, from, to timezone.Date) (*HistoryResponse, error) {

	// Collect session IDs for batch edit count query
	sessionIDs := make([]int64, len(sessions))
	for i, session := range sessions {
		sessionIDs[i] = session.ID
	}

	// Batch fetch manual edit counts. System-authored edits (auto-checkout) and
	// stamp-time deviation reasons (#1844) are excluded so they don't surface
	// as "Manuell korrigiert".
	editCounts, err := s.auditRepo.CountManualBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get edit counts: %w", err)
	}
	auditCounts, err := s.auditRepo.CountBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit counts: %w", err)
	}
	breaksBySession, err := s.breakRepo.GetBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get session breaks: %w", err)
	}
	// Some narrow repository adapters return a nil map for unsupported bulk
	// reads. Preserve the single-row contract for those adapters; the database
	// repository always returns a non-nil map and stays on the flat query path.
	if breaksBySession == nil {
		breaksBySession = make(map[int64][]*activeModels.WorkSessionBreak, len(sessionIDs))
		for _, sessionID := range sessionIDs {
			breaks, lookupErr := s.breakRepo.GetBySessionID(ctx, sessionID)
			if lookupErr != nil {
				return nil, fmt.Errorf("failed to get breaks for session %d: %w", sessionID, lookupErr)
			}
			breaksBySession[sessionID] = breaks
		}
	}

	// Wrap each session in SessionResponse with calculated fields and breaks
	now := s.now()
	responses := make([]*SessionResponse, len(sessions))
	for i, session := range sessions {
		breaks := breaksBySession[session.ID]

		// The as-of clock of a block is its own end, not the wall clock: an
		// open block that has passed its live limit stopped counting there
		// (BalanceSessionEnd), and work AND break time have to be measured
		// against that same instant. Pinning a fake check-out on a copy
		// instead would silence totalBreakMinutes — it treats a checked-out
		// block's cache as complete — and a break still running across the
		// limit would be booked as work (#2402).
		asOf := BalanceSessionEnd(session, now)
		responses[i] = &SessionResponse{
			WorkSession: session,
			// A running break is NOT in the BreakMinutes cache, so netMinutes
			// alone would keep counting it as worked time and the day rows
			// would climb while the Monatskarte and the week KPI (which both
			// deduct it) stand still (#1842). The breaks are already loaded
			// above — no extra query.
			BreakMinutes:     totalBreakMinutes(session, breaks, asOf),
			NetMinutes:       netMinutesWithBreaks(session, breaks, asOf),
			IsOvertime:       isOvertime(session, breaks, asOf),
			IsBreakCompliant: isBreakCompliant(session, breaks, asOf),
			Breaks:           breaks,
			EditCount:        editCounts[session.ID],
			AuditCount:       auditCounts[session.ID],
		}
	}

	targetsByWeek := s.getWeeklyTargetsForSummaries(ctx, staffID, responses)

	// Build weekly summaries
	weeklySummaries := s.buildWeeklySummaries(responses, targetsByWeek, from, to)

	return &HistoryResponse{
		Sessions:        responses,
		WeeklySummaries: weeklySummaries,
	}, nil
}

// buildWeeklySummaries aggregates session data by ISO week, restricted to the
// requested [from, to] range.
//
// A block is laid out on the Berlin days its minutes actually fall on, and it
// may reach past either border of the range: the intersecting query returns a
// night block that began before `from`, and a block started on `to` runs into
// the next morning. Aggregating it whole would credit weeks the caller never
// asked about — including ISO weeks with no visible row to explain them — so
// only the days inside the range are counted (#2402).
func (s *workSessionService) buildWeeklySummaries(sessions []*SessionResponse, targetsByWeek map[summaryWeekKey]int, from, to timezone.Date) []WeeklySummary {
	weekMap := make(map[summaryWeekKey]*WeeklySummary)
	var weekOrder []summaryWeekKey
	now := time.Now()

	for _, session := range sessions {
		end := BalanceSessionEnd(session.WorkSession, now)
		countedWeeks := make(map[summaryWeekKey]struct{})
		// The first day is derived from the check-in instant, never from the
		// stored date: an admin Nachtrag files a block under a date it may
		// choose freely (CreateSessionAsAdmin), so a block stamped 23:00 and
		// filed on the following day would lose its pre-midnight minutes here
		// while the balance — which starts at the check-in — still counts them.
		firstDay, lastDay := timezone.DateFromTime(session.CheckInTime), timezone.DateFromTime(end)
		if firstDay.Before(from) {
			firstDay = from
		}
		if lastDay.After(to) {
			lastDay = to
		}
		for day, minutes := range netMinutesByDate(session.WorkSession, session.Breaks, end, firstDay, lastDay) {
			year, week := day.UTCMidnight().ISOWeek()
			key := summaryWeekKey{Year: year, Week: week}

			summary, exists := weekMap[key]
			if !exists {
				var targetMinutes *int
				if target, ok := targetsByWeek[key]; ok {
					targetCopy := target
					targetMinutes = &targetCopy
				}
				summary = &WeeklySummary{
					WeekNumber:    week,
					Year:          year,
					TargetMinutes: targetMinutes,
				}
				weekMap[key] = summary
				weekOrder = append(weekOrder, key)
			}

			summary.TotalNetMinutes += minutes
			if _, counted := countedWeeks[key]; !counted {
				summary.SessionCount++
				countedWeeks[key] = struct{}{}
			}
		}
	}

	// Convert to sorted slice and compute derived fields
	const maxWeeklyMinutes = 48 * 60 // §3 ArbZG
	summaries := make([]WeeklySummary, 0, len(weekOrder))
	for _, key := range weekOrder {
		ws := weekMap[key]
		ws.IsOverWeeklyMax = ws.TotalNetMinutes > maxWeeklyMinutes

		if ws.TargetMinutes != nil {
			delta := ws.TotalNetMinutes - *ws.TargetMinutes
			ws.DeltaMinutes = &delta
		}

		summaries = append(summaries, *ws)
	}

	return summaries
}

func (s *workSessionService) getWeeklyTargetsForSummaries(ctx context.Context, staffID int64, sessions []*SessionResponse) map[summaryWeekKey]int {
	if len(sessions) == 0 {
		return nil
	}
	holidaySet, ok := s.holidayDatesForWeeks(ctx, sessionWeekStarts(sessions))
	if !ok {
		return nil
	}
	staff := s.resolveStaffForTargets(ctx, staffID)
	if s.scheduleRepo != nil {
		targets := s.weeklyTargetsFromDateValidSchedule(ctx, staffID, staff, sessions, holidaySet)
		if len(targets) > 0 {
			return targets
		}
	}
	if staff != nil && staff.WorkTimeModelID != nil {
		if s.workModelRepo == nil {
			return nil
		}
		model, err := s.workModelRepo.FindByID(ctx, *staff.WorkTimeModelID)
		if err != nil {
			return nil
		}
		anchor := model.RotationAnchorDate
		if staff.RotationAnchorDate != nil {
			anchor = workforceDate(*staff.RotationAnchorDate)
		}
		targets := configModels.WeeklyTargetsFromModel(model, anchor, workforceDates(sessionWeekStarts(sessions)))
		for weekStart, target := range targets {
			targets[weekStart] = target - holidayModelMinutes(model, anchor, calendarDate(weekStart), holidaySet)
		}
		converted := make(map[timezone.Date]int, len(targets))
		for weekStart, target := range targets {
			converted[calendarDate(weekStart)] = target
		}
		return summaryKeysOf(converted)
	}
	return nil
}

// holidayDatesForWeeks loads the public holidays covering the given weeks.
// ok=false signals a resolver failure — then NO weekly Soll is shown rather
// than one that ignores holidays and fakes minus hours (#1418 3a).
func (s *workSessionService) holidayDatesForWeeks(ctx context.Context, weekStarts []timezone.Date) (map[timezone.Date]bool, bool) {
	if s.holidayReader == nil || len(weekStarts) == 0 {
		return nil, true
	}
	from, to := weekStarts[0], weekStarts[0]
	for _, weekStart := range weekStarts[1:] {
		if weekStart.Before(from) {
			from = weekStart
		}
		if weekStart.After(to) {
			to = weekStart
		}
	}
	set, err := s.holidayReader.HolidayDates(ctx, from, to.AddDays(6))
	if err != nil {
		s.getLogger().Warn("failed to load public holidays for weekly summaries",
			"error", err.Error(),
		)
		return nil, false
	}
	return set, true
}

// holidayScheduleMinutes sums the schedule Soll of the week's holiday days —
// the amount a holiday week's target shrinks by.
func holidayScheduleMinutes(entries []*configModels.StaffWorkSchedule, staffAnchor *timezone.Date, weekStart timezone.Date, holidaySet map[timezone.Date]bool) int {
	total := 0
	for offset := 0; offset < 7; offset++ {
		day := weekStart.AddDays(offset)
		if !holidaySet[day] {
			continue
		}
		dayTarget, _ := configModels.DailyTargetFromSchedule(entries, workforceDatePointer(staffAnchor), workforceDate(day))
		total += dayTarget
	}
	return total
}

// holidayModelMinutes is holidayScheduleMinutes for the work-time-model
// fallback path.
func holidayModelMinutes(model *configModels.WorkTimeModel, anchor configModels.CalendarDate, weekStart timezone.Date, holidaySet map[timezone.Date]bool) int {
	total := 0
	for offset := 0; offset < 7; offset++ {
		day := weekStart.AddDays(offset)
		if !holidaySet[day] {
			continue
		}
		dayTarget, _ := configModels.DailyTargetFromModel(model, anchor, workforceDate(day))
		total += dayTarget
	}
	return total
}

func (s *workSessionService) resolveStaffForTargets(ctx context.Context, staffID int64) *userModels.Staff {
	if s.staffRepo == nil {
		return nil
	}
	staff, err := s.staffRepo.FindByID(ctx, staffID)
	if err != nil {
		return nil
	}
	return staff
}

// weeklyTargetsFromDateValidSchedule resolves the weekly Soll from date-valid
// staff_work_schedules rows via the shared models/config helpers: one batched
// read over the span of all session weeks, then per-week in-memory resolution
// (validity windows, rotation weeks) through config.WeeklyTargetFromSchedule —
// the same helper the Dienstplan weekly summaries use.
func (s *workSessionService) weeklyTargetsFromDateValidSchedule(
	ctx context.Context,
	staffID int64,
	staff *userModels.Staff,
	sessions []*SessionResponse,
	holidaySet map[timezone.Date]bool,
) map[summaryWeekKey]int {
	weekStarts := sessionWeekStarts(sessions)
	if len(weekStarts) == 0 {
		return nil
	}
	from, to := weekStarts[0], weekStarts[0]
	for _, weekStart := range weekStarts[1:] {
		if weekStart.Before(from) {
			from = weekStart
		}
		if weekStart.After(to) {
			to = weekStart
		}
	}
	entries, err := s.scheduleRepo.FindByStaffIDsValidInRange(ctx, []int64{staffID}, workforceDate(from), workforceDate(to.AddDays(6)))
	if err != nil || len(entries) == 0 {
		return nil
	}
	targetsByWeek := make(map[summaryWeekKey]int)
	for _, weekStart := range weekStarts {
		if target, ok := configModels.WeeklyTargetFromSchedule(entries, workforceDatePointer(staffAnchorOf(staff)), workforceDate(weekStart)); ok {
			target -= holidayScheduleMinutes(entries, staffAnchorOf(staff), weekStart, holidaySet)
			targetsByWeek[summaryKeyOf(weekStart)] = target
		}
	}
	if len(targetsByWeek) == 0 {
		return nil
	}
	return targetsByWeek
}

// staffAnchorOf returns the staff-level rotation anchor when one is set.
func staffAnchorOf(staff *userModels.Staff) *timezone.Date {
	if staff == nil {
		return nil
	}
	return staff.RotationAnchorDate
}

// sessionWeekStarts returns the deduplicated Monday week starts of the given
// sessions in first-seen order.
func sessionWeekStarts(sessions []*SessionResponse) []timezone.Date {
	weekStarts := make([]timezone.Date, 0)
	seen := make(map[timezone.Date]struct{})
	now := time.Now()
	for _, session := range sessions {
		end := BalanceSessionEnd(session.WorkSession, now)
		for day := timezone.DateFromTime(session.CheckInTime); !day.After(timezone.DateFromTime(end)); day = day.AddDays(1) {
			weekStart := calendarDate(configModels.MondayOf(workforceDate(day)))
			if _, ok := seen[weekStart]; ok {
				continue
			}
			seen[weekStart] = struct{}{}
			weekStarts = append(weekStarts, weekStart)
		}
	}
	return weekStarts
}

// summaryKeyOf maps a Monday week start onto the ISO year/week summary key
// (a Monday uniquely determines its ISO week, so the mapping is loss-free).
func summaryKeyOf(weekStart timezone.Date) summaryWeekKey {
	year, week := weekStart.UTCMidnight().ISOWeek()
	return summaryWeekKey{Year: year, Week: week}
}

// summaryKeysOf rekeys the Monday-date targets of the shared model resolver
// onto summary week keys.
func summaryKeysOf(targets map[timezone.Date]int) map[summaryWeekKey]int {
	if len(targets) == 0 {
		return nil
	}
	targetsByWeek := make(map[summaryWeekKey]int, len(targets))
	for weekStart, target := range targets {
		targetsByWeek[summaryKeyOf(weekStart)] = target
	}
	return targetsByWeek
}

// GetSessionEdits returns the audit trail for a work session
func (s *workSessionService) GetSessionEdits(ctx context.Context, staffID, sessionID int64) ([]*WorkSessionEditView, error) {
	// Verify ownership: session must belong to requesting staff
	session, err := s.repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, s.handleSessionNotFoundError(err)
	}
	if session.StaffID != staffID {
		return nil, fmt.Errorf("session does not belong to requesting staff")
	}

	return s.loadSessionEditsView(ctx, session, sessionID)
}

// GetSessionEditsForStaff is the admin-facing counterpart. The session must
// belong to the staff member named in the URL. Without that check an admin
// could load any session's edits by guessing IDs (within the tenant, RLS
// still applies). Permission gating happens at the route level.
func (s *workSessionService) GetSessionEditsForStaff(ctx context.Context, staffID, sessionID int64) ([]*WorkSessionEditView, error) {
	session, err := s.repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, s.handleSessionNotFoundError(err)
	}
	if session.StaffID != staffID {
		return nil, fmt.Errorf("session does not belong to staff %d", staffID)
	}

	return s.loadSessionEditsView(ctx, session, sessionID)
}

// loadSessionEditsView is the shared body that loads edits and decorates
// them with editor display names. Names are resolved through a single batch
// staff+person query so we never N+1 the audit log.
func (s *workSessionService) loadSessionEditsView(ctx context.Context, session *activeModels.WorkSession, sessionID int64) ([]*WorkSessionEditView, error) {
	edits, err := s.auditRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session edits: %w", err)
	}
	if len(edits) == 0 {
		return []*WorkSessionEditView{}, nil
	}

	// Collect editor staff IDs and resolve them to names in one shot.
	editorIDSet := make(map[int64]struct{}, len(edits))
	for _, e := range edits {
		editorIDSet[e.EditedBy] = struct{}{}
	}
	editorIDs := slices.Collect(maps.Keys(editorIDSet))

	staffMap := map[int64]*userModels.Staff{}
	if s.staffRepo != nil {
		var err error
		staffMap, err = s.staffRepo.FindWithPersonByIDs(ctx, editorIDs)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("failed to resolve editor names for audit view",
					slog.String("error", err.Error()),
				)
			}
			staffMap = map[int64]*userModels.Staff{}
		}
	}
	if staffMap == nil {
		staffMap = map[int64]*userModels.Staff{}
	}

	views := make([]*WorkSessionEditView, len(edits))
	for i, e := range edits {
		name := ""
		if e.EditedBy == auditModels.SystemEditorID {
			name = "System"
		} else if staff, ok := staffMap[e.EditedBy]; ok && staff != nil && staff.Person != nil {
			name = strings.TrimSpace(staff.Person.FirstName + " " + staff.Person.LastName)
		}
		views[i] = &WorkSessionEditView{
			WorkSessionEdit: e,
			EditorName:      name,
			IsSelfEdit:      e.EditedBy == session.StaffID,
		}
	}
	return views, nil
}

// GetTodayPresenceMap returns a map of staff IDs to their work status for today
func (s *workSessionService) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	presenceMap, err := s.repo.GetTodayPresenceMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get today's presence map: %w", err)
	}

	return presenceMap, nil
}

// CleanupOpenSessions closes sessions still open from before yesterday. This
// leaves a full following calendar day for legitimate night blocks to be
// checked out normally before they become stale.
func (s *workSessionService) CleanupOpenSessions(ctx context.Context) (int, error) {
	// Today's Berlin calendar day for the PostgreSQL DATE column
	today := timezone.DateFromTime(s.now())

	// Keep sessions opened yesterday available through today. A night block
	// may cross midnight and must not be closed by the first cleanup run.
	openSessions, err := s.repo.GetOpenSessions(ctx, today.AddDays(-1))
	if err != nil {
		return 0, fmt.Errorf("failed to get open sessions: %w", err)
	}
	staffIDs := make([]int64, 0, len(openSessions))
	for _, session := range openSessions {
		staffIDs = append(staffIDs, session.StaffID)
	}
	if err := s.lockStaffBalanceWritesOrdered(ctx, staffIDs); err != nil {
		return 0, err
	}

	count := 0
	for _, session := range openSessions {
		// Set check-out time to 23:59:59 of the session date in Berlin timezone.
		endOfDay := session.Date.EndOfDay()

		// End any active break before closing the session
		if err := s.endActiveBreakIfExists(ctx, session.ID, endOfDay); err != nil {
			s.getLogger().WarnContext(ctx, "failed to end active break during cleanup",
				slog.Int64("session_id", session.ID),
				slog.String("error", err.Error()))
			// Continue cleanup even if break ending fails
		}

		closed, err := s.repo.CloseSession(ctx, session.ID, endOfDay, true)
		if err != nil {
			return count, fmt.Errorf("failed to close session %d: %w", session.ID, err)
		}
		if closed {
			count++
		}
	}

	if count > 0 {
		s.broadcastTimeTrackingChanged(ctx)
	}
	return count, nil
}

// AutoCheckoutDueSessions closes open sessions at their planned shift end (#1798).
func (s *workSessionService) AutoCheckoutDueSessions(ctx context.Context, grace time.Duration) (int, error) {
	if s.staffShiftRepo == nil {
		return 0, nil
	}

	now := s.now()
	// GetOpenSessions filters date < beforeDate; passing tomorrow includes
	// today's open sessions, which is where forgotten checkouts live.
	tomorrow := timezone.DateFromTime(now).AddDays(1)
	openSessions, err := s.repo.GetOpenSessions(ctx, tomorrow)
	if err != nil {
		return 0, fmt.Errorf("failed to get open sessions: %w", err)
	}
	if len(openSessions) == 0 {
		return 0, nil
	}
	staffIDs := make([]int64, 0, len(openSessions))
	for _, session := range openSessions {
		staffIDs = append(staffIDs, session.StaffID)
	}
	if err := s.lockStaffBalanceWritesOrdered(ctx, staffIDs); err != nil {
		return 0, err
	}

	// Batch the shift lookups per session date; latest shift end per staff wins.
	latestEndByDate := make(map[timezone.Date]map[int64]*scheduleModels.StaffShift)
	byDate := make(map[timezone.Date][]int64)
	for _, session := range openSessions {
		byDate[session.Date] = append(byDate[session.Date], session.StaffID)
	}
	for date, staffIDs := range byDate {
		shifts, err := s.staffShiftRepo.FindByStaffIDsAndDate(ctx, staffIDs, date)
		if err != nil {
			return 0, fmt.Errorf("failed to load shifts for %s: %w", date.String(), err)
		}
		latest := make(map[int64]*scheduleModels.StaffShift, len(shifts))
		for _, shift := range shifts {
			// A cancelled shift does not take place (#1841): never close a
			// session against the planned end of a shift the person is absent
			// for. The nightly cleanup handles a still-open session instead.
			if shift.Cancelled {
				continue
			}
			current, ok := latest[shift.StaffID]
			if !ok || shift.EndInstant().After(current.EndInstant()) {
				latest[shift.StaffID] = shift
			}
		}
		latestEndByDate[date] = latest
	}

	tenantID := tenant.FromContext(ctx)
	count := 0
	for _, listedSession := range openSessions {
		session, err := s.repo.LockOpenByIDForUpdate(ctx, listedSession.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.getLogger().InfoContext(ctx, "skipping auto-checkout side effects because session was already closed",
					slog.Int64("session_id", listedSession.ID),
					slog.Int64("staff_id", listedSession.StaffID))
				continue
			}
			return count, fmt.Errorf("failed to lock auto-checkout session %d: %w", listedSession.ID, err)
		}

		shift, ok := latestEndByDate[session.Date][session.StaffID]
		if !ok {
			continue // no shift planned that day — nightly cleanup handles it
		}
		closeAt := shift.EndInstant()
		if !now.After(closeAt.Add(grace)) {
			continue // not yet due
		}
		if !closeAt.After(autoCheckoutEffectiveStart(session)) {
			// Checked in or reopened after the planned shift end — never
			// fabricate a checkout before the effective work start.
			continue
		}

		breaks, err := s.breakRepo.GetBySessionID(ctx, session.ID)
		if err != nil {
			s.getLogger().WarnContext(ctx, "failed to inspect breaks during auto-checkout",
				slog.Int64("session_id", session.ID),
				slog.String("error", err.Error()))
			continue
		}

		_, lateBreak := activeBreakAndLateBreakAfter(breaks, closeAt)
		if lateBreak != nil {
			attrs := []any{
				slog.Int64("session_id", session.ID),
				slog.Int64("break_id", lateBreak.ID),
				slog.Time("break_started_at", lateBreak.StartedAt),
				slog.Time("planned_end", closeAt),
			}
			if lateBreak.EndedAt != nil {
				attrs = append(attrs, slog.Time("break_ended_at", *lateBreak.EndedAt))
			}
			s.getLogger().WarnContext(ctx, "skipping auto-checkout because a break crosses planned shift end", attrs...)
			continue
		}

		closed, err := s.repo.CloseSession(ctx, session.ID, closeAt, true)
		if err != nil {
			return count, fmt.Errorf("failed to auto-checkout session %d: %w", session.ID, err)
		}
		if !closed {
			s.getLogger().InfoContext(ctx, "skipping auto-checkout side effects because session was already closed",
				slog.Int64("session_id", session.ID),
				slog.Int64("staff_id", session.StaffID))
			continue
		}

		breaks, err = s.breakRepo.GetBySessionID(ctx, session.ID)
		if err != nil {
			return count, fmt.Errorf("failed to re-inspect breaks after auto-checkout close for session %d: %w", session.ID, err)
		}
		activeBreak, lateBreak := activeBreakAndLateBreakAfter(breaks, closeAt)
		if lateBreak != nil {
			return count, fmt.Errorf("late break detected after auto-checkout close for session %d", session.ID)
		}

		if activeBreak != nil {
			if err := s.endActiveBreak(ctx, session.ID, activeBreak, closeAt); err != nil {
				return count, fmt.Errorf("failed to end active break during auto-checkout for session %d: %w", session.ID, err)
			}
		}

		// End open supervisions, exactly like manual checkout does.
		s.endActiveSupervisionsOnCheckout(ctx, session.StaffID)

		newVal := closeAt.Format(time.RFC3339)
		note := "Automatische Ausstempelung zum geplanten Schichtende"
		edit := &auditModels.WorkSessionEdit{
			SessionID: session.ID,
			StaffID:   session.StaffID,
			EditedBy:  auditModels.SystemEditorID,
			FieldName: auditModels.FieldCheckOutTime,
			OldValue:  nil,
			NewValue:  &newVal,
			Notes:     &note,
		}
		edit.SetTenantID(tenantID)
		if err := s.auditRepo.CreateBatch(ctx, []*auditModels.WorkSessionEdit{edit}); err != nil {
			// The scheduler runs this inside a per-tenant transaction, so
			// returning here rolls the unaudited checkout back with it.
			return count, fmt.Errorf("failed to write auto-checkout audit entry for session %d: %w", session.ID, err)
		}

		s.getLogger().InfoContext(ctx, "session auto-checked-out at planned shift end",
			slog.Int64("session_id", session.ID),
			slog.Int64("staff_id", session.StaffID),
			slog.String("check_out_time", newVal))
		count++
	}

	if count > 0 {
		s.broadcastTimeTrackingChanged(ctx)
	}
	return count, nil
}

func autoCheckoutEffectiveStart(session *activeModels.WorkSession) time.Time {
	if session == nil {
		return time.Time{}
	}
	if session.ReopenedAt != nil {
		return *session.ReopenedAt
	}
	return session.CheckInTime
}

func activeBreakAndLateBreakAfter(breaks []*activeModels.WorkSessionBreak, closeAt time.Time) (*activeModels.WorkSessionBreak, *activeModels.WorkSessionBreak) {
	var activeBreak *activeModels.WorkSessionBreak
	for _, brk := range breaks {
		if brk == nil {
			continue
		}
		if brk.EndedAt == nil {
			activeBreak = brk
		}
		if brk.StartedAt.After(closeAt) {
			return activeBreak, brk
		}
		if brk.EndedAt != nil && brk.EndedAt.After(closeAt) {
			return activeBreak, brk
		}
	}
	return activeBreak, nil
}

func (s *workSessionService) endActiveBreak(ctx context.Context, sessionID int64, activeBreak *activeModels.WorkSessionBreak, endAt time.Time) error {
	duration := int(math.Round(endAt.Sub(activeBreak.StartedAt).Minutes()))
	if err := s.breakRepo.EndBreak(ctx, activeBreak.ID, endAt, duration); err != nil {
		return fmt.Errorf("failed to end active break: %w", err)
	}

	if err := s.recalcBreakMinutes(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to update break minutes: %w", err)
	}
	return nil
}

// EnsureCheckedIn ensures a staff member is checked in, creating a session if needed.
// `source` is forwarded to CheckIn so the channel is recorded faithfully.
func (s *workSessionService) EnsureCheckedIn(ctx context.Context, staffID int64, source string) (*activeModels.WorkSession, error) {
	// Check if already checked in. Across all days, like the check-in guard:
	// a block that crossed Berlin midnight is still running, and starting a
	// supervision must return it instead of running into "already checked in".
	currentSession, err := s.repo.GetLatestOpenByStaffID(ctx, staffID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check current session: %w", err)
	}

	if currentSession != nil && currentSession.IsActive() {
		return currentSession, nil
	}

	// Check if there's already a checked-out block today. Deliberately no
	// auto-created second block (#2402): whoever stamped out made a
	// conscious decision, and a supervision start must not silently begin a
	// new work block behind their back.
	today := timezone.DateFromTime(s.now())
	todaySessions, err := s.listSessionsByStaffAndDate(ctx, staffID, today)
	if err != nil {
		return nil, fmt.Errorf("failed to check today's session: %w", err)
	}

	if len(todaySessions) > 0 {
		// Already checked out today, don't re-check-in
		return nil, nil
	}

	// No session today, create one with the channel the caller passed in.
	// The F9 deviation gate is bypassed: this auto-stamp fires because the
	// staff member started a supervision, and that flow cannot collect a
	// reason (see checkIn).
	return s.checkIn(ctx, staffID, activeModels.WorkSessionStatusPresent, source, "", false)
}

// German weekday names for export
var germanWeekdays = [7]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}

// German absence type labels for export
var germanAbsenceTypeLabels = map[string]string{
	activeModels.AbsenceTypeSick:     "Krank",
	activeModels.AbsenceTypeVacation: "Urlaub",
	activeModels.AbsenceTypeTraining: "Fortbildung",
	activeModels.AbsenceTypeOther:    "Sonstige",
	activeModels.AbsenceTypeCompTime: "Freizeitausgleich",
}

// exportRow represents a single row in the export (either a work session or an absence day)
type exportRow struct {
	Date timezone.Date
	// Start orders multiple blocks of the same day chronologically (#2402).
	// Absence rows carry the zero value and sort before the day's sessions.
	Start time.Time
	Row   []string
}

// timeTrackingConfidentialityNote matches the wording of the other
// printed exports (see slotlists.confidentialityNote).
const timeTrackingConfidentialityNote = "Vertraulich, nur für berechtigte Personen. Nach Gebrauch sicher vernichten."

// timeTrackingColumns is the single-staff export's column set; the labels
// double as the CSV/XLSX header row.
func timeTrackingColumns() []listexport.Column {
	return []listexport.Column{
		{ID: "tt_date", Label: "Datum"},
		{ID: "tt_weekday", Label: "Wochentag"},
		{ID: "tt_start", Label: "Start"},
		{ID: "tt_end", Label: "Ende"},
		{ID: "tt_break", Label: "Pause (Min)"},
		{ID: "tt_net", Label: "Netto (Std)"},
		{ID: "tt_status", Label: "Status"},
		{ID: "tt_source", Label: "Quelle"},
		{ID: "tt_notes", Label: "Bemerkungen"},
	}
}

func timeTrackingHeaders() []string {
	columns := timeTrackingColumns()
	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = column.Label
	}
	return headers
}

// ExportSessions renders the single-staff export of work sessions and
// absences. CSV and XLSX stay plain data files (shared writers below);
// PDF renders through the listexport design (#1568) as the print artifact.
func (s *workSessionService) ExportSessions(ctx context.Context, staffID int64, from, to timezone.Date, format string) (*ExportFile, error) {
	// Deliberately the stored-date read, not the intersecting one: an export
	// row is a whole block, and a night block that reaches into the next month
	// must be listed once, under the month it started in. Reading by interval
	// would put it into both months' files and double it in any sum drawn from
	// them. The absences below are clamped to the same range for that reason.
	historyResp, err := s.GetHistory(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions for export: %w", err)
	}

	// Load absences for the same date range
	var absences []*activeModels.StaffAbsence
	if s.absenceRepo != nil {
		absences, err = s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, from, to)
		if err != nil {
			return nil, fmt.Errorf("failed to get absences for export: %w", err)
		}
		// Resolve the school's own Abwesenheitsarten so the export prints the
		// wording the school entered, not the generic "Sonstige" (#2403).
		StampAbsenceTypeLabels(ctx, s.absenceTypes, absences)
	}

	// Build merged rows sorted by date
	rows := s.buildExportRows(historyResp.Sessions, clampAbsencesToRange(absences, from, to))

	fromStr := from.String()
	toStr := to.String()

	cells := make([][]string, 0, len(rows))
	for _, er := range rows {
		cells = append(cells, er.Row)
	}

	switch format {
	case "pdf":
		doc, err := s.buildTimeTrackingDocument(ctx, staffID, rows, from, to)
		if err != nil {
			return nil, err
		}
		file, err := listexport.NewService().Render(doc, listexport.FormatPDF, "zeiterfassung")
		if err != nil {
			return nil, err
		}
		return &ExportFile{
			Data:        file.Data,
			Filename:    fmt.Sprintf("zeiterfassung_%s_%s.pdf", fromStr, toStr),
			ContentType: file.ContentType,
		}, nil
	case "xlsx":
		data, err := writeExportXLSX("Zeiterfassung", timeTrackingHeaders(), stringsRowsToAny(cells), nil)
		if err != nil {
			return nil, err
		}
		return &ExportFile{
			Data:        data,
			Filename:    fmt.Sprintf("zeiterfassung_%s_%s.xlsx", fromStr, toStr),
			ContentType: contentTypeXLSX,
		}, nil
	default:
		// Bemerkungen (column 8) carries untrusted free text — formula-escape
		// it like the cross-staff CSV always has.
		data, err := writeExportCSV(timeTrackingHeaders(), cells, []int{8})
		if err != nil {
			return nil, err
		}
		return &ExportFile{
			Data:        data,
			Filename:    fmt.Sprintf("zeiterfassung_%s_%s.csv", fromStr, toStr),
			ContentType: contentTypeCSV,
		}, nil
	}
}

// buildTimeTrackingDocument shapes the merged export rows into the shared
// listexport document: title block with the staff member's name, the
// requested period as a filter pill, and the confidentiality footer.
func (s *workSessionService) buildTimeTrackingDocument(ctx context.Context, staffID int64, rows []exportRow, from, to timezone.Date) (listexport.Document, error) {
	subtitle := "Arbeitszeiten und Abwesenheiten"
	if s.staffRepo != nil {
		staffMap, err := s.staffRepo.FindWithPersonByIDs(ctx, []int64{staffID})
		if err != nil {
			return listexport.Document{}, fmt.Errorf("failed to load staff for export: %w", err)
		}
		if staff, ok := staffMap[staffID]; ok && staff != nil && staff.Person != nil {
			subtitle = staff.Person.FirstName + " " + staff.Person.LastName
		}
	}

	columns := timeTrackingColumns()
	docRows := make([]listexport.Row, 0, len(rows))
	for _, er := range rows {
		values := make(map[listexport.ColumnID]string, len(columns))
		for i, column := range columns {
			if i < len(er.Row) {
				values[column.ID] = er.Row[i]
			}
		}
		docRows = append(docRows, listexport.Row{Values: values})
	}

	return listexport.Document{
		Title:       "Zeiterfassung",
		Subtitle:    subtitle,
		GeneratedAt: s.now(),
		Filters:     []string{"Zeitraum: " + from.Format("02.01.2006") + " bis " + to.Format("02.01.2006")},
		Columns:     columns,
		Rows:        docRows,
		Footer:      timeTrackingConfidentialityNote,
	}, nil
}

// clampAbsencesToRange returns shallow copies whose expanded day ranges stay
// inside the requested export period. Repository range lookups deliberately
// return overlapping absences, including records that start before `from` or
// end after `to`.
func clampAbsencesToRange(absences []*activeModels.StaffAbsence, from, to timezone.Date) []*activeModels.StaffAbsence {
	clamped := make([]*activeModels.StaffAbsence, 0, len(absences))
	for _, absence := range absences {
		if absence == nil || absence.DateEnd.Before(from) || to.Before(absence.DateStart) {
			continue
		}
		copy := *absence
		if copy.DateStart.Before(from) {
			copy.DateStart = from
		}
		if to.Before(copy.DateEnd) {
			copy.DateEnd = to
		}
		clamped = append(clamped, &copy)
	}
	return clamped
}

// buildExportRows merges session rows and absence rows, sorted by date
func (s *workSessionService) buildExportRows(sessions []*SessionResponse, absences []*activeModels.StaffAbsence) []exportRow {
	var rows []exportRow

	// Add session rows (a day can carry several blocks since #2402)
	for _, sr := range sessions {
		rows = append(rows, exportRow{
			Date:  sr.Date,
			Start: sr.CheckInTime,
			Row:   s.sessionToRow(sr),
		})
	}

	// Add absence rows (one row per day in the absence range)
	for _, absence := range absences {
		if absence.Status != activeModels.AbsenceStatusReported &&
			absence.Status != activeModels.AbsenceStatusApproved {
			continue
		}
		// The school's own wording wins when the absence carries one (#2403);
		// the standard German label is the fallback, the raw type the last
		// resort for a value this map has not caught up with.
		label := absence.AbsenceTypeLabel
		if label == "" {
			label = germanAbsenceTypeLabels[absence.AbsenceType]
		}
		if label == "" {
			label = absence.AbsenceType
		}

		d := absence.DateStart
		for !d.After(absence.DateEnd) {
			datum := d.Format("02.01.2006")
			wochentag := germanWeekdays[d.Weekday()]
			// Column layout must match the export header (9 cells):
			// Datum, Wochentag, Start, Ende, Pause, Netto, Status, Quelle, Bemerkungen.
			// Absences have no Quelle (the staff member did not stamp at all),
			// so that cell stays empty rather than borrowing the "-" sentinel
			// already used by quelleLabel for legacy work_sessions of unknown
			// channel. Those are two different facts and must read distinctly.
			rows = append(rows, exportRow{
				Date: d,
				Row:  []string{datum, wochentag, "--", "--", "--", "--", label, "", absence.Note},
			})
			d = d.AddDays(1)
		}
	}

	// Sort by date, blocks of the same day by check-in time
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date.Before(rows[j].Date)
		}
		return rows[i].Start.Before(rows[j].Start)
	})

	return rows
}

func (s *workSessionService) sessionToRow(sr *SessionResponse) []string {
	sess := sr.WorkSession

	datum := sess.Date.Format("02.01.2006")
	wochentag := germanWeekdays[sess.Date.Weekday()]

	start := sess.CheckInTime.Format("15:04")

	ende := ""
	if sess.CheckOutTime != nil {
		ende = sess.CheckOutTime.Format("15:04")
	}

	// Read the pause from the response-level total, not the model cache: for an
	// open session with a running break the cache (sess.BreakMinutes) still holds
	// only ENDED breaks and would print "Pause 0" next to a Netto that already
	// deducts the running break (sr.NetMinutes). sr.BreakMinutes is the live
	// total that pairs with that Netto, so the export row stays internally
	// consistent (#1842).
	pauseMin := strconv.Itoa(sr.BreakMinutes)

	// Net as "Xh YYmin"
	netMins := sr.NetMinutes
	h := netMins / 60
	m := netMins % 60
	netto := fmt.Sprintf("%dh %02dmin", h, m)

	// Status carries the underlying work mode and is never overwritten by
	// system events. The OGS-Leitung must always be able to see whether
	// the row was Vor Ort or Homeoffice, even if the session was later
	// auto-closed or manually corrected.
	status := "Vor Ort"
	if sess.Status == activeModels.WorkSessionStatusHomeOffice {
		status = "Homeoffice"
	}

	// Quelle records the originating channel, with system-event overlays:
	// a manual correction is the most informative signal and wins; an
	// auto-checkout still tells the reader the session was closed by the
	// scheduler rather than the staff member; otherwise the persisted
	// source (App / NFC) is shown verbatim.
	quelle := quelleLabel(sess.Source)
	if sess.AutoCheckedOut {
		quelle = "Auto-Checkout"
	}
	if sr.EditCount > 0 {
		quelle = "Manuell korrigiert"
	}

	return []string{datum, wochentag, start, ende, pauseMin, netto, status, quelle, sess.Notes}
}

// quelleLabel renders the export "Quelle" cell from the persisted source.
// 'unknown' marks rows that pre-date migration 1.15.54. No channel was ever
// recorded, so we render "-" rather than guess. Anything else falls through
// to App, matching the DB column default for in-flight rows that may exist
// briefly between migration and new-server boot.
func quelleLabel(source string) string {
	switch source {
	case activeModels.WorkSessionSourceNFC:
		return "NFC"
	case activeModels.WorkSessionSourceUnknown:
		return "-"
	default:
		return "App"
	}
}

// AutoEndExpiredBreaks ends all breaks whose planned_end_time has passed
// Returns the number of breaks that were ended
func (s *workSessionService) AutoEndExpiredBreaks(ctx context.Context) (int, error) {
	now := time.Now()

	// Get all expired breaks
	expiredBreaks, err := s.breakRepo.GetExpiredBreaks(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("failed to get expired breaks: %w", err)
	}

	if len(expiredBreaks) == 0 {
		return 0, nil
	}
	sessionIDs := make([]interface{}, 0, len(expiredBreaks))
	for _, brk := range expiredBreaks {
		sessionIDs = append(sessionIDs, brk.SessionID)
	}
	sessions, err := s.repo.List(ctx, &modelBase.QueryOptions{
		Filter: modelBase.NewFilter().Where("id", modelBase.OpIn, sessionIDs),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to resolve work sessions for balance lock: %w", err)
	}
	if sessions == nil {
		for _, brk := range expiredBreaks {
			session, lookupErr := s.repo.FindByID(ctx, brk.SessionID)
			if lookupErr != nil {
				return 0, fmt.Errorf("failed to resolve work session for balance lock: %w", lookupErr)
			}
			if session.ID == 0 {
				session.ID = brk.SessionID
			}
			sessions = append(sessions, session)
		}
	}
	sessionsByID := make(map[int64]*activeModels.WorkSession, len(sessions))
	for _, session := range sessions {
		sessionsByID[session.ID] = session
	}
	staffIDs := make([]int64, 0, len(expiredBreaks))
	for _, brk := range expiredBreaks {
		session, ok := sessionsByID[brk.SessionID]
		if !ok {
			return 0, fmt.Errorf("failed to resolve work session %d for balance lock", brk.SessionID)
		}
		staffIDs = append(staffIDs, session.StaffID)
	}
	if err := s.lockStaffBalanceWritesOrdered(ctx, staffIDs); err != nil {
		return 0, err
	}

	count := 0
	for _, brk := range expiredBreaks {
		// Use planned_end_time as the end time for accurate duration
		endTime := *brk.PlannedEndTime
		duration := int(math.Round(endTime.Sub(brk.StartedAt).Minutes()))

		if err := s.breakRepo.EndBreak(ctx, brk.ID, endTime, duration); err != nil {
			s.getLogger().WarnContext(ctx, "failed to auto-end break",
				slog.Int64("break_id", brk.ID),
				slog.String("error", err.Error()))
			continue
		}

		// Recalculate break minutes for the session
		if err := s.recalcBreakMinutes(ctx, brk.SessionID); err != nil {
			s.getLogger().WarnContext(ctx, "failed to recalc break minutes",
				slog.Int64("session_id", brk.SessionID),
				slog.String("error", err.Error()))
		}

		count++
	}

	if count > 0 {
		s.broadcastTimeTrackingChanged(ctx)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Staff work-schedule operations (issue #584: moved verbatim out of api/staff)
// ---------------------------------------------------------------------------

// GetStaffIDsWithSupervisionToday returns staff IDs with supervision activity today.
func (s *workSessionService) GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error) {
	return s.supervisorRepo.GetStaffIDsWithSupervisionToday(ctx)
}

// GetWorkTimeModelByID retrieves a work time model with its entries.
func (s *workSessionService) GetWorkTimeModelByID(ctx context.Context, id int64) (*configModels.WorkTimeModel, error) {
	return s.workModelRepo.FindByID(ctx, id)
}

// GetCurrentScheduleRows retrieves the staff member's current schedule snapshot.
func (s *workSessionService) GetCurrentScheduleRows(ctx context.Context, staffID int64) ([]*configModels.StaffWorkSchedule, error) {
	return s.scheduleRepo.GetCurrentByStaffID(ctx, staffID)
}

// AssignScheduleTemplate snapshots the template's entries as the staff
// member's schedule and binds the template to the staff row.
func (s *workSessionService) AssignScheduleTemplate(ctx context.Context, staff *userModels.Staff, modelID int64) error {
	model, err := s.workModelRepo.FindByID(ctx, modelID)
	if err != nil {
		return fmt.Errorf("template not found: %w", err)
	}

	entries := modelEntriesToScheduleRows(model.Entries, model.RotationLength)
	anchor := model.RotationAnchorDate
	if err := s.scheduleRepo.ReplaceSchedule(ctx, staff.ID, entries, anchor); err != nil {
		return fmt.Errorf("write assigned schedule snapshot: %w", err)
	}

	staff.WorkTimeModelID = &model.ID
	staffAnchor := calendarDate(anchor)
	staff.RotationAnchorDate = &staffAnchor
	if err := s.staffRepo.Update(ctx, staff); err != nil {
		return fmt.Errorf("bind template to staff: %w", err)
	}
	return nil
}

// ApplyCustomScheduleRows replaces the schedule with custom rows and unbinds
// any assigned template.
func (s *workSessionService) ApplyCustomScheduleRows(ctx context.Context, staff *userModels.Staff, entries []*configModels.StaffWorkSchedule, anchor timezone.Date) error {
	// An omitted anchor keeps the staff-level one; the new version must be
	// stamped with that same effective anchor, or it would silently re-parity
	// once the staff anchor moves.
	effective := anchor
	if effective.IsZero() && staff.RotationAnchorDate != nil {
		effective = *staff.RotationAnchorDate
	}
	// First rotational schedule of a staff member who has no anchor anywhere:
	// stamp today, which is the version's valid_from. Leaving the column NULL
	// would let a later template assignment write a staff-level anchor that
	// these rows then fall back to, re-paritying their A/B weeks and moving a
	// historical Saldo.
	if effective.IsZero() && isRotationalSchedule(entries) {
		effective = timezone.DateFromTime(s.now())
	}
	if err := s.scheduleRepo.ReplaceSchedule(ctx, staff.ID, entries, workforceDate(effective)); err != nil {
		return fmt.Errorf("write custom schedule: %w", err)
	}

	staff.WorkTimeModelID = nil
	if !effective.IsZero() {
		staff.RotationAnchorDate = &effective
	}
	if err := s.staffRepo.Update(ctx, staff); err != nil {
		return fmt.Errorf("unbind template: %w", err)
	}

	return nil
}

// SaveCustomScheduleAsTemplate persists the rows as a new reusable work time
// model and binds it to the staff member.
func (s *workSessionService) SaveCustomScheduleAsTemplate(ctx context.Context, staff *userModels.Staff, name string, rotation int, anchor timezone.Date, entries []*configModels.WorkTimeModelEntry) error {
	if anchor.IsZero() {
		anchor = timezone.DateFromTime(s.now())
	}
	model := &configModels.WorkTimeModel{
		Name:               name,
		RotationLength:     rotation,
		RotationAnchorDate: workforceDate(anchor),
	}
	if err := s.workModelRepo.Create(ctx, model, entries); err != nil {
		return err
	}
	scheduleRows := modelEntriesToScheduleRows(entries, rotation)
	if err := s.scheduleRepo.ReplaceSchedule(ctx, staff.ID, scheduleRows, workforceDate(anchor)); err != nil {
		return fmt.Errorf("write saved template schedule snapshot: %w", err)
	}

	staff.WorkTimeModelID = &model.ID
	staff.RotationAnchorDate = &anchor
	if err := s.staffRepo.Update(ctx, staff); err != nil {
		return fmt.Errorf("bind freshly created template: %w", err)
	}
	return nil
}

// UpdateSchedule resolves the requested mode and applies the schedule change.
func (s *workSessionService) UpdateSchedule(ctx context.Context, staff *userModels.Staff, in ScheduleUpdateInput) error {
	mode := in.Mode
	if mode == "" {
		// Backwards compatibility: missing mode + flat entries means the
		// caller still uses the single-week, no-rotation contract.
		mode = "custom"
		if in.RotationLength == 0 {
			in.RotationLength = 1
		}
	}

	var err error
	switch mode {
	case "template":
		if in.ModelID == nil || *in.ModelID == 0 {
			return scheduleValidationErrorf("model_id is required for mode=template")
		}
		err = s.AssignScheduleTemplate(ctx, staff, *in.ModelID)
	case "custom":
		err = s.applyCustomSchedule(ctx, staff, in)
	default:
		return scheduleValidationErrorf("invalid mode %q", mode)
	}
	if err != nil {
		return err
	}
	s.broadcastTimeTrackingChanged(ctx)
	return nil
}

func (s *workSessionService) applyCustomSchedule(ctx context.Context, staff *userModels.Staff, in ScheduleUpdateInput) error {
	rotation := in.RotationLength
	if rotation == 0 {
		rotation = 1
	}
	if rotation < 1 || rotation > configModels.WorkTimeModelMaxRotation {
		return scheduleValidationErrorf("rotation_length must be between 1 and %d", configModels.WorkTimeModelMaxRotation)
	}

	anchor := timezone.Date("")
	if in.RotationAnchorDate != "" {
		parsed, err := timezone.ParseDate(in.RotationAnchorDate)
		if err != nil {
			return scheduleValidationErrorf("invalid rotation_anchor_date: %v", err)
		}
		anchor = parsed
	}

	entries, templateEntries, err := buildScheduleEntries(in.Entries, rotation)
	if err != nil {
		return err
	}

	if in.SaveAsTemplateName != "" {
		if err := s.SaveCustomScheduleAsTemplate(ctx, staff, in.SaveAsTemplateName, rotation, anchor, templateEntries); err != nil {
			return fmt.Errorf("save as template: %w", err)
		}
		return nil
	}

	return s.ApplyCustomScheduleRows(ctx, staff, entries, anchor)
}

func buildScheduleEntries(reqEntries []ScheduleEntry, rotation int) ([]*configModels.StaffWorkSchedule, []*configModels.WorkTimeModelEntry, error) {
	entries := make([]*configModels.StaffWorkSchedule, 0, len(reqEntries))
	templateEntries := make([]*configModels.WorkTimeModelEntry, 0, len(reqEntries))
	seenSlots := make(map[string]struct{}, len(reqEntries))
	for _, e := range reqEntries {
		if e.TargetMinutes <= 0 {
			continue
		}
		if err := validateScheduleEntryRequest(e, rotation, seenSlots); err != nil {
			return nil, nil, err
		}
		startTime, err := parseScheduleStartTime(e.StartTime)
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, &configModels.StaffWorkSchedule{
			WeekIndex:      e.WeekIndex,
			RotationLength: rotation,
			DayOfWeek:      e.DayOfWeek,
			TargetMinutes:  e.TargetMinutes,
			StartTime:      startTime,
		})
		templateEntries = append(templateEntries, &configModels.WorkTimeModelEntry{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
			StartTime:     startTime,
		})
	}
	return entries, templateEntries, nil
}

func parseScheduleStartTime(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("15:04", *raw)
	if err != nil {
		return nil, scheduleValidationErrorf("start_time must be HH:MM")
	}
	wallClock := timezone.NormalizeWallClock(parsed)
	return &wallClock, nil
}

func validateScheduleEntryRequest(e ScheduleEntry, rotation int, seenSlots map[string]struct{}) error {
	if e.WeekIndex < 0 || e.WeekIndex >= rotation {
		return scheduleValidationErrorf("week_index %d outside rotation_length %d", e.WeekIndex, rotation)
	}
	if e.DayOfWeek < configModels.DayMonday || e.DayOfWeek > configModels.DaySunday {
		return scheduleValidationErrorf("day_of_week must be between 0 and 6")
	}
	if e.TargetMinutes > scheduleEntryMaxTargetMinutes {
		return scheduleValidationErrorf("target_minutes must be between 0 and %d", scheduleEntryMaxTargetMinutes)
	}
	slot := fmt.Sprintf("%d:%d", e.WeekIndex, e.DayOfWeek)
	if _, ok := seenSlots[slot]; ok {
		return scheduleValidationErrorf("duplicate schedule entry for week_index %d and day_of_week %d", e.WeekIndex, e.DayOfWeek)
	}
	seenSlots[slot] = struct{}{}
	return nil
}

// isRotationalSchedule reports whether the rows span more than one week, i.e.
// whether their parity depends on a rotation anchor at all. Single-week
// schedules have no parity, so they keep a NULL anchor.
func isRotationalSchedule(entries []*configModels.StaffWorkSchedule) bool {
	for _, e := range entries {
		if e != nil && (e.RotationLength > 1 || e.WeekIndex > 0) {
			return true
		}
	}
	return false
}

// modelEntriesToScheduleRows converts work-time-model entries into schedule
// snapshot rows, dropping non-positive target minutes.
func modelEntriesToScheduleRows(modelEntries []*configModels.WorkTimeModelEntry, rotation int) []*configModels.StaffWorkSchedule {
	rows := make([]*configModels.StaffWorkSchedule, 0, len(modelEntries))
	for _, e := range modelEntries {
		if e.TargetMinutes <= 0 {
			continue
		}
		rows = append(rows, &configModels.StaffWorkSchedule{
			WeekIndex:      e.WeekIndex,
			RotationLength: rotation,
			DayOfWeek:      e.DayOfWeek,
			TargetMinutes:  e.TargetMinutes,
			StartTime:      e.StartTime,
		})
	}
	return rows
}
