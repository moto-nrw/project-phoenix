package active

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
)

// School check-in action values shared by the single and batch web endpoints.
const (
	SchoolCheckinActionIn  = "in"
	SchoolCheckinActionOut = "out"
)

// SchoolCheckinBatchItem is the per-student outcome of a batch call. OK=false
// means the id was not actionable (unknown, another tenant's student —
// invisible through the tenant filter —, or graduated); those students are
// skipped while the rest of the batch proceeds. Status carries the final
// attendance status for OK items so callers need no follow-up fetch.
type SchoolCheckinBatchItem struct {
	StudentID int64
	OK        bool
	Changed   bool
	Status    string
}

// SchoolCheckinBatchResult aggregates a batch call. Results preserve the
// caller's id order (first occurrence wins for duplicates), independent of the
// canonical write order used internally.
type SchoolCheckinBatchResult struct {
	Results   []SchoolCheckinBatchItem
	Succeeded int
	Failed    int
}

// IsSchoolCheckinNoop reports whether the requested action is already
// satisfied by the student's current attendance status ("on_yard" counts as
// present). Pure decision logic, exported for the single-student handler's
// short-circuit. The batch below deliberately does NOT use it: skipping a
// write based on a pre-read status races a concurrent transition — the
// state-checked writes themselves are the only trustworthy no-op detectors.
func IsSchoolCheckinNoop(action, currentStatus string) bool {
	alreadyPresent := currentStatus == "checked_in" || currentStatus == "on_yard"
	alreadyAbsent := currentStatus == "not_checked_in" || currentStatus == "checked_out"
	switch action {
	case SchoolCheckinActionIn:
		return alreadyPresent
	case SchoolCheckinActionOut:
		return alreadyAbsent
	default:
		return false
	}
}

// ProcessSchoolCheckinBatch applies one explicit check-in/out action to a set
// of students (#2359, Sammelauswahl in der Kindersuche).
//
// Reads are batched — one students query for id resolution, one post-write
// status refresh, one staff-presence stamp — so a full selection issues a
// handful of queries instead of an N+1 per-student cascade. Only the
// state-transition writes stay per student: performCheckIn/performCheckOut
// carry the race-safe transition logic (and visit ending on checkout) that
// has no batch form.
//
// Writes run in ascending student-id order regardless of the caller's order:
// two overlapping batches over the same students would otherwise take their
// row locks in opposite orders and deadlock in PostgreSQL. The result list is
// assembled separately in the caller's order.
//
// Every resolvable student gets the write — there is no status pre-read and
// no skip for apparent no-ops (review #2372). A skip based on a pre-read
// status races a concurrent transition: an "in" batch that read "checked_in"
// and skipped would report success while a parallel checkout lands and the
// student stays out. The writes are individually race-safe AND idempotent
// (ON CONFLICT insert for in, state-checked UPDATE for out), so executing
// them unconditionally is correct, and their Changed result is the ground
// truth for whether THIS call moved anything (false on an absorbed no-op).
//
// Failure semantics: the caller runs the batch inside one tenant transaction,
// so an unexpected write error must fail the entire call — earlier writes
// would never commit, and reporting them per-student as OK would lie. Only
// conditions detected without poisoning the transaction (unknown/foreign id,
// graduation — including the locked per-write revalidation losing a race,
// for check-ins and checkouts alike) become OK=false items.
func (s *service) ProcessSchoolCheckinBatch(
	ctx context.Context,
	studentIDs []int64,
	staffID int64,
	action string,
) (*SchoolCheckinBatchResult, error) {
	if action != SchoolCheckinActionIn && action != SchoolCheckinActionOut {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: fmt.Errorf("unknown action %q", action)}
	}

	students, err := s.UsersService.GetStudentsByIDs(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}

	// ONE attendance date for the whole batch, snapshotted before the write
	// loop (review #2372). Per-item TodayDate() calls would let a batch
	// crossing Berlin midnight write its early items for yesterday and its
	// late items for today — and the status refresh below, scoped to the same
	// snapshot date, would then misreport the early items. The write
	// TIMESTAMPS are deliberately NOT part of the snapshot: each transition
	// takes a fresh time.Now() in applySchoolCheckinWrite, because a checkout
	// stamped with the batch-start instant could close an attendance row a
	// concurrent check-in committed AFTER that instant — check_out_time before
	// check_in_time violates chk_checkin_before_checkout and would roll back
	// the entire batch (review #2372).
	today := timezone.TodayDate()

	// Canonical write order: ascending, deduplicated (see doc comment).
	writeOrder := append([]int64(nil), studentIDs...)
	slices.Sort(writeOrder)
	writeOrder = slices.Compact(writeOrder)

	outcomes := make(map[int64]SchoolCheckinBatchItem, len(writeOrder))
	written := make([]int64, 0, len(writeOrder))
	stampPresence := false
	for _, studentID := range writeOrder {
		student := students[studentID]
		// IsAlumnus is nil-safe by design: a missing map entry (unknown id or
		// another tenant's student) and a graduated one are both skippable.
		if student == nil || student.IsAlumnus() {
			outcomes[studentID] = SchoolCheckinBatchItem{StudentID: studentID}
			continue
		}
		result, writeErr := s.applySchoolCheckinWrite(ctx, studentID, staffID, action, today)
		if writeErr != nil {
			// Graduation (or deletion) race after the batch pre-check above:
			// the locked per-write revalidation — performCheckIn's own guard
			// for "in", the batch's pre-checkout guard for "out" — rejects
			// before writing anything, so the transaction stays clean and the
			// student can be skipped like any unknown id.
			if errors.Is(writeErr, ErrStudentGraduated) || errors.Is(writeErr, ErrStudentNotFound) {
				outcomes[studentID] = SchoolCheckinBatchItem{StudentID: studentID}
				continue
			}
			return nil, writeErr
		}
		outcomes[studentID] = SchoolCheckinBatchItem{StudentID: studentID, OK: true, Changed: result.Changed}
		written = append(written, studentID)
		// Same predicate as ensureStaffPresenceForAttendanceResult on the
		// single-student path: an idempotent no-op checkout is not an
		// on-duty action and must not open a work session by itself.
		if result.Action != "checked_out" || result.AttendanceID != 0 {
			stampPresence = true
		}
	}

	// One staff presence auto-stamp for the whole batch (#1439). Every write
	// above acts for the same staff member, so stamping per student would
	// only repeat the identical EnsureCheckedIn work-session lookup once per
	// child, up to the full batch cap. Best-effort exactly like the
	// single-student path: a failure is logged and never fails the batch.
	if stampPresence {
		s.ensureStaffPresence(ctx, staffID, attendanceStampSource(ctx))
	}

	// Back-fill the final status for every written student (changed or
	// race-absorbed alike) from ONE post-write attendance query. It runs
	// inside the same tenant transaction as the writes and reads the SNAPSHOT
	// date, not a re-derived "today" — after a midnight crossing a fresh
	// TodayDate() would look at the next day's (empty) rows and misreport
	// every just-written student as not_checked_in (review #2372).
	if len(written) > 0 {
		rows, refreshErr := s.AttendanceRepo.FindForDateByStudentIDs(ctx, today, written)
		if refreshErr != nil {
			return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: refreshErr}
		}
		// Rows are ordered check_in_time ASC per student — keep the latest.
		latest := make(map[int64]*active.Attendance, len(written))
		for _, row := range rows {
			latest[row.StudentID] = row
		}
		for _, studentID := range written {
			item := outcomes[studentID]
			if row := latest[studentID]; row != nil {
				item.Status = deriveAttendanceStatus(row)
			} else {
				// No row on the snapshot date: an idempotent checkout of a
				// student who never checked in today. Mirrors the missing-row
				// semantics of GetStudentsAttendanceStatuses.
				item.Status = "not_checked_in"
			}
			outcomes[studentID] = item
		}
	}

	// Assemble in the caller's order, first occurrence winning for duplicates.
	result := &SchoolCheckinBatchResult{Results: make([]SchoolCheckinBatchItem, 0, len(outcomes))}
	seen := make(map[int64]struct{}, len(outcomes))
	for _, studentID := range studentIDs {
		if _, ok := seen[studentID]; ok {
			continue
		}
		seen[studentID] = struct{}{}
		item := outcomes[studentID]
		result.Results = append(result.Results, item)
		if item.OK {
			result.Succeeded++
		} else {
			result.Failed++
		}
	}
	return result, nil
}

