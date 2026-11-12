package timetracking

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// sessionUpdateContext holds state during session update to avoid passing many parameters.
type sessionUpdateContext struct {
	session    *WorkSession
	sessionID  int64
	staffID    int64
	now        time.Time
	auditEdits []*WorkSessionEdit
	notes      *string
}

func (uc *sessionUpdateContext) addAuditEdit(field string, oldVal, newVal *string) {
	uc.auditEdits = append(uc.auditEdits, &WorkSessionEdit{
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

func (s *workSessionService) UpdateSession(ctx context.Context, staffID int64, sessionID int64, updates SessionUpdateRequest) (*WorkSession, error) {
	if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	sessionRecords, err := s.repo.FindWorkSession(ctx, sessionID)
	session := workSessionReadRecord(sessionRecords, err)
	err = rowReadError("find by id", err, workforce.ErrWorkSessionNotFound)
	if err != nil {
		return nil, s.handleSessionNotFoundError(err)
	}

	if session.StaffID != staffID {
		return nil, workforce.ErrSessionUpdateNotOwned
	}

	if err := s.ensureSelfEditJustified(ctx, session, updates); err != nil {
		return nil, err
	}
	// Work cannot have started yet in the future. An open session with such a
	// check-in could not be closed by the nightly cleanup (#6288).
	if updates.CheckInTime != nil && !session.CheckInTime.Equal(*updates.CheckInTime) && updates.CheckInTime.After(s.now()) {
		return nil, workforce.ErrSessionCheckInInFuture
	}

	return s.applySessionUpdate(ctx, staffID, session, updates)
}

// ensureSelfEditJustified enforces the notes a self-edit must carry.
//
// A status change (Vor Ort ↔ Homeoffice) carries audit weight and must be
// justified. Otherwise the audit trail loses meaning. Requires the caller
// to send the reason in `notes` on the same request.
//
// F8: with the deviation-reason setting active, self-edits of the
// recorded times (check-in, check-out, break minutes — including
// per-break duration edits, which are break-minute changes) need notes
// too, not only status changes. For the scalar fields the change
// detection mirrors applyTimeFieldUpdates/applyBreakUpdates, so an
// unchanged resend never trips the gate; a non-empty per-break list
// always counts as an edit intent (comparing it against stored rows
// would need an extra query before the ownership checks ran).
func (s *workSessionService) ensureSelfEditJustified(ctx context.Context, session *WorkSession, updates SessionUpdateRequest) error {
	notesMissing := updates.Notes == nil || strings.TrimSpace(*updates.Notes) == ""
	if updates.Status != nil && *updates.Status != session.Status && notesMissing {
		return workforce.ErrSessionStatusNoteRequired
	}
	if !s.selfEditChangesRecordedTimes(session, updates) || !notesMissing {
		return nil
	}
	required, err := s.deviationReasonRequired(ctx)
	if err != nil {
		return err
	}
	if required {
		return workforce.ErrSessionTimesNoteRequired
	}
	return nil
}

// deviationReasonRequired resolves the F8/F9 tenant setting; nil settings
// (tests, minimal wiring) mean the gate is off.
func (s *workSessionService) deviationReasonRequired(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	enabled, err := s.settings.RequireDeviationReason(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to resolve deviation-reason setting: %w", err)
	}
	return enabled, nil
}

// selfEditChangesRecordedTimes reports whether the update would change
// check_in_time, check_out_time, or break minutes on the session.
func (s *workSessionService) selfEditChangesRecordedTimes(session *WorkSession, updates SessionUpdateRequest) bool {
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

func (s *workSessionService) UpdateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID, sessionID int64, updates SessionUpdateRequest) (*WorkSession, error) {
	if err := s.lockStaffBalanceWrites(ctx, targetStaffID); err != nil {
		return nil, err
	}
	sessionRecords, err := s.repo.FindWorkSession(ctx, sessionID)
	session := workSessionReadRecord(sessionRecords, err)
	err = rowReadError("find by id", err, workforce.ErrWorkSessionNotFound)
	if err != nil {
		return nil, s.handleSessionNotFoundError(err)
	}

	if session.StaffID != targetStaffID {
		return nil, &workforce.TimeTrackingError{Kind: workforce.ErrAdminSessionNotOwned, Cause: fmt.Errorf("session does not belong to staff %d", targetStaffID)}
	}

	if updates.Notes == nil || strings.TrimSpace(*updates.Notes) == "" {
		return nil, workforce.ErrAdminSessionNoteRequired
	}

	return s.applySessionUpdate(ctx, editorStaffID, session, updates)
}

// applySessionUpdate is the shared body used by both self-edit and admin
// edit. editorStaffID lands in audit.work_session_edits.edited_by so the
// MA-side can distinguish "ich selbst" from "Florian (Admin)" without an
// extra column.
func (s *workSessionService) applySessionUpdate(ctx context.Context, editorStaffID int64, session *WorkSession, updates SessionUpdateRequest) (*WorkSession, error) {
	uc := &sessionUpdateContext{
		session:   session,
		sessionID: session.ID,
		staffID:   editorStaffID,
		now:       time.Now(),
		notes:     updates.Notes,
	}

	recordedDay, recordedCheckIn := session.Date, session.CheckInTime
	s.applyTimeFieldUpdates(uc, updates)
	s.applySimpleFieldUpdates(uc, updates)

	session.UpdatedBy = &editorStaffID
	session.UpdatedAt = uc.now

	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf(errInvalidSessionData, err)
	}
	if err := checkEditedSessionDay(session, recordedDay, recordedCheckIn); err != nil {
		return nil, err
	}

	// Edited times must not slide into a sibling block.
	if err := s.assertNoBlockOverlap(ctx, session.StaffID, session.ID, session.CheckInTime, session.CheckOutTime); err != nil {
		return nil, err
	}

	// Validate the proposed session and all pauses together before any write.
	if err := s.applyBreakUpdates(ctx, uc, updates); err != nil {
		return nil, err
	}

	stored, err := s.repo.UpdateWorkSession(ctx, WorkSessionToCapability(session))
	err = workSessionWriteError("update", err)
	if errors.Is(err, workforce.ErrWorkSessionNotFound) {
		err = rowsAffectedError("update WorkSession")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}
	applyWorkSessionToRecord(session, stored)

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

func (s *workSessionService) CreateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID int64, req AdminCreateSessionRequest) (*WorkSession, error) {
	if err := validateAdminCreateSession(targetStaffID, &req); err != nil {
		return nil, err
	}
	if err := s.lockStaffBalanceWrites(ctx, targetStaffID); err != nil {
		return nil, err
	}

	checkOut := req.CheckOutTime
	date := calendar.DateFromTime(req.Date)
	// A day can carry several blocks since #2402, so the Nachtrag no longer
	// collides with an existing session per se — but it must not overlap one.
	if err := s.assertNoBlockOverlap(ctx, targetStaffID, 0, req.CheckInTime, &checkOut); err != nil {
		return nil, err
	}
	session := &WorkSession{
		StaffID:      targetStaffID,
		Date:         date,
		Status:       req.Status,
		Source:       WorkSessionSourceApp,
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

	stored, err := s.repo.CreateWorkSession(ctx, WorkSessionToCapability(session))
	err = workSessionWriteError("create", err)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	applyWorkSessionToRecord(session, stored)

	edits := adminCreateAuditEdits(session.ID, targetStaffID, editorStaffID, date, req, tenant.FromContext(ctx))
	if err := s.auditRepo.CreateBatch(ctx, edits); err != nil {
		return nil, fmt.Errorf("failed to create audit entries: %w", err)
	}

	s.broadcastTimeTrackingChanged(ctx)
	return session, nil
}

// validateAdminCreateSession checks an admin Nachtrag before anything is
// locked or read, and defaults a missing status to present.
func validateAdminCreateSession(targetStaffID int64, req *AdminCreateSessionRequest) error {
	if targetStaffID <= 0 {
		return workforce.ErrTargetStaffRequired
	}
	if strings.TrimSpace(req.Notes) == "" {
		return workforce.ErrAdminSessionCreateNoteRequired
	}
	if req.CheckInTime.IsZero() || req.CheckOutTime.IsZero() {
		return fmt.Errorf("check-in and check-out are required")
	}
	if !req.CheckOutTime.After(req.CheckInTime) {
		return workforce.ErrSessionCheckoutOrder
	}
	if req.Status == "" {
		req.Status = WorkSessionStatusPresent
	}
	if req.BreakMinutes < 0 {
		return workforce.ErrAdminBreakMinutesNegative
	}
	return checkInWithinSessionDay(calendar.DateFromTime(req.Date), req.CheckInTime)
}

// checkEditedSessionDay applies checkInWithinSessionDay only when an edit
// changed the day or the check-in, so a row an earlier edit already broke can
// still get its notes and be repaired.
func checkEditedSessionDay(session *WorkSession, recordedDay calendar.Date, recordedCheckIn time.Time) error {
	if session.Date == recordedDay && session.CheckInTime.Equal(recordedCheckIn) {
		return nil
	}
	return checkInWithinSessionDay(session.Date, session.CheckInTime)
}

// checkInWithinSessionDay rejects a session filed on a day before its
// check-in. The nightly cleanup closes a forgotten session at the end of its
// day; that close must lie after the check-in (#6288).
func checkInWithinSessionDay(day calendar.Date, checkIn time.Time) error {
	// The day ends at the next Berlin midnight, not at 23:59:59: a check-in in
	// the last second's fractions still belongs to it, on 23- and 25-hour days
	// as well.
	if !checkIn.Before(day.AddDays(1).BerlinMidnight()) {
		return workforce.ErrSessionCheckInAfterDate
	}
	return nil
}

// adminCreateAuditEdits builds the audit trail of an admin Nachtrag: one
// entry per field so the EditHistoryAccordion can render the create as a
// normal change list with empty "Vorher" values.
func adminCreateAuditEdits(sessionID, targetStaffID, editorStaffID int64, date calendar.Date, req AdminCreateSessionRequest, tenantID int64) []*WorkSessionEdit {
	now := time.Now()
	notesPtr := req.Notes
	strPtr := func(s string) *string { return &s }
	baseEdit := func(field string, newVal string) *WorkSessionEdit {
		e := &WorkSessionEdit{
			SessionID: sessionID,
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
	return []*WorkSessionEdit{
		baseEdit("date", date.String()),
		baseEdit("check_in_time", req.CheckInTime.Format(time.RFC3339)),
		baseEdit("check_out_time", req.CheckOutTime.Format(time.RFC3339)),
		baseEdit("break_minutes", fmt.Sprintf("%d", req.BreakMinutes)),
		baseEdit("status", req.Status),
	}
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
	siblingsRecords, err := s.repo.ListOverlappingWorkSessions(ctx, []int64{staffID}, checkIn, checkOut)
	siblings := workSessionsToRecords(siblingsRecords)
	err = rowReadError("list overlapping by staff ID", err, nil)
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
func (s *workSessionService) closeStaleOpenBlock(ctx context.Context, session *WorkSession, at time.Time) error {
	usable, err := inBestEffortSavepoint(ctx, func(ctx context.Context) error {
		return s.endActiveBreakIfExists(ctx, session.ID, at)
	})
	if !usable {
		return fmt.Errorf("failed to end active break of stale work session %d: %w", session.ID, err)
	}
	if err != nil {
		// Mirrors CleanupOpenSessions: a break that cannot be ended must not
		// keep the block open — that would leave the staff member unable to
		// stamp, which is the very lockout this repair exists to prevent.
		s.getLogger().WarnContext(ctx, "failed to end active break of a stale work session",
			slog.Int64("session_id", session.ID),
			slog.String("error", err.Error()))
	}
	_, err = s.repo.CloseWorkSession(ctx, session.ID, at, true)
	if err = workSessionWriteError("close session", err); err != nil {
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
func ExpireStaleOpenBlock(session *WorkSession, now time.Time) (*WorkSession, bool) {
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
func expireStaleOpenBlocks(siblings []*WorkSession, now time.Time) []*WorkSession {
	expired := make([]*WorkSession, len(siblings))
	for i, sibling := range siblings {
		expired[i], _ = ExpireStaleOpenBlock(sibling, now)
	}
	return expired
}

// assertNoBlockOverlapIn is the list-based body of assertNoBlockOverlap, kept
// separate so the interval arithmetic can be exercised without a database.
func assertNoBlockOverlapIn(siblings []*WorkSession, excludeID int64, checkIn time.Time, checkOut *time.Time) error {
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
		return &workforce.TimeTrackingError{Kind: workforce.ErrWorkSessionOverlap, Cause: fmt.Errorf("work session overlaps an existing block (%s–%s)",
			sibling.CheckInTime.In(calendar.Berlin).Format("15:04"),
			formatBlockEnd(sibling.CheckOutTime))}
	}
	return nil
}

func formatBlockEnd(checkOut *time.Time) string {
	if checkOut == nil {
		return "offen"
	}
	return checkOut.In(calendar.Berlin).Format("15:04")
}

func (s *workSessionService) handleSessionNotFoundError(err error) error {
	if isRecordNotFound(err) {
		return workforce.ErrSessionNotFound
	}
	return fmt.Errorf("failed to get session: %w", err)
}

func (s *workSessionService) applyTimeFieldUpdates(uc *sessionUpdateContext, updates SessionUpdateRequest) {
	strPtr := func(str string) *string { return &str }

	if updates.Date != nil {
		// The wire carries an RFC3339 instant; only its Berlin calendar day
		// matters for the DATE column.
		newDate := calendar.DateFromTime(*updates.Date)
		if uc.session.Date != newDate {
			uc.addAuditEdit("date", strPtr(uc.session.Date.String()), strPtr(newDate.String()))
			uc.session.Date = newDate
		}
	}

	if updates.CheckInTime != nil {
		// Compare actual time points, not string representations (timezone-safe)
		if !uc.session.CheckInTime.Equal(*updates.CheckInTime) {
			oldVal := uc.session.CheckInTime.Format(time.RFC3339)
			newVal := updates.CheckInTime.Format(time.RFC3339)
			uc.addAuditEdit("check_in_time", strPtr(oldVal), strPtr(newVal))
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
			uc.addAuditEdit("check_out_time", strPtr(oldVal), strPtr(newVal))
		}
		uc.session.CheckOutTime = updates.CheckOutTime
	}
}

func (s *workSessionService) applySimpleFieldUpdates(uc *sessionUpdateContext, updates SessionUpdateRequest) {
	strPtr := func(str string) *string { return &str }

	if updates.Status != nil && uc.session.Status != *updates.Status {
		uc.addAuditEdit("status", strPtr(uc.session.Status), updates.Status)
		uc.session.Status = *updates.Status
	}

	if updates.Notes != nil && uc.session.Notes != *updates.Notes {
		uc.addAuditEdit("notes", strPtr(uc.session.Notes), updates.Notes)
		uc.session.Notes = *updates.Notes
	}
}
