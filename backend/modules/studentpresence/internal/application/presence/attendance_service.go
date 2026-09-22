package presence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Attendance tracking operations

func (s *service) GetStudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*AttendanceStatus, error) {
	if len(studentIDs) == 0 {
		return map[int64]*AttendanceStatus{}, nil
	}

	statuses := make(map[int64]*AttendanceStatus, len(studentIDs))

	if s.SchoolPresence == nil {
		return nil, &ActiveError{Op: "GetStudentsAttendanceStatuses", Err: ErrDatabaseOperation}
	}
	today := s.todayDate()
	attendanceRecords, err := s.SchoolPresence.ListSchoolStatuses(ctx, studentIDs, today.String())
	if err != nil {
		return nil, &ActiveError{Op: "GetStudentsAttendanceStatuses", Err: ErrDatabaseOperation}
	}
	for _, attendance := range attendanceRecords {
		status := &AttendanceStatus{
			StudentID:    attendance.StudentID,
			Status:       attendance.Status,
			Date:         today,
			CheckInTime:  attendance.CheckInTime,
			CheckOutTime: attendance.CheckOutTime,
			YardSince:    attendance.YardSince,
		}
		statuses[attendance.StudentID] = status
	}

	return statuses, nil
}

// GetStudentAttendanceStatus gets today's latest attendance record and determines status
func (s *service) GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*AttendanceStatus, error) {
	if s.SchoolPresence == nil {
		return nil, &ActiveError{Op: "GetStudentAttendanceStatus", Err: ErrDatabaseOperation}
	}
	today := s.todayDate()
	statuses, err := s.SchoolPresence.ListSchoolStatuses(ctx, []int64{studentID}, today.String())
	if err != nil || len(statuses) != 1 {
		return nil, &ActiveError{Op: "GetStudentAttendanceStatus", Err: ErrDatabaseOperation}
	}
	attendance := statuses[0]

	result := &AttendanceStatus{
		StudentID:    studentID,
		Status:       attendance.Status,
		Date:         today,
		CheckInTime:  attendance.CheckInTime,
		CheckOutTime: attendance.CheckOutTime,
		YardSince:    attendance.YardSince,
	}

	if attendance.CheckedInBy > 0 {
		result.CheckedInBy = s.getStaffNameByID(ctx, attendance.CheckedInBy)
	}

	if attendance.CheckedOutBy != nil && *attendance.CheckedOutBy > 0 {
		result.CheckedOutBy = s.getStaffNameByID(ctx, *attendance.CheckedOutBy)
	}
	return result, nil
}

// getStaffNameByID retrieves staff member's full name by ID
func (s *service) getStaffNameByID(ctx context.Context, staffID int64) string {
	name, err := s.StaffNames.StaffName(ctx, staffID)
	if err != nil {
		return ""
	}
	return name
}

// ToggleStudentAttendance toggles the attendance state (check-in or check-out)
// skipAuthCheck: if true, skips authorization check (used when caller already authorized).
//
// IMPORTANT: only safe when the caller serializes scans (e.g. an IoT kiosk).
// Web callers MUST use CheckInStudent / CheckOutStudent instead — under
// concurrency the read-then-flip here can swap a desired "in" into an "out"
// because the second caller's internal re-read sees the first caller's
// commit and flips the action.
func (s *service) ToggleStudentAttendance(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*AttendanceResult, error) {
	var result *AttendanceResult
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.toggleStudentAttendance(txCtx, studentID, staffID, deviceID, skipAuthCheck)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) toggleStudentAttendance(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*AttendanceResult, error) {
	authorizedStaffID, err := s.authorizeAttendanceToggle(ctx, studentID, staffID, deviceID, skipAuthCheck)
	if err != nil {
		return nil, err
	}

	currentStatus, err := s.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
	}

	now := s.now()
	today := timezone.DateFromTime(now)

	// "on_yard" is a sub-state of "checked_in" (still on premises) — toggling
	// from either should perform a checkout. Only "not_checked_in" and
	// "checked_out" start a fresh check-in.
	var result *AttendanceResult
	if currentStatus.Status == "not_checked_in" || currentStatus.Status == "checked_out" {
		result, err = s.performCheckIn(ctx, studentID, authorizedStaffID, deviceID, now, today, checkinTypeToggle)
	} else {
		result, err = s.performCheckOut(ctx, studentID, authorizedStaffID, deviceID, now, today, checkoutTypeToggle)
	}
	if err != nil {
		return nil, err
	}

	// A staff member marking a student's attendance is working right now —
	// auto-open their work session so they show as "Anwesend" (issue #1439).
	s.ensureStaffPresenceForAttendanceResult(ctx, authorizedStaffID, result)

	return result, nil
}

