package timetracking

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (s *workSessionService) CheckIn(ctx context.Context, staffID int64, status, source, reason string) (*WorkSession, error) {
	return s.checkIn(ctx, staffID, status, source, reason, true)
}

func (s *workSessionService) CheckInOn(ctx context.Context, staffID int64, _ calendar.Date, status, source, reason string) (*WorkSession, error) {
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
func (s *workSessionService) checkIn(ctx context.Context, staffID int64, status, source, reason string, enforceDeviationGate bool) (*WorkSession, error) {
	if err := validateCheckInChannel(status, source); err != nil {
		return nil, err
	}
	if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, err
	}

	now := s.now()
	stampDay := calendar.DateFromTime(now)

	siblings, err := s.presentSiblings(ctx, staffID, now)
	if err != nil {
		return nil, err
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

	// The session is created on the day of its own stamp: the shift and
	// deviation checks below have to be read on that same day, or a stamp
	// taken just after midnight is measured against the previous day's shift.
	var deviation *plannedDeviation
	if len(existingBlocks) == 0 {
		if deviation, err = s.gateFirstBlock(ctx, staffID, stampDay, now, reason, enforceDeviationGate); err != nil {
			return nil, err
		}
	}

	session, err := s.createCheckInSession(ctx, staffID, stampDay, status, source, now)
	if err != nil {
		return nil, err
	}

	if deviation != nil {
		if err := s.recordDeviationReason(ctx, session, deviation, reason, now); err != nil {
			return nil, err
		}
	}

	s.broadcastTimeTrackingChanged(ctx)
	return session, nil
}

// validateCheckInChannel accepts the two work statuses and the two stamping
// channels a check-in may carry.
func validateCheckInChannel(status, source string) error {
	if status != WorkSessionStatusPresent && status != WorkSessionStatusHomeOffice {
		return workforce.ErrSessionStatusInvalid
	}
	if source != WorkSessionSourceApp && source != WorkSessionSourceNFC {
		return workforce.ErrSessionSourceInvalid
	}
	return nil
}

// presentSiblings reads every block that reaches into the present, in one
// read: the open ones (they run to infinity, so the query returns them
// whatever day they were filed on) and the closed ones that still end after
// "now". The same rows answer both questions of a check-in — whether somebody
// is clocked in, and whether the new block would overlap an existing one.
// Stale open blocks among them are closed before they are returned.
func (s *workSessionService) presentSiblings(ctx context.Context, staffID int64, now time.Time) ([]*WorkSession, error) {
	siblingsRecords, err := s.repo.ListOverlappingWorkSessions(ctx, []int64{staffID}, now, nil)
	siblings := workSessionsToRecords(siblingsRecords)
	err = rowReadError("list overlapping by staff ID", err, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing session: %w", err)
	}
	if err := s.closeStaleOpenSiblings(ctx, siblings, now); err != nil {
		return nil, err
	}
	return siblings, nil
}

// closeStaleOpenSiblings refuses the check-in while a block is open. An open
// block anywhere means the person is clocked in — the check is deliberately
// NOT limited to the stamp's day. A block opened before Berlin midnight is
// still running after it; starting a second one there would leave two open
// blocks, and the checkout (which closes exactly one) would then report a
// different day's work than the totals do. The still-open block has to be
// closed first.
//
// A block whose live limit has passed is the exception, and it has to be
// CLOSED here, not merely skipped: the database permits a single open block
// per staff member (uq_work_sessions_staff_date_open), so a forgotten
// checkout that is only ignored lets this guard pass and then fails the
// INSERT — locking the person out of the clock entirely, which is exactly
// what #2402 set out to fix. It is closed at the instant it stopped counting
// as work everywhere else (BalanceSessionEnd), so no total, balance or
// history row changes value; the row merely stops hanging open and
// disappears from the kiosk's running block.
func (s *workSessionService) closeStaleOpenSiblings(ctx context.Context, siblings []*WorkSession, now time.Time) error {
	for _, sibling := range siblings {
		if sibling.CheckOutTime != nil {
			continue
		}
		expired, stale := ExpireStaleOpenBlock(sibling, now)
		if !stale {
			return workforce.ErrAlreadyCheckedIn
		}
		if err := s.closeStaleOpenBlock(ctx, sibling, *expired.CheckOutTime); err != nil {
			return err
		}
	}
	return nil
}

// createCheckInSession validates and stores the new open block filed on the
// day of its own stamp.
func (s *workSessionService) createCheckInSession(ctx context.Context, staffID int64, stampDay calendar.Date, status, source string, now time.Time) (*WorkSession, error) {
	session := &WorkSession{
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
	stored, err := s.repo.CreateWorkSession(ctx, WorkSessionToCapability(session))
	err = workSessionWriteError("create", err)
	if err != nil {
		return nil, fmt.Errorf("failed to create work session: %w", err)
	}
	applyWorkSessionToRecord(session, stored)
	return session, nil
}

// gateFirstBlock runs the check-in gates on the day's FIRST block only: a
// later block resumes a work day that has already begun, exactly like the
// pre-#2402 reopen path, which was exempt for the same reason. The returned
// F9 deviation is recorded once the session exists.
func (s *workSessionService) gateFirstBlock(ctx context.Context, staffID int64, day calendar.Date, now time.Time, reason string, enforceDeviationGate bool) (*plannedDeviation, error) {
	if err := s.ensurePlannedStartReached(ctx, staffID, day, now); err != nil {
		return nil, err
	}
	// F9: checking in more than the tolerance before the planned shift start
	// needs a reason ("früher kommen").
	if !enforceDeviationGate {
		return nil, nil
	}
	deviation, err := s.detectPlannedDeviation(ctx, staffID, day, now, deviationActionCheckIn)
	if err != nil {
		return nil, err
	}
	if deviation != nil && strings.TrimSpace(reason) == "" {
		return nil, deviation.requiredError()
	}
	return deviation, nil
}

// ensurePlannedStartReached implements the planned-start lock (#3825): with
// the tenant setting on, a check-in opens at the earliest planned shift start
// of the day minus the configured tolerance. Arriving late is never refused,
// and a day without a shift is not locked, so stepping in on a free day works;
// an early start outside the window is entered afterwards by the leadership.
func (s *workSessionService) ensurePlannedStartReached(ctx context.Context, staffID int64, day calendar.Date, now time.Time) error {
	if s.settings == nil {
		return nil
	}
	enabled, err := s.settings.EnforcePlannedStart(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve planned-start setting: %w", err)
	}
	if !enabled {
		return nil
	}
	if s.staffShiftRepo == nil {
		return fmt.Errorf("staff shift repository not configured")
	}
	plannedStart, _, ok, err := s.plannedDayWindow(ctx, staffID, day)
	if err != nil || !ok {
		return err
	}
	toleranceMinutes, err := s.settings.PlannedStartToleranceMinutes(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve planned-start tolerance setting: %w", err)
	}
	if opensAt := workforce.CheckInOpensAt(plannedStart, toleranceMinutes); now.Before(opensAt) {
		return &PlannedStartNotReachedError{
			PlannedStartTime: opensAt.In(calendar.Berlin).Format("15:04"),
			CurrentTime:      now.In(calendar.Berlin).Format("15:04"),
		}
	}
	return nil
}

// plannedDayWindow loads the staff member's shifts (schedule.staff_shifts) of
// day and folds them into the window both check-in gates measure against.
func (s *workSessionService) plannedDayWindow(ctx context.Context, staffID int64, day calendar.Date) (start, end time.Time, ok bool, err error) {
	shifts, err := s.staffShiftRepo.FindByStaffIDsAndDate(ctx, []int64{staffID}, day)
	if err != nil {
		return start, end, false, fmt.Errorf("failed to load planned shifts: %w", err)
	}
	spans := make([]workforce.PlannedShiftSpan, len(shifts))
	for i, shift := range shifts {
		spans[i] = workforce.PlannedShiftSpan{Start: shift.StartInstant(), End: shift.EndInstant(), Cancelled: shift.Cancelled}
	}
	start, end, ok = workforce.PlannedDayWindow(spans)
	return start, end, ok, nil
}

func (s *workSessionService) EnsureCheckedIn(ctx context.Context, staffID int64, source string) (*WorkSession, error) {
	// Check if already checked in. Across all days, like the check-in guard:
	// a block that crossed Berlin midnight is still running, and starting a
	// supervision must return it instead of running into "already checked in".
	currentSessionRecords, err := s.repo.LatestOpenWorkSession(ctx, staffID)
	currentSession := workSessionReadRecord(currentSessionRecords, err)
	err = rowReadError("get latest open by staff ID", err, workforce.ErrWorkSessionNotFound)
	if err != nil && !isRecordNotFound(err) {
		return nil, fmt.Errorf("failed to check current session: %w", err)
	}

	if currentSession != nil && currentSession.IsActive() {
		return currentSession, nil
	}

	// Check if there's already a checked-out block today. Deliberately no
	// auto-created second block (#2402): whoever stamped out made a
	// conscious decision, and a supervision start must not silently begin a
	// new work block behind their back.
	today := calendar.DateFromTime(s.now())
	todaySessions, err := s.listSessionsByStaffAndDate(ctx, staffID, today)
	if err != nil {
		return nil, fmt.Errorf("failed to check today's session: %w", err)
	}

	if len(todaySessions) > 0 {
		// Already checked out today, don't re-check-in
		return nil, nil
	}

	// External caregivers record no working time (#3823), with or without
	// an account (#4573): their supervision opens no block. Asked only now,
	// when a block would be opened.
	external, err := s.externalCaregivers.IsExternalCaregiver(ctx, staffID)
	if err != nil {
		return nil, fmt.Errorf("failed to check external caregiver: %w", err)
	}
	if external {
		return nil, nil
	}

	// No session today, create one with the channel the caller passed in.
	// The F9 deviation gate is bypassed: this auto-stamp fires because the
	// staff member started a supervision, and that flow cannot collect a
	// reason (see checkIn).
	return s.checkIn(ctx, staffID, WorkSessionStatusPresent, source, "", false)
}
