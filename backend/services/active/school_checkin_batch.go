package active

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
// present). Pure decision logic, exported so the single-student handler can
// share the exact same no-op rule.
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
// Reads are batched — one students query, one attendance-status query, one
// post-write refresh — so a full selection issues a handful of queries instead
// of an N+1 per-student cascade. Only the state-transition writes stay per
// student: CheckInStudent/CheckOutStudent carry the race-safe transition logic
// (and visit ending on checkout) that has no batch form.
//
// Writes run in ascending student-id order regardless of the caller's order:
// two overlapping batches over the same students would otherwise take their
// row locks in opposite orders and deadlock in PostgreSQL. The result list is
// assembled separately in the caller's order.
//
// Per-student Changed comes from the write itself, not from the pre-read
// status: a concurrent request may have established the target state between
// the batch's status read and the write, and that absorbed write is a no-op
// (Changed=false), not a change.
//
// Failure semantics: the caller runs the batch inside one tenant transaction,
// so an unexpected write error must fail the entire call — earlier writes
// would never commit, and reporting them per-student as OK would lie. Only
// conditions detected without poisoning the transaction (unknown/foreign id,
// graduation — including CheckInStudent's own pre-write graduation guard
// losing a race) become OK=false items.
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
	// One query for all current statuses. Ids missing from the students map
	// get a synthetic "not_checked_in" entry here; those students are skipped
	// as OK=false below and their entry is never read.
	statuses, err := s.GetStudentsAttendanceStatuses(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	// Canonical write order: ascending, deduplicated (see doc comment).
	writeOrder := append([]int64(nil), studentIDs...)
	slices.Sort(writeOrder)
	writeOrder = slices.Compact(writeOrder)

	outcomes := make(map[int64]SchoolCheckinBatchItem, len(writeOrder))
	written := make([]int64, 0, len(writeOrder))
	for _, studentID := range writeOrder {
		student := students[studentID]
		// IsAlumnus is nil-safe by design: a missing map entry (unknown id or
		// another tenant's student) and a graduated one are both skippable.
		if student == nil || student.IsAlumnus() {
			outcomes[studentID] = SchoolCheckinBatchItem{StudentID: studentID}
			continue
		}
		if current := statuses[studentID]; IsSchoolCheckinNoop(action, current.Status) {
			outcomes[studentID] = SchoolCheckinBatchItem{StudentID: studentID, OK: true, Status: current.Status}
			continue
		}
		result, writeErr := s.applySchoolCheckinWrite(ctx, studentID, staffID, action)
		if writeErr != nil {
			// Graduation race after the batch pre-check above: CheckInStudent's
			// own guard rejects before writing anything, so the transaction
			// stays clean and the student can be skipped like any unknown id.
			if errors.Is(writeErr, ErrStudentGraduated) {
				outcomes[studentID] = SchoolCheckinBatchItem{StudentID: studentID}
				continue
			}
			return nil, writeErr
		}
		outcomes[studentID] = SchoolCheckinBatchItem{StudentID: studentID, OK: true, Changed: result.Changed}
		written = append(written, studentID)
	}

	// Back-fill the final status for every written student (changed or
	// race-absorbed alike) from ONE post-write attendance query. It runs
	// inside the same tenant transaction as the writes, so it observes the
	// batch's own freshly-written rows.
	if len(written) > 0 {
		updated, refreshErr := s.GetStudentsAttendanceStatuses(ctx, written)
		if refreshErr != nil {
			return nil, refreshErr
		}
		for _, studentID := range written {
			item := outcomes[studentID]
			item.Status = updated[studentID].Status
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
// student of a batch. Action-explicit for the race reason documented on
// ToggleStudentAttendance: CheckIn/CheckOut are individually race-safe
// (ON CONFLICT for in, state-checked UPDATE for out), while a toggle could
// flip a concurrent "in" into an "out". A checkout also ends any open room
// visit in the same request transaction (#895). skipAuthCheck is true because
// the batch contract requires the caller to be authorized already.
func (s *service) applySchoolCheckinWrite(ctx context.Context, studentID, staffID int64, action string) (*AttendanceResult, error) {
	if action == SchoolCheckinActionIn {
		return s.CheckInStudent(ctx, studentID, staffID, 0, true)
	}
	return s.CheckOutStudent(ctx, studentID, staffID, true)
}