// attendanceStampSource picks the work-session source channel for the
// presence auto-stamp: kiosk-driven requests stamp as nfc, web requests as app.
func (s *service) attendanceStampSource(ctx context.Context) string {
	if s.attendancePrincipal(ctx).IsIoT {
		return stampSourceNFC
	}
	return stampSourceApp
}

func (s *service) ensureStaffPresenceForAttendanceResult(
	ctx context.Context,
	staffID int64,
	result *AttendanceResult,
) {
	if result == nil || (result.Action == "checked_out" && result.AttendanceID == 0) {
		return
	}
	s.ensureStaffPresence(ctx, staffID, s.attendanceStampSource(ctx))
}

// ensureStaffPresence best-effort-opens today's work session for the staff
// member so the triggering action marks them present (issue #1439). Mirrors
// the auto-stamp on kiosk session start (assignMultipleSupervisorsNonCritical):
// failures are logged, never returned, and an already-closed session for
// today is left untouched (EnsureCheckedIn returns nil, nil).
func (s *service) ensureStaffPresence(ctx context.Context, staffID int64, source string) {
	if staffID <= 0 || s.WorkSessionService == nil {
		return
	}

	s.runBestEffortDB(ctx, "staff_presence_checkin", func() error {
		session, err := s.WorkSessionService.EnsureCheckedIn(ctx, staffID, source)
		if err != nil {
			return err
		}
		if session == nil {
			s.getLogger().DebugContext(ctx, "staff already checked out today, presence auto-stamp skipped",
				slog.Int64("staff_id", staffID),
			)
		}
		return nil
	}, func(err error) {
		s.getLogger().WarnContext(ctx, "auto work-session check-in failed",
			slog.Int64("staff_id", staffID),
			slog.String("source", source),
			slog.String("error", err.Error()),
		)
	})
}

// CheckInStudent applies "in" unconditionally. Concurrency-safe: the
// underlying insert uses ON CONFLICT DO NOTHING against the partial unique
// index, so the loser of an in/in race is absorbed and reports the existing
// open row as the canonical result. Action is always "checked_in".
func (s *service) CheckInStudent(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*AttendanceResult, error) {
	var result *AttendanceResult
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.checkInStudent(txCtx, studentID, staffID, deviceID, skipAuthCheck)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) checkInStudent(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*AttendanceResult, error) {
	authorizedStaffID, err := s.authorizeAttendanceToggle(ctx, studentID, staffID, deviceID, skipAuthCheck)
	if err != nil {
		return nil, err
	}
	now := s.now()
	today := timezone.DateFromTime(now)
	result, err := s.performCheckIn(ctx, studentID, authorizedStaffID, deviceID, now, today, checkinTypeWeb)
	if err != nil {
		return nil, err
	}
	// Marking a student present marks the acting staff member present too
	// (issue #1439) — this is the binary-mode web check-in path.
	s.ensureStaffPresenceForAttendanceResult(ctx, authorizedStaffID, result)
	return result, nil
}

