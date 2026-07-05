package active

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/xuri/excelize/v2"
)

// Error message constants to avoid duplication
const (
	errNoActiveSession    = "no active session found"
	errGetCurrentSession  = "failed to get current session: %w"
	errInvalidSessionData = "invalid session data: %w"
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
	NetMinutes        int                              `json:"net_minutes"`
	IsOvertime        bool                             `json:"is_overtime"`
	IsBreakCompliant  bool                             `json:"is_break_compliant"`
	RestPeriodWarning *string                          `json:"rest_period_warning,omitempty"`
	Breaks            []*activeModels.WorkSessionBreak `json:"breaks"`
	EditCount         int                              `json:"edit_count"`
	AuditCount        int                              `json:"audit_count"`
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

// ReopenStatusConflictError is returned by CheckIn when the staff member
// already has a checked-out session for today and the requested status
// differs from the existing one. Reopening would silently change the status
// without an audit edit; the caller (App UI) must instead reopen with the
// existing status and then go through UpdateSession (which requires a
// notes reason and emits a FieldStatus edit).
//
// The frontend disambiguates this from other 409s via the typed code
// "reopen_status_conflict" produced by api/time-tracking/errors.go.
type ReopenStatusConflictError struct {
	SessionID       int64
	ExistingStatus  string
	RequestedStatus string
}

func (e *ReopenStatusConflictError) Error() string {
	return "reopen status conflict"
}

// WorkSessionService defines operations for staff time tracking
type WorkSessionService interface {
	// CheckIn opens or reopens today's session for staffID. `source` records
	// the channel that triggered the check-in (app/nfc) so the export can
	// label "Vor Ort (App)" vs "Vor Ort (NFC)" without inferring it from
	// status alone (Issue #1368).
	//
	// Reopen rules: the originating Source AND Status are both preserved.
	// The channel is an audit-relevant fact with no audit edit to capture
	// a change; status changes carry audit weight and must go through
	// UpdateSession (which gates on a notes reason). If `status` differs
	// from the existing session's status, CheckIn returns
	// *ReopenStatusConflictError instead of silently overwriting.
	CheckIn(ctx context.Context, staffID int64, status, source string) (*activeModels.WorkSession, error)
	CheckOut(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*activeModels.WorkSessionBreak, error)
	EndBreak(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
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
	GetCurrentSession(ctx context.Context, staffID int64) (*activeModels.WorkSession, error)
	GetHistory(ctx context.Context, staffID int64, from, to timezone.Date) (*HistoryResponse, error)
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
	ExportSessions(ctx context.Context, staffID int64, from, to timezone.Date, format string) ([]byte, string, error)
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
}

// workSessionService implements WorkSessionService
type workSessionService struct {
	repo           activeModels.WorkSessionRepository
	breakRepo      activeModels.WorkSessionBreakRepository
	auditRepo      auditModels.WorkSessionEditRepository
	absenceRepo    activeModels.StaffAbsenceRepository
	supervisorRepo activeModels.GroupSupervisorRepository
	staffRepo      userModels.StaffRepository
	scheduleRepo   configModels.StaffWorkScheduleRepository
	workModelRepo  configModels.WorkTimeModelRepository
	staffShiftRepo scheduleModels.StaffShiftRepository
	logger         *slog.Logger
}

// SetStaffShiftRepo injects the planned-shift repository used by
// AutoCheckoutDueSessions (#1798). Setter instead of a constructor param to
// keep the already long NewWorkSessionService signature stable; the factory
// calls it right after construction.
func (s *workSessionService) SetStaffShiftRepo(repo scheduleModels.StaffShiftRepository) {
	s.staffShiftRepo = repo
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (s *workSessionService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// NewWorkSessionService creates a new work session service
func NewWorkSessionService(repo activeModels.WorkSessionRepository, breakRepo activeModels.WorkSessionBreakRepository, auditRepo auditModels.WorkSessionEditRepository, absenceRepo activeModels.StaffAbsenceRepository, supervisorRepo activeModels.GroupSupervisorRepository, staffRepo userModels.StaffRepository, scheduleRepo configModels.StaffWorkScheduleRepository, workModelRepo configModels.WorkTimeModelRepository, logger *slog.Logger) WorkSessionService {
	return &workSessionService{repo: repo, breakRepo: breakRepo, auditRepo: auditRepo, absenceRepo: absenceRepo, supervisorRepo: supervisorRepo, staffRepo: staffRepo, scheduleRepo: scheduleRepo, workModelRepo: workModelRepo, logger: logger}
}

// CheckIn creates a new work session for the staff member.
// Status must be explicitly chosen. Empty values are rejected so the caller
// (HTTP handler or internal worker) cannot accidentally fall back to "present".
func (s *workSessionService) CheckIn(ctx context.Context, staffID int64, status, source string) (*activeModels.WorkSession, error) {
	if status != activeModels.WorkSessionStatusPresent && status != activeModels.WorkSessionStatusHomeOffice {
		return nil, fmt.Errorf("status must be 'present' or 'home_office'")
	}
	if source != activeModels.WorkSessionSourceApp && source != activeModels.WorkSessionSourceNFC {
		return nil, fmt.Errorf("source must be 'app' or 'nfc'")
	}

	// Today's Berlin calendar day for the PostgreSQL DATE column
	today := timezone.TodayDate()

	// Check if there's already a session today
	existingSession, err := s.repo.GetByStaffAndDate(ctx, staffID, today)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check existing session: %w", err)
	}

	if existingSession != nil {
		if existingSession.IsActive() {
			return nil, fmt.Errorf("already checked in")
		}
		// A status mismatch on reopen would silently rewrite an audit-relevant
		// field with no FieldStatus edit emitted. Force the caller to reopen
		// with the existing status, then change it via UpdateSession (which
		// gates on a notes reason and produces the audit edit).
		if existingSession.Status != status {
			return nil, &ReopenStatusConflictError{
				SessionID:       existingSession.ID,
				ExistingStatus:  existingSession.Status,
				RequestedStatus: status,
			}
		}
		// Re-open the checked-out session (accidental checkout recovery).
		// Source and Status are both preserved (see reopenSession comment).
		return s.reopenSession(ctx, existingSession, staffID)
	}

	// Create new session
	now := time.Now()
	session := &activeModels.WorkSession{
		StaffID:      staffID,
		Date:         today,
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

	return session, nil
}

// reopenSession clears checkout on an existing session so the staff member
// can continue working. Both Source and Status are intentionally preserved:
// the originating channel and the previously-chosen work mode are
// audit-relevant facts, and overwriting either on reopen would silently
// drop the signal with no audit edit. CheckIn rejects the call with
// ReopenStatusConflictError before this point if the requested status
// differs from the existing one. The caller is expected to follow up
// with UpdateSession (which gates on a notes reason).
func (s *workSessionService) reopenSession(ctx context.Context, session *activeModels.WorkSession, staffID int64) (*activeModels.WorkSession, error) {
	now := time.Now()
	session.CheckOutTime = nil
	session.AutoCheckedOut = false
	session.ReopenedAt = &now
	session.UpdatedBy = &staffID

	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf(errInvalidSessionData, err)
	}

	if err := s.repo.Update(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to reopen session: %w", err)
	}

	return session, nil
}

// CheckOut ends the current work session for the staff member
func (s *workSessionService) CheckOut(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	// Get current active session
	session, err := s.repo.GetCurrentByStaffID(ctx, staffID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(errNoActiveSession)
		}
		return nil, fmt.Errorf(errGetCurrentSession, err)
	}

	if session == nil {
		return nil, errors.New(errNoActiveSession)
	}

	// End any active break before checkout
	now := time.Now()
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

	// End all active supervisions for this staff member (fire-and-forget)
	s.endActiveSupervisionsOnCheckout(ctx, staffID)

	// Re-fetch the updated session
	updatedSession, err := s.repo.FindByID(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve updated session: %w", err)
	}

	return updatedSession, nil
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
	ended, err := s.supervisorRepo.EndAllActiveByStaffID(ctx, staffID)
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

// StartBreak starts a new break for the current session
// If plannedDurationMinutes is provided (1-120), sets planned_end_time for auto-end
func (s *workSessionService) StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*activeModels.WorkSessionBreak, error) {
	// Get today's active session
	session, err := s.repo.GetCurrentByStaffIDForUpdate(ctx, staffID)
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
		if *plannedDurationMinutes < 1 || *plannedDurationMinutes > 120 {
			return nil, errors.New("planned_duration_minutes must be between 1 and 120")
		}
	}

	// Create a new break
	now := time.Now()
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

	return brk, nil
}

// EndBreak ends the current active break for the staff member's session
func (s *workSessionService) EndBreak(ctx context.Context, staffID int64) (*activeModels.WorkSession, error) {
	// Get today's active session
	session, err := s.repo.GetCurrentByStaffID(ctx, staffID)
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

// recalcBreakMinutes sums all break durations for a session and updates the cache
func (s *workSessionService) recalcBreakMinutes(ctx context.Context, sessionID int64) error {
	breaks, err := s.breakRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get breaks for recalc: %w", err)
	}

	totalMinutes := 0
	for _, brk := range breaks {
		if brk.EndedAt != nil {
			totalMinutes += brk.DurationMinutes
		} else {
			// Active break: compute live duration
			totalMinutes += int(math.Round(time.Since(brk.StartedAt).Minutes()))
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

	return s.applySessionUpdate(ctx, staffID, session, updates)
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

	if err := s.applyBreakUpdates(ctx, uc, updates); err != nil {
		return nil, err
	}

	s.applySimpleFieldUpdates(uc, updates)

	session.UpdatedBy = &editorStaffID
	session.UpdatedAt = uc.now

	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf(errInvalidSessionData, err)
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

	checkOut := req.CheckOutTime
	date := timezone.DateFromTime(req.Date)
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

	return session, nil
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

	// Re-fetch session to get updated break_minutes
	uc.session, err = s.repo.FindByID(ctx, uc.sessionID)
	if err != nil {
		return fmt.Errorf("failed to re-fetch session: %w", err)
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

	newEndedAt := brk.StartedAt.Add(time.Duration(bu.DurationMinutes) * time.Minute)
	if err := s.breakRepo.UpdateDuration(ctx, bu.ID, bu.DurationMinutes, newEndedAt); err != nil {
		return fmt.Errorf("failed to update break %d: %w", bu.ID, err)
	}

	oldVal := strconv.Itoa(brk.DurationMinutes)
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

// GetHistory returns work sessions for a staff member in a date range with weekly aggregation
func (s *workSessionService) GetHistory(ctx context.Context, staffID int64, from, to timezone.Date) (*HistoryResponse, error) {
	sessions, err := s.repo.GetHistoryByStaffID(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get session history: %w", err)
	}

	// Collect session IDs for batch edit count query
	sessionIDs := make([]int64, len(sessions))
	for i, session := range sessions {
		sessionIDs[i] = session.ID
	}

	// Batch fetch manual edit counts. System-authored edits (auto-checkout)
	// are excluded so they don't surface as "Manuell korrigiert".
	editCounts, err := s.auditRepo.CountManualBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get edit counts: %w", err)
	}
	auditCounts, err := s.auditRepo.CountBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit counts: %w", err)
	}

	// Wrap each session in SessionResponse with calculated fields and breaks
	now := time.Now()
	responses := make([]*SessionResponse, len(sessions))
	for i, session := range sessions {
		breaks, err := s.breakRepo.GetBySessionID(ctx, session.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get breaks for session %d: %w", session.ID, err)
		}

		responses[i] = &SessionResponse{
			WorkSession:      session,
			NetMinutes:       netMinutes(session, now),
			IsOvertime:       isOvertime(session, now),
			IsBreakCompliant: isBreakCompliant(session, now),
			Breaks:           breaks,
			EditCount:        editCounts[session.ID],
			AuditCount:       auditCounts[session.ID],
		}
	}

	targetsByWeek := s.getWeeklyTargetsForSummaries(ctx, staffID, responses)

	// Build weekly summaries
	weeklySummaries := s.buildWeeklySummaries(responses, targetsByWeek)

	return &HistoryResponse{
		Sessions:        responses,
		WeeklySummaries: weeklySummaries,
	}, nil
}

// buildWeeklySummaries aggregates session data by ISO week
func (s *workSessionService) buildWeeklySummaries(sessions []*SessionResponse, targetsByWeek map[summaryWeekKey]int) []WeeklySummary {
	weekMap := make(map[summaryWeekKey]*WeeklySummary)
	var weekOrder []summaryWeekKey

	for _, session := range sessions {
		year, week := session.Date.UTCMidnight().ISOWeek()
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

		summary.TotalNetMinutes += session.NetMinutes
		summary.SessionCount++
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
	staff := s.resolveStaffForTargets(ctx, staffID)
	if s.scheduleRepo != nil {
		targets := s.weeklyTargetsFromDateValidSchedule(ctx, staffID, staff, sessions)
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
			anchor = *staff.RotationAnchorDate
		}
		return weeklyTargetsFromModel(model, anchor, sessions)
	}
	return nil
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

func (s *workSessionService) weeklyTargetsFromDateValidSchedule(
	ctx context.Context,
	staffID int64,
	staff *userModels.Staff,
	sessions []*SessionResponse,
) map[summaryWeekKey]int {
	targetsByWeek := make(map[summaryWeekKey]int)
	seen := make(map[summaryWeekKey]bool)
	for _, session := range sessions {
		year, week := session.Date.UTCMidnight().ISOWeek()
		key := summaryWeekKey{Year: year, Week: week}
		if seen[key] {
			continue
		}
		seen[key] = true
		target, ok := s.weeklyTargetFromDateValidSchedule(ctx, staffID, staff, isoWeekStart(session.Date))
		if ok {
			targetsByWeek[key] = target
		}
	}
	if len(targetsByWeek) == 0 {
		return nil
	}
	return targetsByWeek
}

func (s *workSessionService) weeklyTargetFromDateValidSchedule(
	ctx context.Context,
	staffID int64,
	staff *userModels.Staff,
	weekStart timezone.Date,
) (int, bool) {
	total := 0
	found := false
	for offset := 0; offset < 7; offset++ {
		date := weekStart.AddDays(offset)
		entries, err := s.scheduleRepo.GetByStaffIDAndDate(ctx, staffID, date)
		if err != nil || len(entries) == 0 {
			continue
		}
		anchor := resolveScheduleAnchorFromStaff(staff, entries)
		rotationWeek := configModels.ResolveWeekIndex(scheduleRotationLength(entries), isoWeekStart(anchor), isoWeekStart(date))
		dayIndex := isoDayIndex(date)
		for _, entry := range entries {
			if entry.WeekIndex == rotationWeek && entry.DayOfWeek == dayIndex {
				total += entry.TargetMinutes
				found = true
			}
		}
	}
	return total, found
}

func weeklyTargetsFromModel(model *configModels.WorkTimeModel, anchor timezone.Date, sessions []*SessionResponse) map[summaryWeekKey]int {
	if model == nil || len(model.Entries) == 0 || len(sessions) == 0 {
		return nil
	}
	rotation := model.RotationLength
	if rotation < 1 {
		rotation = 1
	}
	targetsByRotationWeek := make(map[int]int, rotation)
	for _, e := range model.Entries {
		targetsByRotationWeek[e.WeekIndex] += e.TargetMinutes
	}
	return weeklyTargetsFromRotationTargets(targetsByRotationWeek, rotation, anchor, sessions)
}

func weeklyTargetsFromRotationTargets(targetsByRotationWeek map[int]int, rotation int, anchor timezone.Date, sessions []*SessionResponse) map[summaryWeekKey]int {
	if len(targetsByRotationWeek) == 0 {
		return nil
	}
	targetsByWeek := make(map[summaryWeekKey]int)
	for _, session := range sessions {
		year, week := session.Date.UTCMidnight().ISOWeek()
		key := summaryWeekKey{Year: year, Week: week}
		if _, ok := targetsByWeek[key]; ok {
			continue
		}
		weekStart := isoWeekStart(session.Date)
		rotationWeek := configModels.ResolveWeekIndex(rotation, isoWeekStart(anchor), weekStart)
		if target, ok := targetsByRotationWeek[rotationWeek]; ok {
			targetsByWeek[key] = target
		}
	}
	return targetsByWeek
}

func resolveScheduleAnchorFromStaff(staff *userModels.Staff, entries []*configModels.StaffWorkSchedule) timezone.Date {
	if staff != nil && staff.RotationAnchorDate != nil {
		return *staff.RotationAnchorDate
	}
	var earliest timezone.Date
	for _, e := range entries {
		if earliest.IsZero() || e.ValidFrom.Before(earliest) {
			earliest = e.ValidFrom
		}
	}
	return earliest
}

func scheduleRotationLength(entries []*configModels.StaffWorkSchedule) int {
	rotation := 1
	for _, e := range entries {
		if e.RotationLength > rotation {
			rotation = e.RotationLength
		}
	}
	if rotation < 1 {
		return 1
	}
	return rotation
}

func isoWeekStart(date timezone.Date) timezone.Date {
	weekday := int(date.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return date.AddDays(1 - weekday)
}

func isoDayIndex(date timezone.Date) int {
	weekday := int(date.Weekday())
	if weekday == 0 {
		return configModels.DaySunday
	}
	return weekday - 1
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
	editorIDs := make([]int64, 0, len(editorIDSet))
	for id := range editorIDSet {
		editorIDs = append(editorIDs, id)
	}

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

// CleanupOpenSessions closes all sessions that are still open before today
func (s *workSessionService) CleanupOpenSessions(ctx context.Context) (int, error) {
	// Today's Berlin calendar day for the PostgreSQL DATE column
	today := timezone.TodayDate()

	// Get all open sessions before today
	openSessions, err := s.repo.GetOpenSessions(ctx, today)
	if err != nil {
		return 0, fmt.Errorf("failed to get open sessions: %w", err)
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

	return count, nil
}

// AutoCheckoutDueSessions closes open sessions at their planned shift end (#1798).
func (s *workSessionService) AutoCheckoutDueSessions(ctx context.Context, grace time.Duration) (int, error) {
	if s.staffShiftRepo == nil {
		return 0, nil
	}

	now := timezone.Now()
	// GetOpenSessions filters date < beforeDate; passing tomorrow includes
	// today's open sessions, which is where forgotten checkouts live.
	tomorrow := timezone.TodayDate().AddDays(1)
	openSessions, err := s.repo.GetOpenSessions(ctx, tomorrow)
	if err != nil {
		return 0, fmt.Errorf("failed to get open sessions: %w", err)
	}
	if len(openSessions) == 0 {
		return 0, nil
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
	// Check if already checked in
	currentSession, err := s.repo.GetCurrentByStaffID(ctx, staffID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check current session: %w", err)
	}

	if currentSession != nil && currentSession.IsActive() {
		return currentSession, nil
	}

	// Check if there's already a checked-out session today
	today := timezone.TodayDate()
	todaySession, err := s.repo.GetByStaffAndDate(ctx, staffID, today)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check today's session: %w", err)
	}

	if todaySession != nil {
		// Already checked out today, don't re-check-in
		return nil, nil
	}

	// No session today, create one with the channel the caller passed in.
	return s.CheckIn(ctx, staffID, activeModels.WorkSessionStatusPresent, source)
}

// German weekday names for export
var germanWeekdays = [7]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}

// German absence type labels for export
var germanAbsenceTypeLabels = map[string]string{
	activeModels.AbsenceTypeSick:     "Krank",
	activeModels.AbsenceTypeVacation: "Urlaub",
	activeModels.AbsenceTypeTraining: "Fortbildung",
	activeModels.AbsenceTypeOther:    "Sonstige",
}

// exportRow represents a single row in the export (either a work session or an absence day)
type exportRow struct {
	Date timezone.Date
	Row  []string
}

// ExportSessions generates a CSV or XLSX export of work sessions and absences
func (s *workSessionService) ExportSessions(ctx context.Context, staffID int64, from, to timezone.Date, format string) ([]byte, string, error) {
	historyResp, err := s.GetHistory(ctx, staffID, from, to)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get sessions for export: %w", err)
	}

	// Load absences for the same date range
	var absences []*activeModels.StaffAbsence
	if s.absenceRepo != nil {
		absences, err = s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, from, to)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get absences for export: %w", err)
		}
	}

	// Build merged rows sorted by date
	rows := s.buildExportRows(historyResp.Sessions, absences)

	fromStr := from.String()
	toStr := to.String()

	switch format {
	case "xlsx":
		data, err := s.exportXLSX(rows)
		if err != nil {
			return nil, "", err
		}
		return data, fmt.Sprintf("zeiterfassung_%s_%s.xlsx", fromStr, toStr), nil
	default:
		data, err := s.exportCSV(rows)
		if err != nil {
			return nil, "", err
		}
		return data, fmt.Sprintf("zeiterfassung_%s_%s.csv", fromStr, toStr), nil
	}
}

// buildExportRows merges session rows and absence rows, sorted by date
func (s *workSessionService) buildExportRows(sessions []*SessionResponse, absences []*activeModels.StaffAbsence) []exportRow {
	var rows []exportRow

	// Add session rows
	for _, sr := range sessions {
		rows = append(rows, exportRow{
			Date: sr.Date,
			Row:  s.sessionToRow(sr),
		})
	}

	// Add absence rows (one row per day in the absence range)
	for _, absence := range absences {
		if absence.Status != activeModels.AbsenceStatusReported &&
			absence.Status != activeModels.AbsenceStatusApproved {
			continue
		}
		label := germanAbsenceTypeLabels[absence.AbsenceType]
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

	// Sort by date
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Date.Before(rows[j].Date)
	})

	return rows
}

func (s *workSessionService) exportCSV(rows []exportRow) ([]byte, error) {
	var buf bytes.Buffer

	// UTF-8 BOM for Excel compatibility
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(&buf)
	w.Comma = ';'

	// Header
	if err := w.Write([]string{"Datum", "Wochentag", "Start", "Ende", "Pause (Min)", "Netto (Std)", "Status", "Quelle", "Bemerkungen"}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}

	for _, er := range rows {
		if err := w.Write(er.Row); err != nil {
			return nil, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("CSV write error: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *workSessionService) exportXLSX(rows []exportRow) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheet := "Zeiterfassung"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, fmt.Errorf("failed to create sheet: %w", err)
	}
	f.SetActiveSheet(idx)
	// Remove default "Sheet1" if it exists and is different
	if sheet != "Sheet1" {
		_ = f.DeleteSheet("Sheet1")
	}

	headers := []string{"Datum", "Wochentag", "Start", "Ende", "Pause (Min)", "Netto (Std)", "Status", "Quelle", "Bemerkungen"}

	// Header style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E2E8F0"}, Pattern: 1},
	})

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	// Data rows
	for rowIdx, er := range rows {
		for colIdx, val := range er.Row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheet, cell, val)
		}
	}

	// Auto-width columns
	for i := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheet, col, col, 16)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to write XLSX: %w", err)
	}

	return buf.Bytes(), nil
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

	pauseMin := strconv.Itoa(sess.BreakMinutes)

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
	if err := s.scheduleRepo.ReplaceSchedule(ctx, staff.ID, entries); err != nil {
		return fmt.Errorf("write assigned schedule snapshot: %w", err)
	}

	staff.WorkTimeModelID = &model.ID
	anchor := model.RotationAnchorDate
	staff.RotationAnchorDate = &anchor
	if err := s.staffRepo.Update(ctx, staff); err != nil {
		return fmt.Errorf("bind template to staff: %w", err)
	}
	return nil
}

