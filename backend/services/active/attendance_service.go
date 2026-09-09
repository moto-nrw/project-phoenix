package active

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/realtime"
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
	staff, err := s.StaffRepo.FindByID(ctx, staffID)
	if err != nil || staff == nil {
		return ""
	}

	person, err := s.UsersService.Get(ctx, staff.PersonID)
	if err != nil || person == nil {
		return ""
	}

	return fmt.Sprintf("%s %s", person.FirstName, person.LastName)
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
func attendanceStampSource(ctx context.Context) string {
	if device.IsIoTDeviceRequest(ctx) {
		return active.WorkSessionSourceNFC
	}
	return active.WorkSessionSourceApp
}

func (s *service) ensureStaffPresenceForAttendanceResult(
	ctx context.Context,
	staffID int64,
	result *AttendanceResult,
) {
	if result == nil || (result.Action == "checked_out" && result.AttendanceID == 0) {
		return
	}
	s.ensureStaffPresence(ctx, staffID, attendanceStampSource(ctx))
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
	if authenticatedStaff := device.StaffFromCtx(ctx); authenticatedStaff != nil {
		staffID = authenticatedStaff.ID
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

	isIoTDevice := device.IsIoTDeviceRequest(ctx)

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

	resolvedDeviceID, err := s.resolveDeviceIDForAttendance(ctx, deviceID)
	if err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: errors.Join(ErrDatabaseOperation, err)}
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
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: err}
	}
	if !inserted {
		// Another concurrent "in" already created the open attendance row.
		// Treat as success — the desired end state (open attendance) holds.
		// Deliberately silent: the winner of the race already broadcast, and
		// nothing moved here (mirror of the closed == nil path in
		// performCheckOut). The re-fetch is scoped to the caller's snapshot
		// date, not a re-derived "today" (review #2372): a batch crossing
		// Berlin midnight after its insert conflict would otherwise query the
		// next day, find no row, and abort an otherwise idempotent batch.
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
		if err := s.autoClearStudentSickness(ctx, studentID); err != nil {
			return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: errors.Join(ErrDatabaseOperation, err)}
		}
		if err := s.autoClearStudentExcused(ctx, studentID); err != nil {
			return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: errors.Join(ErrDatabaseOperation, err)}
		}
		if err := s.autoClearPlannedStudentStatuses(ctx, studentID); err != nil {
			return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: errors.Join(ErrDatabaseOperation, err)}
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

	if err := s.autoClearStudentSickness(ctx, studentID); err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if err := s.autoClearStudentExcused(ctx, studentID); err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if err := s.autoClearPlannedStudentStatuses(ctx, studentID); err != nil {
		return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	if mode == PresenceModeBinary && s.AttendanceSyncer != nil {
		if _, err := s.AttendanceSyncer.MirrorCheckInAt(ctx, studentID, now); err != nil {
			return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("sync roomless check-in: %w", err)}
		}
	}

	s.registerCheckinBroadcast(ctx, studentID, checkinType)

	s.trackProductEvent(ctx, "student_checked_in", map[string]any{
		"method": attendanceMethod(ctx),
	})

	return &AttendanceResult{
		Action:       "checked_in",
		AttendanceID: attendance.ID,
		StudentID:    studentID,
		Timestamp:    now,
		Changed:      true,
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

	var snapshot *AttendanceSnapshot
	if s.AttendanceSyncer != nil {
		if mode == PresenceModeBinary {
			// Binary mode has no visit provenance, so close the latest mirrored
			// open slot. Run this even for idempotent attendance checkout to heal
			// slot rows left open by older code — but only while the checkout
			// still targets now's own day: the mirror derives its slot day from
			// the timestamp, so a batch item closing its snapshot day after a
			// Berlin-midnight rollover would otherwise close a slot of the NEW
			// day whose attendance this checkout never touched (review #2372,
			// same day-scoping as the visit cleanup above).
			if timezone.DateFromTime(now) == today {
				if err := s.AttendanceSyncer.MirrorCheckOutAt(ctx, studentID, now); err != nil {
					return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("sync roomless checkout: %w", err)}
				}
			}
		} else if endedVisit != nil {
			// Detailed mode has exact source provenance through the ended visit.
			snapshot, err = s.AttendanceSyncer.MirrorCheckOutForVisit(ctx, presenceVisitSnapshot(endedVisit))
			if err != nil {
				return nil, &ActiveError{Op: "ToggleStudentAttendance", Err: fmt.Errorf("sync visit checkout: %w", err)}
			}
		}
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
		"method":        attendanceMethod(ctx),
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

// registerCheckoutBroadcast queues the SSE fan-out of an attendance checkout
// that closed a row or healed an orphaned visit. It covers every entry point
// into performCheckOut: the staff web checkout
// (POST /active/visits/student/{id}/checkout), the school check-in/out endpoint
// used by the binary-mode "Kinder an- und abmelden" flow, the kiosk toggle, and
// the kiosk daily checkout. None of them emitted anything between 976b32f and
// #2113 — ending the visit moved from service.EndVisit (which broadcasts) to
// the repository call above (which does not), so other staff tabs kept showing
// the child as present indefinitely: the room, search and detail views
// revalidate on neither focus nor interval.
//
// Two invariants:
//
//  1. The student display data is read HERE, inside the request transaction.
//     Only the emission is deferred to tenant.RegisterAfterCommit — by the time
//     hooks run, the tx carried in ctx is closed, so a repository read from
//     inside the hook would fail, and reading outside the tenant tx would be
//     blocked by RLS. Deferring matters because TenantTxMiddleware commits at
//     the end of the request: a client woken before the commit refetches the
//     pre-checkout state, and nothing corrects it afterwards.
//  2. Callers pass endedVisit == nil when no room visit was closed, which
//     selects the roomless event shape. Broadcaster identity rules follow the
//     existing helpers: the child id rides only the group-scoped topics, never
//     the tenant-wide invalidation events (#2085).
func (s *service) registerCheckoutBroadcast(
	ctx context.Context,
	studentID int64,
	endedVisit *studentpresence.Visit,
	snapshot *AttendanceSnapshot,
	checkoutType string,
) {
	// Siehe registerCheckinBroadcast: die Eltern-Weckung haengt an der
	// Anwesenheit, nicht am Personal-Broadcaster.
	s.wakeGuardiansAfterCommit(ctx, studentID)

	if s.Broadcaster == nil {
		return
	}

	studentRec := s.getStudentForSSE(ctx, studentID)

	tenant.RegisterAfterCommit(ctx, func() {
		if endedVisit != nil {
			// Ordinary visit checkouts historically had no source. Only carry the
			// daily checkout's stable wire value onto the visit-shaped heal path.
			source := ""
			if checkoutType == checkoutTypeDaily {
				source = dailyCheckoutSource
			}
			s.emitVisitCheckout(ctx, endedVisit, snapshot, studentRec, source)
			return
		}
		s.emitRoomlessCheckout(ctx, studentID, studentRec, checkoutSourceLabel(checkoutType))
	})
}

// registerCheckinBroadcast queues the SSE fan-out of an attendance check-in
// that actually opened a row. Mirror of registerCheckoutBroadcast, same two
// invariants — the display data is read HERE, inside the request transaction,
// and only the emission is deferred past the commit.
//
// Covers the check-in paths that write attendance without a room visit: the
// school check-in/out endpoint used by the "Kinder an- und abmelden" flow, the
// binary-mode kiosk scan, and the door kiosk's attendance toggle in either mode.
// All three were silent, so a colleague's open room, search or detail view kept
// showing the child as absent — none of them revalidates on focus or interval.
//
// Deliberately not gated on presence mode: the detailed-mode ROOM check-in
// takes the CreateVisit path, which writes its own attendance row and emits
// broadcastVisitCreated. The two call sets are disjoint, so no request can emit
// both.
func (s *service) registerCheckinBroadcast(ctx context.Context, studentID int64, checkinType string) {
	// Die Sorgeberechtigten werden unabhaengig vom Broadcaster geweckt: ihr
	// Tagesstatus haengt an der Anwesenheit, nicht an den Personal-Topics.
	s.wakeGuardiansAfterCommit(ctx, studentID)

	if s.Broadcaster == nil {
		return
	}

	studentRec := s.getStudentForSSE(ctx, studentID)

	tenant.RegisterAfterCommit(ctx, func() {
		s.emitRoomlessCheckin(ctx, studentID, studentRec, checkinType)
	})
}

type deviceSupervisorUnavailableError struct {
	reason string
}

func (e *deviceSupervisorUnavailableError) Error() string {
	return e.reason
}

// getDeviceSupervisorID retrieves the supervisor staff ID for a device's active group.
// Expected absence states use a typed error so daily checkout may safely fall
// back to device attribution without swallowing repository failures.
func (s *service) getDeviceSupervisorID(ctx context.Context, deviceID int64) (int64, error) {
	// Find active group for device
	activeGroup, err := s.GroupRepo.FindActiveByDeviceID(ctx, deviceID)
	if err != nil {
		// Handle case where no active group exists for this device
		if errors.Is(err, ErrNoActiveSession) {
			return 0, &deviceSupervisorUnavailableError{reason: fmt.Sprintf("no active group assigned to device %d", deviceID)}
		}
		return 0, fmt.Errorf("error finding active group for device %d: %w", deviceID, err)
	}

	if activeGroup == nil {
		return 0, &deviceSupervisorUnavailableError{reason: fmt.Sprintf("no active group assigned to device %d", deviceID)}
	}

	// Get supervisors for the active group
	supervisors, err := s.FindSupervisorsByActiveGroupID(ctx, activeGroup.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to get supervisors for group %d: %w", activeGroup.ID, err)
	}

	if len(supervisors) == 0 {
		return 0, &deviceSupervisorUnavailableError{reason: fmt.Sprintf("no supervisors assigned to active group %d", activeGroup.ID)}
	}

	// Use first active supervisor
	now := time.Now()
	for _, supervisor := range supervisors {
		if IsSupervisorActive(supervisor, now) {
			return supervisor.StaffID, nil
		}
	}

	return 0, &deviceSupervisorUnavailableError{reason: fmt.Sprintf("no active supervisors found in group %d", activeGroup.ID)}
}

// CheckTeacherStudentAccess checks if a teacher has access to mark attendance for a student
func (s *service) CheckTeacherStudentAccess(ctx context.Context, teacherID, studentID int64) (bool, error) {
	// Get teacher from staff ID
	teacher, err := s.TeacherRepo.FindByStaffID(ctx, teacherID)
	if err != nil {
		return false, &ActiveError{Op: "CheckTeacherStudentAccess", Err: err}
	}
	if teacher == nil {
		return false, nil
	}

	// Get teacher's groups via educationService
	teacherGroups, err := s.EducationService.GetTeacherGroups(ctx, teacher.ID)
	if err != nil {
		return false, &ActiveError{Op: "CheckTeacherStudentAccess", Err: err}
	}

	// Get student info
	student, err := s.StudentRepo.FindByID(ctx, studentID)
	if err != nil {
		if base.IsNoRows(err) {
			return false, nil
		}
		return false, &ActiveError{Op: "CheckTeacherStudentAccess", Err: err}
	}
	if student == nil || student.GroupID == nil {
		return false, nil
	}

	// Check if student.GroupID is in teacher's groups
	for _, group := range teacherGroups {
		if group.ID == *student.GroupID {
			return true, nil
		}
	}

	return false, nil
}

// emitRoomlessCheckout publishes a checkout that has no room context. Used when
// no visit ended — a binary-mode tenant (which keeps no visit rows at all) or a
// kiosk daily checkout, where the visit was already closed when the child left
// the room.
func (s *service) emitRoomlessCheckout(
	ctx context.Context,
	studentID int64,
	studentRec *userModels.Student,
	source string,
) {
	s.emitRoomlessAttendanceChange(ctx, realtime.EventStudentCheckOut, studentID, studentRec, source)
}

// emitRoomlessCheckin publishes a check-in that has no room context: attendance
// opened without the child entering a room. That is every binary-mode check-in
// (the tenant keeps no visit rows) and the door kiosk's attendance toggle in
// either mode. The detailed-mode room check-in does NOT come through here — it
// runs through CreateVisit, which writes its own attendance row and emits
// broadcastVisitCreated, so the two can never fire for the same request.
func (s *service) emitRoomlessCheckin(
	ctx context.Context,
	studentID int64,
	studentRec *userModels.Student,
	source string,
) {
	s.emitRoomlessAttendanceChange(ctx, realtime.EventStudentCheckIn, studentID, studentRec, source)
}

// emitRoomlessAttendanceChange publishes an attendance change with no room
// context: the student's educational (OGS) group topic plus the tenant-wide
// dashboard refresh. There is no active group to scope to, hence no active-group
// topic and no active_supervision_changed — no room roster changed.
//
// Takes the display data pre-resolved for the same reason as
// emitVisitCheckout — see its doc comment.
func (s *service) emitRoomlessAttendanceChange(
	ctx context.Context,
	eventType realtime.EventType,
	studentID int64,
	studentRec *userModels.Student,
	source string,
) {
	if s.Broadcaster == nil {
		return
	}

	studentIDStr := fmt.Sprintf("%d", studentID)
	eduGroupIDs := eduGroupIDsOf(studentRec)

	data := realtime.EventData{
		StudentID: &studentIDStr,
		Source:    &source,
	}
	if len(eduGroupIDs) > 0 {
		data.GroupIDs = &eduGroupIDs
	}
	event := realtime.NewEvent(
		eventType,
		"", // no active group — the child is not in a room
		data,
	)

	// Broadcast to educational (OGS) group topic so the "Meine Gruppe" page updates
	s.broadcastToEducationalGroup(ctx, studentRec, event)

	// Notify every client of the tenant so dashboard counts and the search
	// page refresh — the educational group broadcast only reaches staff in
	// that group, but the search page is used by all staff. Scoped to the
	// student's educational group when known (#2057).
	s.broadcastDashboardCountsChanged(ctx, eduGroupIDs)
}

// ConfirmDailyCheckout processes the deferred daily-checkout confirmation for an
// IoT device. Normally the student's visit was already ended by the checkin
// handler (student is "unterwegs") and this only updates the attendance record
// when the student confirms "nach Hause". If a visit is still open,
// CheckOutStudentFromDevice ends it in the same request transaction (issue #895).
//
// The student must already have an attendance record for today (status
// "checked_in" or "checked_out"); otherwise ErrNoAttendanceRecordForCheckout is
// returned. Attendance is only mutated when destination is "zuhause" and the
// student is still "checked_in"; a concurrent checkout is treated as an
// idempotent no-op.
func (s *service) ConfirmDailyCheckout(ctx context.Context, studentID, deviceID int64, destination string) (*DailyCheckoutResult, error) {
	var result *DailyCheckoutResult
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.confirmDailyCheckout(txCtx, studentID, deviceID, destination)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) confirmDailyCheckout(ctx context.Context, studentID, deviceID int64, destination string) (*DailyCheckoutResult, error) {
	s.getLogger().InfoContext(ctx, "confirming daily checkout",
		slog.Int64("student_id", studentID),
		slog.String("destination", destination),
	)

	currentStatus, err := s.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		s.getLogger().ErrorContext(ctx, "failed to get attendance status",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if currentStatus.Status != "checked_in" && currentStatus.Status != "checked_out" {
		s.getLogger().ErrorContext(ctx, "student has no attendance record for today",
			slog.Int64("student_id", studentID),
			slog.String("status", currentStatus.Status),
		)
		return nil, ErrNoAttendanceRecordForCheckout
	}

	if destination == "zuhause" {
		switch currentStatus.Status {
		case "checked_out":
			s.getLogger().DebugContext(ctx, "student already checked out, skipping attendance toggle",
				slog.Int64("student_id", studentID),
			)
		case "checked_in":
			// CheckOutStudentFromDevice broadcasts the student_checkout /
			// dashboard_counts_changed pair itself, after the request
			// transaction commits (#2113) — the explicit BroadcastDailyCheckout
			// that used to sit here fired before the commit and is now a
			// duplicate.
			if _, err := s.CheckOutStudentFromDevice(ctx, studentID, deviceID); err != nil {
				s.getLogger().ErrorContext(ctx, "failed to update attendance for daily checkout",
					slog.Int64("student_id", studentID),
					slog.String("error", err.Error()),
				)
				return nil, err
			}
		}
	}

	action := "checked_out_daily"
	if destination == "unterwegs" {
		action = "checked_out"
	}

	s.getLogger().InfoContext(ctx, "daily checkout confirmed",
		slog.Int64("student_id", studentID),
		slog.String("action", action),
		slog.String("destination", destination),
	)

	return &DailyCheckoutResult{Action: action}, nil
}

// ======== Unclaimed Groups Management (Deviceless Claiming) ========

// GetUnclaimedActiveGroups returns all active groups that have no supervisors
// This is used for deviceless rooms like Schulhof where teachers claim supervision via frontend
func (s *service) GetUnclaimedActiveGroups(ctx context.Context) ([]*active.Group, error) {
	rows, err := s.SchoolPresence.UnclaimedGroups(ctx, s.todayDate().String())
	if err != nil {
		return nil, &ActiveError{Op: "GetUnclaimedActiveGroups", Err: err}
	}
	groups := make([]*active.Group, 0, len(rows))
	if len(rows) == 0 {
		return groups, nil
	}
	roomIDs, templateIDs := make([]int64, 0, len(rows)), make([]int64, 0, len(rows))
	for _, row := range rows {
		roomIDs = append(roomIDs, row.RoomID)
		if row.GroupID != nil {
			templateIDs = append(templateIDs, *row.GroupID)
		}
	}
	rooms, err := s.RoomRepo.FindByIDs(ctx, roomIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetUnclaimedActiveGroups", Err: err}
	}
	templates, err := s.ActivityGroupRepo.FindByIDs(ctx, templateIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetUnclaimedActiveGroups", Err: err}
	}
	roomsByID := make(map[int64]*facilities.Room, len(rooms))
	for _, room := range rooms {
		roomsByID[room.ID] = room
	}
	templatesByID := make(map[int64]*activities.Group, len(templates))
	for _, template := range templates {
		// This endpoint historically includes the template without its category relation.
		copy := *template
		copy.Category = nil
		templatesByID[copy.ID] = &copy
	}
	for _, row := range rows {
		group := &active.Group{Model: base.Model{ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, StartTime: row.StartTime, EndTime: row.EndTime, LastActivity: row.LastActivity, TimeoutMinutes: row.TimeoutMinutes, GroupID: row.GroupID, DeviceID: row.DeviceID, RoomID: row.RoomID}
		group.SetTenantID(row.TenantID)
		group.Room = roomsByID[row.RoomID]
		if group.Room == nil || group.Room.Name != "Schulhof" {
			continue
		}
		if row.GroupID != nil {
			group.ActualGroup = templatesByID[*row.GroupID]
		}
		groups = append(groups, group)
	}
	return groups, nil
}

// ClaimActiveGroup allows a staff member to claim supervision of an active group
// This is primarily used for deviceless rooms like Schulhof
func (s *service) ClaimActiveGroup(ctx context.Context, groupID, staffID int64, role string) (*active.GroupSupervisor, error) {
	if role == "" {
		role = "supervisor"
	}
	var result *active.GroupSupervisor
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		if err := s.validateStaffExists(txCtx, staffID); err != nil {
			return err
		}
		row, err := s.SchoolPresence.ClaimGroup(txCtx, studentpresence.GroupClaim{GroupID: groupID, StaffID: staffID, Role: role, Date: s.todayDate().String()})
		switch {
		case errors.Is(err, studentpresence.ErrAlreadySupervising):
			return ErrStaffAlreadySupervising
		case errors.Is(err, studentpresence.ErrGroupNotFound), errors.Is(err, studentpresence.ErrGroupEnded):
			return err
		case err != nil:
			return ErrDatabaseOperation
		}
		date, err := timezone.ParseDate(row.StartDate)
		if err != nil {
			return err
		}
		result = &active.GroupSupervisor{Model: base.Model{ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, GroupID: row.GroupID, StaffID: row.StaffID, Role: row.Role, StartDate: date}
		result.SetTenantID(row.TenantID)
		source := active.WorkSessionSourceApp
		if device.IsIoTDeviceRequest(txCtx) {
			source = active.WorkSessionSourceNFC
		}
		s.ensureStaffPresence(txCtx, staffID, source)
		return nil
	})
	if err != nil {
		return nil, &ActiveError{Op: "ClaimActiveGroup", Err: err}
	}
	return result, nil
}