// CheckOutStudent applies "out" unconditionally via the state-checked
// CloseOpenForToday repository method. If no open row exists (already closed,
// never checked in, or another concurrent caller already closed it), returns
// an idempotent successful result with Action="checked_out" and no
// AttendanceID — the caller can re-fetch the latest status if they need
// timestamps for display. Any open room visit is ended as part of the same
// operation (issue #895) — callers don't need a separate EndVisit call.
func (s *service) CheckOutStudent(ctx context.Context, studentID, staffID int64, skipAuthCheck bool) (*AttendanceResult, error) {
	var result *AttendanceResult
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.checkOutStudent(txCtx, studentID, staffID, skipAuthCheck)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) checkOutStudent(ctx context.Context, studentID, staffID int64, skipAuthCheck bool) (*AttendanceResult, error) {
	// Auth path is shared with the toggle — pass deviceID=0 because the
	// caller is web-side (no kiosk involved); IsIoTDeviceRequest is false
	// for web so authorizeWebToggle runs and validates teacher access.
	authorizedStaffID, err := s.authorizeAttendanceToggle(ctx, studentID, staffID, 0, skipAuthCheck)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	result, err := s.performCheckOut(ctx, studentID, authorizedStaffID, 0, now, timezone.DateFromTime(now), checkoutTypeWeb)
	if err != nil {
		return nil, err
	}
	// A real checkout is an on-duty action. Idempotent retries must not create
	// a work session when they did not mutate attendance.
	s.ensureStaffPresenceForAttendanceResult(ctx, authorizedStaffID, result)
	return result, nil
}

