package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
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
// Missing timetable assignments are valid no-ops. Read and write failures
// return errors so the caller rolls back all presence and timetable writes.
//
// Implementations live in services/schedule. A nil AttendanceSyncer is
// valid at construction time (tests, early-boot servers without the full
// factory wire-up) — callers must handle nil.
type AttendanceSyncer interface {
	// MirrorCheckInForVisit is called after the authoritative visit write. It
	// looks up the instance bridged to visit.ActiveGroupID, then the
	// instance_students row for visit.StudentID. If both exist, it opens
	// observed presence from expected/day-status absence or reopens a prior
	// checked-out presence with the new visit's entry time.
	//
	// Returns a snapshot of the resulting row (or the pre-existing row, in
	// the already-present case, so SSE still reflects state). Returns nil
	// when there is no matching timetable instance. Failures return an error.
	MirrorCheckInForVisit(ctx context.Context, visit *studentpresence.Visit) (*AttendanceSnapshot, error)

	// MirrorCheckInAt resolves a roomless check-in to exactly one currently
	// scheduled student slot. Zero or multiple matches remain unassigned.
	MirrorCheckInAt(ctx context.Context, studentID int64, at time.Time) (*AttendanceSnapshot, error)

	// MirrorCheckOutForVisit is called at visit end. It stamps the slot's
	// checkout time without changing its attendance status, then returns the
	// current snapshot for SSE display.
	MirrorCheckOutForVisit(ctx context.Context, visit *studentpresence.Visit) (*AttendanceSnapshot, error)

	// MirrorVisitRevision replaces a slot interval only when it still matches
	// previous. Read and write failures must roll back the caller's visit edit.
	MirrorVisitRevision(ctx context.Context, previous, updated *studentpresence.Visit) error

	// MirrorCheckOutAt closes the most recently checked-in open slot for a
	// roomless attendance flow.
	MirrorCheckOutAt(ctx context.Context, studentID int64, at time.Time) error

	// MirrorCheckInAtBatch is MirrorCheckInAt for many students at one shared
	// instant: one candidate query and one guarded UPDATE for the whole batch
	// instead of two round trips per student (review #2372). No snapshots —
	// the batch caller emits bulk SSE events, which carry no attendance
	// enrichment by design (#848).
	MirrorCheckInAtBatch(ctx context.Context, studentIDs []int64, at time.Time) error

	// MirrorCheckOutAtBatch is MirrorCheckOutAt for many students at one
	// shared instant: one slot query and one guarded UPDATE for the batch.
	MirrorCheckOutAtBatch(ctx context.Context, studentIDs []int64, at time.Time) error

	// MirrorCheckOutForVisits is MirrorCheckOutForVisit for a set of visits
	// ended at one shared instant: instances are resolved once per distinct
	// active group and every slot checkout lands in one guarded UPDATE. No
	// snapshots, same reason as MirrorCheckInAtBatch.
	MirrorCheckOutForVisits(ctx context.Context, visits []*studentpresence.Visit, at time.Time) error
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