// ApplyCustomScheduleRows replaces the schedule with custom rows and unbinds
// any assigned template.
func (s *workSessionService) ApplyCustomScheduleRows(ctx context.Context, staff *userModels.Staff, entries []*configModels.StaffWorkSchedule, anchor timezone.Date) error {
	if err := s.scheduleRepo.ReplaceSchedule(ctx, staff.ID, entries); err != nil {
		return fmt.Errorf("write custom schedule: %w", err)
	}

	staff.WorkTimeModelID = nil
	if !anchor.IsZero() {
		staff.RotationAnchorDate = &anchor
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
		anchor = timezone.TodayDate()
	}
	model := &configModels.WorkTimeModel{
		Name:               name,
		RotationLength:     rotation,
		RotationAnchorDate: anchor,
	}
	if err := s.workModelRepo.Create(ctx, model, entries); err != nil {
		return err
	}
	scheduleRows := modelEntriesToScheduleRows(entries, rotation)
	if err := s.scheduleRepo.ReplaceSchedule(ctx, staff.ID, scheduleRows); err != nil {
		return fmt.Errorf("write saved template schedule snapshot: %w", err)
	}

	staff.WorkTimeModelID = &model.ID
	staff.RotationAnchorDate = &anchor
	if err := s.staffRepo.Update(ctx, staff); err != nil {
		return fmt.Errorf("bind freshly created template: %w", err)
	}
	return nil
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
		})
	}
	return rows
}