// CheckOutStudentFromDevice applies "out" for an authenticated IoT device.
// A verified personal staff credential wins over the device session supervisor;
// when neither exists, the authenticated device remains the audit principal.
func (s *service) CheckOutStudentFromDevice(ctx context.Context, studentID, deviceID int64) (*AttendanceResult, error) {
	staffID := int64(0)
	if principal := s.attendancePrincipal(ctx); principal.HasStaff {
		staffID = principal.StaffID
	} else {
		resolvedStaffID, err := s.getDeviceSupervisorID(ctx, deviceID)
		if err != nil {
			var unavailable *deviceSupervisorUnavailableError
			if !errors.As(err, &unavailable) {
				return nil, &ActiveError{
					Op:  "ToggleStudentAttendance",
					Err: fmt.Errorf("resolve daily checkout staff attribution: %w", err),
				}
			}
			s.getLogger().DebugContext(ctx, "daily checkout has no staff attribution; using authenticated device",
				slog.Int64("device_id", deviceID),
				slog.String("reason", unavailable.Error()),
			)
		} else {
			staffID = resolvedStaffID
		}
	}
	var result *AttendanceResult
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		now := time.Now()
		var err error
		result, err = s.performCheckOut(txCtx, studentID, staffID, deviceID, now, timezone.DateFromTime(now), checkoutTypeDaily)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// authorizeAttendanceToggle handles authorization and returns the staff ID to use
func (s *service) authorizeAttendanceToggle(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (int64, error) {
	if skipAuthCheck {
		return staffID, nil
	}

	isIoTDevice := s.attendancePrincipal(ctx).IsIoT

	if isIoTDevice {
		return s.authorizeIoTDeviceToggle(ctx, deviceID)
	}

	return s.authorizeWebToggle(ctx, studentID, staffID)
}

// authorizeWebToggle authorizes web/manual attendance toggle. Any staff
// member may toggle any student (#2329) — the former education-group /
// room-supervision gate is gone; the caller's staff identity was already
// resolved from the JWT.
func (s *service) authorizeWebToggle(_ context.Context, _, staffID int64) (int64, error) {
	return staffID, nil
}

// authorizeIoTDeviceToggle authorizes IoT device attendance toggle
func (s *service) authorizeIoTDeviceToggle(ctx context.Context, deviceID int64) (int64, error) {
	supervisorStaffID, err := s.getDeviceSupervisorID(ctx, deviceID)
	if err != nil {
		return 0, &ActiveError{
			Op:  "ToggleStudentAttendance",
			Err: fmt.Errorf("device must have an active group with supervisors: %w", err),
		}
	}
	return supervisorStaffID, nil
}

// performCheckIn creates a new attendance record for check-in.
// deviceID == 0 marks a web-originated check-in with no kiosk involved;
// in that case we fall back to the virtual WEB-MANUAL-001 device so the
// FK column always points at a real iot.devices row.
//
// The insert is guarded by a partial unique index on
// (student_id, date) WHERE check_out_time IS NULL (migration 1.15.42), so a
// concurrent second "in" call is silently absorbed via ON CONFLICT and we
// re-fetch the open row to return as the canonical result.
//
// A check-in that actually opened a row fans out over SSE after the request
// transaction commits (#2113); the absorbed one stays silent.
func (s *service) performCheckIn(ctx context.Context, studentID, staffID, deviceID int64, now time.Time, today timezone.Date, checkinType string) (*AttendanceResult, error) {
	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	// Reject (and lock against a concurrent graduation) a graduated alumnus
	// before writing attendance — the binary-mode / attendance-toggle counterpart
	// to the CreateVisit guard (#405).
	if err := s.ensureStudentCheckinAllowed(ctx, studentID); err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
	}

	attendance, inserted, err := s.openAttendance(ctx, studentID, staffID, deviceID, now, today)
	if err != nil {
		return nil, err
	}
	if !inserted {
		return s.absorbConcurrentCheckIn(ctx, studentID, today, mode)
	}

	if err := s.autoClearCheckinStatuses(ctx, studentID, "ToggleStudentAttendance"); err != nil {
		return nil, err
	}
	if mode == PresenceModeBinary && s.AttendanceSyncer != nil {
		if _, err := s.AttendanceSyncer.MirrorCheckInAt(ctx, studentID, now); err != nil {
			return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("sync roomless check-in: %w", err)}
		}
	}

	s.registerCheckinBroadcast(ctx, studentID, checkinType)

	s.trackProductEvent(ctx, "student_checked_in", map[string]any{
		"method": s.attendanceMethod(ctx),
	})

	return &AttendanceResult{
		Action:       "checked_in",
		AttendanceID: attendance.ID,
		StudentID:    studentID,
		Timestamp:    now,
		Changed:      true,
	}, nil
}

// openAttendance inserts today's open attendance row; inserted is false when
// a concurrent check-in already opened it.
func (s *service) openAttendance(ctx context.Context, studentID, staffID, deviceID int64, now time.Time, today timezone.Date) (studentpresence.Attendance, bool, error) {
	resolvedDeviceID, err := s.resolveDeviceIDForAttendance(ctx, deviceID)
	if err != nil {
		return studentpresence.Attendance{}, false, &ActiveError{Op: "ToggleStudentAttendance", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	attendance := studentpresence.Attendance{
		StudentID:   studentID,
		Date:        today.String(),
		CheckInTime: now,
		CheckedInBy: staffID,
		DeviceID:    resolvedDeviceID,
	}
	attendance.TenantID = tenant.FromContext(ctx)

	attendance, inserted, err := s.SchoolPresence.EnsureAttendance(ctx, attendance)
	if err != nil {
		return studentpresence.Attendance{}, false, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
	}
	return attendance, inserted, nil
}

// absorbConcurrentCheckIn answers a check-in whose insert lost against a
// concurrent "in" that already created the open attendance row. It is treated
// as success — the desired end state (open attendance) holds. Deliberately
// silent: the winner of the race already broadcast, and nothing moved here
// (mirror of the closed == nil path in performCheckOut). The re-fetch is
// scoped to the caller's snapshot date, not a re-derived "today" (review
// #2372): a batch crossing Berlin midnight after its insert conflict would
// otherwise query the next day, find no row, and abort an otherwise
// idempotent batch.
func (s *service) absorbConcurrentCheckIn(ctx context.Context, studentID int64, today timezone.Date, mode string) (*AttendanceResult, error) {
	rows, fetchErr := s.SchoolPresence.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{studentID}, FromDate: today.String(), UntilDate: today.String()})
	if fetchErr != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fetchErr}
	}
	if len(rows) == 0 {
		return nil, &ActiveError{
			Op:  "ToggleStudentAttendance",
			Err: fmt.Errorf("attendance row missing after insert conflict for student %d on %s", studentID, today),
		}
	}
	// Rows come back check_in_time ASC — the last one is the open row the
	// concurrent winner created.
	existing := rows[len(rows)-1]
	if err := s.autoClearCheckinStatuses(ctx, studentID, "ToggleStudentAttendance"); err != nil {
		return nil, err
	}
	if mode == PresenceModeBinary && s.AttendanceSyncer != nil {
		if _, err := s.AttendanceSyncer.MirrorCheckInAt(ctx, studentID, existing.CheckInTime); err != nil {
			return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("sync existing roomless check-in: %w", err)}
		}
	}
	return &AttendanceResult{
		Action:       "checked_in",
		AttendanceID: existing.ID,
		StudentID:    studentID,
		Timestamp:    existing.CheckInTime,
		Changed:      false, // the concurrent winner opened the row, not this call
	}, nil
}

