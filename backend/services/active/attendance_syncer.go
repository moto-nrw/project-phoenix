package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// AttendanceSnapshot is the minimal view of a schedule.instance_students row
// that the active service needs to enrich SSE events. The pointer shape
// matches realtime.EventData so broadcast helpers can copy fields directly.
//
// Declared in the active package (and not, say, services/schedule) so there
// is no cyclic-import risk — the schedule package already depends on active
// for its bridge semantics, but active must not import schedule.
type AttendanceSnapshot struct {
	Status      string
	Substatus   *string
	Note        *string
	InstanceID  int64
	IsUnplanned bool
}

// AttendanceSyncer is the optional dependency active.service calls on visit
// write / visit end to (a) mirror status changes into schedule.instance_students
// and (b) resolve the current attendance row so SSE events carry the three
// WP-B10 fields.
//
// Every method in this interface is graceful-degradation-by-design: the
// implementation MUST swallow every error internally and return nil on any
// failure (no matching instance, no enrolled student, DB error, panic).
// That way active.service can call these methods unconditionally and never
// block a visit write on attendance-sync failure. The return is "did we
// find an attendance row" — not "did an error occur".
//
// Implementations live in services/schedule. A nil AttendanceSyncer is
// valid at construction time (tests, early-boot servers without the full
// factory wire-up) — callers must handle nil.
type AttendanceSyncer interface {
	// MirrorCheckInForVisit is called AFTER visitRepo.Create succeeds. It
	// looks up the instance bridged to visit.ActiveGroupID, then the
	// instance_students row for visit.StudentID. If both exist, it opens
	// observed presence from expected/day-status absence or reopens a prior
	// checked-out presence with the new visit's entry time.
	//
	// Returns a snapshot of the resulting row (or the pre-existing row, in
	// the already-present case, so SSE still reflects state). Returns nil
	// for walk-ins, pre-start races, tenant mismatches, or any error.
	MirrorCheckInForVisit(ctx context.Context, visit *active.Visit) *AttendanceSnapshot

	// MirrorCheckInAt resolves a roomless check-in to exactly one currently
	// scheduled student slot. Zero or multiple matches remain unassigned.
	MirrorCheckInAt(ctx context.Context, studentID int64, at time.Time) *AttendanceSnapshot

	// MirrorCheckOutForVisit is called at visit end. It stamps the slot's
	// checkout time without changing its attendance status, then returns the
	// current snapshot for SSE display.
	MirrorCheckOutForVisit(ctx context.Context, visit *active.Visit) *AttendanceSnapshot

	// MirrorVisitRevision replaces a slot interval only when it still matches
	// previous. It is used when staff edit or reopen an existing visit.
	MirrorVisitRevision(ctx context.Context, previous, updated *active.Visit)

	// MirrorCheckOutAt closes the most recently checked-in open slot for a
	// roomless attendance flow.
	MirrorCheckOutAt(ctx context.Context, studentID int64, at time.Time)
}

// applyAttendanceSnapshot copies the three WP-B10 fields from snapshot into
// the EventData. Nil snapshot is a no-op — the realtime event goes out
// with attendance_* omitted (omitempty). Status is always set when snapshot
// is non-nil (it's a required column); Substatus and Note may be nil.
func applyAttendanceSnapshot(data *realtime.EventData, snapshot *AttendanceSnapshot) {
	if data == nil || snapshot == nil {
		return
	}
	status := snapshot.Status
	data.AttendanceStatus = &status
	data.AttendanceSubstatus = snapshot.Substatus
	data.AttendanceNote = snapshot.Note
}