// applySchoolCheckinWrite performs the single state-transition write for one
// student of a batch — the same performCheckIn/performCheckOut cores the
// single-student CheckInStudent/CheckOutStudent wrappers use, minus their
// per-call staff presence stamp (the batch stamps once for the whole run)
// and minus authorization, which the batch contract requires the caller to
// have done already (mirror of skipAuthCheck on the single-student path).
// today is the batch-wide snapshot date; the timestamp is a fresh
// time.Now() per transition — see the snapshot comment in
// ProcessSchoolCheckinBatch for why the two differ.
// Action-explicit for the race reason documented on ToggleStudentAttendance:
// the cores are individually race-safe (ON CONFLICT for in, state-checked
// UPDATE for out), while a toggle could flip a concurrent "in" into an
// "out". A checkout also ends any open room visit in the same request
// transaction (#895) — scoped to the snapshot day: a batch crossing Berlin
// midnight closes yesterday's attendance without ending a room visit (or
// binary-mode slot mirror) the student started on the new day (review
// #2372, see endOpenVisitForStudent / performCheckOut).
func (s *service) applySchoolCheckinWrite(ctx context.Context, studentID, staffID int64, action string, today timezone.Date) (*AttendanceResult, error) {
	if action == SchoolCheckinActionIn {
		return s.performCheckIn(ctx, studentID, staffID, 0, time.Now(), today, checkinTypeWeb)
	}
	// Batch checkouts revalidate the alumnus status under the student-row
	// lock, exactly as performCheckIn does for check-ins: the batch's bulk
	// id-resolution read runs unlocked, so a graduation committing after it
	// would otherwise slip through and report OK for a student the batch
	// contract requires as not_found. The lock also serializes against the
	// concurrent grade-transition apply itself (#405). The checkout timestamp
	// is taken after the lock, so it postdates any check-in this write can
	// observe.
	if err := s.ensureStudentCheckinAllowed(ctx, studentID); err != nil {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}
	return s.performCheckOut(ctx, studentID, staffID, time.Now(), today, checkoutTypeWeb)
}