// endOpenVisitForStudent enforces the invariant "attendance checked_out =>
// no open visit" (issue #895) for the attendance day being checked out. day
// names that calendar day: a visit whose Berlin entry date is LATER than day
// belongs to a newer care session and is left untouched — a batch checkout
// crossing Berlin midnight closes its snapshot day's attendance and must not
// end a room visit the student started after the rollover (review #2372).
// Visits entered on day itself or earlier (orphaned leftovers) are still
// ended. Single-student callers derive day from their own now, so for them
// every open visit qualifies and behavior is unchanged. It returns the ended
// row so callers can mirror the same checkout into slot attendance. A missing
// or newer-day visit returns nil; every other failure propagates so the
// request transaction rolls back.
func (s *service) endOpenVisitForStudent(ctx context.Context, studentID int64, day timezone.Date) (*studentpresence.Visit, error) {
	visit, err := s.GetStudentCurrentVisit(ctx, studentID)
	if err != nil {
		if errors.Is(err, ErrVisitNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if timezone.DateFromTime(visit.EntryTime).After(day) {
		return nil, nil
	}
	closed, err := s.SchoolPresence.CloseVisits(ctx, []int64{visit.ID}, time.Now())
	if err != nil {
		return nil, err
	}
	if len(closed) != 0 {
		return &closed[0], nil
	}
	// A successful state-checked close can affect no rows when another
	// caller already closed the interval. Resolve that state, never a write error.
	ended, err := s.SchoolPresence.FindVisit(ctx, visit.ID)
	if err != nil {
		return nil, err
	}
	if ended == nil || ended.ExitTime == nil {
		return nil, ErrVisitNotFound
	}
	return ended, nil
}

// performCheckOut closes the open attendance row for the student via a
// state-checked UPDATE WHERE check_out_time IS NULL. today names the
// calendar day whose row is closed: single checkouts pass the day of their
// own now, the batch passes its batch-wide snapshot so a batch crossing
// Berlin midnight closes the same day it read and refreshes (review #2372).
// Three key properties:
//
//  1. Concurrency-safe: a second concurrent "out" call (or an "in" call that
//     lost the race against another "out") simply finds no open row to
//     update; we report idempotent success rather than corrupting state.
//  2. Yard sub-state is cleared as part of the same UPDATE so detailed-mode
//     callers don't observe an inconsistent (CheckOutTime set, YardSince
//     still set) row even briefly.
//  3. Any open room visit entered on (or before) today is ended in the same
//     request transaction (issue #895) — including on the idempotent
//     no-open-row path, so a checkout of any kind heals an orphaned visit
//     left behind by older code. A visit entered AFTER today is a newer
//     session's and stays open (see endOpenVisitForStudent).
//  4. A checkout that closed attendance or healed an orphaned visit fans out
//     over SSE after the request transaction commits (#2113).
func (s *service) performCheckOut(ctx context.Context, studentID, staffID, checkoutDeviceID int64, now time.Time, today timezone.Date, checkoutType string) (*AttendanceResult, error) {
	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if err := s.SchoolPresence.LockStudentAttendance(ctx, studentID); err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("lock attendance checkout: %w", err)}
	}
	closedRows, err := s.SchoolPresence.CloseAttendance(ctx, studentpresence.AttendanceCheckout{StudentIDs: []int64{studentID}, Date: today.String(), At: now, StaffID: staffID, DeviceID: checkoutDeviceID})
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("database error during state-checked checkout: %w", err)}
	}

	endedVisit, err := s.endOpenVisitForStudent(ctx, studentID, today)
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("end open visit during checkout: %w", err)}
	}

	snapshot, err := s.mirrorAttendanceCheckout(ctx, mode, studentID, now, today, endedVisit)
	if err != nil {
		return nil, err
	}

	if len(closedRows) == 0 {
		// No open row — student is already checked out (or never checked in
		// today). An orphaned visit may still have moved, so broadcast that
		// checkout; only a true no-op stays silent.
		if endedVisit != nil {
			s.registerCheckoutBroadcast(ctx, studentID, endedVisit, snapshot, checkoutType)
		}
		return &AttendanceResult{
			Action:    "checked_out",
			StudentID: studentID,
			Timestamp: now,
			Changed:   false, // no open row — attendance was already in the target state
		}, nil
	}

	s.registerCheckoutBroadcast(ctx, studentID, endedVisit, snapshot, checkoutType)

	s.trackProductEvent(ctx, "student_checked_out", map[string]any{
		"method":        s.attendanceMethod(ctx),
		"checkout_type": checkoutType,
	})

	return &AttendanceResult{
		Action:       "checked_out",
		AttendanceID: closedRows[0].ID,
		StudentID:    studentID,
		Timestamp:    now,
		Changed:      true,
	}, nil
}

// mirrorAttendanceCheckout mirrors a checkout into slot attendance. It returns
// the slot snapshot of the ended visit in detailed mode.
func (s *service) mirrorAttendanceCheckout(ctx context.Context, mode string, studentID int64, now time.Time, today timezone.Date, endedVisit *studentpresence.Visit) (*AttendanceSnapshot, error) {
	if s.AttendanceSyncer == nil {
		return nil, nil
	}
	if mode == PresenceModeBinary {
		// Binary mode has no visit provenance, so close the latest mirrored
		// open slot. Run this even for idempotent attendance checkout to heal
		// slot rows left open by older code — but only while the checkout
		// still targets now's own day: the mirror derives its slot day from
		// the timestamp, so a batch item closing its snapshot day after a
		// Berlin-midnight rollover would otherwise close a slot of the NEW
		// day whose attendance this checkout never touched (review #2372,
		// same day-scoping as the visit cleanup in performCheckOut).
		if timezone.DateFromTime(now) == today {
			if err := s.AttendanceSyncer.MirrorCheckOutAt(ctx, studentID, now); err != nil {
				return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("sync roomless checkout: %w", err)}
			}
		}
		return nil, nil
	}
	if endedVisit == nil {
		return nil, nil
	}
	// Detailed mode has exact source provenance through the ended visit.
	snapshot, err := s.AttendanceSyncer.MirrorCheckOutForVisit(ctx, presenceVisitSnapshot(endedVisit))
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("sync visit checkout: %w", err)}
	}
	return snapshot, nil
}
